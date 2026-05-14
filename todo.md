# DeadDrop Production Roadmap

DeadDrop is a local-first coding task inbox. The core security and deployment infrastructure are complete. This roadmap tracks the final features required for a professional production release.

## Completed Milestones

- [x] **Core Architecture**: FastAPI server + Go worker polling architecture.
- [x] **Security Hardening**: CSRF protection, Secure Cookies, and constant-time token comparisons.
- [x] **Worker Safety**: Root user prevention and Git worktree path validation.
- [x] **Deployment**: "Deploy to Render" blueprint and Supabase Postgres integration.
- [x] **Workspace Management**: Local manifest-based repo registration and routing.
- [x] **Audit Loop**: Log streaming, deterministic receipt extraction, and automated git diff capture.

## High Priority: Production Hardening

### 1. Mid-Task Cancellation Protocol
Allow the server to signal a polling worker to terminate a running agent process mid-job.
- [ ] Add `cancellation_requested` flag to jobs table.
- [ ] Add `/api/jobs/{id}/request-cancel` owner endpoint.
- [ ] Update worker heartbeat/poll to check for cancellation flag.
- [ ] Implement `SIGKILL` process-group termination in worker.

### 2. Rate Limiting
Protect the server from brute-force login and API spam.
- [ ] Integrate `slowapi` or similar middleware.
- [ ] Apply rate limits to `/login` and `/api/worker/register`.

### 3. Maintenance & Stability
- [ ] **Log Pruning**: Automatic cleanup of logs older than 30 days to save DB space.
- [ ] **Gemini Stress Test**: Verify full workflow with a complex real-world task using Gemini CLI.
- [ ] **Token Rotation Tooling**: Simple script or documentation for fast secret rotation.

## Optional: Future Enhancements
- [ ] **Accept/Reject Flow**: UI to apply the captured diff locally with one click.
- [ ] **Artifact Retention**: Store specific files (like build artifacts) alongside logs.
- [ ] **Multi-Worker Support**: Allow routing to specific named workers (e.g., `home-pc`, `office-pc`).

## Verification Commands
```bash
# Server tests
cd server && uv run pytest

# Worker build
cd worker && go build .

# Smoke run (Local)
# 1. Start server with tokens
# 2. Run worker with --agent mock --run-once
```
