# Deployment

DeadDrop has two deployment units:

- a durable server with PostgreSQL
- a worker on the machine that owns the source repositories

The worker only needs outbound HTTPS access to the server.

## Option 1: Docker Compose

This is the recommended self-hosted path for one machine or private network. PostgreSQL data persists in a named Docker volume.

Requirements:

- Docker Engine or Docker Desktop with Compose
- ports `8000` available locally, or a reverse proxy for public use

From the repository root:

```bash
export OWNER_TOKEN="$(openssl rand -hex 32)"
export WORKER_TOKEN="$(openssl rand -hex 32)"
export POSTGRES_PASSWORD="$(openssl rand -hex 32)"
export SECURE_COOKIES=false
docker compose up -d --build
```

Check health:

```bash
curl --fail http://localhost:8000/healthz
curl --fail http://localhost:8000/readyz
```

Open `http://localhost:8000/login` and enter `OWNER_TOKEN`.

Operational commands:

```bash
docker compose logs -f server
docker compose pull
docker compose up -d --build
docker compose down
```

`docker compose down` keeps the database volume. `docker compose down -v` deletes it and must only be used when permanent data removal is intended.

For internet exposure, place the server behind a TLS reverse proxy, set `SECURE_COOKIES=true`, restrict access where practical, and back up the `deaddrop-data` volume.

## Option 2: Render and managed PostgreSQL

The root `render.yaml` builds `server/Dockerfile`. Any PostgreSQL provider with a standard connection URL works; Supabase is one option.

Create a Render Blueprint from your fork and set:

- `DATABASE_URL`: persistent PostgreSQL URL
- `OWNER_TOKEN`: a random owner secret
- `WORKER_TOKEN`: a different random worker secret
- `SECURE_COOKIES`: `true`

The Docker image honors Render's injected `PORT`. Server startup applies pending Alembic migrations before accepting traffic.

For Supabase, use its pooled PostgreSQL connection string when required by the hosting network. Keep the password URL-encoded if it contains reserved URL characters.

Do not use Render's ephemeral local filesystem as the production database.

## Local development server

```bash
cd server
uv sync --frozen
export OWNER_TOKEN=owner-dev
export WORKER_TOKEN=worker-dev
export DATABASE_URL=sqlite:///./deaddrop.db
export SECURE_COOKIES=false
uv run uvicorn app.main:app --reload
```

SQLite is intentionally a development path. Use PostgreSQL for a durable shared installation.

## Worker installation

Download the appropriate `deaddrop-worker` binary and verify it against `SHA256SUMS` from [GitHub Releases](https://github.com/haroon0x/DeadDrop/releases), or build it:

```bash
cd worker
go build -trimpath -o deaddrop-worker .
```

Create a manifest:

```bash
./deaddrop-worker init \
  --repo /absolute/path/to/project \
  --verify "python -m pytest"
```

Run against the hosted server:

```bash
./deaddrop-worker run \
  --server https://deaddrop.example.com \
  --token "$WORKER_TOKEN" \
  --manifest deaddrop.manifest.json \
  --agent gemini
```

Use [Worker service](worker-service.md) to keep it running under the operating system's service manager.

## Upgrade procedure

1. Back up PostgreSQL.
2. Read the release notes and migration notes.
3. Update the server image or source checkout.
4. Restart the server; startup applies migrations.
5. Replace and restart the worker binary.
6. Verify `/readyz`, create a small task, and confirm its receipt.

Server and worker should normally run the same release. Attempt IDs prevent an outdated or stale process from overwriting a newer active attempt, but API compatibility is only guaranteed within the supported release line.

## Secrets

Use independent high-entropy tokens. Never commit them to the repository or manifest. Rotate both tokens after suspected exposure.

The worker token authorizes local work claims and result submission. Anyone with it and server access can impersonate the worker. The owner token exposes prompts, logs, patches, and job controls.

For a service manager, keep `WORKER_TOKEN` in a file readable only by the worker user. For hosted server secrets, use the platform's secret manager.

## Backups and recovery

Back up the PostgreSQL database, including the Compose volume or managed-provider snapshots. The server database contains prompts, logs, receipts, and patches.

The source repositories remain the authority for code. DeadDrop never applies returned patches to them. Pending terminal results live on the worker machine and replay automatically after connectivity returns.

## Production checklist

- HTTPS enabled
- `SECURE_COOKIES=true`
- distinct random owner and worker tokens
- persistent PostgreSQL with backups
- worker running as non-root dedicated user
- only intended Git workspaces configured
- trusted verification commands configured
- server and worker on the same release
- `/healthz` and `/readyz` monitored
