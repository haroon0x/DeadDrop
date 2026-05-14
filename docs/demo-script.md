# Demo Script

1. Generate local tokens: `export OWNER_TOKEN="$(openssl rand -base64 32)" WORKER_TOKEN="$(openssl rand -base64 32)"`.
2. Start server with `uvicorn app.main:app --reload`.
3. Open `http://localhost:8000/login` and enter your `OWNER_TOKEN`.
4. In `examples/demo-repo`, run `git init && git add . && git commit -m "demo baseline"` once.
5. Create job: `Fix the failing test in the demo repo. Do not commit.`
6. Start worker with `--manifest deaddrop.manifest.example.json --agent mock`.
7. Watch job move from `queued` to `running` to `completed`.
8. Open receipt and show logs plus git diff.

Gemini demo:

```bash
cd worker
go run . run --server http://localhost:8000 --token "$WORKER_TOKEN" --worker local \
  --repo ../examples/demo-repo --repo-alias default --agent gemini
```
