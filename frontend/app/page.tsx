import Image from "next/image";
import Link from "next/link";
import {
  ArrowRight,
  ArrowsClockwise,
  CheckSquareOffset,
  HardDrives,
  Prohibit,
  ShieldCheck,
  TreeStructure,
} from "@phosphor-icons/react/dist/ssr";
import { articles } from "@/lib/articles";
import { marks } from "@/lib/marks";

const steps: [string, string, string][] = [
  [
    "01",
    "Drop a task",
    "Use the private browser inbox from your phone or desktop. The server stores an alias, never your local path.",
  ],
  [
    "02",
    "Claim with a lease",
    "Your worker polls outbound, receives a unique attempt, and keeps ownership alive with heartbeats.",
  ],
  [
    "03",
    "Work in isolation",
    "The agent runs in a detached worktree at source HEAD. Local edits and untracked files stay outside.",
  ],
  [
    "04",
    "Review the evidence",
    "DeadDrop returns logs, verification, changed files, and a binary patch. It never commits or merges.",
  ],
];

const transcript = [
  ["system", "resolved baseline 9f2c1ab on route alias default"],
  ["system", "created detached worktree /tmp/deaddrop-job-184/workspace"],
  ["agent", "editing server/app/results.py"],
  ["verify", "go test ./... "],
  ["stdout", "ok  deaddrop/worker/internal/result  0.412s"],
  ["verify", "exit 0"],
  ["system", "captured binary patch, 2 files changed"],
  ["system", "removed worktree, source workspace untouched"],
] as const;

export default function Home() {
  return (
    <>
      <section className="hero">
        <div className="hero-head">
          <p
            className="label label-acid bracket enter"
            style={{ "--i": 0 } as React.CSSProperties}
          >
            Self-hosted coding task inbox
          </p>
          <h1 className="d1 enter" style={{ "--i": 1 } as React.CSSProperties}>
            Leave a task.
            <br />
            Come back to <span className="acid">evidence</span>.
          </h1>
        </div>

        <div className="hero-lower">
          <div className="hero-copy">
            <p className="lede enter" style={{ "--i": 2 } as React.CSSProperties}>
              DeadDrop runs your local coding agent in a disposable Git worktree
              and returns a patch you review before anything is applied.
            </p>
            <div
              className="actions enter"
              style={{ "--i": 3 } as React.CSSProperties}
            >
              <Link className="btn" href="/docs">
                Run it yourself
                <ArrowRight size={14} weight="bold" />
              </Link>
              <Link className="btn-ghost" href="/demo">
                See a receipt
              </Link>
            </div>
          </div>

          <figure className="hero-figure">
            <Image
              src="/img/converge.webp"
              alt="An abstract plotter drawing of many parallel paths converging through a single narrow channel and fanning out again"
              width={1600}
              height={880}
              priority
            />
            <figcaption className="label">
              Many tasks in. One attempt at a time. Evidence out.
            </figcaption>
          </figure>
        </div>
      </section>

      <div className="shell">
        <div className="strip">
          <p>Runs on</p>
          <ul>
            {marks.map((mark) => (
              <li key={mark.slug}>
                <svg viewBox="0 0 24 24" role="img" aria-label={mark.name}>
                  <path d={mark.d} />
                </svg>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <section className="shell band" id="how-it-works">
        <div className="sec-head reveal">
          <p className="label label-acid bracket">How it works</p>
          <h2 className="d2">
            A queue on the web.
            <br />
            Execution on your machine.
          </h2>
          <p className="body">
            The server coordinates. The worker decides which local path is
            trusted and produces the evidence you review.
          </p>
        </div>
        <figure className="sheet reveal">
          <Image
            src="/img/loop.webp"
            alt="Technical drawing of the DeadDrop job path: browser to server, worker polling outbound, a detached worktree, and the receipt returning to the owner"
            width={1600}
            height={860}
          />
          <figcaption className="label">
            The whole path on one sheet
          </figcaption>
        </figure>

        <div className="module steps reveal">
          {steps.map(([n, title, copy]) => (
            <article key={n}>
              <span>{n}</span>
              <h3>{title}</h3>
              <p>{copy}</p>
            </article>
          ))}
        </div>
      </section>

      <hr className="hr" />

      <section className="shell band">
        <div className="split">
          <figure className="figure-block plate reveal">
            <Image
              src="/img/isolation.webp"
              alt="A triangulated structural lattice, each cell held separate by its own frame"
              width={1400}
              height={1050}
            />
            <figcaption className="figure-tag">
              <span className="label">Source HEAD</span>
              <span className="label label-acid">Detached worktree</span>
            </figcaption>
          </figure>
          <div className="split-copy reveal">
            <h2 className="d2">
              Autonomy without source-directory roulette.
            </h2>
            <p className="body">
              DeadDrop treats your working directory as source material, not a
              scratchpad. Jobs start from the current commit in a temporary
              worktree, so half-finished edits and untracked files never enter
              the diff.
            </p>
            <dl className="ledger">
              <div>
                <dt>Baseline</dt>
                <dd>Source HEAD at claim time</dd>
              </div>
              <div>
                <dt>Working copy</dt>
                <dd>Detached, temporary, removed after</dd>
              </div>
              <div>
                <dt>Your dirty files</dt>
                <dd>Never touched, never staged</dd>
              </div>
            </dl>
            <Link className="tlink" href="/blog/disposable-worktrees">
              Why disposable worktrees matter
              <ArrowRight size={13} weight="bold" />
            </Link>
          </div>
        </div>
      </section>

      <section className="shell band">
        <div className="sec-head reveal">
          <h2 className="d2">The receipt is the product.</h2>
          <p className="body">
            Summaries come from the agent. Changed files come from Git.
            Verification comes from commands the worker actually ran, and status
            comes from the exit code it observed.
          </p>
        </div>
        <div className="term reveal">
          <pre>
            {transcript.map(([channel, line], i) => (
              <div key={i}>
                <b>{channel.padEnd(7)}</b>
                <i>{line}</i>
              </div>
            ))}
          </pre>
          <div className="term-side">
            <span className="pill">
              <CheckSquareOffset size={13} weight="bold" />
              Completed
            </span>
            <dl className="ledger">
              <div>
                <dt>Changed files</dt>
                <dd>2</dd>
              </div>
              <div>
                <dt>Verification</dt>
                <dd>exit 0</dd>
              </div>
              <div>
                <dt>Patch</dt>
                <dd>Ready to review</dd>
              </div>
              <div>
                <dt>Source workspace</dt>
                <dd>Untouched</dd>
              </div>
            </dl>
            <Link className="tlink" href="/demo">
              Open the full receipt
              <ArrowRight size={13} weight="bold" />
            </Link>
          </div>
        </div>
      </section>

      <section className="shell band">
        <div className="sec-head reveal">
          <h2 className="d2">Built to survive a bad night.</h2>
          <p className="body">
            Networks drop, workers die, and laptops close their lids. Every one
            of those is a state the queue already knows how to leave.
          </p>
        </div>
        <div className="bento reveal">
          <div className="b-wide">
            <ArrowsClockwise size={22} weight="light" />
            <h3>Recoverable attempts</h3>
            <p>
              Expired leases requeue the work. A stale worker cannot overwrite a
              newer attempt, because the attempt ID it holds is already invalid.
            </p>
          </div>
          <div className="b-tall b-figure">
            <Image
              src="/img/queue.webp"
              alt="A rail junction where many tracks converge under signal gantries"
              width={1600}
              height={1000}
            />
          </div>
          <div className="b-mid">
            <HardDrives size={22} weight="light" />
            <h3>Durable result replay</h3>
            <p>
              A network failure spools the terminal result to disk and replays
              it before new work is claimed.
            </p>
          </div>
          <div className="b-mid b-dither">
            <Prohibit size={22} weight="light" />
            <h3>Real cancellation</h3>
            <p>
              A cancellation reaches the heartbeat loop and kills the local
              command process group.
            </p>
          </div>
          <div className="b-half">
            <TreeStructure size={22} weight="light" />
            <h3>Outbound only</h3>
            <p>
              No tunnel and no inbound port on your machine. The worker opens
              every connection.
            </p>
          </div>
          <div className="b-half">
            <ShieldCheck size={22} weight="light" />
            <h3>Self-hosted state</h3>
            <p>
              FastAPI and PostgreSQL under Compose, or whatever host you already
              trust.
            </p>
          </div>
        </div>
      </section>

      <section className="shell band">
        <div className="hazard reveal" />
        <div className="trust reveal">
          <div className="trust-figure">
            <Image
              src="/img/boundary.webp"
              alt="The riveted steel understructure of a bridge tower seen from below"
              width={1200}
              height={1500}
            />
          </div>
          <div className="trust-copy">
            <p className="label label-acid">Read this before you run it</p>
            <h2 className="d3">
              DeadDrop isolates Git state. It does not sandbox your machine.
            </h2>
            <p className="body">
              A worktree is a repository boundary, not a container. The agent
              still inherits the filesystem, network, tools, and credentials
              available to the worker account.
            </p>
            <p className="body">
              Run a dedicated non-root user, expose only the repositories you
              intend, and read every patch before you apply it.
            </p>
            <Link
              className="tlink"
              href="https://github.com/haroon0x/DeadDrop/blob/main/SECURITY.md"
            >
              Read the security model
              <ArrowRight size={13} weight="bold" />
            </Link>
          </div>
        </div>
      </section>

      <section className="shell band">
        <div className="split" style={{ alignItems: "start" }}>
          <div className="split-copy reveal">
            <h2 className="d2">Notes from building it.</h2>
            <p className="body">
              Worktrees, receipts, leases, and the unglamorous details between a
              prompt and a patch.
            </p>
            <figure className="figure-block plate" style={{ marginTop: 32 }}>
              <Image
                src="/img/taxonomy.webp"
                alt="Machine components sorted into neat columns by type and size"
                width={1400}
                height={900}
              />
            </figure>
          </div>
          <ul className="index-list reveal">
            {articles.map((article) => (
              <li key={article.slug}>
                <Link href={`/blog/${article.slug}`}>
                  <div>
                    <h3>{article.title}</h3>
                    <p>{article.deck}</p>
                  </div>
                  <div className="index-meta">
                    <span className="label label-acid">{article.category}</span>
                    <span className="label">{article.minutes}</span>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </section>

      <section className="shell">
        <div className="closing">
          <h2 className="d2 reveal">
            Stop watching the terminal. Start reading the receipt.
          </h2>
          <p className="lede reveal">
            One Compose file, one worker binary, one trusted repository alias.
          </p>
          <Link className="btn reveal" href="/docs">
            Run it yourself
            <ArrowRight size={14} weight="bold" />
          </Link>
        </div>
      </section>
    </>
  );
}
