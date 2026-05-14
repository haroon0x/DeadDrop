# DeadDrop

DeadDrop is a local-first coding task inbox. Leave a coding task from your phone or browser, then a worker running on your PC picks it up, runs your local coding agent inside the configured repo, and sends back logs, status, summary, and git diff.

Tagline: **Leave a coding task. Come back to the diff.**

## Why It Exists

DeadDrop is for async coding work where your code, credentials, and tools stay on your machine. It is job-first: every task has status, logs, output, diff, and a receipt. It is not a chatbot, remote desktop, or general assistant.

## Architecture

```text
Phone/browser
  -> FastAPI server on Render
  -> SQLAlchemy job inbox
  <- Go worker polls with WORKER_TOKEN
  -> local repo + Gemini/mock/custom agent
  -> logs + summary + git diff back to server
```

The server never connects to your PC. The worker polls outbound.

## Quickstart

Start server with `uv`:

```bash
cd server
uv sync
export OWNER_TOKEN="$(openssl rand -base64 32)"
export WORKER_TOKEN="$(openssl rand -base64 32)"
uv run uvicorn app.main:app --reload
```

Open `http://localhost:8000/login`, enter your `OWNER_TOKEN`, and drop a task. The form asks for title, repo, and prompt. Worker routing stays internal.

Start worker:

```bash
cd worker
go run . run \
  --server http://localhost:8000 \
  --token "$WORKER_TOKEN" \
  --worker local \
  --manifest deaddrop.manifest.example.json \
  --agent mock
```

Try task: `Fix the failing test in the demo repo. Do not commit.`

For a clean demo diff, initialize the demo repo once:

```bash
cd examples/demo-repo
git init
git add .
git commit -m "demo baseline"
```

## Server Local Development

Environment:

- `OWNER_TOKEN`: browser/API token
- `WORKER_TOKEN`: worker token
- `DATABASE_URL`: SQLAlchemy database URL, defaults to `sqlite:///./deaddrop.db`
- `DEMO_MODE`: optional, defaults to enabled
- `SECURE_COOKIES`: set to `true` behind HTTPS in production

Run tests:

```bash
cd server
uv run pytest
```

## Worker Local Development

Build:

```bash
cd worker
go build -o deaddrop-worker .
```

Flags:

- `--server`
- `--token`
- `--worker`
- `--manifest`
- `--repo`
- `--repo-alias`
- `--agent` (`mock`, `gemini`, `custom`)
- `--poll-interval`
- `--agent-timeout`
- `--run-once`
- `--dry-run`
- `--command-template`

## Gemini CLI

Default Gemini mode runs unattended inside the selected repo:

```bash
gemini --skip-trust --approval-mode yolo --output-format text -p "{{prompt}}"
```

The worker prompt tells Gemini to avoid commits/pushes and finish with a `DEADDROP_RECEIPT` block. Content inside that block is free-form: Gemini can return an audit, answer a question about file lines, explain a blocker, or summarize code edits. Worker extracts that receipt into the job summary and captures `git diff` itself. If Gemini exits successfully but omits receipt markers, the worker marks the job failed.

Override if your install differs:

```bash
go run . run --server http://localhost:8000 --token "$WORKER_TOKEN" --worker local \
  --repo ../examples/demo-repo --repo-alias default --agent custom \
  --command-template 'npx @google/gemini-cli -p "{{prompt}}"'
```

## Workspace Manifest

The server never stores absolute local paths. The worker owns a trusted local workspace manifest and registers aliases with the server on startup. Each path must be a git worktree root; pointing an alias at a subdirectory of another repo is rejected so diffs cannot include unrelated files.

```json
{
  "repos": [
    {
      "alias": "demo",
      "name": "Demo repo",
      "path": "../examples/demo-repo"
    }
  ]
}
```

Run:

```bash
cd worker
go run . run --server http://localhost:8000 --token "$WORKER_TOKEN" --worker local \
  --manifest deaddrop.manifest.example.json --agent gemini
```

Phone UI can pick `repo_alias` from registered repos. The server stores only alias, display name, worker name, and timestamps. Only the local worker knows and maps the real workspace directory.

Use the manifest for any directory you want Gemini to work in:

```json
{
  "repos": [
    {
      "alias": "my-app",
      "name": "My app",
      "path": "/absolute/path/to/my-app"
    }
  ]
}
```

Gemini receives the user's task and DeadDrop's safety prompt, runs inside that workspace, and returns a receipt. The human reviews the captured diff before committing locally.

MVP uses one internal worker named `local`. The server stores it only to route queued jobs to the polling worker; users should not need to choose it.

## Deploying Server To Render

Use the root `render.yaml` blueprint for hosted deploys. It builds `server/Dockerfile`.

Set these Render environment variables:

- `OWNER_TOKEN`: generate locally with `openssl rand -base64 32`
- `WORKER_TOKEN`: generate locally with `openssl rand -base64 32`; use the same value when starting your local worker
- `DATABASE_URL`: Supabase Postgres connection string
- `SECURE_COOKIES=true`

Do not add `PORT` to `.env.example`. Render injects `PORT` at runtime, and `server/Dockerfile` falls back to `8000` for local Docker runs.

For production/demo, set `DATABASE_URL` to the Supabase Postgres connection string. Do not rely on Render local filesystem persistence. Local development can use SQLite with `DATABASE_URL=sqlite:///./deaddrop.db`.

## Security Model

DeadDrop uses bearer tokens. Browser/API requests need `OWNER_TOKEN`. Worker requests need `WORKER_TOKEN`.

Only run the worker against servers and tokens you trust. A queued task can cause your local agent command to edit files inside configured manifest repos. The worker ignores server-provided paths and uses only local manifest/flag paths.

DeadDrop does not commit by default. Gemini is explicitly told not to run `git commit` or `git push`; dashboard shows diff and receipt so human can accept/reject later.

Worker commands have an agent timeout (`--agent-timeout`, default 900 seconds). Timed-out agents are killed by process group, mark the job failed, and still upload logs/diff where possible.

For smoke checks or one-shot process managers, use `--run-once`. The worker registers repos, polls once, processes at most one job, reports the result, and exits.

## Limitations

- No OAuth
- No multi-user SaaS
- No billing
- No remote terminal
- No arbitrary repo switching from server
- No complex sandboxing
- Running job cancellation is not implemented
- Production requires Supabase Postgres through `DATABASE_URL`

## What I Would Build Next

- Supabase deployment guide and live smoke
- Running-job cancellation endpoint backed by worker-side abort signal
- Accept/reject flow that can create a local commit after human approval
- Artifact retention
- Per-worker token records
