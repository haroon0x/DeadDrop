import json
import os
import socket
import subprocess
import sys
import urllib.request
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SERVER = ROOT / "server"
WORKER = ROOT / "worker"


def run(*args: str, cwd: Path) -> None:
    subprocess.run(args, cwd=cwd, check=True, capture_output=True, text=True)


def request(url: str, token: str, body: dict | None = None) -> dict:
    data = json.dumps(body).encode() if body is not None else None
    method = "POST" if body is not None else "GET"
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
        method=method,
    )
    with urllib.request.urlopen(req) as response:
        return json.load(response)


def download(url: str, token: str) -> tuple[bytes, dict[str, str]]:
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
    with urllib.request.urlopen(req) as response:
        return response.read(), {key.lower(): value for key, value in response.headers.items()}


def test_server_worker_workflow_isolated_and_verified(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    run("git", "init", "-q", cwd=repo)
    run("git", "config", "user.email", "e2e@example.com", cwd=repo)
    run("git", "config", "user.name", "E2E", cwd=repo)
    (repo / "app.py").write_text("def add(a, b):\n    return a - b\n", encoding="utf-8")
    (repo / "test_app.py").write_text("from app import add\n\ndef test_add():\n    assert add(2, 3) == 5\n", encoding="utf-8")
    run("git", "add", "app.py", "test_app.py", cwd=repo)
    run("git", "commit", "-qm", "baseline", cwd=repo)
    baseline = subprocess.run(
        ["git", "rev-parse", "HEAD"], cwd=repo, check=True, capture_output=True, text=True
    ).stdout.strip()
    (repo / "app.py").write_text("def add(a, b):\n    return a - b\n\nLOCAL = True\n", encoding="utf-8")
    (repo / "local.txt").write_text("untracked\n", encoding="utf-8")

    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    base = f"http://127.0.0.1:{port}"
    env = os.environ.copy()
    env.update(
        {
            "OWNER_TOKEN": "owner-e2e",
            "WORKER_TOKEN": "worker-e2e",
            "DATABASE_URL": f"sqlite:///{tmp_path / 'deaddrop.db'}",
            "SECURE_COOKIES": "false",
        }
    )
    server = subprocess.Popen(
        [sys.executable, "-m", "uvicorn", "app.main:app", "--host", "127.0.0.1", "--port", str(port)],
        cwd=SERVER,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    cancel_worker = None
    try:
        for line in server.stdout:
            if "Application startup complete." in line:
                break
            if "Application startup failed" in line or "address already in use" in line:
                raise RuntimeError(line.strip())
        else:
            raise RuntimeError("server exited before readiness")

        job = request(
            f"{base}/api/jobs",
            "owner-e2e",
            {"title": "Fix add", "prompt": "Fix app.py so add returns a + b"},
        )
        worker_env = os.environ.copy()
        worker_env["PATH"] = str(SERVER / ".venv" / "bin") + os.pathsep + worker_env["PATH"]
        subprocess.run(
            [
                "go",
                "run",
                ".",
                "run",
                "--server",
                base,
                "--token",
                "worker-e2e",
                "--worker",
                "local",
                "--repo",
                str(repo),
                "--repo-alias",
                "default",
                "--agent",
                "mock",
                "--verify",
                "python -m pytest",
                "--run-once",
            ],
            cwd=WORKER,
            env=worker_env,
            check=True,
            capture_output=True,
            text=True,
        )
        result = request(f"{base}/api/jobs/{job['id']}", "owner-e2e")
        assert result["status"] == "completed"
        assert result["attempt_number"] == 1
        assert result["baseline_commit"] == baseline
        assert result["receipt"]["changed_files"] == ["app.py"]
        assert result["receipt"]["verification"] == [
            {
                "command": "python -m pytest",
                "status": "passed",
                "summary": "Worker observed exit code 0",
            }
        ]
        assert "return a + b" in result["git_diff"]
        patch, headers = download(f"{base}/api/jobs/{job['id']}/patch", "owner-e2e")
        assert headers["content-disposition"] == f'attachment; filename="deaddrop-job-{job["id"]}.patch"'
        patch_path = tmp_path / f"deaddrop-job-{job['id']}.patch"
        patch_path.write_bytes(patch)
        applied = tmp_path / "applied"
        run("git", "clone", "-q", str(repo), str(applied), cwd=tmp_path)
        run("git", "apply", "--check", str(patch_path), cwd=applied)
        run("git", "apply", str(patch_path), cwd=applied)
        run(sys.executable, "-m", "pytest", "-q", cwd=applied)
        assert "LOCAL = True" in (repo / "app.py").read_text(encoding="utf-8")
        source_status = subprocess.run(
            ["git", "status", "--porcelain"], cwd=repo, check=True, capture_output=True, text=True
        ).stdout
        assert " M app.py" in source_status
        assert "?? local.txt" in source_status
        worktrees = subprocess.run(
            ["git", "worktree", "list", "--porcelain"], cwd=repo, check=True, capture_output=True, text=True
        ).stdout
        assert worktrees.count("worktree ") == 1

        cancel_job = request(
            f"{base}/api/jobs",
            "owner-e2e",
            {"title": "Cancel agent", "prompt": "Wait until cancelled"},
        )
        cancel_worker = subprocess.Popen(
            [
                "go",
                "run",
                ".",
                "run",
                "--server",
                base,
                "--token",
                "worker-e2e",
                "--worker",
                "local",
                "--repo",
                str(repo),
                "--repo-alias",
                "default",
                "--agent",
                "custom",
                "--command-template",
                "echo AGENT_STARTED; sleep 60",
                "--run-once",
            ],
            cwd=WORKER,
            env=worker_env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        for line in cancel_worker.stdout:
            if "AGENT_STARTED" in line and "stdout:" in line:
                break
        else:
            raise RuntimeError("worker exited before agent start")
        requested = request(f"{base}/api/jobs/{cancel_job['id']}/cancel", "owner-e2e", {})
        assert requested["cancel_requested_at"] is not None
        cancel_worker.communicate(timeout=20)
        assert cancel_worker.returncode == 0
        cancelled = request(f"{base}/api/jobs/{cancel_job['id']}", "owner-e2e")
        assert cancelled["status"] == "cancelled"
        assert cancelled["exit_code"] == 130
        assert subprocess.run(
            ["git", "status", "--porcelain"], cwd=repo, check=True, capture_output=True, text=True
        ).stdout == source_status
    finally:
        if cancel_worker is not None and cancel_worker.poll() is None:
            cancel_worker.terminate()
            try:
                cancel_worker.wait(timeout=5)
            except subprocess.TimeoutExpired:
                cancel_worker.kill()
                cancel_worker.wait()
        server.terminate()
        try:
            server.wait(timeout=5)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait()
