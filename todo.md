# DeadDrop roadmap

Ordered by what stands between the software and someone actually using it.
Nothing here is a promise, and nothing here forbids a direction that turns out
to be better. Argue with it.

## Next

- [ ] **Sandbox the verification step.** Verification executes agent-authored
      code as the worker user with that user's credentials. This is the largest
      real risk in the project and it outranks every feature below. Start by
      documenting a container-based worker, then look at cutting network access
      for the verify command.
- [ ] **Cut the install to one step.** Postgres plus FastAPI plus a Go worker is
      a three-service install for a tool nobody has adopted yet. A single-binary
      or single-container mode backed by SQLite would remove the largest
      adoption barrier. Distribution friction is the bottleneck, not durability.
- [ ] **Close the loop from patch to applied.** Producing a patch is only half
      the job; today the last mile is downloading a file and running three Git
      commands by hand. A `deaddrop apply <id>` that applies to a scratch branch
      keeps the human as the gate while removing the tedium. Pair it with a
      receipt view that puts the diff, the verification output, and the agent
      transcript side by side.

## After that

- [ ] Per-worker credentials and revocation, so one compromised machine does not
      mean rotating every token.
- [ ] Retention controls for old logs, patches, and receipts, which will
      otherwise quietly fill a small database.
- [ ] Login and API rate limiting suitable for multi-instance deployment.
- [ ] An operator-visible view of attempt history and lost-worker recovery.
- [ ] Run and record a full Gemini-mode end-to-end job. Everything verified so
      far runs through the mock agent.

## Ideas worth arguing about

- **Remote workers as placement.** A worker already runs anywhere and polls
  outbound, so "run this job on the big machine instead of the laptop" is mostly
  a routing feature, not a new architecture. Making the agent process survive a
  dropped connection would also fix a real gap: today, if the worker dies
  mid-job, the agent dies with it and the work is lost.
- **Live attachable sessions.** Shipping a running agent session to another
  machine and attaching to it is a genuinely different product. A bounded job
  has a terminal state and can therefore produce a receipt; an open session has
  none, so there is nothing to adjudicate. Worth building, possibly as a sibling
  tool, but it should be chosen deliberately rather than arrived at by adding a
  flag.

## Deliberately not doing yet

- Signed macOS builds and deep Windows service documentation. Signing costs time
  and money, and there is no user base asking for it.

## Done

Attempts, leases, heartbeats, cancellation, stale recovery, and idempotent
terminal writes. Worker-authoritative receipts. Disposable worktrees with
baseline-relative binary patches. Alembic migrations. Durable Compose
deployment, release binaries, and checksums. Agent presets for the common
coding CLIs with per-job selection. Repository routing bounded by the worker
manifest. The public site and the self-hosted app UI on one design system.
