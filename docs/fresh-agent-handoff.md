# Fresh Agent Handoff

Read in this order:

1. `todo.md`
2. `spec.md`
3. `README.md`
4. `docs/architecture.md`
5. `worker/README.md`

## Current Shape

DeadDrop is a FastAPI server plus a Go polling worker.

- Server code: `server/app/`
- Worker code: `worker/`
- Demo repo: `examples/demo-repo/`
- MVP worker name: `local`
- Default agent: Gemini CLI
- Mock agent: deterministic demo mode that fixes `examples/demo-repo/app.py`

The server stores jobs, logs, repo aliases, display names, summaries, and diffs through SQLAlchemy. Local dev can use SQLite. Production/demo uses Supabase Postgres through `DATABASE_URL`; do not rely on Render local filesystem persistence. The server must not store local absolute paths. The worker owns local path trust through `worker/deaddrop.manifest.example.json` or `--repo`.

## Run Locally

Terminal 1:

```bash
cd server
export OWNER_TOKEN=owner_dev
export WORKER_TOKEN=worker_dev
uv run uvicorn app.main:app --reload
```

Terminal 2:

```bash
cd worker
go run . run \
  --server http://localhost:8000 \
  --token worker_dev \
  --worker local \
  --manifest deaddrop.manifest.example.json \
  --agent mock
```

Open `http://localhost:8000`, log in with `owner_dev`, create a job for repo `demo`.

## Worker Contract

The worker must:

- Register manifest repo aliases on startup.
- Claim only jobs routed to its worker name.
- Resolve `repo_alias` locally.
- Require configured repo paths to be git worktree roots.
- Run the agent inside the selected repo.
- Stream non-empty logs only.
- Enforce `--agent-timeout`.
- Kill the spawned process group on timeout.
- Capture `git diff` after agent exit.
- Require `DEADDROP_RECEIPT` and `DEADDROP_RECEIPT_END` when agent exits `0`.
- Complete or fail every claimed job.
- Never commit or push by default.

## Production Concerns

Do not call this production-ready until these are addressed:

- Supabase Postgres `DATABASE_URL` must be configured on hosted deployments.
- Running-job cancellation that sends an abort signal to the worker.
- Deployment docs verified on the chosen host.

## Minimal Verification

Run:

```bash
cd server && pytest -q
cd worker && go test ./...
```

Then do one smoke run: create job through API/UI, start mock worker, confirm job completes and `git_diff` only contains demo repo files.
