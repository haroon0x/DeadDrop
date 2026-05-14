# DeadDrop Worker

Run local coding jobs from a trusted DeadDrop server.

```bash
go run . run \
  --server http://localhost:8000 \
  --token worker_dev \
  --worker local \
  --manifest deaddrop.manifest.example.json \
  --agent mock
```

Use `--agent gemini` for Gemini CLI or `--agent custom --command-template 'your-command "{{prompt}}"'`.

`--repo` and `--repo-alias` still work for one repo. `--manifest` is preferred because it registers repo aliases with server so phone UI can show a dropdown without seeing local absolute paths.

Default Gemini command:

```bash
gemini --skip-trust --approval-mode yolo --output-format text -p "{{prompt}}"
```

Use `--agent-timeout 900` to control max agent runtime in seconds.

Gemini must wrap its final answer with `DEADDROP_RECEIPT` and `DEADDROP_RECEIPT_END`. Content inside those markers is free-form and can answer the user task directly. Missing markers on a zero-exit agent run are treated as failure because DeadDrop needs a reliable receipt.
