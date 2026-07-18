# Implementation Plans

Reconciled by the improve skill on 2026-07-18 at commit `ea6c3f9`. Execute
plans in the order below unless their dependency fields say otherwise. Each
executor must read the selected plan completely, honor its STOP conditions,
and update its status row when done.

The earlier June 2026 roadmap documents remain useful historical product
research, but their execution order is stale. The current open-source direction
puts dependency security, isolated worktrees, durable job delivery, transport
reliability, and authoritative receipts ahead of product-surface expansion.

## Execution order and status

| Plan | Title | Priority | Effort | Depends on | Status |
|---|---|---|---|---|---|
| [001](001-secure-production-dependencies.md) | Remove known vulnerabilities from the production server dependency set | P1 | S | — | DONE — reviewed at `160b883` |

## Completed direction

The 2026-07-18 execution completed isolated worktrees, complete relative
patches, durable HTTP delivery, attempt leases, running cancellation, stale
recovery, authoritative receipts, versioned migrations, automated E2E,
open-source distribution, operator documentation, and the public project site.

Current release work is tracked in [`todo.md`](../todo.md).

## Direction documents

| Document | Status | Note |
|---|---|---|
| [000](000-product-roadmap.md) | STALE | Superseded by the 2026-07-18 audit; feature ideas remain reference material. |
| [pmf](pmf.md) | STALE | The competitive premise predates official Claude and Codex remote-control offerings and current open-source alternatives. |

## Dependency notes

- Dependency security can land independently and is the first safe change.
- Minimal operational characterization coverage must be explicitly authorized
  before tests are added for the worker refactors.
- Worktree isolation must land before accept/reject, scheduled modifying jobs,
  or multi-job concurrency.
- Database migrations must precede job lease and cancellation schema changes.
- Durable transport and idempotent server updates must be designed together so
  retries cannot corrupt terminal job state.
- Authoritative receipts depend on isolated, baseline-relative Git state.

## Findings considered and rejected

- General remote-control feature parity: not worth pursuing because official
  agent products and mature open-source tools already own interactive mobile
  session control.
- Chat UI, remote terminal, React rewrite, microservices, OAuth, billing, and
  multi-tenant SaaS: rejected until the narrow open-source maintenance workflow
  demonstrates repeat usage.
- Fixing only untracked-file display with `git add -N`: rejected because it
  still mixes pre-existing user changes with agent changes and leaves the live
  checkout exposed.
- PWA polish, rich diff rendering, and artifact uploads: deferred until the
  worker safety and reliability contract is trustworthy.

Status values: TODO | IN PROGRESS | DONE | BLOCKED | REJECTED
