import type { Metadata } from "next";
import Image from "next/image";
import Link from "next/link";
import { ArrowRight } from "@phosphor-icons/react/dist/ssr";

export const metadata: Metadata = { title: "Architecture" };

const principles: [string, string, string][] = [
  [
    "01",
    "Paths stay local",
    "The server stores a route alias. Only the worker knows which absolute directory it maps to.",
  ],
  [
    "02",
    "Edits stay disposable",
    "The source workspace is read-only during the job. Agents work from a detached baseline worktree.",
  ],
  [
    "03",
    "Facts come from tools",
    "Git supplies changed files and the patch. Trusted commands supply verification status.",
  ],
  [
    "04",
    "Results survive networks",
    "Bounded retries fall back to an atomic local spool that replays before new claims.",
  ],
];

export default function Architecture() {
  return (
    <div className="shell">
      <section className="page-hero">
        <p className="label label-acid bracket">Architecture</p>
        <h1 className="d2">
          The server coordinates.
          <br />
          The worker proves.
        </h1>
        <p className="lede">
          DeadDrop keeps public job state separate from local execution and
          makes every handoff explicit.
        </p>
      </section>

      <section className="band">
        <figure className="sheet reveal">
          <Image
            src="/img/loop.webp"
            alt="Technical drawing of the DeadDrop job path: browser to server, worker polling outbound, a detached worktree, and the receipt returning to the owner"
            width={1600}
            height={860}
          />
          <figcaption className="label">Job execution path</figcaption>
        </figure>
      </section>

      <hr className="hr" />

      <section className="band">
        <div className="split">
          <div className="split-copy reveal">
            <h2 className="d2">A job can retry without accepting a ghost result.</h2>
            <p className="body">
              If a lease expires, the attempt becomes <strong>lost</strong> and
              the job returns to queued. A new attempt ID makes late writes from
              the vanished worker invalid.
            </p>
          </div>
          <figure className="figure-block plate reveal">
            <Image
              src="/img/queue.webp"
              alt="A rail junction where many tracks converge under signal gantries"
              width={1600}
              height={1000}
            />
            <figcaption className="figure-tag">
              <span className="label">One attempt owns the track</span>
            </figcaption>
          </figure>
        </div>

        <div className="states reveal" style={{ marginTop: 40 }}>
          <div>
            <b>Queued</b>
            <span>Waiting for a worker</span>
          </div>
          <div>
            <b>Running</b>
            <span>A unique attempt owns the lease</span>
          </div>
          <div>
            <b>Completed, failed, cancelled</b>
            <span>Evidence retained either way</span>
          </div>
        </div>
      </section>

      <section className="band">
        <div className="sec-head reveal">
          <h2 className="d2">Four rules the system does not bend.</h2>
        </div>
        <div className="module steps reveal">
          {principles.map(([n, title, copy]) => (
            <article key={n}>
              <span>{n}</span>
              <h3>{title}</h3>
              <p>{copy}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="band">
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
            <p className="label label-acid">Trust boundary</p>
            <h2 className="d3">Isolation is precise, not magical.</h2>
            <p className="body">
              The worktree protects source Git state. It does not sandbox the
              filesystem, network, credentials, or tools available to the worker
              user.
            </p>
            <p className="body">
              Run a dedicated non-root worker, expose only intended
              repositories, and review every returned patch.
            </p>
            <Link
              className="tlink"
              href="https://github.com/haroon0x/DeadDrop/blob/main/SECURITY.md"
            >
              Read the security policy
              <ArrowRight size={13} weight="bold" />
            </Link>
          </div>
        </div>
      </section>
    </div>
  );
}
