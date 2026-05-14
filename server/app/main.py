import os
from pathlib import Path

from fastapi import Depends, FastAPI, Form, HTTPException, Request, Response
from fastapi.responses import HTMLResponse, JSONResponse, RedirectResponse
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates
from sqlalchemy import insert, text, update

from . import models
from .auth import owner_from_request, require_owner, require_worker, secure_cookies, validate_auth_config
from .db import (
    claim_next_job,
    connect,
    create_job_record,
    get_job,
    init_db,
    jobs,
    job_logs,
    list_jobs as list_job_records,
    list_worker_repos,
    now_iso,
    touch_worker,
    upsert_worker_repo,
)
from .schemas import CompleteJob, FailJob, JobCreate, LogCreate, WorkerRegister

BASE_DIR = Path(__file__).resolve().parent

app = FastAPI(title="DeadDrop")
app.mount("/static", StaticFiles(directory=BASE_DIR / "static"), name="static")
templates = Jinja2Templates(directory=BASE_DIR / "templates")


@app.on_event("startup")
def startup() -> None:
    validate_auth_config()
    init_db()


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
        return templates.TemplateResponse("landing.html", {"request": request})
    with connect() as conn:
        rows = list_job_records(conn)
    return templates.TemplateResponse("dashboard.html", {"request": request, "jobs": rows})


@app.get("/login", response_class=HTMLResponse)
def login_page(request: Request):
    return templates.TemplateResponse("login.html", {"request": request, "error": ""})


@app.post("/login")
def login(request: Request, token: str = Form(...)):
    from .auth import owner_token

    if token != owner_token():
        return templates.TemplateResponse("login.html", {"request": request, "error": "Invalid token"}, status_code=401)
    response = RedirectResponse("/", status_code=303)
    response.set_cookie(
        "owner_token",
        token,
        max_age=60 * 60 * 24 * 30,
        httponly=True,
        secure=secure_cookies(),
        samesite="lax",
    )
    return response


@app.get("/logout")
def logout():
    response = RedirectResponse("/login", status_code=303)
    response.delete_cookie("owner_token", secure=secure_cookies(), samesite="lax")
    return response


@app.get("/jobs/new", response_class=HTMLResponse)
def new_job_page(request: Request):
    ensure_owner_page(request)
    with connect() as conn:
        repos = list_worker_repos(conn)
    return templates.TemplateResponse("new_job.html", {"request": request, "repos": repos})


@app.post("/jobs")
def create_job_form(
    request: Request,
    title: str = Form(...),
    prompt: str = Form(...),
    repo_alias: str = Form("default"),
    worker_name: str = Form("local"),
):
    ensure_owner_page(request)
    job = JobCreate(title=title, prompt=prompt, repo_alias=repo_alias or "default", worker_name=worker_name or "local")
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
    return templates.TemplateResponse("job_detail.html", {"request": request, "job": job})


@app.post("/jobs/{job_id}/cancel")
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
    return templates.TemplateResponse("_job_receipt.html", {"request": request, "job": job})


@app.get("/demo", response_class=HTMLResponse)
def demo(request: Request):
    return templates.TemplateResponse("demo.html", {"request": request})


@app.post("/api/jobs", dependencies=[Depends(require_owner)])
def create_job(job: JobCreate):
    ts = now_iso()
    with connect() as conn:
        job_id = create_job_record(
            conn,
            {
                "title": job.title,
                "prompt": job.prompt,
                "repo_alias": job.repo_alias,
                "worker_name": job.worker_name,
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
            return JSONResponse({"message": "Running cancellation is not supported yet"}, status_code=409)
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


@app.post("/api/worker/jobs/{job_id}/heartbeat", dependencies=[Depends(require_worker)])
def heartbeat(job_id: int, worker_name: str = "local"):
    ts = now_iso()
    with connect() as conn:
        touch_worker(conn, worker_name)
        conn.execute(update(jobs).where(jobs.c.id == job_id).values(updated_at=ts))
    return {"ok": True}


@app.post("/api/worker/jobs/{job_id}/logs", dependencies=[Depends(require_worker)])
def append_log(job_id: int, log: LogCreate):
    if log.stream not in {"stdout", "stderr", "system"}:
        raise HTTPException(status_code=422, detail="stream must be stdout, stderr, or system")
    ts = now_iso()
    with connect() as conn:
        if not get_job(conn, job_id):
            raise HTTPException(status_code=404, detail="Job not found")
        conn.execute(
            insert(job_logs).values(job_id=job_id, timestamp=ts, stream=log.stream, content=log.content)
        )
        conn.execute(update(jobs).where(jobs.c.id == job_id).values(updated_at=ts))
    return {"ok": True}


@app.post("/api/worker/jobs/{job_id}/complete", dependencies=[Depends(require_worker)])
def complete_job(job_id: int, body: CompleteJob):
    ts = now_iso()
    with connect() as conn:
        if not get_job(conn, job_id):
            raise HTTPException(status_code=404, detail="Job not found")
        conn.execute(
            update(jobs)
            .where(jobs.c.id == job_id)
            .values(
                status=models.COMPLETED,
                exit_code=body.exit_code,
                final_summary=body.final_summary,
                git_diff=body.git_diff,
                updated_at=ts,
                completed_at=ts,
            )
        )
        return {"ok": True, "id": job_id, "status": models.COMPLETED}


@app.post("/api/worker/jobs/{job_id}/fail", dependencies=[Depends(require_worker)])
def fail_job(job_id: int, body: FailJob):
    ts = now_iso()
    with connect() as conn:
        if not get_job(conn, job_id):
            raise HTTPException(status_code=404, detail="Job not found")
        conn.execute(
            update(jobs)
            .where(jobs.c.id == job_id)
            .values(
                status=models.FAILED,
                exit_code=body.exit_code,
                error_message=body.error_message,
                final_summary=body.final_summary,
                git_diff=body.git_diff,
                updated_at=ts,
                completed_at=ts,
            )
        )
        return {"ok": True, "id": job_id, "status": models.FAILED}
