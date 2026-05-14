# Demo Script

1. Start server with `OWNER_TOKEN=owner_dev WORKER_TOKEN=worker_dev uvicorn app.main:app --reload`.
2. Open `http://localhost:8000/login` and enter `owner_dev`.
3. In `examples/demo-repo`, run `git init && git add . && git commit -m "demo baseline"` once.
4. Create job: `Fix the failing test in the demo repo. Do not commit.`
5. Start worker with `--manifest deaddrop.manifest.example.json --agent mock`.
6. Watch job move from `queued` to `running` to `completed`.
7. Open receipt and show logs plus git diff.

Gemini demo:

```bash
cd worker
go run . run --server http://localhost:8000 --token worker_dev --worker local \
  --repo ../examples/demo-repo --repo-alias default --agent gemini
```
