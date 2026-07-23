# Coding agents

DeadDrop does not care which coding CLI you use. The worker only needs a command
that edits files inside the job worktree. Everything DeadDrop reports comes from
the worker, not from the agent:

- **changed files** come from `git diff` against the job baseline
- **verification** comes from the commands in your manifest
- **status** comes from the exit code the worker observed
- **the patch** comes from Git

So an agent never has to speak a DeadDrop protocol. If it can edit a checkout,
it works.

## Choosing an agent

Set a default when you start the worker:

```bash
./deaddrop-worker run \
  --server http://localhost:8000 \
  --token "$WORKER_TOKEN" \
  --manifest deaddrop.manifest.json \
  --agent claude
```

Individual jobs can override it from the browser. The worker honours the request
only if it recognises the name, and logs the substitution on the job. The worker
is the only party that knows which CLIs are actually installed, so a job asking
for an agent the machine does not have will fail with that CLI's own error.

## Presets

These are starting points, not a compatibility guarantee. CLI flags move between
releases, so check the command against the version you have installed.

| `--agent` | Command it runs |
| --- | --- |
| `claude` | `claude --print --permission-mode acceptEdits {{prompt}}` |
| `codex` | `codex exec --full-auto {{prompt}}` |
| `aider` | `aider --yes --no-auto-commits --no-check-update --message {{prompt}}` |
| `cursor` | `cursor-agent --print {{prompt}}` |
| `opencode` | `opencode run {{prompt}}` |
| `gemini` | `gemini --skip-trust --approval-mode yolo --output-format json -p <prompt>` |
| `mock` | Bundled deterministic edit, for proving the loop without a provider |

Two details worth knowing:

- The `aider` preset disables auto-commits. Aider commits by default, and
  DeadDrop must remain the only thing that decides what lands in your history.
- Presets run non-interactively. A CLI waiting for a confirmation prompt will
  hit the agent timeout instead of finishing.

## Overriding a preset

`--command-template` replaces the command for any agent, so you do not need
`--agent custom` to adjust one flag:

```bash
./deaddrop-worker run \
  --agent claude \
  --command-template 'claude --print --permission-mode bypassPermissions {{prompt}}'
```

## Any other CLI

Use `custom` with your own command:

```bash
./deaddrop-worker run \
  --agent custom \
  --command-template 'my-agent --edit --repo {{repo}} {{prompt}}'
```

Available placeholders:

| Placeholder | Value |
| --- | --- |
| `{{prompt}}` | The full prompt, including workspace context |
| `{{task}}` | Just the task text you typed in the browser |
| `{{repo}}` | Absolute path to the job worktree |

Placeholders are shell-quoted when substituted, so a prompt containing quotes or
semicolons cannot break out of its argument.

## Writing a good task

The agent runs unattended in a throwaway worktree, so say what "done" looks like
rather than describing a conversation:

- name the files or area to change
- state the check that should pass
- say explicitly not to commit, since some CLIs try by default
