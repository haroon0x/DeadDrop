# DeadDrop Worker

Run local coding jobs from a trusted DeadDrop server.

```bash
go run . run \
  --server http://localhost:8000 \
  --token "$WORKER_TOKEN" \
  --worker local \
  --manifest deaddrop.manifest.example.json \
  --agent mock
```

Use `--agent gemini` for Gemini CLI or `--agent custom --command-template 'your-command "{{prompt}}"'`.

`--repo` and `--repo-alias` still work for one repo. `--manifest` is preferred because it registers repo aliases with server so phone UI can show a dropdown without seeing local absolute paths.

Each configured path must be a git worktree root. The worker rejects subdirectories inside a larger repo because `git diff` would otherwise include unrelated parent-repo changes. If you want Gemini to work in any directory, make that directory a git repo first, then add it to the manifest.

Default Gemini command:

```bash
gemini --skip-trust --approval-mode yolo --output-format text -p "{{prompt}}"
```

Use `--agent-timeout 900` to control max agent runtime in seconds.

Gemini should wrap its final answer with `DEADDROP_RECEIPT` and `DEADDROP_RECEIPT_END`. Content inside those markers is free-form and can answer the user task directly. If Gemini emits the start marker but forgets the end marker, the worker keeps the output from the start marker as the receipt so simple tasks do not fail. If no receipt marker appears on a zero-exit run, the worker fails the job because DeadDrop needs a reliable receipt.

Use `--run-once` for smoke tests or one-shot process managers. The worker registers repos, polls once, processes at most one job, reports completion/failure, and exits.

Robustness checklist for future worker changes:

- Always complete or fail a claimed job.
- Stream logs but skip blank log content.
- Keep command execution inside the resolved workspace.
- Preserve `--agent-timeout`.
- Preserve process-group killing on timeout so child processes do not survive the worker.
- Capture final `git diff` even on failure when possible.
- Do not add auto-commit behavior unless there is an explicit human accept step.
