import os
from contextlib import asynccontextmanager
from pathlib import Path
from secrets import compare_digest

from fastapi import Depends, FastAPI, Form, HTTPException, Request, Response
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates
from sqlalchemy import insert, text, update

from . import models
from .auth import (
    generate_csrf_token,
    owner_from_request,
    require_owner,
    require_worker,
    secure_cookies,
    validate_auth_config,
    verify_csrf_token,
)
from .db import (
    claim_next_job,
    connect,
    create_job_record,
    get_job,
    init_db,
    jobs,
    job_attempts,
    job_logs,
    lease_expires_iso,
    list_jobs as list_job_records,
    list_worker_repos,
    now_iso,
    touch_worker,
    upsert_worker_repo,
)
from .schemas import AttemptRequest, CancelledJob, CompleteJob, FailJob, JobCreate, LogCreate, WorkerRegister

BASE_DIR = Path(__file__).resolve().parent


@asynccontextmanager
async def lifespan(_: FastAPI):
    validate_auth_config()
    init_db()
    yield


app = FastAPI(
    title="DeadDrop",
    lifespan=lifespan,
    docs_url="/api/docs",
    redoc_url="/api/redoc",
    openapi_url="/api/openapi.json",
)
app.mount("/static", StaticFiles(directory=BASE_DIR / "static"), name="static")
templates = Jinja2Templates(directory=BASE_DIR / "templates")

DEFAULT_WORKER_NAME = "local"
DEFAULT_REPO_ALIAS = "default"


@app.get("/healthz")
def healthz():
    return {"ok": True}


@app.get("/readyz")
def readyz():
    with connect() as conn:
        conn.execute(text("SELECT 1")).fetchone()
    return {"ok": True}


def ensure_owner_page(request: Request) -> None:
    if not owner_from_request(request):
        raise HTTPException(status_code=303, headers={"Location": "/login"})


@app.get("/", response_class=HTMLResponse)
def dashboard(request: Request):
    if not owner_from_request(request):
        return templates.TemplateResponse(request, "landing.html", {"request": request})
    with connect() as conn:
        rows = list_job_records(conn)
    csrf_token = request.cookies.get("csrf_token")
    return templates.TemplateResponse(request, "dashboard.html", {"request": request, "jobs": rows, "csrf_token": csrf_token})


@app.get("/login", response_class=HTMLResponse)
def login_page(request: Request):
    return templates.TemplateResponse(request, "login.html", {"request": request, "error": ""})


@app.post("/login")
def login(request: Request, token: str = Form(...)):
    from .auth import owner_token

    if not compare_digest(token, owner_token()):
        return templates.TemplateResponse(request, "login.html", {"request": request, "error": "Invalid token"}, status_code=401)
    response = RedirectResponse("/", status_code=303)
    # 1. Set Owner Token
    response.set_cookie(
        "owner_token",
        token,
        max_age=60 * 60 * 24 * 30,
        httponly=True,
        secure=secure_cookies(),
        samesite="lax",
    )
    # 2. Set CSRF Token (Not HttpOnly so JS can read it if needed, but primarily for Double Submit)
    response.set_cookie(
        "csrf_token",
        generate_csrf_token(),
        max_age=60 * 60 * 24 * 30,
        httponly=False,
        secure=secure_cookies(),
        samesite="lax",
    )
    return response


@app.get("/logout")
def logout():
    response = RedirectResponse("/login", status_code=303)
    response.delete_cookie("owner_token", secure=secure_cookies(), samesite="lax")
    response.delete_cookie("csrf_token", secure=secure_cookies(), samesite="lax")
    return response


@app.get("/jobs/new", response_class=HTMLResponse)
def new_job_page(request: Request):
    ensure_owner_page(request)
    csrf_token = request.cookies.get("csrf_token")
    with connect() as conn:
        repos = list_worker_repos(conn)
    return templates.TemplateResponse(
        request,
        "new_job.html",
        {
            "request": request,
            "csrf_token": csrf_token,
            "repos": repos,
            "agents": models.AGENTS,
            "default_repo": DEFAULT_REPO_ALIAS,
        },
    )


@app.post("/jobs", dependencies=[Depends(verify_csrf_token)])
def create_job_form(
    request: Request,
    title: str = Form(...),
    prompt: str = Form(...),
    repo_alias: str = Form(DEFAULT_REPO_ALIAS),
    agent: str = Form(models.DEFAULT_AGENT),
):
    ensure_owner_page(request)
    try:
        job = JobCreate(title=title, prompt=prompt, repo_alias=repo_alias, agent=agent)
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    created = create_job(job)
    return RedirectResponse(f"/jobs/{created['id']}", status_code=303)


@app.get("/jobs/{job_id}", response_class=HTMLResponse)
def job_detail_page(request: Request, job_id: int):
    ensure_owner_page(request)
    before = request.query_params.get("before_log_id")
    before_log_id = int(before) if before and before.isdigit() else None
    with connect() as conn:
        job = get_job(conn, job_id, include_logs=True, before_log_id=before_log_id)
    if not job:
        raise HTTPException(status_code=404, detail="Job not found")
    csrf_token = request.cookies.get("csrf_token")
    return templates.TemplateResponse(request, "job_detail.html", {"request": request, "job": job, "csrf_token": csrf_token})


@app.post("/jobs/{job_id}/cancel", dependencies=[Depends(verify_csrf_token)])
def cancel_job_form(request: Request, job_id: int):
    ensure_owner_page(request)
    cancel_job(job_id)
    return RedirectResponse(f"/jobs/{job_id}", status_code=303)


@app.get("/jobs/{job_id}/fragment", response_class=HTMLResponse)
def job_fragment(request: Request, job_id: int):
    ensure_owner_page(request)
    before = request.query_params.get("before_log_id")
    before_log_id = int(before) if before and before.isdigit() else None
    with connect() as conn:
        job = get_job(conn, job_id, include_logs=True, before_log_id=before_log_id)
    if not job:
        raise HTTPException(status_code=404, detail="Job not found")
    csrf_token = request.cookies.get("csrf_token")
    return templates.TemplateResponse(request, "_job_receipt.html", {"request": request, "job": job, "csrf_token": csrf_token})


def patch_response(job_id: int) -> Response:
    with connect() as conn:
        job = get_job(conn, job_id)
    if not job:
        raise HTTPException(status_code=404, detail="Job not found")
    if not job["git_diff"]:
        raise HTTPException(status_code=409, detail="Job has no patch")
    return Response(
        content=job["git_diff"],
        media_type="text/x-diff",
        headers={
            "Content-Disposition": f'attachment; filename="deaddrop-job-{job_id}.patch"',
            "Cache-Control": "private, no-store",
            "X-Content-Type-Options": "nosniff",
        },
    )


@app.get("/jobs/{job_id}/patch")
def download_job_patch(request: Request, job_id: int):
    ensure_owner_page(request)
    return patch_response(job_id)


@app.get("/api/jobs/{job_id}/patch", dependencies=[Depends(require_owner)])
def api_job_patch(job_id: int):
    return patch_response(job_id)


@app.get("/demo", response_class=HTMLResponse)
def demo(request: Request):
    return templates.TemplateResponse(request, "demo.html", {"request": request})


@app.get("/docs", response_class=HTMLResponse)
def public_docs(request: Request):
    return templates.TemplateResponse(request, "docs.html", {"request": request})


@app.get("/docs/architecture", response_class=HTMLResponse)
def public_architecture(request: Request):
    return templates.TemplateResponse(request, "public_architecture.html", {"request": request})


@app.get("/updates", response_class=HTMLResponse)
def updates(request: Request):
    return templates.TemplateResponse(request, "updates.html", {"request": request})


@app.get("/blog", response_class=HTMLResponse)
def blog(request: Request):
    return templates.TemplateResponse(request, "blog.html", {"request": request})


@app.get("/blog/disposable-worktrees", response_class=HTMLResponse)
def disposable_worktrees_post(request: Request):
    return templates.TemplateResponse(request, "blog_disposable_worktrees.html", {"request": request})


@app.get("/blog/evidence-based-receipts", response_class=HTMLResponse)
def evidence_receipts_post(request: Request):
    return templates.TemplateResponse(request, "blog_evidence_receipts.html", {"request": request})


@app.get("/blog/leases-for-local-agents", response_class=HTMLResponse)
def leases_post(request: Request):
    return templates.TemplateResponse(request, "blog_leases.html", {"request": request})


def resolve_repo_alias(conn, requested: str) -> str:
    """Owners may only target repositories a worker has actually registered.

    The manifest is the trust boundary, so routing stays bounded by what the
    worker opted in to rather than by whatever an owner token asks for.
    """
    alias = (requested or DEFAULT_REPO_ALIAS).strip()
    if alias == DEFAULT_REPO_ALIAS:
        return alias
    known = {row["repo_alias"] for row in list_worker_repos(conn)}
    if alias not in known:
        raise HTTPException(
            status_code=422,
            detail=f"repo_alias {alias!r} is not registered by any worker",
        )
    return alias


@app.post("/api/jobs", dependencies=[Depends(require_owner)])
def create_job(job: JobCreate):
    ts = now_iso()
    with connect() as conn:
        job_id = create_job_record(
            conn,
            {
                "title": job.title,
                "prompt": job.prompt,
                "repo_alias": resolve_repo_alias(conn, job.repo_alias),
                "worker_name": DEFAULT_WORKER_NAME,
                "agent": job.agent or None,
                "status": models.QUEUED,
                "created_at": ts,
                "updated_at": ts,
            },
        )
        return get_job(conn, job_id, include_logs=True)


@app.get("/api/jobs", dependencies=[Depends(require_owner)])
def list_jobs():
    with connect() as conn:
        return list_job_records(conn)


@app.get("/api/repos", dependencies=[Depends(require_owner)])
def list_repos():
    with connect() as conn:
        return list_worker_repos(conn)


@app.get("/api/jobs/{job_id}", dependencies=[Depends(require_owner)])
def api_job(job_id: int, log_limit: int = 200, before_log_id: int | None = None):
    with connect() as conn:
        job = get_job(conn, job_id, include_logs=True, log_limit=log_limit, before_log_id=before_log_id)
    if not job:
        raise HTTPException(status_code=404, detail="Job not found")
    return job


@app.post("/api/jobs/{job_id}/cancel", dependencies=[Depends(require_owner)])
def cancel_job(job_id: int):
    ts = now_iso()
    with connect() as conn:
        job = get_job(conn, job_id)
        if not job:
            raise HTTPException(status_code=404, detail="Job not found")
        if job["status"] == models.RUNNING:
            conn.execute(
                update(jobs)
                .where(jobs.c.id == job_id, jobs.c.status == models.RUNNING)
                .values(cancel_requested_at=ts, updated_at=ts)
            )
            return get_job(conn, job_id, include_logs=True)
        if job["status"] != models.QUEUED:
            return {"message": f"Job is already {job['status']}"}
        conn.execute(
            update(jobs)
            .where(jobs.c.id == job_id)
            .values(status=models.CANCELLED, updated_at=ts, completed_at=ts)
        )
        return get_job(conn, job_id, include_logs=True)


@app.get("/api/worker/next", dependencies=[Depends(require_worker)])
def worker_next(worker_name: str = "local"):
    ts = now_iso()
    with connect() as conn:
        touch_worker(conn, worker_name)
        job = claim_next_job(conn, worker_name, ts)
        if not job:
            return Response(status_code=204)
        return job


@app.post("/api/worker/register", dependencies=[Depends(require_worker)])
def register_worker(body: WorkerRegister):
    ts = now_iso()
    with connect() as conn:
        touch_worker(conn, body.worker_name)
        for repo in body.repos:
            alias = repo.repo_alias.strip()
            if not alias:
                continue
            display = repo.display_name.strip() or alias
            upsert_worker_repo(conn, body.worker_name, alias, display, ts)
    return {"ok": True}


def require_active_attempt(job: dict, attempt_id: str) -> None:
    if job["status"] != models.RUNNING:
        raise HTTPException(status_code=409, detail=f"Job is already {job['status']}")
    if job["attempt_id"] != attempt_id:
        raise HTTPException(status_code=409, detail="Job attempt is no longer active")


def cancel_running_attempt(conn, job: dict, body: CancelledJob, ts: str) -> dict:
    result = conn.execute(
        update(jobs)
        .where(jobs.c.id == job["id"], jobs.c.status == models.RUNNING, jobs.c.attempt_id == body.attempt_id)
        .values(
            status=models.CANCELLED,
            exit_code=body.exit_code,
            final_summary=body.final_summary,
            receipt_json=body.receipt_json,
            git_diff=body.git_diff,
            baseline_commit=body.baseline_commit,
            updated_at=ts,
            completed_at=ts,
            heartbeat_at=ts,
            lease_expires_at=None,
        )
    )
    if result.rowcount != 1:
        raise HTTPException(status_code=409, detail="Job attempt is no longer active")
    conn.execute(
        update(job_attempts)
        .where(job_attempts.c.attempt_id == body.attempt_id)
        .values(status=models.CANCELLED, exit_code=body.exit_code, heartbeat_at=ts, finished_at=ts)
    )
    conn.execute(
        insert(job_logs).values(
            job_id=job["id"],
            attempt_id=body.attempt_id,
            timestamp=ts,
            stream="system",
            content="Server accepted cancelled result",
        )
    )
    return {"ok": True, "id": job["id"], "status": models.CANCELLED}


@app.post("/api/worker/jobs/{job_id}/heartbeat", dependencies=[Depends(require_worker)])
def heartbeat(job_id: int, body: AttemptRequest):
    ts = now_iso()
    lease_expires_at = lease_expires_iso()
    with connect() as conn:
        job = get_job(conn, job_id)
        if not job:
            raise HTTPException(status_code=404, detail="Job not found")
        require_active_attempt(job, body.attempt_id)
        touch_worker(conn, job["worker_name"])
        result = conn.execute(
            update(jobs)
            .where(jobs.c.id == job_id, jobs.c.status == models.RUNNING, jobs.c.attempt_id == body.attempt_id)
            .values(updated_at=ts, heartbeat_at=ts, lease_expires_at=lease_expires_at)
        )
        if result.rowcount != 1:
            raise HTTPException(status_code=409, detail="Job attempt is no longer active")
        conn.execute(
            update(job_attempts)
            .where(job_attempts.c.attempt_id == body.attempt_id)
            .values(heartbeat_at=ts, lease_expires_at=lease_expires_at)
        )
        return {"ok": True, "cancel_requested": job["cancel_requested_at"] is not None}


@app.post("/api/worker/jobs/{job_id}/logs", dependencies=[Depends(require_worker)])
def append_log(job_id: int, log: LogCreate):
    if log.stream not in {"stdout", "stderr", "system"}:
        raise HTTPException(status_code=422, detail="stream must be stdout, stderr, or system")
    ts = now_iso()
    with connect() as conn:
        job = get_job(conn, job_id)
        if not job:
            raise HTTPException(status_code=404, detail="Job not found")
        require_active_attempt(job, log.attempt_id)
        result = conn.execute(
            update(jobs)
            .where(jobs.c.id == job_id, jobs.c.status == models.RUNNING, jobs.c.attempt_id == log.attempt_id)
            .values(updated_at=ts)
        )
        if result.rowcount != 1:
            raise HTTPException(status_code=409, detail="Job attempt is no longer active")
        conn.execute(
            insert(job_logs).values(
                job_id=job_id,
                attempt_id=log.attempt_id,
                timestamp=ts,
                stream=log.stream,
                content=log.content,
            )
        )
    return {"ok": True}


@app.post("/api/worker/jobs/{job_id}/complete", dependencies=[Depends(require_worker)])
def complete_job(job_id: int, body: CompleteJob):
    ts = now_iso()
    with connect() as conn:
        job = get_job(conn, job_id)
        if not job:
            raise HTTPException(status_code=404, detail="Job not found")
        if job["status"] == models.COMPLETED and job["attempt_id"] == body.attempt_id:
            return {"ok": True, "id": job_id, "status": models.COMPLETED}
        require_active_attempt(job, body.attempt_id)
        if job["cancel_requested_at"] is not None:
            return cancel_running_attempt(
                conn,
                job,
                CancelledJob(
                    attempt_id=body.attempt_id,
                    exit_code=130,
                    final_summary=body.final_summary,
                    receipt_json=body.receipt_json,
                    git_diff=body.git_diff,
                    baseline_commit=body.baseline_commit,
                ),
                ts,
            )
        result = conn.execute(
            update(jobs)
            .where(jobs.c.id == job_id, jobs.c.status == models.RUNNING, jobs.c.attempt_id == body.attempt_id)
            .values(
                status=models.COMPLETED,
                exit_code=body.exit_code,
                final_summary=body.final_summary,
                receipt_json=body.receipt_json,
                git_diff=body.git_diff,
                baseline_commit=body.baseline_commit,
                updated_at=ts,
                completed_at=ts,
                heartbeat_at=ts,
                lease_expires_at=None,
            )
        )
        if result.rowcount != 1:
            raise HTTPException(status_code=409, detail="Job attempt is no longer active")
        conn.execute(
            update(job_attempts)
            .where(job_attempts.c.attempt_id == body.attempt_id)
            .values(status=models.COMPLETED, exit_code=body.exit_code, heartbeat_at=ts, finished_at=ts)
        )
        conn.execute(
            insert(job_logs).values(
                job_id=job_id,
                attempt_id=body.attempt_id,
                timestamp=ts,
                stream="system",
                content="Server accepted completed result",
            )
        )
        return {"ok": True, "id": job_id, "status": models.COMPLETED}


@app.post("/api/worker/jobs/{job_id}/fail", dependencies=[Depends(require_worker)])
def fail_job(job_id: int, body: FailJob):
    ts = now_iso()
    with connect() as conn:
        job = get_job(conn, job_id)
        if not job:
            raise HTTPException(status_code=404, detail="Job not found")
        if job["status"] == models.FAILED and job["attempt_id"] == body.attempt_id:
            return {"ok": True, "id": job_id, "status": models.FAILED}
        require_active_attempt(job, body.attempt_id)
        if job["cancel_requested_at"] is not None:
            return cancel_running_attempt(
                conn,
                job,
                CancelledJob(
                    attempt_id=body.attempt_id,
                    exit_code=body.exit_code,
                    final_summary=body.final_summary,
                    receipt_json=body.receipt_json,
                    git_diff=body.git_diff,
                    baseline_commit=body.baseline_commit,
                ),
                ts,
            )
        result = conn.execute(
            update(jobs)
            .where(jobs.c.id == job_id, jobs.c.status == models.RUNNING, jobs.c.attempt_id == body.attempt_id)
            .values(
                status=models.FAILED,
                exit_code=body.exit_code,
                error_message=body.error_message,
                final_summary=body.final_summary,
                receipt_json=body.receipt_json,
                git_diff=body.git_diff,
                baseline_commit=body.baseline_commit,
                updated_at=ts,
                completed_at=ts,
                heartbeat_at=ts,
                lease_expires_at=None,
            )
        )
        if result.rowcount != 1:
            raise HTTPException(status_code=409, detail="Job attempt is no longer active")
        conn.execute(
            update(job_attempts)
            .where(job_attempts.c.attempt_id == body.attempt_id)
            .values(
                status=models.FAILED,
                exit_code=body.exit_code,
                error_message=body.error_message,
                heartbeat_at=ts,
                finished_at=ts,
            )
        )
        conn.execute(
            insert(job_logs).values(
                job_id=job_id,
                attempt_id=body.attempt_id,
                timestamp=ts,
                stream="system",
                content="Server accepted failed result",
            )
        )
        return {"ok": True, "id": job_id, "status": models.FAILED}


@app.post("/api/worker/jobs/{job_id}/cancelled", dependencies=[Depends(require_worker)])
def cancelled_job(job_id: int, body: CancelledJob):
    ts = now_iso()
    with connect() as conn:
        job = get_job(conn, job_id)
        if not job:
            raise HTTPException(status_code=404, detail="Job not found")
        if job["status"] == models.CANCELLED and job["attempt_id"] == body.attempt_id:
            return {"ok": True, "id": job_id, "status": models.CANCELLED}
        require_active_attempt(job, body.attempt_id)
        return cancel_running_attempt(conn, job, body, ts)
