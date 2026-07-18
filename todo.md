# DeadDrop roadmap

## Completed foundation

- [x] Harden and audit server dependencies in CI
- [x] Run jobs in isolated Git worktrees and preserve source state
- [x] Capture complete baseline-relative binary patches and changed files
- [x] Add bounded HTTP transport, log batching, retries, and durable result replay
- [x] Add Alembic migrations, attempt history, leases, heartbeats, and stale recovery
- [x] Add running cancellation with local process-group termination
- [x] Make terminal writes idempotent and reject stale attempts
- [x] Make receipt status, changed files, and verification worker-authoritative
- [x] Add automated server, worker, migration, transport, and E2E coverage
- [x] Add durable Compose/PostgreSQL deployment and worker service guidance
- [x] Add setup command, release binaries, checksums, and release process
- [x] Add contributor, security, support, architecture, and deployment docs
- [x] Expand frontend into landing, docs, architecture, updates, and technical blog

## Next releases

- [ ] Run and record a full Gemini-mode E2E job when provider capacity is available
- [ ] Add server-side retention controls for old logs, patches, and receipts
- [ ] Add per-worker credentials and explicit worker revocation
- [ ] Add login and API rate limiting suitable for multi-instance deployment
- [ ] Add an operator-visible view of attempt history and lost-worker recovery
- [ ] Package a signed macOS worker and document Windows service installation in depth
- [ ] Add patch download and local apply instructions without automatic acceptance
