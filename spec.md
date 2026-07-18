# DeadDrop project specification

## Product definition

DeadDrop is a self-hosted, local-first coding task inbox. An owner creates a job in a browser. A local worker polls outbound, runs a coding agent in an isolated Git worktree, verifies the result, and returns logs, a structured receipt, and a reviewable patch.

The project is an open-source tool for individual developers and small trusted teams. It is not a hosted multi-user SaaS, remote desktop, general chatbot, or automatic code-merging system.

## Required properties

- The server never stores local absolute repository paths.
- The worker never requires an inbound connection.
- Every modifying job runs outside the source workspace.
- Dirty and untracked source files survive unchanged.
- Every claim has one unique attempt and expiring lease.
- A stale attempt cannot write logs or terminal results.
- Running jobs can be cancelled through the outbound heartbeat path.
- Terminal results survive temporary server unavailability.
- Changed files, verification, and status come from worker evidence.
- DeadDrop never commits, pushes, merges, or applies a patch automatically.

## Server

- FastAPI, Jinja2, and SQLAlchemy
- SQLite for development; PostgreSQL for durable deployment
- Versioned Alembic migrations
- Separate owner and worker bearer tokens
- HTTP-only owner session cookie and CSRF-protected forms
- Jobs, attempts, logs, worker registrations, receipts, and patches
- Health and readiness endpoints

## Worker

- Go CLI distributed as source and release binaries
- `init`, `run`, and `version` commands
- Manifest-owned alias-to-path mapping
- Gemini, custom, and deterministic mock agents
- Detached worktree at source `HEAD`
- Bounded agent and verification commands
- Process-group termination on timeout or cancellation
- Batched logs, HTTP timeouts, retries, and durable result spool
- Five-second heartbeats against a sixty-second lease

## Job lifecycle

```text
queued -> running -> completed
                  -> failed
                  -> cancelled

running + expired lease -> lost attempt -> queued
```

Every re-claim increments `attempt_number`. Worker writes carry `attempt_id`. Same-attempt terminal delivery is idempotent.

## Receipt contract

The agent returns JSON containing:

- `status`
- `summary`
- `changed_files`
- `verification`
- `blockers`
- `notes`

The worker validates the structure and replaces `status`, `changed_files`, and `verification` with observed evidence before sending it to the server. A zero-exit agent without a valid structured receipt fails.

## Public project surface

- Product landing page
- Operator quickstart documentation
- Public architecture explanation
- Project updates
- Technical blog
- Contributor, support, security, and release policies
- Durable Compose deployment
- Tagged cross-platform worker releases with checksums

## Explicit non-goals

- OAuth and multi-user authorization
- billing
- arbitrary remote shell access
- server-selected local paths
- repository indexing
- operating-system sandboxing
- automatic patch acceptance
- automatic commits, pushes, or merges
