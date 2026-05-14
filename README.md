<p align="center">
  
</p>

<h1 align="center">DeadDrop</h1>

<p align="center">
  Your local coding agent, reachable from anywhere.
</p>

<p align="center">
  <a href="https://deaddrop-dpk8.onrender.com/">Demo</a>
  ·
  <a href="#features">Features</a>
  ·
  <a href="#quickstart">Quickstart</a>
  ·
  <a href="#how-it-works">How it works</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/local--first-agent_queue-BBFF00?style=for-the-badge&labelColor=101010" />
  <img src="https://img.shields.io/badge/phone_to_repo-ready-BBFF00?style=for-the-badge&labelColor=101010" />
  <img src="https://img.shields.io/badge/diff-ready-BBFF00?style=for-the-badge&labelColor=101010" />
</p>
<img width="1916" height="821" alt="image" src="https://github.com/user-attachments/assets/4e52f0cf-c971-4fb2-b277-5f3af4b6472e" />


DeadDrop is a local-first coding task inbox. Leave a coding task from your phone or browser, then a worker running on your PC picks it up, runs your local coding agent inside the configured repo, and sends back logs, status, summary, and git diff.

### DEMO

https://github.com/user-attachments/assets/7d771eeb-e6fe-436a-b7bf-0814b9b408a2


Tagline: **Leave a coding task. Come back to the diff.**

[![Deploy to Render](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy)

## Deploy Your Own in 5 Minutes

DeadDrop is designed for sovereign use. You can host your own private task inbox for free on Render and Supabase.

1.  **Fork this repo** to your GitHub account.
2.  **Click the button above** (or create a "Blueprint" on Render pointing to your fork).
3.  **Follow the [Detailed Deployment Guide](docs/deployment.md)** to set up your free Supabase database and Render environment variables.
4.  **Start your local worker** using your `WORKER_TOKEN` and your Render URL.

---

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
export OWNER_TOKEN="$(openssl rand -base64 32)" # Generates a secure 32-byte random secret
export WORKER_TOKEN="$(openssl rand -base64 32)"
uv run uvicorn app.main:app --reload
```

Open `http://localhost:8000/login`, enter your `OWNER_TOKEN`, and drop a task. The form asks for title and prompt only. Jobs route to the fixed local worker workspace (`local` / `default`).

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

Default Gemini mode runs unattended inside the configured workspace:

```bash
gemini --skip-trust --approval-mode yolo --output-format json -p "{{prompt}}"
```

The worker prompt tells Gemini to avoid commits/pushes and finish with a structured `DEADDROP_RECEIPT_JSON` block. Worker validates that JSON, renders it as result/changed-files/verification/blocker sections, and still streams terminal stdout/stderr into collapsible live logs. It captures `git diff` when the workspace is inside a git worktree. If Gemini exits successfully but omits receipt markers, the worker marks the job failed.

Override if your install differs:

```bash
go run . run --server http://localhost:8000 --token "$WORKER_TOKEN" --worker local \
  --repo ../examples/demo-repo --repo-alias default --agent custom \
  --command-template 'npx @google/gemini-cli -p "{{prompt}}"'
```

## Workspace Manifest

The server never stores absolute local paths. The worker owns the trusted workspace path. Browser-created jobs always route to `worker_name=local` and `repo_alias=default`, so start the worker with one fixed workspace using alias `default`. The path can be any existing directory. If it is inside a git worktree, DeadDrop captures `git status -- .` and `git diff -- .` scoped to that workspace.

```json
{
  "repos": [
    {
      "alias": "default",
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

The phone UI does not select repositories. The server stores the default alias and worker name, while only the local worker knows and maps the real workspace directory.

Use the manifest for the directory you want Gemini to work in:

```json
{
  "repos": [
    {
      "alias": "default",
      "name": "My app",
      "path": "/absolute/path/to/my-app"
    }
  ]
}
```

Gemini receives the user's task and DeadDrop's safety prompt, runs inside that workspace, and returns a receipt. The human reviews the captured diff before committing locally.

MVP uses one internal worker named `local`. The server stores it only to route queued jobs to the polling worker; users should not need to choose it.

## Deploying Server To Render

Use the root `render.yaml` blueprint for hosted deploys. It builds a secure Docker container using the provided `server/Dockerfile`.

### Configuration Checklist:

1.  **Tokens**: Generate two unique, long random strings (e.g., `openssl rand -base64 32`).
    *   `OWNER_TOKEN`: Used to log into the web dashboard.
    *   `WORKER_TOKEN`: Used by your local Go worker to authenticate.
2.  **Database**: Create a free project on Supabase.
    *   Go to **Settings > Database > Connection Pooler**.
    *   Set mode to **Transaction**.
    *   Copy the **Pooled Connection String** (Port 6543).
3.  **Render Environment**:
    *   `DATABASE_URL`: Set to your pooled Supabase URL.
    *   `SECURE_COOKIES`: Set to `true` (enforces HTTPS/SSL).
    *   `OWNER_TOKEN` & `WORKER_TOKEN`: Set to the strings you generated.

Once deployed, your server is a secure, private inbox for your coding tasks.

## Security Model

DeadDrop is built with a "Trust but Verify" security model:

1.  **Authentication**: All requests require a `Bearer` token.
    -   Browser/API requests use `OWNER_TOKEN`.
    -   Worker polling requests use `WORKER_TOKEN`.
    -   Comparisons use constant-time checks (`compare_digest`) to prevent timing attacks.
2.  **CSRF Protection**: All dashboard forms use the Double Submit Cookie pattern with a cryptographically secure token to prevent Cross-Site Request Forgery.
3.  **Transit Security (MITM)**: 
    -   In production, `SECURE_COOKIES` defaults to `true`, forcing cookies to be sent only over HTTPS. 
    -   Use the Supabase Connection Pooler (Port 6543) for production database connections to ensure IPv4 compatibility and encrypted transit.
4.  **Worker Hardening**:
    -   **No Root**: The worker refuses to run as `root` (UID 0) to minimize impact if a task is malicious.
    -   **Path Isolation**: The worker only operates within directories explicitly defined by `--repo` or your local manifest. Git status and diff capture are scoped to that workspace path.
    -   **No Auto-Commit**: Gemini is explicitly instructed to never `git commit` or `git push`. You review the diff on the dashboard and apply it manually.
5.  **Agent Safety**: Worker commands have an agent timeout (`--agent-timeout`, default 900 seconds). Timed-out agents are killed by process group, mark the job failed, and still upload logs/diff where possible.

Only run the worker against servers and tokens you trust. A queued task can cause your local agent command to edit files inside configured manifest repos.

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
