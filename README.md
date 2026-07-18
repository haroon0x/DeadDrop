# DeadDrop

Leave a coding task. Come back to a verified patch.

DeadDrop is a self-hosted coding task inbox. Its web server stores jobs and receipts; a worker on your machine polls outbound, runs your local coding agent in an isolated Git worktree, verifies the result, and returns logs plus a reviewable patch.

This repository is an open-source project for individual developers and small trusted teams. It is not a hosted multi-tenant SaaS, remote shell, or automatic merge service.

## What works

- Browser task creation with persistent token login and CSRF protection
- Outbound-only worker connection; no inbound access to the developer machine
- Gemini CLI, deterministic mock, and custom command modes
- Per-job detached Git worktrees that preserve dirty and untracked source files
- Complete binary Git patches relative to the source repository's `HEAD`
- Authenticated patch downloads with the exact baseline commit and safe apply steps
- Worker-observed changed files and configured verification results
- Live log batching, request timeouts, retries, and durable result replay
- Job attempts, leases, heartbeats, stale-worker recovery, and running cancellation
- SQLite for local development and PostgreSQL for durable deployment
- Alembic migrations applied automatically at server startup
- Linux, macOS, and Windows worker binaries on tagged releases

## Architecture

```text
Browser
  |
  v
FastAPI server ---- PostgreSQL
  ^                    jobs, attempts, logs, receipts
  |
  | outbound HTTPS polling
  |
Go worker ---- detached Git worktree ---- coding agent
                 |                         Gemini/custom/mock
                 v
            verification + binary patch
```

The server stores repository aliases, never local absolute paths. The worker owns the alias-to-path mapping and runs with the same operating-system permissions as the local user.

See [Architecture](docs/architecture.md) for lifecycle, isolation, recovery, and trust boundaries.

## Quickstart

Requirements:

- Docker with Compose
- Git
- A DeadDrop worker binary or Go 1.22+
- Gemini CLI only when using `--agent gemini`

Start a durable local server:

```bash
export OWNER_TOKEN="$(openssl rand -hex 32)"
export WORKER_TOKEN="$(openssl rand -hex 32)"
export POSTGRES_PASSWORD="$(openssl rand -hex 32)"
docker compose up -d --build
```

Open `http://localhost:8000/login` and enter `OWNER_TOKEN`.

Build the worker from source:

```bash
cd worker
go build -o deaddrop-worker .
```

Tagged releases also publish worker binaries and `SHA256SUMS` on the [GitHub Releases page](https://github.com/haroon0x/DeadDrop/releases).

Create a manifest for the Git repository or committed subdirectory the agent may edit:

```bash
./deaddrop-worker init \
  --repo /absolute/path/to/project \
  --verify "go test ./..."
```

Start the worker:

```bash
./deaddrop-worker run \
  --server http://localhost:8000 \
  --token "$WORKER_TOKEN" \
  --manifest deaddrop.manifest.json \
  --agent gemini
```

Create a task in the browser. The default route is worker `local`, repository alias `default`.

For a deterministic first run, use `--agent mock` with `examples/demo-repo` and the task `Fix app.py so add returns a + b`.

## Apply a returned patch

Open the completed job and select **Download .patch**. The receipt shows the exact baseline commit used by the worker.

Commit or stash unrelated local changes, change into the configured workspace, and inspect the patch before applying it:

```bash
git apply --stat /path/to/deaddrop-job-42.patch
git apply --check /path/to/deaddrop-job-42.patch
git apply /path/to/deaddrop-job-42.patch
```

Run the project verification again, review `git diff`, and commit only when the result is acceptable. If the branch has moved since the displayed baseline, inspect the patch first and use `git apply --3way` only when you are prepared to resolve conflicts.

Authenticated API clients can download the same artifact from `GET /api/jobs/{job_id}/patch` with the owner bearer token.

## Workspace configuration

`deaddrop-worker init` writes this shape:

```json
{
  "repos": [
    {
      "alias": "default",
      "name": "my-project",
      "path": "/absolute/path/to/my-project",
      "verify": ["go test ./...", "go vet ./..."]
    }
  ]
}
```

Each path must be a Git worktree root or a committed directory inside one. DeadDrop creates a detached worktree at the source repository's current `HEAD`; local dirty and untracked files are not copied into the job. The returned patch and changed-file paths are scoped to the configured directory.

For one workspace without a manifest:

```bash
./deaddrop-worker run \
  --server http://localhost:8000 \
  --token "$WORKER_TOKEN" \
  --repo /absolute/path/to/project \
  --repo-alias default \
  --verify "python -m pytest" \
  --agent custom \
  --command-template 'your-agent "{{prompt}}"'
```

Repeat `--verify` for multiple commands. Verification commands run after the agent. Any failed command fails the job. Receipt changed files and verification results come from worker-observed evidence, not agent claims.

## Worker commands

```text
deaddrop-worker version
deaddrop-worker init --repo PATH [--name NAME] [--output FILE] [--verify COMMAND]
deaddrop-worker run --server URL --token TOKEN (--manifest FILE | --repo PATH) [flags]
```

Run flags:

- `--worker`: worker route, default `local`
- `--repo-alias`: single-repository alias, default `default`
- `--agent`: `gemini`, `mock`, or `custom`
- `--command-template`: required for custom mode; supports `{{prompt}}`, `{{task}}`, and `{{repo}}`
- `--verify`: trusted local verification command; repeatable
- `--poll-interval`: idle poll interval in seconds, default `3`
- `--agent-timeout`: agent and verification timeout in seconds, default `900`
- `--run-once`: claim at most one job and exit
- `--dry-run`: render agent behavior without executing it

## Reliability model

Every claim receives a unique attempt ID and lease. The worker renews the lease every five seconds. If a worker disappears, the next poll recovers the expired job and creates a new attempt; results from the stale attempt are rejected.

Completion and failure delivery are idempotent. Retryable requests use bounded retries. If final delivery still fails, the worker stores the result under its user configuration directory and replays it before claiming more work.

Running cancellation is cooperative through heartbeats but forceful at the local process boundary: the worker observes the request, cancels the command context, kills the command process group, captures any partial patch, and acknowledges the cancelled attempt.

## Development

Server:

```bash
cd server
uv sync --frozen
export OWNER_TOKEN=owner-dev
export WORKER_TOKEN=worker-dev
export DATABASE_URL=sqlite:///./deaddrop.db
export SECURE_COOKIES=false
uv run uvicorn app.main:app --reload
```

Worker:

```bash
cd worker
go test ./...
go vet ./...
```

Full workflow:

```bash
server/.venv/bin/python -m pytest -q e2e
```

Server tests and migration drift check:

```bash
cd server
uv run pytest -q
uv run alembic check
```

## Security

A task can cause the configured coding agent to execute commands with the worker user's permissions. Use a dedicated non-root user, configure only repositories you are willing to expose to that agent, keep the DeadDrop control repository outside agent workspaces, and connect only to a server and worker token you trust.

DeadDrop isolates Git state; it is not an operating-system sandbox. See [Security Policy](SECURITY.md) and [Architecture](docs/architecture.md#trust-boundaries).

## Project scope

DeadDrop intentionally does not provide multi-user accounts, billing, arbitrary remote shell access, automatic commits, automatic pushes, or automatic merges. The human reviews and applies the returned patch.

## Documentation

- [Architecture](docs/architecture.md)
- [Deployment](docs/deployment.md)
- [Worker service](docs/worker-service.md)
- [Release process](docs/releases.md)
- [Contributing](CONTRIBUTING.md)
- [Support](SUPPORT.md)

DeadDrop is licensed under [GPL-3.0](LICENSE).
