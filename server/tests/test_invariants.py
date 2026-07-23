"""The properties DeadDrop actually promises, as executable checks.

These used to live in spec.md as prose. Prose invariants are guesses that the
next reader inherits without evidence; a failing test is a fact. Each test below
names one promise and tries to break it. If a promise stops being true, this
file is where you find out.
"""

import os
import sys
from pathlib import Path

from fastapi.testclient import TestClient
from sqlalchemy import select

os.environ["OWNER_TOKEN"] = "owner_test"
os.environ["WORKER_TOKEN"] = "worker_test"
os.environ["DATABASE_URL"] = "sqlite:////tmp/deaddrop_test.db"
os.environ["SECURE_COOKIES"] = "false"
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from app.db import connect, init_db, jobs, reset_engine_for_tests  # noqa: E402
from app.main import app  # noqa: E402


def client():
    try:
        os.remove("/tmp/deaddrop_test.db")
    except FileNotFoundError:
        pass
    reset_engine_for_tests()
    init_db()
    return TestClient(app)


def owner_headers():
    return {"Authorization": "Bearer owner_test"}


def worker_headers():
    return {"Authorization": "Bearer worker_test"}


def make_job(c, title="t", prompt="p"):
    return c.post("/api/jobs", json={"title": title, "prompt": prompt}, headers=owner_headers()).json()


def claim(c):
    return c.get("/api/worker/next", headers=worker_headers()).json()


# ---------------------------------------------------------------------------
# Promise: the server never learns where your code lives on disk.
# ---------------------------------------------------------------------------


def test_server_stores_a_repo_alias_and_never_an_absolute_path():
    c = client()
    make_job(c)
    with connect() as conn:
        row = conn.execute(select(jobs)).mappings().first()
    stored = " ".join(str(v) for v in row.values() if v is not None)
    assert "/home/" not in stored
    assert "C:\\" not in stored
    assert row["repo_alias"] == "default"


def test_worker_registration_accepts_aliases_without_paths():
    c = client()
    res = c.post(
        "/api/worker/register",
        headers=worker_headers(),
        json={"worker_name": "local", "repos": [{"repo_alias": "demo", "display_name": "Demo"}]},
    )
    assert res.status_code == 200
    listed = c.get("/api/repos", headers=owner_headers()).json()
    assert all("path" not in row for row in listed)


# ---------------------------------------------------------------------------
# Promise: one claim owns the job. A vanished worker cannot come back and write.
# ---------------------------------------------------------------------------


def test_superseded_attempt_cannot_write_a_terminal_result():
    c = client()
    job = make_job(c)
    first = claim(c)

    # Force the lease to expire so the job is reissued under a new attempt.
    c.post(
        f"/api/worker/jobs/{job['id']}/heartbeat",
        headers=worker_headers(),
        json={"attempt_id": first["attempt_id"]},
    )
    with connect() as conn:
        from sqlalchemy import update

        conn.execute(update(jobs).where(jobs.c.id == job["id"]).values(lease_expires_at="1970-01-01T00:00:00+00:00"))
    second = claim(c)
    assert second["attempt_id"] != first["attempt_id"]

    late = c.post(
        f"/api/worker/jobs/{job['id']}/complete",
        headers=worker_headers(),
        json={"attempt_id": first["attempt_id"], "exit_code": 0, "final_summary": "ghost"},
    )
    assert late.status_code == 409

    current = c.get(f"/api/jobs/{job['id']}", headers=owner_headers()).json()
    assert current["final_summary"] != "ghost"


def test_superseded_attempt_cannot_write_logs():
    c = client()
    job = make_job(c)
    first = claim(c)
    with connect() as conn:
        from sqlalchemy import update

        conn.execute(update(jobs).where(jobs.c.id == job["id"]).values(lease_expires_at="1970-01-01T00:00:00+00:00"))
    claim(c)

    res = c.post(
        f"/api/worker/jobs/{job['id']}/logs",
        headers=worker_headers(),
        json={"attempt_id": first["attempt_id"], "stream": "stdout", "content": "from the grave"},
    )
    assert res.status_code == 409


# ---------------------------------------------------------------------------
# Promise: delivering the same terminal result twice is safe.
# ---------------------------------------------------------------------------


def test_repeated_terminal_delivery_is_idempotent():
    c = client()
    job = make_job(c)
    attempt = claim(c)
    body = {"attempt_id": attempt["attempt_id"], "exit_code": 0, "final_summary": "done once"}

    first = c.post(f"/api/worker/jobs/{job['id']}/complete", headers=worker_headers(), json=body)
    second = c.post(f"/api/worker/jobs/{job['id']}/complete", headers=worker_headers(), json=body)

    assert first.status_code == 200
    assert second.status_code in (200, 409)
    final = c.get(f"/api/jobs/{job['id']}", headers=owner_headers()).json()
    assert final["status"] == "completed"
    assert final["final_summary"] == "done once"


# ---------------------------------------------------------------------------
# Promise: DeadDrop reports what it observed, not what the agent claimed.
# ---------------------------------------------------------------------------


def test_agent_cannot_declare_its_own_success():
    """An agent that exits non-zero while insisting it succeeded is still a
    failure. Status comes from the observed exit code."""
    c = client()
    job = make_job(c)
    attempt = claim(c)
    c.post(
        f"/api/worker/jobs/{job['id']}/fail",
        headers=worker_headers(),
        json={
            "attempt_id": attempt["attempt_id"],
            "exit_code": 1,
            "error_message": "verification failed",
            "receipt_json": '{"status": "completed", "summary": "all good!"}',
        },
    )
    final = c.get(f"/api/jobs/{job['id']}", headers=owner_headers()).json()
    assert final["status"] == "failed"
    assert final["exit_code"] == 1


# ---------------------------------------------------------------------------
# Promise: DeadDrop hands you a patch. It never applies one.
# ---------------------------------------------------------------------------


def test_no_route_applies_commits_or_pushes_anything():
    paths = {route.path for route in app.routes}
    forbidden = {"apply", "commit", "push", "merge"}
    offenders = [p for p in paths if any(word in p.lower() for word in forbidden)]
    assert offenders == [], f"routes suggest automatic application: {offenders}"


def test_patch_is_offered_as_a_download_not_an_action():
    c = client()
    job = make_job(c)
    attempt = claim(c)
    c.post(
        f"/api/worker/jobs/{job['id']}/complete",
        headers=worker_headers(),
        json={
            "attempt_id": attempt["attempt_id"],
            "exit_code": 0,
            "final_summary": "s",
            "git_diff": "diff --git a/x b/x\n",
        },
    )
    res = c.get(f"/api/jobs/{job['id']}/patch", headers=owner_headers())
    assert res.status_code == 200
    assert res.text.startswith("diff --git")


# ---------------------------------------------------------------------------
# Promise: the worker reaches out. The server never reaches in.
# ---------------------------------------------------------------------------


def test_every_worker_route_is_worker_initiated():
    """There is no endpoint the server can call on a worker, because the worker
    has no address. Every worker interaction is a route the worker calls."""
    worker_paths = [route.path for route in app.routes if "/worker" in route.path]
    assert worker_paths, "expected worker-facing routes"
    for path in worker_paths:
        assert path.startswith("/api/worker"), path


def test_worker_token_cannot_read_owner_surfaces():
    c = client()
    make_job(c)
    assert c.get("/api/jobs", headers=worker_headers()).status_code == 401


def test_owner_token_cannot_claim_work():
    c = client()
    make_job(c)
    assert c.get("/api/worker/next", headers=owner_headers()).status_code == 401
