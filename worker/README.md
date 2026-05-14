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

Browser-created jobs route to `worker=local` and `repo_alias=default`. Use `--repo /path/to/workspace --repo-alias default` for one fixed workspace, or make the manifest entry use alias `default`.

Each configured path can be any existing directory. Gemini runs with that directory as its working directory. If the workspace is inside a git worktree, DeadDrop captures `git status -- .` and `git diff -- .` scoped to that workspace; if not, git capture is skipped.

Default Gemini command:

```bash
gemini --skip-trust --approval-mode yolo --output-format text -p "{{prompt}}"
```

The built-in Gemini mode executes Gemini directly without a shell and redacts the prompt from live logs. `--command-template` still uses a shell for custom commands.

Use `--agent-timeout 900` to control max agent runtime in seconds.

Gemini should wrap its final answer with `DEADDROP_RECEIPT_JSON` and `DEADDROP_RECEIPT_JSON_END`. Content inside those markers must be valid JSON with `status`, `summary`, `changed_files`, `verification`, `blockers`, and `notes`. The worker renders that JSON as clean receipt sections and keeps raw stdout/stderr in live logs. If no receipt marker appears on a zero-exit run, the worker fails the job because DeadDrop needs a reliable receipt.

Use `--run-once` for smoke tests or one-shot process managers. The worker registers workspaces, polls once, processes at most one job, reports completion/failure, and exits.

Robustness checklist for future worker changes:

- Always complete or fail a claimed job.
- Stream logs but skip blank log content.
- Keep command execution inside the resolved workspace.
- Preserve `--agent-timeout`.
- Preserve process-group killing on timeout so child processes do not survive the worker.
- Capture final `git diff` even on failure when possible.
- Do not add auto-commit behavior unless there is an explicit human accept step.
