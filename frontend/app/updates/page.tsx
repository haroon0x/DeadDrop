import type { Metadata } from "next";

export const metadata: Metadata = { title: "Updates" };

const updates: [string, string, string, string][] = [
  [
    "Reliability foundation",
    "Agent jobs became recoverable units of work.",
    "Added attempts, sixty-second leases, five-second heartbeats, cancellation, stale recovery, and idempotent terminal writes.",
    "server · worker · database",
  ],
  [
    "Evidence layer",
    "Receipts stopped trusting agent claims.",
    "Changed files now come from Git, verification comes from configured worker commands, and status comes from observed exit state.",
    "receipts · verification",
  ],
  [
    "Source isolation",
    "Every job moved into a disposable worktree.",
    "Jobs start from source HEAD, preserve dirty and untracked files, capture binary patches, and remove their temporary worktree.",
    "git · safety",
  ],
  [
    "Open-source distribution",
    "The repository gained a real operator path.",
    "Durable Compose deployment, migrations, worker setup, release binaries, checksums, contributor docs, support, and security reporting.",
    "operations · community",
  ],
];

export default function Updates() {
  return (
    <div className="shell">
      <section className="page-hero">
        <p className="label label-acid bracket">Project updates</p>
        <h1 className="d2">
          Building the boring parts until they become the product.
        </h1>
        <p className="lede">
          A public record of reliability, distribution, and interface work.
        </p>
      </section>

      <section className="updates band" style={{ borderTop: 0 }}>
        {updates.map(([label, title, copy, tags]) => (
          <article key={label} className="reveal">
            <div>
              <span className="label label-acid">{label}</span>
            </div>
            <div>
              <h2>{title}</h2>
              <p>{copy}</p>
              <p className="label" style={{ marginTop: 16 }}>
                {tags}
              </p>
            </div>
          </article>
        ))}
      </section>
    </div>
  );
}
