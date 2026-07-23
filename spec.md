# DeadDrop specification

> **What this document is.** A description of what the software does today and
> what it promises. The promises are not enforced by this file. They are
> enforced by `server/tests/test_invariants.py`, which tries to break each one.
> If this document and that suite ever disagree, the suite is right.
>
> **What this document is not.** A list of things DeadDrop is forbidden to
> become. Earlier revisions carried non-goals like "not a remote desktop" and
> "no arbitrary remote shell access." Those were positioning statements, not
> engineering constraints, and they quietly pre-rejected directions that
> deserved to be argued on their merits. They have been removed. Decide
> direction by reasoning about the artifact, not by consulting this file.

## What it does

An owner types a coding task into a web queue. A worker process on the
developer's own machine polls that queue outbound, runs a coding-agent CLI
against a throwaway copy of the repository, verifies the result with the
developer's own commands, and returns logs, a receipt, and a patch. The human
decides whether any of it lands.

## The promises

Each of these is a named test in `server/tests/test_invariants.py`.

| Promise | Why it matters |
| --- | --- |
| The server stores a repo alias, never an absolute path | The control plane never learns your filesystem layout |
| Every worker interaction is worker-initiated | Nothing needs to reach into the developer's machine |
| One claim owns a job; a superseded attempt cannot write logs or results | A worker that vanished and came back cannot corrupt newer work |
| Repeated terminal delivery is idempotent | A retry after a network failure is safe |
| Status comes from the observed exit code, not the agent's claim | An agent cannot declare its own success |
| No route applies, commits, pushes, or merges | The human stays the only writer to the repository |
| Owners can only route to repositories a worker registered | The worker's manifest is the trust boundary |

Two further promises are enforced in the worker rather than the server, and are
covered by `worker/runner_test.go`:

- Jobs run in a detached worktree at the source commit, so uncommitted and
  untracked files in the developer's checkout are never touched or captured.
- Changed files and the patch come from `git diff` against the job baseline.

## Server

- FastAPI, Jinja2, SQLAlchemy Core
- SQLite for development, PostgreSQL for durable deployment
- Versioned Alembic migrations
- Separate owner and worker bearer tokens
- HTTP-only owner session cookie, CSRF-protected forms
- Jobs, attempts, logs, worker registrations, receipts, patches
- Health and readiness endpoints

## Worker

- Go CLI distributed as source and release binaries
- `init`, `run`, and `version` commands
- Manifest-owned alias-to-path mapping
- Agent presets for common coding CLIs, plus custom and mock modes
  (see [docs/agents.md](docs/agents.md))
- Detached worktree at source `HEAD`
- Bounded agent and verification commands
- Process-group termination on timeout or cancellation
- Batched logs, HTTP timeouts, retries, durable result spool
- Five-second heartbeats against a sixty-second lease

## Job lifecycle

```text
queued -> running -> completed
                  -> failed
                  -> cancelled

running + expired lease -> lost attempt -> queued
```

Every re-claim increments `attempt_number`. Worker writes carry `attempt_id`.
Same-attempt terminal delivery is idempotent.

## Receipt contract

The agent may return JSON containing `status`, `summary`, `changed_files`,
`verification`, `blockers`, and `notes`.

The worker then **replaces** `status`, `changed_files`, and `verification` with
what it observed, and keeps only the prose fields from the agent. That is the
whole trust model in one sentence: the agent narrates, the worker adjudicates.

## Known weakness

Verification runs agent-authored code on the developer's machine with the
developer's ambient credentials. Git isolation does not help here: a test suite
the agent just edited can read anything the worker user can read. Sandboxing
this step is the most valuable outstanding security work. See
[SECURITY.md](SECURITY.md).
