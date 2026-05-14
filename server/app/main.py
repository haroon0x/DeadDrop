import os
from pathlib import Path

from fastapi import Depends, FastAPI, Form, HTTPException, Request, Response
from fastapi.responses import HTMLResponse, JSONResponse, RedirectResponse
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates

from . import models
from .auth import owner_from_request, require_owner, require_worker
from .db import connect, get_job, init_db, now_iso, touch_worker
from .schemas import CompleteJob, FailJob, JobCreate, LogCreate, WorkerRegister

BASE_DIR = Path(__file__).resolve().parent

app = FastAPI(title="DeadDrop")
app.mount("/static", StaticFiles(directory=BASE_DIR / "static"), name="static")
templates = Jinja2Templates(directory=BASE_DIR / "templates")


@app.on_event("startup")
def startup() -> None:
    init_db()


def ensure_owner_page(request: Request) -> None:
    if not owner_from_request(request):
        raise HTTPException(status_code=303, headers={"Location": "/login"})


@app.get("/", response_class=HTMLResponse)
def dashboard(request: Request):
    ensure_owner_page(request)
    with connect() as conn:
        jobs = [dict(row) for row in conn.execute("SELECT * FROM jobs ORDER BY datetime(created_at) DESC, id DESC")]
    return templates.TemplateResponse("dashboard.html", {"request": request, "jobs": jobs})


@app.get("/login", response_class=HTMLResponse)
def login_page(request: Request):
    return templates.TemplateResponse("login.html", {"request": request, "error": ""})


@app.post("/login")
def login(request: Request, token: str = Form(...)):
    from .auth import owner_token

    if token != owner_token():
        return templates.TemplateResponse("login.html", {"request": request, "error": "Invalid token"}, status_code=401)
    response = RedirectResponse("/", status_code=303)
    response.set_cookie("owner_token", token, httponly=True, samesite="lax")
    return response


@app.get("/logout")
def logout():
    response = RedirectResponse("/login", status_code=303)
    response.delete_cookie("owner_token")
    return response


@app.get("/jobs/new", response_class=HTMLResponse)
def new_job_page(request: Request):
    ensure_owner_page(request)
    with connect() as conn:
        repos = [
            dict(row)
            for row in conn.execute(
                "SELECT * FROM worker_repos ORDER BY worker_name, repo_alias"
            ).fetchall()
        ]
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
    with connect() as conn:
        job = get_job(conn, job_id, include_logs=True)
    if not job:
        raise HTTPException(status_code=404, detail="Job not found")
    return templates.TemplateResponse("job_detail.html", {"request": request, "job": job})


@app.get("/jobs/{job_id}/fragment", response_class=HTMLResponse)
def job_fragment(request: Request, job_id: int):
    ensure_owner_page(request)
    with connect() as conn:
        job = get_job(conn, job_id, include_logs=True)
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
        cur = conn.execute(
            """
            INSERT INTO jobs (title, prompt, repo_alias, worker_name, status, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?)
            """,
            (job.title, job.prompt, job.repo_alias, job.worker_name, models.QUEUED, ts, ts),
        )
        return get_job(conn, cur.lastrowid, include_logs=True)


@app.get("/api/jobs", dependencies=[Depends(require_owner)])
def list_jobs():
    with connect() as conn:
        return [dict(row) for row in conn.execute("SELECT * FROM jobs ORDER BY datetime(created_at) DESC, id DESC")]


@app.get("/api/repos", dependencies=[Depends(require_owner)])
def list_repos():
    with connect() as conn:
        return [
            dict(row)
            for row in conn.execute(
                "SELECT worker_name, repo_alias, display_name, last_seen_at FROM worker_repos ORDER BY worker_name, repo_alias"
            ).fetchall()
        ]


@app.get("/api/jobs/{job_id}", dependencies=[Depends(require_owner)])
def api_job(job_id: int):
    with connect() as conn:
        job = get_job(conn, job_id, include_logs=True)
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
            "UPDATE jobs SET status = ?, updated_at = ?, completed_at = ? WHERE id = ?",
            (models.CANCELLED, ts, ts, job_id),
        )
        return get_job(conn, job_id, include_logs=True)


@app.get("/api/worker/next", dependencies=[Depends(require_worker)])
def worker_next(worker_name: str = "local"):
    ts = now_iso()
    with connect() as conn:
        conn.execute("BEGIN IMMEDIATE")
        touch_worker(conn, worker_name)
        row = conn.execute(
            """
            SELECT id FROM jobs
            WHERE status = ? AND worker_name = ?
            ORDER BY datetime(created_at) ASC, id ASC
            LIMIT 1
            """,
            (models.QUEUED, worker_name),
        ).fetchone()
        if not row:
            return Response(status_code=204)
        conn.execute(
            "UPDATE jobs SET status = ?, started_at = ?, updated_at = ? WHERE id = ?",
            (models.RUNNING, ts, ts, row["id"]),
        )
        return get_job(conn, row["id"], include_logs=True)


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
            conn.execute(
                """
                INSERT INTO worker_repos (worker_name, repo_alias, display_name, last_seen_at, created_at)
                VALUES (?, ?, ?, ?, ?)
                ON CONFLICT(worker_name, repo_alias)
                DO UPDATE SET display_name = excluded.display_name, last_seen_at = excluded.last_seen_at
                """,
                (body.worker_name, alias, display, ts, ts),
            )
    return {"ok": True}


@app.post("/api/worker/jobs/{job_id}/heartbeat", dependencies=[Depends(require_worker)])
def heartbeat(job_id: int, worker_name: str = "local"):
    ts = now_iso()
    with connect() as conn:
        touch_worker(conn, worker_name)
        conn.execute("UPDATE jobs SET updated_at = ? WHERE id = ?", (ts, job_id))
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
            "INSERT INTO job_logs (job_id, timestamp, stream, content) VALUES (?, ?, ?, ?)",
            (job_id, ts, log.stream, log.content),
        )
        conn.execute("UPDATE jobs SET updated_at = ? WHERE id = ?", (ts, job_id))
    return {"ok": True}


@app.post("/api/worker/jobs/{job_id}/complete", dependencies=[Depends(require_worker)])
def complete_job(job_id: int, body: CompleteJob):
    ts = now_iso()
    with connect() as conn:
        if not get_job(conn, job_id):
            raise HTTPException(status_code=404, detail="Job not found")
        conn.execute(
            """
            UPDATE jobs
            SET status = ?, exit_code = ?, final_summary = ?, git_diff = ?, updated_at = ?, completed_at = ?
            WHERE id = ?
            """,
            (models.COMPLETED, body.exit_code, body.final_summary, body.git_diff, ts, ts, job_id),
        )
        return get_job(conn, job_id, include_logs=True)


@app.post("/api/worker/jobs/{job_id}/fail", dependencies=[Depends(require_worker)])
def fail_job(job_id: int, body: FailJob):
    ts = now_iso()
    with connect() as conn:
        if not get_job(conn, job_id):
            raise HTTPException(status_code=404, detail="Job not found")
        conn.execute(
            """
            UPDATE jobs
            SET status = ?, exit_code = ?, error_message = ?, final_summary = ?, git_diff = ?, updated_at = ?, completed_at = ?
            WHERE id = ?
            """,
            (models.FAILED, body.exit_code, body.error_message, body.final_summary, body.git_diff, ts, ts, job_id),
        )
        return get_job(conn, job_id, include_logs=True)
