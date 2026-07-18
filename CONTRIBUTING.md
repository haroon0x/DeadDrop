# Contributing

DeadDrop welcomes focused bug fixes, reliability improvements, documentation, and small features that preserve the local-first trust model.

## Start here

Read:

- [README](README.md)
- [Architecture](docs/architecture.md)
- [Security Policy](SECURITY.md)

Open an issue before a large protocol, persistence, authentication, or product-scope change. Small fixes can go directly to a pull request.

## Development setup

Requirements:

- Python 3.13 and uv
- Go 1.22+
- Git
- Docker for the durable PostgreSQL path

Install server dependencies:

```bash
cd server
uv sync --frozen
```

Run the checks:

```bash
cd server
uv run pytest -q
uv run alembic check
uv run pip-audit

cd ../worker
go test ./...
go vet ./...

cd ..
server/.venv/bin/python -m pytest -q e2e
```

## Change expectations

- Keep the server unaware of local absolute repository paths.
- Preserve outbound-only worker networking.
- Treat prompts and agent output as untrusted.
- Keep source workspaces untouched; modifications belong in temporary worktrees.
- Derive receipt facts from worker evidence.
- Add a migration for every schema change and keep `alembic check` clean.
- Do not add automatic commit, push, merge, or arbitrary remote-shell behavior.
- Update documentation when behavior, flags, deployment, or trust boundaries change.

## Pull requests

Describe the user problem, implementation, verification performed, and security or migration impact. Keep pull requests small enough to review. CI must pass before merge.

Use conventional commit subjects where practical, such as `fix(worker): preserve pending results` or `docs: explain lease recovery`.

## Reporting vulnerabilities

Do not open public issues for vulnerabilities. Follow [SECURITY.md](SECURITY.md).
