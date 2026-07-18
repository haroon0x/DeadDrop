# DeadDrop — Product & Feature Roadmap

> **Planned at**: commit `ea6c3f9`, 2026-06-11
> **Type**: direction document (design/roadmap — not a step-by-step executor plan).
> Individual items below should be spun out into numbered executable plans
> (`plans/NNN-<slug>.md`) before implementation. Effort scale: S < 1 day,
> M = 1–3 days, L ≈ 1 week+.

Companion document: [pmf.md](pmf.md) — who this is for, positioning, and how to
validate product-market fit. This file is the *what to build*; pmf.md is the
*why anyone will care*.

---

## 1. Where the project stands today

DeadDrop is a working MVP of a local-first coding-task inbox:

- **Server** — FastAPI + SQLAlchemy Core + Jinja2 (`server/app/`, ~760 lines of
  Python). Token auth (`OWNER_TOKEN` / `WORKER_TOKEN`), CSRF double-submit
  cookies, SQLite locally / Supabase Postgres in production, deployed on Render
  via Docker (`render.yaml`, `server/Dockerfile`).
- **Worker** — Go CLI (`worker/`, ~870 lines). Polls `/api/worker/next`, runs
  `mock` / `gemini` / `custom` agent commands inside manifest-mapped local
  workspaces (`worker/runner.go:55-71`), streams logs, enforces process-group
  timeouts, extracts a structured `DEADDROP_RECEIPT_JSON` receipt, captures
  scoped `git diff`.
- **UI** — 8 Jinja templates + one 354-line CSS file. Landing, login,
  dashboard, new-job form, job detail with live-log polling, demo page.
- **CI** — GitHub Actions runs `uv run pytest` (server) and `go test ./...`
  (worker) plus `render.yaml` lint (`.github/workflows/ci.yaml`).

Verification baseline (use these as gates in every derived plan):

| Purpose | Command | Expected |
|---|---|---|
| Server tests | `cd server && OWNER_TOKEN=test_token WORKER_TOKEN=test_token DATABASE_URL=sqlite:///:memory: SECURE_COOKIES=false uv run pytest` | all pass |
| Worker tests | `cd worker && go test ./...` | all pass |
| Worker build | `cd worker && go build -o deaddrop-worker .` | exit 0 |

### Deliberate MVP constraints that are now product ceilings

These were the right cuts for an MVP. Each is a hard-coded constraint with the
expansion plumbing already half-built:

| Constraint | Evidence | Half-built expansion path |
|---|---|---|
| One worker named `local`, one repo alias `default` | `server/app/main.py:43-44`, jobs hard-coded to defaults at `main.py:186-187` | `worker_repos` table + `/api/repos` endpoint (`main.py:202-205`) already exist; worker manifest already supports multiple repos (`worker/config.go`) |
| No running-job cancellation | `main.py:224-225` returns 409 "Running cancellation is not supported yet" | Heartbeat endpoint exists (`main.py:261-267`) but the worker **never calls it** — it's a ready-made cancel-signal channel |
| Single shared `WORKER_TOKEN` | `server/app/auth.py` | `workers.token_hash` column exists and is never read or written (`server/app/db.py:35`) |
| Gemini-only first-class agent | `runner.go:55-71`; `custom` mode + `--command-template` (`runner.go:214-223`) already generalize the mechanism | The receipt prompt (`runner.go:115-167`) is agent-agnostic |
| No notifications | No service worker, no push, no webhook anywhere in `server/` | Job completion handlers (`main.py:288-352`) are the single obvious hook point |

### Known correctness gap worth fixing regardless of roadmap

`captureGitDiff` runs `git diff -- .` (`worker/runner.go:401-406`), which shows
**unstaged changes to tracked files only**. If the agent creates a *new* file,
it appears in `git status --short` as `??` but **not in the diff the user
reviews**. For a product whose tagline is "come back to the diff", this is a
trust hole: a user can approve a "clean" diff while new files landed silently.
Fix: `git add -N -- .` before diffing (then reset), or append
`git diff --no-index /dev/null <untracked>` per untracked file. **Effort: S.
Do this first.**

---

## 2. Product thesis

**The wedge**: every serious cloud coding agent (Codex cloud, Jules, Copilot
coding agent, Cursor background agents) runs *their* sandbox on *their*
infrastructure. DeadDrop is the inverse: the agent runs on **your** machine
with **your** credentials, local databases, private packages, and licensed
toolchains — and the phone is just a remote control for a queue. The hosted
part stores only prompts, logs, receipts, and diffs; never paths, never code
checkouts, never secrets.

The product promise is one sentence: **"Leave a coding task. Come back to the
diff."** Everything in Phase 1 exists to make that sentence actually true —
today you can *leave* a task, but you can't be *told* when to come back, can't
stop a runaway job, can't reply "almost — also fix the test", and you might
not even see the whole diff (see the untracked-files gap above).

See [pmf.md](pmf.md) for the competitive map and validation plan.

---

## 3. Roadmap

### Phase 1 — Close the loop (the promise becomes true)

Ship these before anything else. Each removes a daily-use deal-breaker.

#### 1.1 Push notifications on job completion — **Effort: M, the single highest-leverage feature**

The core flow is "leave and come back", but nothing tells the user *when* to
come back; they must re-poll their own dashboard, which destroys the async
value proposition.

Design (layered, cheapest first):

1. **Webhook out** (S): optional `NOTIFY_WEBHOOK_URL` env on the server. On
   `complete_job` / `fail_job` (`main.py:288-352`), POST
   `{job_id, title, status, summary_excerpt, url}`. This single feature covers
   ntfy.sh, Telegram bots, Slack, Discord via their generic webhook bridges —
   users self-serve their channel of choice. Document the ntfy.sh recipe in the
   README (install ntfy app → subscribe to topic → set env var → done).
2. **Web Push** (M, later): VAPID keys, `push_subscriptions` table, service
   worker. Only worth it after the PWA work in 3.1; do not start here.

Schema: none for webhook; fire-and-forget with a 5s timeout and a `system` log
line on failure. Out of scope: retries, notification preferences UI.

#### 1.2 Running-job cancellation — **Effort: M**

Already on the maintainer's own list (README "What I Would Build Next",
`todo.md`). A runaway agent burning API quota with no kill switch is
disqualifying for daily use.

Design — use the dormant heartbeat channel:

1. Server: `cancel_job` (`main.py:217-233`) on a `running` job sets a new
   `cancel_requested` column (Text timestamp) instead of returning 409.
2. Worker: while the agent command runs, a goroutine POSTs
   `/api/worker/jobs/{id}/heartbeat` every ~5s (endpoint already exists,
   `main.py:261-267`; worker currently never calls it). Extend the heartbeat
   *response* to include `{"cancel_requested": bool}`.
3. On cancel: worker kills the process group (mechanism already exists for
   timeouts, `runner.go:328`), captures whatever diff exists, reports a new
   terminal status `cancelled` via `fail` with a distinct exit code (e.g. 130)
   and `error_message: "cancelled by user"`.
4. UI: job detail shows a "Stop job" button for `running` jobs (same CSRF form
   pattern as the existing queued-cancel at `_job_receipt.html:20-25`).

Bonus the heartbeat gives for free: a `last_heartbeat_at` column enables
**stale-job detection** — server marks jobs `failed` ("worker lost") if no
heartbeat for N minutes. Today a killed worker leaves jobs stuck `running`
forever (this exact failure already happened during development — see
`todo.md` line 11).

#### 1.3 Follow-up tasks (the revise loop) — **Effort: M**

One-shot tasks make every imperfect diff a dead end: the user must write a new
task from scratch, and the agent has no memory of what it just did. A
`Follow up` button on a completed/failed job creates a new job with
`parent_job_id` (new nullable FK column on `jobs`). The worker, on a job with a
parent, prepends to the prompt (`buildPrompt`, `runner.go:115-167`): the parent
prompt, the parent receipt JSON, and the parent diff (truncated, e.g. 8 KB).
Job detail page shows the chain ("↳ follow-up of #12"). This is the cheapest
possible "conversation" — no chat UI, stays job-first, matches the spec's
taste ("not a chatbot") while killing the biggest workflow dead end.

#### 1.4 Repo picker (multi-workspace from the phone) — **Effort: S**

The product is a *task inbox for your machine*, but it can only reach one
directory. All plumbing exists: the manifest supports multiple repos, the
worker registers them (`worker/main.go:90-96`), the server stores them
(`worker_repos` table) and serves them (`GET /api/repos`, `main.py:202-205`) —
the UI just never asks. Change: `new_job.html` gets a `<select>` populated from
registered repos (default preselected); `create_job_form` (`main.py:128-137`)
accepts `repo_alias`, validates it against `worker_repos`, stores it. The
worker already routes by `job.RepoAlias` (`runner.go:41-44`). This was
deliberately cut in commit `47df1af`-era simplification — un-cut it; it is the
single cheapest "this feels like a real product" feature.

#### 1.5 Fix the live-update UX — **Effort: S/M**

Current job detail page re-fetches the entire receipt fragment every 2s and
replaces `innerHTML` (`job_detail.html:20-28`). Consequences: user toggles the
"Live logs" or "Git diff" `<details>` open/closed → 2 seconds later it snaps
back; text selection in logs is destroyed mid-copy; scroll position in the log
pane resets; the full log payload re-renders every tick.

Minimal fix (S): persist `<details>` open-state across swaps (read state before
replace, reapply after) and stop polling once status is terminal. Better fix
(M): split the fragment — poll a tiny JSON status endpoint
(`/api/jobs/{id}?log_limit=0` already works) and only fetch *new* log lines
with an `after_log_id` param (mirror of the existing `before_log_id` pagination
in `db.py`), appending to the `<pre>` instead of replacing it. SSE is **not**
needed; polling appended-only is fine at this scale and keeps the server
dependency-free.

### Phase 2 — From "Gemini remote" to "agent inbox" (widen the funnel)

#### 2.1 First-class agent adapters: Claude Code, Codex CLI, opencode — **Effort: M**

Hard-coding Gemini caps the audience at Gemini CLI users; the largest agent-CLI
populations are Claude Code and Codex CLI. The `custom` +
`--command-template` mechanism (`runner.go:214-223`) already proves the
abstraction; what's missing is *named presets* so users don't hand-craft
templates:

- `--agent claude` → `claude -p {{prompt}} --output-format json --dangerously-skip-permissions` with a JSON-envelope parser like the existing `geminiResponseText` (`runner.go:234-247`).
- `--agent codex` → `codex exec {{prompt}} --json` equivalent.
- Refactor: extract an `AgentAdapter` interface (`Command(prompt) []string`,
  `ExtractResponse(output) (string, error)`, `NoiseFilter(line) bool`) so the
  Gemini-specific code (`runner.go:225-263`, noise filter at
  `runner.go:360-366`) becomes one of N adapters instead of special cases.
- Verify exact CLI flags against each tool's current docs at implementation
  time — do not trust this document's recollection of flags.

The receipt prompt (`runner.go:115-167`) is already agent-agnostic and needs no
change. Marketing consequence: the README headline changes from "runs Gemini"
to "runs *your* coding agent — Claude Code, Gemini, Codex, or any command".
That sentence is the product.

#### 2.2 Multiple named workers + per-worker tokens — **Effort: M**

Home desktop + work laptop + homelab box is the natural power-user shape, and
internal routing already keys on `worker_name` everywhere (jobs table,
`claim_next_job`). Missing pieces: UI worker selection on the new-job form
(pairs with 1.4's repo picker — repos are already registered per-worker), and
real per-worker credentials using the dormant `workers.token_hash` column
(`db.py:35`): an owner-only `POST /api/workers` mints a token, stores its hash,
worker auth checks per-worker hash with `compare_digest` fallback to the
global `WORKER_TOKEN` for back-compat. Revocation = delete row. This is also
the security story for "my worker token leaked" — today a leak means rotating
the single shared secret everywhere. Dashboard gets a workers panel:
name, last_seen (already tracked via `touch_worker`), online/offline badge,
registered repos, revoke button.

#### 2.3 Accept / Reject workflow — **Effort: M/L**

Already on the maintainer's list. Closing the review loop from the phone:

- `Accept` → worker (notified via the 1.2 heartbeat/poll channel) runs
  `git switch -c deaddrop/job-<id> && git add -A && git commit -m "<title>\n\n<summary>\n\nJob: <url>"`.
  **Always to a new branch, never the user's current branch** — preserves the
  "no surprise commits" trust contract (`spec.md` excludes auto-commit; a
  *human-approved* commit to a dedicated branch honors the spirit).
- `Reject` → records the verdict server-side; **do not** auto-revert the
  worktree by default (a `git checkout -- .` could destroy the user's own
  uncommitted work sitting in the same tree). Offer revert as an explicit
  second, scarier button gated on `git status` showing only job-related changes.
- Schema: `review_status` (pending/accepted/rejected) + `review_branch` columns.
- New job status flow: `completed` → user verdict → terminal.

This feature is what turns "look at a diff on your phone" into "ship from your
phone". It is also the riskiest trust surface in the roadmap — write its
executable plan with extreme care around the reject path.

#### 2.4 Scheduled / recurring jobs — **Effort: M**

"Every morning: bump deps, run tests, leave me the diff" turns DeadDrop from a
reactive tool into an autonomous maintenance product — a genuinely
differentiated use of the *always-on local worker* that cloud agents can't
match (they can't see your private registry or run your licensed toolchain).
Design: `job_schedules` table (`title, prompt, repo_alias, cron_expr, enabled,
last_spawned_at`); a lightweight scheduler loop in the server (FastAPI startup
task, check every minute, spawn a queued job when due — `croniter` dependency);
schedule CRUD page in the UI. Out of scope: timezone UI (store UTC, document
it), overlap policy beyond "skip if previous spawn still queued/running".

### Phase 3 — Product surface (feel like a product, not a repo)

#### 3.1 PWA: installable, offline-shell, push-ready — **Effort: S/M**

`base.html` has correct viewport meta but no manifest, no icons, no service
worker. Add `manifest.json` (name, theme `#BBFF00` on `#101010` to match the
existing brand), icons, apple-touch-icon, and a minimal service worker (cache
the shell, network-first for data). Result: "Add to Home Screen" gives an
app-like full-screen experience — the product *is* a phone app now, without an
app store. Prerequisite for Web Push (1.1 layer 2).

#### 3.2 Phone-first job creation: templates & quick actions — **Effort: S**

Typing prompts on a phone is the highest-friction step of the core loop.
`new_job.html` is a bare title+textarea. Add: (a) tappable prompt-template
chips above the form ("Run the test suite and report failures", "Fix the
failing tests", "Update dependencies and note breaking changes", "Review
uncommitted changes and summarize risks") that fill the textarea — hard-coded
list first, `prompt_templates` table later; (b) "Re-run" button on any job
detail page (pre-fills the form from that job); (c) auto-derive title from the
first line of the prompt so title becomes optional.

#### 3.3 `deaddrop` CLI client — **Effort: S**

`deaddrop drop "fix the flaky auth test"` from any terminal, plus
`deaddrop jobs` / `deaddrop show <id>` / `deaddrop diff <id> | git apply`.
Implement as subcommands in the existing Go binary (it already has the client
plumbing in `worker/client.go`, using `OWNER_TOKEN` instead of
`WORKER_TOKEN`). The `diff <id> | git apply` pipe is a sleeper feature: it
makes the receipt diff *portable* — review on phone, apply on any checkout.
Also unlocks scripting/CI integration for free.

#### 3.4 Diff & log presentation upgrade — **Effort: M**

The diff is the product's deliverable, and today it renders as a raw `<pre>`
(`_job_receipt.html:88`). Upgrade server-side (keep zero-JS-framework
discipline): parse the unified diff in Python into per-file sections —
collapsible per file, add/remove line coloring, file stats (+12 −3), "copy
diff" and "download .patch" buttons. Logs: stream-colored lines exist
(`log.stream` classes) but add stderr/system visual distinction, auto-scroll
toggle, and a duration ticker on running jobs. No client framework needed;
this is template + CSS work.

#### 3.5 Artifact retention — **Effort: M**

Agents produce more than diffs: test reports, screenshots, built binaries.
Worker uploads files the receipt lists under a new `artifacts` key (size-capped,
e.g. 5 MB each / 20 MB total) to a new `job_artifacts` table (Postgres bytea is
fine at this scale; don't add S3 complexity yet). Job page lists them with
download links. Listed last in this phase because no user has asked yet —
build behind interest.

### Phase 4 — Distribution & growth

- **Single-binary worker releases** (S): goreleaser → GitHub Releases for
  linux/mac/windows; `curl | sh` installer. Today "install the worker" means
  "have a Go toolchain", which filters out most of the audience.
- **`deaddrop init` setup wizard** (M): interactive first-run that writes the
  manifest, validates the server URL/token, runs a self-test job. The
  README's 5-minute deploy claim currently hides ~10 manual steps across
  Render + Supabase + tokens; the wizard owns the worker half of that.
- **Docker-compose self-host path** (S): one `docker compose up` for
  server+Postgres — for the homelab crowd, which is exactly the
  sovereignty-motivated audience.
- **Demo instance** (S): the deployed demo with a mock worker on a loop, reset
  hourly, so the landing page links to a *live* clickable job instead of a
  static `/demo` page.
- **Hosted multi-tenant SaaS**: explicitly **deferred** — see pmf.md. The spec
  excludes it for MVP and the sovereignty positioning makes self-host-first
  the right wedge; revisit only with retention evidence.

---

## 4. UI/UX deep dive (current state, screen by screen)

The design language (dark `#101010`, signal-green `#BBFF00`, terminal motif) is
distinctive and worth keeping. The issues are interaction-level, not visual.

| Screen | Today (evidence) | Fix | Effort |
|---|---|---|---|
| Dashboard (`dashboard.html`) | No auto-refresh — phone users pull-to-refresh a page that doesn't refresh; raw ISO timestamps (`{{ job.created_at }}`); card shows internal jargon `local / default` (`dashboard.html:18`); no status filter or search | Poll a lightweight jobs JSON every ~10s and update badges in place; render relative times ("3m ago") with a tiny JS helper + `<time datetime>` for a11y; show repo *display name* (already in `worker_repos`) instead of alias; status filter chips (all/running/queued/done/failed) | S |
| Job detail (`job_detail.html:20-28`) | 2s full-fragment `innerHTML` swap resets `<details>` state, scroll, and text selection; polling continues forever even on terminal jobs | See 1.5. Also: render `job.prompt` with `white-space: pre-wrap` (currently a `<p>`, collapsing the user's formatting); add elapsed-time ticker for running jobs | S/M |
| Receipt (`_job_receipt.html`) | Good structure (result/files/verification/blockers grid). Diff is one raw `<pre>`; changed-file names are plain text | See 3.4. Link each `changed_files` entry to its section of the rendered diff | M |
| New job (`new_job.html`) | Bare title+textarea; nothing phone-optimized | See 3.2 (template chips, optional title, re-run). Add `autofocus`, `enterkeyhint="send"` | S |
| Login (`login.html`) | Paste a 44-char base64 token on a phone keyboard — the single worst moment of onboarding | Short-term: `autocomplete="current-password"` + `type=password` so password managers store it. Later: device-link flow — logged-in desktop session displays a QR encoding a one-time 5-minute link code; phone scans → session cookie issued. No OAuth needed, stays sovereign | S now, M later |
| Landing (`landing.html`) | Solid copy and terminal motif; references old `DEADDROP_RECEIPT` plain markers (landing.html terminal-lines) while the product now uses `DEADDROP_RECEIPT_JSON` | Refresh terminal mock to the JSON receipt; link the live demo instance (Phase 4) | S |
| Global (`base.html`) | No PWA manifest, no favicon/touch icons, no `theme-color` meta | See 3.1 | S |
| A11y (all) | Status communicated by badge color alone; log pane colors only differentiator for streams | Badge text already present (good); add `aria-live="polite"` on the status badge for screen-reader updates while polling; ensure `#BBFF00`-on-dark passes contrast for text-sized uses | S |

Guiding constraint worth preserving: **no client framework**. The entire UI is
server-rendered Jinja + ~30 lines of JS. Every fix above fits that budget; the
moment someone proposes React here, the self-host story (one Docker container,
no build step) gets worse.

---

## 5. Anti-roadmap (deliberate "no"s, with reasons)

- **Chat UI / conversational agent** — the job-first receipt model *is* the
  product taste (`spec.md`: "not a chatbot"). Follow-up tasks (1.3) capture
  90% of the value with 10% of the surface.
- **Remote terminal / shell access** — destroys the security story ("server
  never controls your machine, worker polls outbound") that justifies the
  entire architecture. Competitors already do this; it's their weakness.
- **Repo indexing / embeddings / code search** — the local agent already has
  the repo; indexing server-side would mean uploading code, breaking the
  "code never leaves your machine" invariant.
- **Arbitrary server-chosen paths** — manifest-only workspace mapping is a
  load-bearing security boundary (`spec.md`, `worker/README.md`); never let
  job payloads carry paths.
- **OAuth / multi-user orgs / billing** — not until pmf.md's retention
  signals fire. Single-owner sovereignty is the wedge, not the limitation.
- **WebSockets/SSE for logs** — appended-only polling (1.5) is sufficient at
  single-user scale and keeps the server boring and Render-friendly.

---

## 6. Recommended execution order

Sequenced for daily-usability first, audience-widening second:

| Order | Item | Effort | Why this position |
|---|---|---|---|
| 1 | Untracked-files diff gap (§1 correctness note) | S | Trust hole in the core deliverable |
| 2 | 1.5 live-update fix + UI table S-items | S/M | Daily-use friction, all small |
| 3 | 1.1 webhook notifications | S | Highest leverage per line of code in the repo |
| 4 | 1.2 running cancellation + stale-job detection | M | Deal-breaker remover; unlocks 2.3's verdict channel |
| 5 | 1.4 repo picker | S | Cheapest "real product" feel; pairs with 2.2 later |
| 6 | 1.3 follow-up tasks | M | Kills the biggest workflow dead end |
| 7 | 2.1 agent adapters (Claude Code, Codex) | M | Widen the funnel before promoting |
| 8 | 4.x single-binary releases + compose | S | Remove the Go-toolchain install filter, then *launch* (see pmf.md §6) |
| 9 | 2.3 accept/reject | M/L | Post-launch, with care |
| 10 | 2.2 multi-worker tokens, 2.4 schedules, 3.x | M | Driven by post-launch feedback |

Rate limiting on `/login` and the APIs (open `todo.md` item) should ride along
with item 8 — required before promoting a public-internet deployment, trivial
with `slowapi` or a fixed-window counter.

Each item above should become a self-contained executable plan
(`plans/NNN-<slug>.md` per the repo's plan template) before implementation;
this document is the map, not the turn-by-turn directions.
