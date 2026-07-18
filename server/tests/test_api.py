import os
import sys
from pathlib import Path

from fastapi.testclient import TestClient

os.environ["OWNER_TOKEN"] = "owner_test"
os.environ["WORKER_TOKEN"] = "worker_test"
os.environ["DATABASE_URL"] = "sqlite:////tmp/deaddrop_test.db"
os.environ["SECURE_COOKIES"] = "false"
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from app.main import app  # noqa: E402
from app.db import connect, init_db, jobs, reset_engine_for_tests  # noqa: E402
from sqlalchemy import update


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


def test_auth_rejects_bad_owner_token():
    c = client()
    res = c.get("/api/jobs", headers={"Authorization": "Bearer bad"})
    assert res.status_code == 401


def test_health_and_ready_endpoints():
    c = client()
    assert c.get("/healthz").json() == {"ok": True}
    assert c.get("/readyz").json() == {"ok": True}


def test_create_job_and_worker_flow():
    c = client()
    register = c.post(
        "/api/worker/register",
        headers=worker_headers(),
        json={"worker_name": "local", "repos": [{"repo_alias": "default", "display_name": "Demo repo"}]},
    )
    assert register.status_code == 200
    repos = c.get("/api/repos", headers=owner_headers())
    assert repos.status_code == 200
    assert repos.json()[0]["repo_alias"] == "default"

    create = c.post(
        "/api/jobs",
        headers=owner_headers(),
        json={"title": "Fix test", "prompt": "Fix failing test"},
    )
    assert create.status_code == 200
    job_id = create.json()["id"]

    next_job = c.get("/api/worker/next?worker_name=local", headers=worker_headers())
    assert next_job.status_code == 200
    assert next_job.json()["id"] == job_id
    assert next_job.json()["status"] == "running"
    attempt_id = next_job.json()["attempt_id"]

    log = c.post(
        f"/api/worker/jobs/{job_id}/logs",
        headers=worker_headers(),
        json={"attempt_id": attempt_id, "stream": "system", "content": "Picked up job"},
    )
    assert log.status_code == 200

    complete = c.post(
        f"/api/worker/jobs/{job_id}/complete",
        headers=worker_headers(),
        json={"attempt_id": attempt_id, "exit_code": 0, "final_summary": "Fixed", "git_diff": "diff --git"},
    )
    assert complete.status_code == 200
    body = complete.json()
    assert body["status"] == "completed"
    assert body["id"] == job_id

    fetched = c.get(f"/api/jobs/{job_id}", headers=owner_headers())
    assert fetched.status_code == 200
    assert fetched.json()["status"] == "completed"
    assert fetched.json()["logs"][0]["content"] == "Picked up job"


def test_new_job_page_hides_repo_and_worker_choice():
    c = client()
    c.post(
        "/api/worker/register",
        headers=worker_headers(),
        json={"worker_name": "local", "repos": [{"repo_alias": "demo", "display_name": "Demo repo"}]},
    )
    res = c.get("/jobs/new", cookies={"owner_token": "owner_test"})
    assert res.status_code == 200
    assert 'name="repo_alias"' not in res.text
    assert 'name="worker_name"' not in res.text
    assert ">Worker<" not in res.text
    assert ">Repo<" not in res.text


def test_owner_supplied_job_routing_is_ignored():
    c = client()
    create = c.post(
        "/api/jobs",
        headers=owner_headers(),
        json={"title": "Route", "prompt": "No-op", "repo_alias": "other", "worker_name": "other"},
    )
    assert create.status_code == 200
    body = create.json()
    assert body["repo_alias"] == "default"
    assert body["worker_name"] == "local"


def test_structured_receipt_renders_as_sections():
    c = client()
    create = c.post(
        "/api/jobs",
        headers=owner_headers(),
        json={"title": "Receipt", "prompt": "Return structured receipt"},
    )
    job_id = create.json()["id"]
    next_job = c.get("/api/worker/next?worker_name=local", headers=worker_headers())
    assert next_job.status_code == 200
    assert next_job.json()["id"] == job_id
    attempt_id = next_job.json()["attempt_id"]
    receipt_json = (
        '{"status":"completed","summary":"Fixed the parser.",'
        '"changed_files":["parser.py"],'
        '"verification":[{"command":"pytest","status":"passed","summary":"3 passed"}],'
        '"blockers":[],"notes":"No commit created."}'
    )
    complete = c.post(
        f"/api/worker/jobs/{job_id}/complete",
        headers=worker_headers(),
        json={"attempt_id": attempt_id, "exit_code": 0, "final_summary": "Fixed the parser.", "receipt_json": receipt_json, "git_diff": ""},
    )
    assert complete.status_code == 200

    api = c.get(f"/api/jobs/{job_id}", headers=owner_headers())
    assert api.json()["receipt"]["summary"] == "Fixed the parser."

    page = c.get(f"/jobs/{job_id}", cookies={"owner_token": "owner_test"})
    assert page.status_code == 200
    assert "Changed files" in page.text
    assert "parser.py" in page.text
    assert "Verification" in page.text
    assert "3 passed" in page.text
    assert "<summary>Live logs</summary>" in page.text
    assert "<summary>Git diff</summary>" in page.text
    assert "Server accepted completed result" in page.text


def test_browser_auth_uses_persistent_cookie_not_query_token():
    c = client()
    query_res = c.get("/?token=owner_test", follow_redirects=False)
    assert query_res.status_code == 200
    assert "Leave a task." in query_res.text
    assert "Your local-agent task queue" not in query_res.text

    login = c.post("/login", data={"token": "owner_test"}, follow_redirects=False)
    assert login.status_code == 303
    cookies = login.cookies
    assert "owner_token" in cookies
    assert "csrf_token" in cookies
    
    # Verify we can access dashboard with the cookies
    res = c.get("/", cookies=cookies)
    assert res.status_code == 200
    assert "Your local-agent task queue" in res.text


def test_queued_job_can_be_cancelled_from_page():
    c = client()
    create = c.post(
        "/api/jobs",
        headers=owner_headers(),
        json={"title": "Cancel me", "prompt": "No-op"},
    )
    job_id = create.json()["id"]

    login = c.post("/login", data={"token": "owner_test"}, follow_redirects=False)
    cookies = login.cookies

    detail = c.get(f"/jobs/{job_id}", cookies=cookies)
    assert detail.status_code == 200
    assert f'action="/jobs/{job_id}/cancel"' in detail.text
    csrf_token = cookies.get("csrf_token")

    cancel = c.post(
        f"/jobs/{job_id}/cancel",
        cookies=cookies,
        data={"csrf_token": csrf_token},
        follow_redirects=False,
    )
    assert cancel.status_code == 303

    job = c.get(f"/api/jobs/{job_id}", headers=owner_headers())
    assert job.json()["status"] == "cancelled"


def test_job_logs_are_paginated():
    c = client()
    create = c.post(
        "/api/jobs",
        headers=owner_headers(),
        json={"title": "Logs", "prompt": "Emit logs"},
    )
    job_id = create.json()["id"]
    next_job = c.get("/api/worker/next?worker_name=local", headers=worker_headers())
    attempt_id = next_job.json()["attempt_id"]
    for i in range(205):
        c.post(
            f"/api/worker/jobs/{job_id}/logs",
            headers=worker_headers(),
            json={"attempt_id": attempt_id, "stream": "stdout", "content": f"log {i}"},
        )

    latest = c.get(f"/api/jobs/{job_id}", headers=owner_headers())
    body = latest.json()
    assert len(body["logs"]) == 200
    assert body["logs"][0]["content"] == "log 5"
    assert body["older_logs_available"] is True

    older = c.get(
        f"/api/jobs/{job_id}?before_log_id={body['oldest_log_id']}",
        headers=owner_headers(),
    )
    older_body = older.json()
    assert len(older_body["logs"]) == 5
    assert older_body["logs"][0]["content"] == "log 0"


def test_worker_cannot_overwrite_terminal_job_state():
    c = client()
    create = c.post(
        "/api/jobs",
        headers=owner_headers(),
        json={"title": "Terminal", "prompt": "No-op"},
    )
    job_id = create.json()["id"]
    next_job = c.get("/api/worker/next?worker_name=local", headers=worker_headers())
    attempt_id = next_job.json()["attempt_id"]
    complete = c.post(
        f"/api/worker/jobs/{job_id}/complete",
        headers=worker_headers(),
        json={"attempt_id": attempt_id, "exit_code": 0, "final_summary": "Done"},
    )
    assert complete.status_code == 200

    replay = c.post(
        f"/api/worker/jobs/{job_id}/complete",
        headers=worker_headers(),
        json={"attempt_id": attempt_id, "exit_code": 0, "final_summary": "Done"},
    )
    assert replay.status_code == 200

    overwrite = c.post(
        f"/api/worker/jobs/{job_id}/fail",
        headers=worker_headers(),
        json={"attempt_id": attempt_id, "exit_code": 1, "error_message": "overwrite"},
    )
    assert overwrite.status_code == 409
    late_log = c.post(
        f"/api/worker/jobs/{job_id}/logs",
        headers=worker_headers(),
        json={"attempt_id": attempt_id, "stream": "system", "content": "late"},
    )
    assert late_log.status_code == 409
    fetched = c.get(f"/api/jobs/{job_id}", headers=owner_headers())
    assert fetched.json()["status"] == "completed"


def test_large_worker_payloads_are_rejected():
    c = client()
    create = c.post(
        "/api/jobs",
        headers=owner_headers(),
        json={"title": "Large", "prompt": "No-op"},
    )
    job_id = create.json()["id"]
    next_job = c.get("/api/worker/next?worker_name=local", headers=worker_headers())
    attempt_id = next_job.json()["attempt_id"]
    too_large_log = c.post(
        f"/api/worker/jobs/{job_id}/logs",
        headers=worker_headers(),
        json={"attempt_id": attempt_id, "stream": "system", "content": "x" * 20001},
    )
    assert too_large_log.status_code == 422


def test_running_job_cancellation_reaches_worker_attempt():
    c = client()
    create = c.post(
        "/api/jobs",
        headers=owner_headers(),
        json={"title": "Cancel running", "prompt": "Wait"},
    )
    job_id = create.json()["id"]
    claimed = c.get("/api/worker/next?worker_name=local", headers=worker_headers()).json()
    attempt_id = claimed["attempt_id"]

    cancel = c.post(f"/api/jobs/{job_id}/cancel", headers=owner_headers())
    assert cancel.status_code == 200
    assert cancel.json()["status"] == "running"
    assert cancel.json()["cancel_requested_at"] is not None

    heartbeat = c.post(
        f"/api/worker/jobs/{job_id}/heartbeat",
        headers=worker_headers(),
        json={"attempt_id": attempt_id},
    )
    assert heartbeat.status_code == 200
    assert heartbeat.json()["cancel_requested"] is True

    cancelled = c.post(
        f"/api/worker/jobs/{job_id}/cancelled",
        headers=worker_headers(),
        json={"attempt_id": attempt_id, "exit_code": 130, "final_summary": "Cancelled"},
    )
    assert cancelled.status_code == 200
    assert cancelled.json()["status"] == "cancelled"


def test_stale_job_is_reissued_with_new_attempt():
    c = client()
    create = c.post(
        "/api/jobs",
        headers=owner_headers(),
        json={"title": "Recover", "prompt": "Wait"},
    )
    job_id = create.json()["id"]
    first = c.get("/api/worker/next?worker_name=local", headers=worker_headers()).json()
    with connect() as conn:
        conn.execute(
            update(jobs)
            .where(jobs.c.id == job_id)
            .values(lease_expires_at="2000-01-01T00:00:00+00:00")
        )

    second = c.get("/api/worker/next?worker_name=local", headers=worker_headers()).json()
    assert second["id"] == job_id
    assert second["attempt_number"] == 2
    assert second["attempt_id"] != first["attempt_id"]

    stale_heartbeat = c.post(
        f"/api/worker/jobs/{job_id}/heartbeat",
        headers=worker_headers(),
        json={"attempt_id": first["attempt_id"]},
    )
    assert stale_heartbeat.status_code == 409


def test_public_project_pages_render():
    c = client()
    pages = {
        "/docs": "From zero to a verified local-agent queue",
        "/docs/architecture": "The server coordinates. The worker proves.",
        "/updates": "Building the boring parts",
        "/blog": "Notes on building agent infrastructure",
        "/blog/disposable-worktrees": "Why every coding-agent job gets a disposable worktree",
        "/blog/evidence-based-receipts": "A receipt should be evidence, not agent prose",
        "/blog/leases-for-local-agents": "Leases make local agents recoverable",
    }
    for path, heading in pages.items():
        response = c.get(path)
        assert response.status_code == 200
        assert heading in response.text
