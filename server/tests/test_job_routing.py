import os
import sys
from pathlib import Path

from fastapi.testclient import TestClient

os.environ["OWNER_TOKEN"] = "owner_test"
os.environ["WORKER_TOKEN"] = "worker_test"
os.environ["DATABASE_URL"] = "sqlite:////tmp/deaddrop_test.db"
os.environ["SECURE_COOKIES"] = "false"
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from app.db import init_db, reset_engine_for_tests  # noqa: E402
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


def create(c, **overrides):
    body = {"title": "t", "prompt": "p"}
    body.update(overrides)
    return c.post("/api/jobs", json=body, headers=owner_headers())


def test_job_records_selected_agent():
    c = client()
    res = create(c, agent="claude")
    assert res.status_code == 200, res.text
    assert res.json()["agent"] == "claude"


def test_agent_defaults_to_worker_choice():
    c = client()
    assert create(c).json()["agent"] is None


def test_unknown_agent_is_rejected():
    c = client()
    assert create(c, agent="not-a-real-agent").status_code == 422


def register_repo(c, alias):
    c.post(
        "/api/worker/register",
        headers=worker_headers(),
        json={"worker_name": "local", "repos": [{"repo_alias": alias, "display_name": alias}]},
    )


def test_registered_repo_alias_is_honoured():
    c = client()
    register_repo(c, "sidecar")
    assert create(c, repo_alias="sidecar").json()["repo_alias"] == "sidecar"


def test_unregistered_repo_alias_is_rejected():
    c = client()
    assert create(c, repo_alias="sidecar").status_code == 422


def test_claimed_job_carries_agent_to_the_worker():
    c = client()
    create(c, agent="aider")
    claimed = c.get("/api/worker/next", headers=worker_headers())
    assert claimed.status_code == 200, claimed.text
    assert claimed.json()["agent"] == "aider"


def test_worker_only_claims_jobs_for_its_own_repo_routing():
    c = client()
    register_repo(c, "sidecar")
    create(c, repo_alias="sidecar", agent="codex")
    claimed = c.get("/api/worker/next", headers=worker_headers()).json()
    assert claimed["repo_alias"] == "sidecar"
    assert claimed["agent"] == "codex"


def test_blank_repo_alias_is_rejected():
    c = client()
    assert create(c, repo_alias="   ").status_code == 422
