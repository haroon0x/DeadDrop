# DeadDrop

DeadDrop is a local-first coding task inbox. Leave a coding task from your phone or browser, then a worker running on your PC picks it up, runs your local coding agent inside the configured repo, and sends back logs, status, summary, and git diff.

Tagline: **Leave a coding task. Come back to the diff.**

## Why It Exists

DeadDrop is for async coding work where your code, credentials, and tools stay on your machine. It is job-first: every task has status, logs, output, diff, and a receipt. It is not a chatbot, remote desktop, or general assistant.

## Architecture

```text
Phone/browser
  -> FastAPI server on Render
  -> SQLite job inbox
  <- Go worker polls with WORKER_TOKEN
  -> local repo + Gemini/mock/custom agent
  -> logs + summary + git diff back to server
```

The server never connects to your PC. The worker polls outbound.

## Quickstart

Start server with `uv`:

```bash
cd deaddrop/server
uv venv
source .venv/bin/activate
uv pip install -r requirements.txt
export OWNER_TOKEN=owner_dev
export WORKER_TOKEN=worker_dev
uvicorn app.main:app --reload
```

Open `http://localhost:8000/login`, enter `owner_dev`, and drop a task. The form asks for title, repo, and prompt. Worker routing stays internal.

Start worker:

```bash
cd deaddrop/worker
go run . run \
  --server http://localhost:8000 \
  --token worker_dev \
  --worker local \
  --manifest deaddrop.manifest.example.json \
  --agent mock
```

Try task: `Fix the failing test in the demo repo. Do not commit.`

For a clean demo diff, initialize the demo repo once:

```bash
cd deaddrop/examples/demo-repo
git init
git add .
git commit -m "demo baseline"
```

## Server Local Development

Environment:

- `OWNER_TOKEN`: browser/API token
- `WORKER_TOKEN`: worker token
- `SQLITE_PATH`: optional DB path, defaults to `./deaddrop.db`
- `DEMO_MODE`: optional, defaults to enabled

Run tests:

```bash
cd deaddrop/server
uv run pytest
```

## Worker Local Development

Build:

```bash
cd deaddrop/worker
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
go run . run --server http://localhost:8000 --token worker_dev --worker local \
  --repo ../examples/demo-repo --repo-alias default --agent custom \
  --command-template 'npx @google/gemini-cli -p "{{prompt}}"'
```

## Workspace Manifest

The server never stores absolute local paths. The worker owns a trusted local workspace manifest and registers aliases with the server on startup:

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
cd deaddrop/worker
go run . run --server http://localhost:8000 --token worker_dev --worker local \
  --manifest deaddrop.manifest.example.json --agent gemini
```

Phone UI can pick `repo_alias` from registered repos. Only local worker maps alias to path.

MVP uses one internal worker named `local`. The server stores it only to route queued jobs to the polling worker; users should not need to choose it.

## Deploying Server To Render

Create a Web Service from this repo and set:

- Root directory: `deaddrop/server`
- Build command: `pip install -r requirements.txt`
- Start command: `uvicorn app.main:app --host 0.0.0.0 --port $PORT`
- Env vars: `OWNER_TOKEN`, `WORKER_TOKEN`, `SQLITE_PATH=/var/data/deaddrop.db`

SQLite on Render is acceptable for MVP demos. Render free filesystems are not durable across restarts/redeploys, even if the worker polls continuously. Use a persistent disk where available, or swap the server DB layer to a free hosted database. Best free SQLite-shaped option: Turso/libSQL. Best free Postgres-shaped options: Supabase or Neon.

## Security Model

DeadDrop uses bearer tokens. Browser/API requests need `OWNER_TOKEN`. Worker requests need `WORKER_TOKEN`.

Only run the worker against servers and tokens you trust. A queued task can cause your local agent command to edit files inside configured manifest repos. The worker ignores server-provided paths and uses only local manifest/flag paths.

DeadDrop does not commit by default. Gemini is explicitly told not to run `git commit` or `git push`; dashboard shows diff and receipt so human can accept/reject later.

Worker commands have an agent timeout (`--agent-timeout`, default 900 seconds). Timed-out agents mark the job failed and still upload logs/diff.

## Limitations

- No OAuth
- No multi-user SaaS
- No billing
- No remote terminal
- No arbitrary repo switching from server
- No complex sandboxing
- Running job cancellation is not implemented
- SQLite is not a durable production database unless backed by persistent disk

## What I Would Build Next

- Accept/reject flow that can create a local commit after human approval
- Better cancellation via process groups
- Log pagination and artifact retention
- Per-worker token records
- Webhook or push notifications
- Durable database for hosted production use
