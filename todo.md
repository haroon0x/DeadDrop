# DeadDrop Todo

## Fresh Agent Handoff
- Read this file first, then `spec.md`, then `README.md`, then `docs/fresh-agent-handoff.md`.
- Current implementation lives at repo root: `server/`, `worker/`, and `examples/`.
- Server uses FastAPI + SQLAlchemy + Jinja templates. Local dev defaults to SQLite; production/demo uses Supabase Postgres via `DATABASE_URL`. Run with `cd server && uv run uvicorn app.main:app --reload`.
- Worker uses Go. Preferred run: `cd worker && go run . run --server http://localhost:8000 --token "$WORKER_TOKEN" --worker local --manifest deaddrop.manifest.example.json --agent mock`.
- Worker manifest registers repo aliases with server. Server stores aliases/display names only, not absolute paths. Phone UI dropdown reads registered repos.
- MVP has one internal worker named `local`; frontend intentionally hides worker choice.
- Gemini CLI exists on this machine: `gemini --version` returned `0.41.2`. A direct smoke command returned `GEMINI_OK`, with extension warnings and one transient 429 retry.
- Full worker Gemini test was attempted. Gemini emitted model-capacity 429s and tool permission warnings, then external `timeout 120` killed worker, leaving test job `running` in `/tmp/deaddrop_smoke.db`. Quote handling for `{{prompt}}` was fixed after this.
- Mock smoke initially showed blank log lines can produce 422 responses. Worker now skips blank log content before posting.
- Gemini default command is `gemini --skip-trust --approval-mode yolo --output-format text -p "{{prompt}}"`.
- Worker extracts final summaries from `DEADDROP_RECEIPT` / `DEADDROP_RECEIPT_END` markers and has `--agent-timeout`.
- Worker has `--run-once` for smoke tests and kills the spawned process group on timeout.
- Receipt content is intentionally free-form; only markers are strict. Missing markers on exit 0 fail the job.
- Render local filesystem persistence is not used for production/demo. Set `DATABASE_URL` to Supabase Postgres. SQLite remains allowed only for quick local tests.
- Demo repo has its own git repo initialized for clean diffs. Reset demo by editing `examples/demo-repo/app.py` back to `return a - b` and committing/resetting as needed.
- Worker now rejects manifest paths that are subdirectories of a larger git repo. Each workspace alias should point at the git worktree root.
- Avoid long comments; keep code functional and small.

## Spec Review
- [x] Read `spec.md`
- [x] Identify MVP risks: DB persistence, local command execution trust boundary, running cancellation scope

## Build
- [x] Scaffold monorepo structure
- [x] Build FastAPI server APIs
- [x] Build SQLAlchemy persistence with local SQLite default
- [x] Build mobile-friendly Jinja pages
- [x] Build Go worker CLI
- [x] Add mock agent mode with deterministic demo fix
- [x] Add Gemini/custom command modes
- [x] Add demo repo
- [x] Add docs and README
- [x] Add minimal server tests
- [x] Add markdown handoff notes for fresh agents
- [x] Add workspace manifest support
- [x] Add worker repo registration API
- [x] Add repo dropdown UI from registered aliases
- [x] Strengthen Gemini prompt: no commit/push, return audit receipt
- [x] Hide worker field from new job UI
- [x] Replace stale original spec with concise current MVP spec
- [x] Add worker-side agent timeout
- [x] Harness Gemini prompt with receipt markers and summary extraction
- [x] Make receipt content free-form while enforcing wrapper markers
- [x] Add public landing page and polished dashboard styling
- [x] Add fresh-agent handoff doc
- [x] Enforce worker repo path is git worktree root

## Verify
- [x] Run server tests
- [x] Run Go tests/build
- [x] Run end-to-end mock smoke check
- [x] Run direct Gemini CLI smoke check
- [x] Rerun end-to-end mock smoke with manifest path
- [x] Run full prompt workflow: register manifest repo, create job, worker claims, mock fixes repo, receipt returns logs/diff/no-commit summary
- [x] Check `/jobs/new` UI shows repo dropdown and hides worker choice
- [x] Verify receipt extraction with mock `DEADDROP_RECEIPT`
- [x] Verify custom agent timeout marks job failed with exit code 124
- [x] Verify zero-exit custom agent without receipt markers fails with exit code 2
- [ ] Run full worker Gemini mode against demo repo after Gemini capacity is available

## Next
- [x] Add persistent auth session UI instead of token query/localStorage bridge
- [x] Add queued-only cancellation in UI
- [x] Add log pagination for large jobs
- [x] Add production DB adapter: SQLAlchemy with `DATABASE_URL` for Supabase Postgres
- [x] Add process-group timeout kill so hung agent child processes do not survive
- [x] Add worker run-once mode for smoke tests and one-shot deployments
- [x] Add `/healthz` and `/readyz` endpoints for hosted monitoring
- [ ] Add running-job cancellation protocol from server to worker
- [ ] Add deployment guide with exact Render + Supabase settings
- [ ] Add accept/reject action after diff review, only if MVP demo still feels incomplete
