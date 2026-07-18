import Link from "next/link";

const steps = [
  ["01", "Drop a task", "Use the private browser inbox from your phone or desktop. The server stores an alias, never your local path."],
  ["02", "Claim with a lease", "Your worker polls outbound, receives a unique attempt, and keeps ownership alive with heartbeats."],
  ["03", "Work in isolation", "The agent runs in a detached worktree at source HEAD. Local edits and untracked files stay outside."],
  ["04", "Review the evidence", "DeadDrop returns logs, verification, changed files, and a binary patch. It never commits or merges."],
];

const features = [
  ["↻", "Recoverable attempts", "Expired leases requeue work. Stale workers cannot overwrite a newer attempt."],
  ["✓", "Evidence-based receipts", "Changed files come from Git. Verification comes from commands the worker actually ran."],
  ["⇣", "Durable result replay", "Network failure stores terminal results locally and replays them before more work is claimed."],
  ["×", "Real cancellation", "A running cancellation reaches the heartbeat loop and kills the local command process group."],
  ["↗", "Outbound only", "No tunnel and no inbound port on your developer machine. The worker initiates every connection."],
  ["⌁", "Self-hosted state", "Run FastAPI with durable PostgreSQL using Compose or your preferred hosting provider."],
];

export default function Home() {
  return <>
    <section className="hero"><div className="hero-copy reveal"><p className="eyebrow"><span /> Self-hosted coding task inbox</p><h1>Leave a task.<br /><em>Come back to evidence.</em></h1><p className="lede">DeadDrop runs your local coding agent in a disposable Git worktree, verifies what happened, and returns a patch without touching your working directory.</p><div className="actions"><Link className="button" href="/docs">Run it yourself <span>→</span></Link><Link className="ghost-button" href="/demo">Explore a receipt</Link></div><div className="proof"><span>Outbound only</span><span>Source untouched</span><span>Worker verified</span></div></div>
      <div className="receipt-window reveal delay"><div className="window-top"><span><i /> job / 184</span><b>completed</b></div><div className="receipt-task"><small>Task</small><strong>Fix the retry race in result delivery</strong></div><div className="receipt-flow"><div><span>01</span><p><strong>Attempt claimed</strong><small>lease 60s · heartbeat 5s</small></p></div><div><span>02</span><p><strong>Worktree isolated</strong><small>source HEAD · dirty files excluded</small></p></div><div><span>03</span><p><strong>Verification passed</strong><small>go test ./... · exit 0</small></p></div></div><div className="receipt-result"><span><small>Changed</small><strong>2 files</strong></span><span><small>Patch</small><strong>ready</strong></span><span><small>Source</small><strong>untouched</strong></span></div></div>
    </section>
    <div className="manifesto"><p>Built for the gap between <em>“let an agent work”</em> and <em>“trust whatever it says.”</em></p></div>
    <section className="section" id="how-it-works"><div className="section-head"><p className="eyebrow">How it works</p><h2>A queue on the web.<br />Execution on your machine.</h2><p>The server coordinates. The worker decides which local path is trusted and produces the evidence you review.</p></div><div className="step-grid">{steps.map(([n, title, copy]) => <article key={n}><span>{n}</span><h3>{title}</h3><p>{copy}</p></article>)}</div></section>
    <section className="section split"><div className="section-head"><p className="eyebrow">A narrower promise</p><h2>Agent autonomy without source-directory roulette.</h2><p>DeadDrop treats your working directory as source material, not a scratchpad.</p><Link className="text-link" href="/blog/disposable-worktrees">Why disposable worktrees matter →</Link></div><div className="isolation"><div><small>Source repo</small><strong>your current HEAD</strong><span>dirty + untracked preserved</span></div><b>detached worktree <i>→</i></b><div className="accent-node"><small>Job workspace</small><strong>agent edits here</strong><span>verify · diff · remove</span></div></div></section>
    <section className="section"><div className="section-head compact"><p className="eyebrow">Reliability is the product</p><h2>The boring machinery is visible.</h2></div><div className="feature-grid">{features.map(([glyph, title, copy]) => <article key={title}><span>{glyph}</span><h3>{title}</h3><p>{copy}</p></article>)}</div></section>
    <section className="section open-callout"><div><p className="eyebrow">Open source, not a SaaS funnel</p><h2>Own the inbox. Own the worker. Keep the review gate.</h2><p>GPL-3.0 software for individuals and small trusted teams. No billing, automatic merge, or remote shell disguised as chat.</p></div><div className="actions"><Link className="button" href="/docs">Read the quickstart</Link><Link className="ghost-button" href="https://github.com/haroon0x/DeadDrop">Browse the source</Link></div></section>
  </>;
}
