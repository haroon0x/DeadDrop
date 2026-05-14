# Architecture

DeadDrop has one hosted server and one local worker.

```text
Browser -> FastAPI -> SQLAlchemy DB
                 ^
                 |
        Go worker polling
                 |
          local repo + agent
```

The worker owns the real repo path and command template. Jobs carry `repo_alias`, not paths. Server never opens an inbound connection to the developer machine.

Workspace aliases must resolve to git worktree roots. This is deliberate: DeadDrop captures `git diff` after the agent exits, so allowing an alias to point at a nested directory inside a larger repo could leak unrelated parent-repo changes into the receipt.

## Workspace Manifest

Worker can start with local JSON manifest:

```json
{
  "repos": [
    { "alias": "demo", "name": "Demo repo", "path": "../examples/demo-repo" }
  ]
}
```

On startup, worker registers aliases and display names with `/api/worker/register`. Server stores only `worker_name`, `repo_alias`, display name, and timestamps. Phone UI uses that list as dropdown.

Gemini runs inside the selected local repo path. DeadDrop leaves autonomous code work to Gemini, but keeps the transport and audit loop deterministic: job claim, bounded command runtime, streamed logs, receipt marker extraction, final status, and captured `git diff`. DeadDrop does not commit.

## Tokens

`OWNER_TOKEN` protects dashboard and owner APIs. `WORKER_TOKEN` protects polling, logs, and completion APIs.

## Persistence

Local SQLite is allowed for quick development with `DATABASE_URL=sqlite:///./deaddrop.db`.

Production/demo must use Supabase Postgres through `DATABASE_URL`. Do not depend on Render local filesystem persistence.
