# Demo script

Start the durable local server:

```bash
export OWNER_TOKEN="$(openssl rand -hex 32)"
export WORKER_TOKEN="$(openssl rand -hex 32)"
export POSTGRES_PASSWORD="$(openssl rand -hex 32)"
docker compose up -d --build
```

Build and start the deterministic worker:

```bash
cd worker
go build -o deaddrop-worker .
./deaddrop-worker run \
  --server http://localhost:8000 \
  --token "$WORKER_TOKEN" \
  --manifest deaddrop.manifest.example.json \
  --agent mock
```

Open `http://localhost:8000/login`, enter `OWNER_TOKEN`, and create:

```text
Title: Fix the demo addition bug
Task: Fix app.py so add returns a + b. Return a clear receipt.
```

Show this sequence:

1. The job moves from queued to running.
2. The worker creates an isolated worktree and runs the failing test.
3. The job completes after worker verification passes.
4. The receipt lists `app.py` from Git evidence.
5. The dashboard shows the baseline-relative patch.
6. `examples/demo-repo/app.py` still contains the original failing implementation.

Repeat with `--agent gemini` when Gemini CLI is installed and authenticated.
