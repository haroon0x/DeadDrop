import os
import sys
from pathlib import Path

from fastapi.testclient import TestClient

os.environ["OWNER_TOKEN"] = "owner_test"
os.environ["WORKER_TOKEN"] = "worker_test"
os.environ["SQLITE_PATH"] = "/tmp/deaddrop_test.db"
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from app.main import app  # noqa: E402
from app.db import init_db  # noqa: E402


def client():
    try:
        os.remove(os.environ["SQLITE_PATH"])
    except FileNotFoundError:
        pass
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
        json={"title": "Fix test", "prompt": "Fix failing test", "repo_alias": "default", "worker_name": "local"},
    )
    assert create.status_code == 200
    job_id = create.json()["id"]

    next_job = c.get("/api/worker/next?worker_name=local", headers=worker_headers())
    assert next_job.status_code == 200
    assert next_job.json()["id"] == job_id
    assert next_job.json()["status"] == "running"

    log = c.post(
        f"/api/worker/jobs/{job_id}/logs",
        headers=worker_headers(),
        json={"stream": "system", "content": "Picked up job"},
    )
    assert log.status_code == 200

    complete = c.post(
        f"/api/worker/jobs/{job_id}/complete",
        headers=worker_headers(),
        json={"exit_code": 0, "final_summary": "Fixed", "git_diff": "diff --git"},
    )
    assert complete.status_code == 200
    body = complete.json()
    assert body["status"] == "completed"
    assert body["logs"][0]["content"] == "Picked up job"


def test_new_job_page_hides_worker_choice_and_shows_repo_dropdown():
    c = client()
    c.post(
        "/api/worker/register",
        headers=worker_headers(),
        json={"worker_name": "local", "repos": [{"repo_alias": "demo", "display_name": "Demo repo"}]},
    )
    res = c.get("/jobs/new", cookies={"owner_token": "owner_test"})
    assert res.status_code == 200
    assert 'name="repo_alias"' in res.text
    assert 'type="hidden" name="worker_name"' in res.text
    assert ">Worker<" not in res.text
