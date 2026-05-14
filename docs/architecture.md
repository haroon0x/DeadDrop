# Architecture

DeadDrop has one hosted server and one local worker.

```text
Browser -> FastAPI -> SQLAlchemy DB
                 ^
                 |
        Go worker polling
                 |
          local workspace + agent
```

The worker owns the real workspace path and command template. Jobs carry `repo_alias`, not paths. Server never opens an inbound connection to the developer machine.

Workspace aliases may resolve to any existing directory. If the workspace is inside a git worktree, DeadDrop captures `git status -- .` and `git diff -- .` scoped to that directory. If not, git capture is skipped.

## Workspace Manifest

Worker can start with local JSON manifest:

```json
{
  "repos": [
    { "alias": "demo", "name": "Demo repo", "path": "../examples/demo-repo" }
  ]
}
```

On startup, worker registers aliases and display names with `/api/worker/register`. Server stores only `worker_name`, `repo_alias`, display name, and timestamps.

Gemini runs inside the configured local workspace path. DeadDrop leaves autonomous code work to Gemini, but keeps the transport and audit loop deterministic: job claim, bounded command runtime, streamed logs, receipt marker extraction, final status, and best-effort scoped `git diff`. DeadDrop does not commit.

## Security and Hardening

- **Authentication**: `OWNER_TOKEN` protects the dashboard and owner APIs. `WORKER_TOKEN` protects polling, logs, and completion APIs. All comparisons use constant-time `compare_digest`.
- **CSRF Protection**: Dashboard state-changing operations are protected by a Double Submit Cookie CSRF mechanism.
- **Secure Cookies**: Production cookies are strictly `HttpOnly`, `SameSite=Lax`, and `Secure` (defaulting to True).
- **Worker Isolation**: The worker refuses to run as `root` and validates configured paths are existing directories.
- **Outbound Only**: The server never connects to the developer machine. All communication is initiated by the worker or the browser via HTTPS.

## Persistence

Local SQLite is allowed for quick development with `DATABASE_URL=sqlite:///./deaddrop.db`.

Production/demo must use Supabase Postgres through `DATABASE_URL`. Do not depend on Render local filesystem persistence.
