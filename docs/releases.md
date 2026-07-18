# Release process

DeadDrop uses semantic version tags such as `v0.2.0`.

## Before tagging

1. Confirm the working tree contains only intended release changes.
2. Run server tests, worker tests, vet, E2E, migration drift check, and dependency audit.
3. Review database migrations and document any operator action.
4. Update user-facing documentation for changed flags, schemas, or deployment steps.
5. Confirm the server and worker remain compatible at the same version.

Verification commands:

```bash
cd server
uv sync --frozen
uv run pytest -q
uv run alembic check
uv run pip-audit

cd ../worker
go test ./...
go vet ./...

cd ..
server/.venv/bin/python -m pytest -q e2e
```

## Publish

Create and push an annotated tag:

```bash
git tag -a v0.2.0 -m "v0.2.0"
git push origin v0.2.0
```

The release workflow builds Linux, macOS, and Windows worker binaries for amd64 and arm64, generates `SHA256SUMS`, and creates GitHub release notes.

## After publishing

1. Download one release binary and verify its checksum.
2. Run `deaddrop-worker version`.
3. Deploy the tagged server image or source.
4. Run one completion job and one cancellation job.
5. Add manual migration or compatibility notes to the GitHub release when needed.

## Compatibility policy

The server and worker are released together. The supported configuration is matching versions. Patch releases should preserve protocol compatibility within the same minor version. Breaking protocol or migration changes require a minor release while the project is pre-1.0 and a major release after 1.0.
