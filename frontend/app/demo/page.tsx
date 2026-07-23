import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, CheckSquareOffset } from "@phosphor-icons/react/dist/ssr";

export const metadata: Metadata = { title: "Demo receipt" };

const log = `system  created isolated Git worktree at baseline 4c17e02
agent   editing app.py
verify  python -m pytest
stdout  test_app.py .                                    [100%]
verify  exit 0
system  captured binary patch, 1 file changed
system  reporting completed result`;

const patch = `diff --git a/app.py b/app.py
--- a/app.py
+++ b/app.py
@@ -1,2 +1,2 @@
 def add(a, b):
-    return a - b
+    return a + b`;

export default function Demo() {
  return (
    <div className="shell">
      <section className="page-hero">
        <p className="label label-acid bracket">Demo receipt</p>
        <h1 className="d2">See the evidence before running a worker.</h1>
        <p className="lede">
          A static example showing the exact shape of a completed DeadDrop job.
        </p>
      </section>

      <section className="band">
        <div className="ledger reveal" style={{ marginBottom: 40 }}>
          <div>
            <dt>Job 184</dt>
            <dd>Fix the failing add test</dd>
          </div>
        </div>

        <div className="demo-grid reveal">
          <article>
            <span className="pill">
              <CheckSquareOffset size={13} weight="bold" />
              Completed
            </span>
            <strong>
              Corrected the add implementation and verified the focused test
              suite.
            </strong>
            <p>The source workspace was not modified.</p>
          </article>
          <article>
            <span className="label">Changed files</span>
            <code>app.py</code>
            <p>Derived from the baseline-relative Git diff, not the summary.</p>
          </article>
          <article>
            <span className="label">Verification</span>
            <strong>python -m pytest</strong>
            <p>Passed, exit 0, observed by the worker.</p>
          </article>
        </div>
      </section>

      <section className="band" style={{ paddingTop: 0 }}>
        <div className="split reveal" style={{ alignItems: "start" }}>
          <div>
            <p className="label" style={{ marginBottom: 14 }}>
              Worker log
            </p>
            <pre>
              <code>{log}</code>
            </pre>
          </div>
          <div>
            <p className="label" style={{ marginBottom: 14 }}>
              Returned patch
            </p>
            <pre>
              <code>{patch}</code>
            </pre>
          </div>
        </div>

        <div style={{ marginTop: 44 }}>
          <Link className="btn" href="/docs">
            Run it yourself
            <ArrowRight size={14} weight="bold" />
          </Link>
        </div>
      </section>
    </div>
  );
}
