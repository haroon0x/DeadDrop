# DeadDrop worker

The worker polls a trusted DeadDrop server, runs one coding job at a time in an isolated Git worktree, verifies the result, and returns an authoritative receipt plus patch.

Build:

```bash
go build -o deaddrop-worker .
```

Create a manifest:

```bash
./deaddrop-worker init \
  --repo /absolute/path/to/project \
  --verify "go test ./..."
```

Run:

```bash
./deaddrop-worker run \
  --server http://localhost:8000 \
  --token "$WORKER_TOKEN" \
  --manifest deaddrop.manifest.json \
  --agent gemini
```

The configured path must be a Git root or committed subdirectory. The source directory remains untouched. Each job runs from a detached worktree at source `HEAD`; dirty and untracked source files are excluded.

Agent modes:

- `gemini`: invokes Gemini CLI directly with JSON output
- `custom`: invokes `sh -c` with `--command-template`
- `mock`: deterministic E2E/demo agent for `app.py` and `test_app.py`

Custom templates support `{{prompt}}`, `{{task}}`, and `{{repo}}`. Prompt and task values are redacted from local command logs.

Repeat `--verify` to run trusted verification commands after the agent. Any failure fails the job. Changed files, verification status, and receipt status are worker-derived.

The worker requires a structured agent receipt. It batches logs, renews attempt leases, handles running cancellation, kills timed-out or cancelled process groups, retries HTTP delivery, and durably replays terminal results before claiming more work.

Run checks:

```bash
go test ./...
go vet ./...
```

See the root [README](../README.md), [Architecture](../docs/architecture.md), and [Worker service](../docs/worker-service.md).
