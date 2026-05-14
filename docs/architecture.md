# Architecture

DeadDrop has one hosted server and one local worker.

```text
Browser -> FastAPI -> SQLite
                 ^
                 |
        Go worker polling
                 |
          local repo + agent
```

The worker owns the real repo path and command template. Jobs carry `repo_alias`, not paths. Server never opens an inbound connection to the developer machine.

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

Gemini runs inside selected local repo path. DeadDrop captures logs, final status, final summary, and `git diff`; it does not commit.

## Tokens

`OWNER_TOKEN` protects dashboard and owner APIs. `WORKER_TOKEN` protects polling, logs, and completion APIs.

## Persistence

Local SQLite is enough for the demo. Free Render filesystem persistence is not guaranteed after restart or deploy. Use Turso/libSQL for the smallest free SQLite-like hosted upgrade, or Supabase/Neon if moving to Postgres is acceptable.
