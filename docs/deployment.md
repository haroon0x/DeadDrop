# Deployment

DeadDrop has three deployment units:

- a static public website
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

## Public website on Render

The public website is an independently deployable Next.js static export. It does not connect to Supabase, expose the owner dashboard, or contain secrets.

Create a new Render **Static Site** from the repository with these settings:

- Root Directory: `frontend`
- Build Command: `npm ci && npm run build`
- Publish Directory: `out`
- Environment Variables: none

Do not convert the existing backend service into a static site. Keep it as a separate web service and create a new Render service for the public site. The root `render.yaml` describes both services when deploying through a Blueprint.

The static export can also be built locally:

```bash
cd frontend
npm ci
npm run build
```

Serve the generated `frontend/out` directory with any static host.

## Render web service and Supabase PostgreSQL

DeadDrop's private application is a persistent Render web service. The root `render.yaml` builds `server/Dockerfile`, binds to Render's injected `PORT`, and uses `/readyz` as the database-aware health check. Supabase supplies PostgreSQL over its IPv4-compatible shared pooler.

Create a Render Blueprint from your fork and set:

- `DATABASE_URL`: persistent PostgreSQL URL
- `OWNER_TOKEN`: a random owner secret
- `WORKER_TOKEN`: a different random worker secret
- `SECURE_COOKIES`: `true`

Server startup applies pending Alembic migrations before accepting traffic.

### Configure the Supabase connection

Render is a persistent backend on an IPv4 network. Use the Supabase **Session pooler** connection string on port `5432`, not the Transaction pooler on port `6543`.

1. Open the active Supabase project.
2. Select **Connect**.
3. Choose **Session pooler**.
4. Enter the database password and copy the complete URI.
5. In Render, open the DeadDrop web service and select **Environment**.
6. Replace `DATABASE_URL` with the copied URI.
7. Select **Save and deploy**.

The URI has this shape:

```text
postgresql://postgres.PROJECT_REF:URL_ENCODED_PASSWORD@aws-REGION.pooler.supabase.com:5432/postgres?sslmode=require
```

The `postgres.PROJECT_REF` username and `aws-REGION.pooler.supabase.com` hostname must come from the same Supabase project's Connect panel. Do not assemble the username, region, or hostname from old values. URL-encode database passwords containing `@`, `:`, `/`, `?`, `#`, `%`, or other reserved URL characters.

After deployment, the Render logs should show Alembic upgrading to the latest revision. Verify both endpoints:

```bash
curl --fail https://YOUR-SERVICE.onrender.com/healthz
curl --fail https://YOUR-SERVICE.onrender.com/readyz
```

### Supabase pooler troubleshooting

This error means the pooler is reachable but the username does not identify a tenant on that pooler:

```text
FATAL: (ENOTFOUND) tenant/user postgres.PROJECT_REF not found
```

It is not a FastAPI, SQLAlchemy, Alembic, DNS, or Render port-binding failure. Replace the entire Render `DATABASE_URL` from the current Supabase **Session pooler** panel. Common causes are a stale project reference, a hostname copied from another project or region, or a database belonging to a deleted or inactive project.

An invalid database password produces an authentication error instead. If the tenant is recognized but authentication fails, reset the Supabase database password, copy a newly generated session-pooler URI, update Render, and redeploy.

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
- Supabase Session pooler URL on port `5432` copied from the active project
- distinct random owner and worker tokens
- persistent PostgreSQL with backups
- worker running as non-root dedicated user
- only intended Git workspaces configured
- trusted verification commands configured
- server and worker on the same release
- `/healthz` and `/readyz` monitored
