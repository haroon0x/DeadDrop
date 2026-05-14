# DeadDrop MVP Spec

DeadDrop is a local-first mission queue for coding agents.

Tagline: **Leave a coding task. Come back to the diff.**

## Product Shape

A user opens a phone-friendly web dashboard, chooses a repo alias, writes a coding task, and submits it. A local worker running on the user's PC polls the hosted server, claims the job, runs Gemini CLI or a configured command inside the trusted local repo, streams logs back, captures `git diff`, and returns a receipt.

The smallest interesting version is:

```text
Phone/browser -> hosted FastAPI inbox -> local polling worker -> trusted repo + Gemini -> logs + summary + diff
```

## Taste And Scope

DeadDrop is not a chatbot, not remote desktop, not a general personal assistant, and not SaaS. It is a job-first developer tool: every task becomes a job with status, logs, summary, diff, and review notes.

Leave out for MVP:

- OAuth
- multi-user SaaS
- billing
- remote terminal
- inbound connection to local PC
- arbitrary server-chosen local paths
- repo indexing
- MCP
- complex sandboxing
- automatic commits by default

## Architecture

### Server

- FastAPI
- SQLAlchemy persistence: SQLite for local development, Supabase Postgres for production/demo through `DATABASE_URL`
- Jinja2 templates and minimal CSS
- Render-deployable
- Token auth
- Stores jobs, logs, worker metadata, and registered repo aliases

### Worker

- Go CLI
- Polls server with `WORKER_TOKEN`
- Runs on user's PC
- Uses local workspace manifest to map `repo_alias` to local paths
- Registers available repos with server on startup
- Executes agent inside selected repo only
- Enforces agent timeout so hung jobs fail cleanly
- Streams stdout/stderr/system logs
- Captures final `git diff`
- Requires receipt markers in agent output and fails successful commands that omit them
- Sends final status, summary, exit code, and diff

## Auth

Two tokens:

- `OWNER_TOKEN`: dashboard and owner APIs
- `WORKER_TOKEN`: worker polling, repo registration, logs, completion

Use bearer auth for APIs. Browser login stores owner token in an HTTP-only cookie.

## Worker Name

MVP has one worker named `local`. `worker_name` is internal routing metadata so `/api/worker/next?worker_name=local` can find jobs for that worker. The frontend should hide worker choice.

## Workspace Manifest

Server must not know absolute local paths. Worker owns a local manifest:

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

Worker registers aliases and display names with server. Dashboard uses these for repo dropdown. Jobs store `repo_alias`, not paths.

## Job Flow

1. User creates job from dashboard.
2. Server stores queued job with `repo_alias` and internal `worker_name=local`.
3. Worker polls `/api/worker/next`.
4. Server atomically marks oldest matching queued job as running.
5. Worker verifies repo exists and is git repo.
6. Worker runs `git status --short` and logs it.
7. Worker builds controlled prompt and runs mock/Gemini/custom command.
8. Worker streams logs.
9. Worker captures `git diff` and final status.
10. Worker marks job completed or failed.
11. Dashboard shows receipt, live logs, summary, and diff.

## Gemini Prompt Requirements

Prompt must tell Gemini:

- Work only inside current repo.
- Do not commit or push.
- Prefer smallest useful change.
- Do not delete unrelated files.
- Run smallest relevant test first when useful.
- Return final answer between `DEADDROP_RECEIPT` and `DEADDROP_RECEIPT_END`
- The content inside receipt markers is free-form and should match the user task. It can be an audit, a code-change summary, a direct answer to a file question, or a blocker explanation.

DeadDrop itself captures diff after Gemini exits.

Default Gemini command:

```bash
gemini --skip-trust --approval-mode yolo --output-format text -p "{{prompt}}"
```

The prompt requires a final receipt between `DEADDROP_RECEIPT` and `DEADDROP_RECEIPT_END`. Worker extracts that block into `final_summary`. If Gemini exits 0 but omits markers, worker marks the job failed because the dashboard would otherwise have no reliable receipt.

## Server Pages

- Dashboard: newest jobs, status, repo alias, created/updated times
- New job: title, repo dropdown, task prompt
- Job detail: prompt, receipt, live logs, summary, error, diff
- Demo page: safe fake completed job

## APIs

Owner:

- `POST /api/jobs`
- `GET /api/jobs`
- `GET /api/jobs/{job_id}`
- `GET /api/repos`
- `POST /api/jobs/{job_id}/cancel`

Worker:

- `POST /api/worker/register`
- `GET /api/worker/next?worker_name=local`
- `POST /api/worker/jobs/{job_id}/heartbeat`
- `POST /api/worker/jobs/{job_id}/logs`
- `POST /api/worker/jobs/{job_id}/complete`
- `POST /api/worker/jobs/{job_id}/fail`

## Demo Requirements

Mock mode must work without Gemini. Demo repo starts with failing test:

```python
def add(a, b):
    return a - b
```

Mock mode changes it to `return a + b`, runs tests, and returns diff.

Gemini mode should be supported and tested when model capacity is available. Gemini service capacity failures should not block the mock demo.

## Persistence Note

SQLite is okay for quick local development only. Production/demo uses Supabase Postgres through `DATABASE_URL`. Render local filesystem persistence must not be used for durable app data.
