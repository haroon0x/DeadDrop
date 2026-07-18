# DeadDrop — Path to Product-Market Fit

> **Written at**: commit `ea6c3f9`, 2026-06-11.
> Companion to [000-product-roadmap.md](000-product-roadmap.md) — that file is
> *what to build*; this file is *who it's for, why they'd switch, and how to
> find out cheaply whether they will*.
>
> Market-landscape claims below reflect the author's knowledge as of early
> 2026. **Re-verify the competitive section in the week before any launch** —
> this space moves monthly.

---

## 1. The problem, in the user's words

> "My coding agent runs in my terminal, on my machine, with my credentials and
> my weird local setup. The moment I stand up from the desk, it's useless. The
> cloud agents that *are* reachable from my phone can't touch my private
> registry, my licensed toolchain, my local database, or my homelab — and I
> don't want to upload my repo to someone else's sandbox anyway."

The agent-CLI wave (Claude Code, Gemini CLI, Codex CLI) made powerful
autonomous coding a *local terminal* experience. That created a new, real gap:
**asynchronous control of a local agent from somewhere else** — couch, commute,
school run — without giving anything up to a cloud sandbox.

DeadDrop's answer is deliberately narrow: a hosted *inbox* (prompts, logs,
receipts, diffs — never code, paths, or secrets) plus an outbound-polling local
worker. Leave a task; come back to the diff.

## 2. Who it's for (ICP, in order of fit)

1. **The sovereignty-minded solo developer** — runs Claude Code/Gemini CLI
   daily, self-hosts things (homelab, Tailscale, ntfy already in their life),
   constitutionally allergic to uploading repos to vendor sandboxes. Finds
   DeadDrop via HN/r/selfhosted. *This person is the wedge.* They tolerate
   setup friction, file good issues, and evangelize.
2. **The side-project parent/commuter** — has 20-minute windows away from the
   desk; wants to queue "fix the failing test" from the phone and review a
   diff later. Less tolerant of setup friction — reaches them only after
   roadmap Phase 4 (single-binary worker, compose file).
3. **The consultant/agency dev with client-confidential code** — contractually
   *cannot* use cloud agent sandboxes. Self-hosted inbox + code-never-leaves
   is a compliance story, not a preference. Small population, high willingness
   to pay; relevant for monetization (§7), not for launch.

Explicit non-targets for now: teams wanting shared queues/orgs/SSO, and
developers who are happy with cloud agent sandboxes (they're served; don't
fight that fight).

## 3. Jobs to be done

| Job | Today's workaround | DeadDrop's answer |
|---|---|---|
| "Kick off a coding task while away from the desk" | SSH+tmux over Tailscale from a phone (miserable typing, no review UX) | Phone web form → queued job |
| "Know when it's done" | Re-ssh and look | Notification (roadmap 1.1) — **does not exist yet; the promise is broken without it** |
| "Review what the agent did before it counts" | Read terminal scrollback | Receipt (status/files/verification/blockers) + captured diff, no auto-commit |
| "Keep an audit trail of what agents did to my repos" | Nothing | Persistent jobs with logs, receipts, diffs |
| "Run nightly maintenance with my local toolchain" | cron + hope | Scheduled jobs (roadmap 2.4) |

The receipt/audit angle is underrated in DeadDrop's own marketing: nobody else
in the local-agent-remote space produces a *structured, persistent record* of
what the agent claimed vs. what the diff shows. That's a trust artifact, and
trust is the whole sales pitch.

## 4. Competitive map

Two axes: **where the agent runs** (vendor cloud ↔ your machine) and **what
controls it** (vendor app ↔ self-hosted/your infra).

**Vendor cloud, vendor app** — OpenAI Codex cloud tasks, Google Jules, GitHub
Copilot coding agent, Cursor background agents, Devin, Claude Code on the web.
Polished, funded, zero-setup. Their structural weakness is the constraint they
can't remove: *your code and credentials live in their sandbox*, and private
infra (local DBs, internal registries, licensed tools) is out of reach.
DeadDrop does not compete with these; it serves the people they structurally
can't.

**Your machine, vendor-hosted control plane** — the directly adjacent
category: Happy (open-source Claude Code mobile client), Omnara (mission
control / mobile for Claude Code), Vibe Kanban (kanban for local agents),
Conductor (parallel local Claude Code on Mac), plus a steady stream of new
entrants. Differences that matter:

- Most are **Claude Code-specific**; DeadDrop is **agent-agnostic** (mock /
  Gemini / any command today; Claude Code & Codex presets are roadmap 2.1 and
  must ship before launch for this claim to be honest).
- Most are **interactive/chat-shaped** (mirror the session to your phone);
  DeadDrop is **job-shaped** (queue → receipt → diff). Interactive mirroring
  is better for babysitting a long session; job-shaped is better for
  fire-and-forget plus audit. This is a genuine taste fork, not a feature gap
  — own it in messaging.
- Most route through **their** relay servers; DeadDrop's control plane is
  **self-hosted on your own free-tier Render/Supabase** (or compose, roadmap
  Phase 4). For ICP #1 this is the deciding feature.

**Your machine, your infra (DIY)** — Tailscale+SSH+tmux, VibeTunnel-style
terminal relays. Free and sovereign, but no review UX, no receipts, no queue,
no notifications. DeadDrop's pitch to these users: "keep the sovereignty,
gain the product."

**Positioning sentence**: *DeadDrop is the self-hosted, agent-agnostic task
inbox for the coding agent on your own machine — leave a task from your phone,
come back to a receipt and a diff. No code, paths, or secrets ever leave your
hardware.*

## 5. PMF hypotheses and the riskiest assumptions

- **H1 (value)**: developers who already run agent CLIs want *asynchronous,
  phone-reachable* control badly enough to deploy two components (hosted inbox
  + local worker). — *Riskiest assumption: the async/job-shaped model is what
  they want, vs. interactive session mirroring (Happy/Omnara shape). Evidence
  either way is currently zero; get it from launch-comment sentiment and from
  whether follow-up tasks (roadmap 1.3) get heavy use (heavy follow-up usage
  ≈ users wanting a conversation ≈ pressure toward interactive).*
- **H2 (differentiation)**: "self-hosted + agent-agnostic + receipts" beats
  "polished but vendor-locked" for a reachable niche (~ICP #1). — *Riskiest
  assumption: that niche is bigger than a few hundred people. r/selfhosted +
  HN reception is the cheap proxy.*
- **H3 (friction)**: setup (Render + Supabase + tokens + worker) can get under
  ~15 minutes without losing the sovereignty story. — *Today it's realistically
  30–60 min with sharp edges (token generation, pooled connection strings,
  Go toolchain). Roadmap Phase 4 exists to fix this; H3 is untestable before
  it ships.*
- **H4 (retention)**: once a user completes ~3 jobs from the phone, weekly
  usage sticks. — *Pure guess; instrument and watch (§6).*

## 6. Validation plan (cheap, sequenced)

**Gate 0 — make the promise true before measuring anything.** Launching
without notifications (1.1), running-cancel (1.2), Claude Code support (2.1),
and the one-line worker install (Phase 4) would test a strawman. Minimum
launchable product = roadmap execution-order items 1–8.

**Gate 1 — founder dogfood (now → launch).** The maintainer uses DeadDrop for
real tasks daily for 2+ weeks. Log every friction moment as an issue. If *you*
reach for SSH instead of DeadDrop even once, that's a finding — write down why.

**Gate 2 — 5–10 design partners (pre-launch).** Recruit from agent-CLI Discord
/ r/ClaudeAI / r/selfhosted. Watch (don't help) two of them do first-run setup
over a screen share — H3 evidence. Success: ≥7/10 reach a completed
phone-created job; ≥4/10 still creating jobs in week 3 unprompted.

**Gate 3 — public launch.** Show HN ("Self-hosted inbox for your local coding
agent — leave a task from your phone, come back to the diff") + r/selfhosted +
agent-CLI communities. The README demo video already exists and is good; add
the live demo instance (roadmap Phase 4). Success signals: not stars —
**worker downloads, and issues that describe *workflows* rather than bugs**.

**Metrics** (instrument before Gate 2; a self-hosted product can't phone home,
so measure via: opt-in anonymous ping at worker startup — version + agent mode
only, documented and off-by-default; GitHub release download counts; and a
funnel you can see on the *demo* instance):

```
deploy server → worker first poll → first job created → first job completed
→ first job created from a phone UA → notifications configured → week-4 job
```

Activation = first completed phone-created job. Retention = ≥1 job/week in
weeks 2–4. North star = **diffs reviewed per user per week**.

**Kill / pivot criteria** (decide in advance, written here so it's honest):
if after a real launch (Gate 3) and 90 days, fewer than ~50 externally-run
workers ever activate, or week-4 retention of activated users is <10%, stop
scaling effort: either (a) pivot the shape toward what the evidence says
(likely: interactive mirroring or deeper scheduled-maintenance autonomy), or
(b) declare it a sharpened portfolio piece and maintain it as such — which is
a legitimate outcome, not a failure.

## 7. Monetization (honest options, none urgent)

The wedge audience self-hosts and expects free OSS. Monetizing before
retention evidence would burn the only growth channel (community goodwill).
Paths, in order of compatibility with the positioning:

1. **OSS + GitHub Sponsors** — costs nothing, signals health. Do at launch.
2. **Hosted convenience tier** ("we run the inbox, your worker stays yours",
   $5–8/mo) — preserves the code-stays-local story because the architecture
   already keeps code off the server; sells operational laziness. Only after
   retention proof; requires multi-tenant auth the spec deliberately excludes
   today.
3. **Team tier** (shared queue, multiple owners, audit export, SSO) — sells
   the *receipt/audit trail* to ICP #3 (client-confidential consultancies).
   Real willingness-to-pay, but it drags in orgs/roles/billing; revisit at
   PMF, not before.

Sequencing rule: **prove weekly retention with free self-hosters first**;
every monetization path above survives that delay, and none survives skipping
it.

## 8. Messaging crib sheet

- Tagline (keep — it's the best asset): **"Leave a coding task. Come back to
  the diff."**
- One-liner: *Self-hosted task inbox for the coding agent on your machine.*
- Three bullets that survive contact with a skeptical HN commenter:
  - Your agent, your machine, your credentials — the server only ever sees
    prompts, logs, receipts, and diffs (and you host the server too).
  - Worker polls outbound; nothing on the internet can reach into your box.
  - Every task ends in a receipt: status, changed files, verification run,
    blockers — and the diff. No auto-commit, ever.
- Words to avoid: "chatbot", "assistant", "platform", "AI-powered". DeadDrop
  is a queue with receipts; the restraint *is* the brand.
