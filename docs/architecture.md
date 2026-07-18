# Architecture

DeadDrop separates the public control plane from local code execution. The server owns durable job state. The worker owns local repository paths, agent processes, verification, and patch generation.

## Components

```text
Browser/API client
      |
      | OWNER_TOKEN
      v
FastAPI server ---------------- PostgreSQL or SQLite
      ^                          jobs
      |                          job_attempts
      | WORKER_TOKEN             job_logs
      |                          worker_repos
Go worker
      |
      +---- source Git repository, read-only during a job
      |
      +---- temporary detached Git worktree
                |
                +---- Gemini CLI, custom command, or mock agent
                +---- trusted verification commands
```

The browser and worker never communicate directly. The worker makes outbound HTTP requests, so the developer machine needs no open port, tunnel, or inbound firewall rule.

## Server responsibilities

The FastAPI server:

- authenticates owners and workers with separate bearer tokens
- provides the browser session, task form, job view, logs, receipt, and patch
- stores jobs, attempts, worker registrations, logs, and results
- creates one active attempt and lease when a worker claims a queued job
- records heartbeats and cancellation requests
- rejects stale-attempt logs and results
- recovers expired attempts before issuing more work
- applies versioned Alembic migrations at startup

The server stores `worker_name` and `repo_alias`, not local filesystem paths.

## Worker responsibilities

The Go worker:

- maps trusted aliases to local Git directories
- registers display names with the server
- replays pending terminal results before claiming new work
- polls for a job and records its unique attempt ID
- creates and later removes an isolated detached Git worktree
- runs one configured agent with a bounded lifetime
- batches logs and maintains the job lease
- interrupts the agent process group after a cancellation request or timeout
- runs trusted verification commands
- derives changed files and a binary patch from Git
- sends a terminal result or durably spools it for replay

The worker processes one job at a time. This keeps local resource use and workspace ownership understandable.

## Job lifecycle

```text
queued
  |
  | claim: attempt ID + lease
  v
running --------------------------+
  |                               |
  | heartbeat renews lease        | lease expires
  |                               |
  |                               v
  |                         attempt = lost
  |                         job = queued
  |
  +---- success ----------> completed
  |
  +---- agent/verify error > failed
  |
  +---- owner cancellation > cancelled
```

Every claim increments `attempt_number` and creates a `job_attempts` row. Worker writes include `attempt_id`. A late result from an expired attempt cannot overwrite the current attempt.

The lease is sixty seconds. The worker heartbeats every five seconds. After three consecutive heartbeat failures, the worker cancels its local command rather than continuing indefinitely without ownership.

## Running cancellation

An owner cancellation of a queued job is immediately terminal. For a running job, the server sets `cancel_requested_at` while preserving the active attempt. The next worker heartbeat returns `cancel_requested=true`.

The worker then cancels the command context and kills the entire spawned process group. It still captures any partial Git patch, stops the heartbeat, and posts `/cancelled`. If completion races with cancellation, the server makes cancellation win and returns an idempotent terminal response.

## Git isolation and patch generation

The configured workspace may be a Git root or a committed subdirectory. For each job, the worker resolves:

```text
source root:      git rev-parse --show-toplevel
baseline:         git rev-parse HEAD
temporary root:   git worktree add --detach <temp>/workspace <baseline>
agent directory:  <temp>/workspace/<configured relative path>
```

The source directory is inspected but never modified. Dirty tracked files and untracked files in the source are not copied into the detached worktree.

After execution, the worker stages all changes only inside the temporary worktree and derives:

- a `git diff --binary --relative <baseline>` patch
- a NUL-delimited `git diff --name-only --relative` changed-file list

This includes tracked edits, deletions, new files, binary files, and commits created by an agent. Paths are relative to the configured workspace. The temporary worktree is force-removed after result capture.

DeadDrop returns a patch; it never applies the patch to the source workspace and never commits or pushes the source repository.

## Receipts and verification

Agents must return structured receipt JSON with a summary. Agent-authored changed files and verification claims are treated as untrusted input.

The worker replaces those fields with evidence:

- `changed_files` comes from the baseline-relative Git diff
- `verification` comes from configured commands executed by the worker
- `status` comes from the observed agent and verification exit state

The agent remains responsible for the human-readable summary, notes, and blockers. A zero-exit agent without a valid structured receipt fails the job.

## HTTP delivery

The worker uses a bounded HTTP client and always consumes and closes response bodies. Registration, logs, heartbeats, and terminal results retry transport errors, HTTP 408, HTTP 429, and server errors. Job claim is not retried because a lost claim response could otherwise claim a second job.

Adjacent logs from the same stream are combined up to the server payload limit. System events flush buffered output so the browser still receives progress during execution.

Terminal results are idempotent for the same attempt. Failed terminal delivery is atomically stored with mode `0600` under:

```text
<user-config-directory>/deaddrop/pending-results/<server-hash>/
```

The worker replays this queue before polling for new work. The server URL hash prevents results from being sent to a different configured server.

## Persistence and migrations

SQLite is suitable for local development and tests. Durable deployments use PostgreSQL. The Compose deployment stores PostgreSQL data in the `deaddrop-data` volume.

Alembic owns schema changes:

- `0001` creates the baseline server schema
- `0002` adds job attempts, leases, cancellation state, and attempt-aware logs

For an existing pre-migration installation, startup stamps the known baseline only when application tables exist and `alembic_version` does not. It then upgrades to the latest revision. New installations upgrade from an empty database.

## Trust boundaries

DeadDrop protects the control path but does not sandbox arbitrary code.

Trusted:

- the server operator
- the configured server URL and worker token
- local manifest paths and verification commands
- the operating-system account running the worker

Untrusted:

- task prompts
- agent output and receipt claims
- files produced by the agent until reviewed

The agent inherits the worker user's filesystem, network, credential, and tool access. Git worktrees isolate repository state, not operating-system capabilities. Run the worker as a dedicated non-root user and expose only intended repositories and credentials.

## Current boundaries

- one active worker route named `local`
- one browser-selected default repository route
- one job at a time per worker process
- shared owner and worker tokens, not per-user identities
- no automatic patch application, commit, push, or merge
- no artifact store beyond logs, receipt JSON, and Git patch
- no operating-system sandbox
