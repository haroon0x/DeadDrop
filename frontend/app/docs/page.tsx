import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "@phosphor-icons/react/dist/ssr";

export const metadata: Metadata = { title: "Documentation" };

const sections: { id: string; n: string; title: string; body: React.ReactNode }[] = [
  {
    id: "requirements",
    n: "01",
    title: "Requirements",
    body: (
      <p>
        You need Docker with Compose, Git, and either a released worker binary
        or Go 1.22 and above. Gemini CLI is only required for Gemini mode.
      </p>
    ),
  },
  {
    id: "server",
    n: "02",
    title: "Start a durable server",
    body: (
      <>
        <p>
          The Compose stack runs FastAPI and PostgreSQL. Secrets stay in your
          shell environment; database state stays in a named volume.
        </p>
        <pre>
          <code>{`export OWNER_TOKEN="$(openssl rand -hex 32)"
export WORKER_TOKEN="$(openssl rand -hex 32)"
export POSTGRES_PASSWORD="$(openssl rand -hex 32)"
docker compose up -d --build`}</code>
        </pre>
        <p>
          Open <code>http://localhost:8000/login</code> and enter{" "}
          <code>OWNER_TOKEN</code>.
        </p>
      </>
    ),
  },
  {
    id: "worker",
    n: "03",
    title: "Build or download the worker",
    body: (
      <>
        <pre>
          <code>{`cd worker
go build -o deaddrop-worker .
./deaddrop-worker version`}</code>
        </pre>
        <p>
          Tagged releases provide Linux, macOS, and Windows binaries with
          SHA-256 checksums.
        </p>
      </>
    ),
  },
  {
    id: "manifest",
    n: "04",
    title: "Create the trust boundary",
    body: (
      <>
        <p>
          The manifest is local. The server receives the alias{" "}
          <code>default</code>, not the absolute path.
        </p>
        <pre>
          <code>{`./deaddrop-worker init \\
  --repo /absolute/path/to/project \\
  --verify "go test ./..."`}</code>
        </pre>
      </>
    ),
  },
  {
    id: "run",
    n: "05",
    title: "Run the worker",
    body: (
      <>
        <pre>
          <code>{`./deaddrop-worker run \\
  --server http://localhost:8000 \\
  --token "$WORKER_TOKEN" \\
  --manifest deaddrop.manifest.json \\
  --agent gemini`}</code>
        </pre>
        <p>
          Use <code>--agent mock</code> for the bundled deterministic demo, or
          custom mode for another CLI.
        </p>
      </>
    ),
  },
  {
    id: "receipts",
    n: "06",
    title: "Read the receipt",
    body: (
      <div className="defs">
        <div>
          <strong>Summary</strong>
          <p>The agent&apos;s human-readable account.</p>
        </div>
        <div>
          <strong>Changed files</strong>
          <p>Derived from the baseline-relative Git diff.</p>
        </div>
        <div>
          <strong>Verification</strong>
          <p>Commands and exit states observed by the worker.</p>
        </div>
        <div>
          <strong>Patch</strong>
          <p>A scoped binary diff; never auto-applied.</p>
        </div>
      </div>
    ),
  },
  {
    id: "apply",
    n: "07",
    title: "Apply a result deliberately",
    body: (
      <>
        <p>
          Download the patch, commit or stash unrelated work, then inspect it
          from the configured workspace.
        </p>
        <pre>
          <code>{`git apply --stat /path/to/job.patch
git apply --check /path/to/job.patch
git apply /path/to/job.patch`}</code>
        </pre>
      </>
    ),
  },
];

export default function Docs() {
  return (
    <>
      <div className="shell">
        <section className="page-hero">
          <p className="label label-acid bracket">Documentation</p>
          <h1 className="d2">From zero to a verified local-agent queue.</h1>
          <p className="lede">
            Run the server, configure one trusted Git workspace, and make the
            worker prove what changed.
          </p>
        </section>

        <div className="docs">
          <nav className="docs-nav">
            {sections.map((section) => (
              <a key={section.id} href={`#${section.id}`}>
                {section.title}
              </a>
            ))}
          </nav>

          <div className="docs-body">
            {sections.map((section) => (
              <section key={section.id} id={section.id}>
                <span>{section.n}</span>
                <h2>{section.title}</h2>
                {section.body}
              </section>
            ))}

            <section>
              <span>08</span>
              <h2>Go deeper</h2>
              <div className="link-stack">
                <Link href="/docs/architecture">
                  <strong>Architecture</strong>
                  <small>
                    Attempts, worktrees, receipts, and trust boundaries
                  </small>
                </Link>
                <Link href="https://github.com/haroon0x/DeadDrop/blob/main/docs/deployment.md">
                  <strong>Deployment guide</strong>
                  <small>Compose, Render, upgrades, secrets, and backups</small>
                </Link>
              </div>
              <Link className="tlink" href="/demo">
                See a finished receipt
                <ArrowRight size={13} weight="bold" />
              </Link>
            </section>
          </div>
        </div>
      </div>
    </>
  );
}
