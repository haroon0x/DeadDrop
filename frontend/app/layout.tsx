import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "DeadDrop · Verified coding tasks for local agents", template: "%s · DeadDrop" },
  description: "A self-hosted coding task inbox that runs local agents in disposable Git worktrees and returns evidence-backed patches.",
};

const nav = [["How it works", "/#how-it-works"], ["Docs", "/docs"], ["Architecture", "/docs/architecture"], ["Blog", "/blog"]];

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body><header className="site-header"><Link className="brand" href="/"><span className="brand-mark">DD</span><span>DeadDrop</span></Link><nav>{nav.map(([label, href]) => <Link key={href} href={href}>{label}</Link>)}</nav><Link className="button small" href="https://github.com/haroon0x/DeadDrop">View on GitHub <span>↗</span></Link></header><main>{children}</main><footer className="site-footer"><div><Link className="brand" href="/"><span className="brand-mark">DD</span><span>DeadDrop</span></Link><p>Local agents. Disposable worktrees. Reviewable evidence.</p></div><div className="footer-links"><Link href="/docs">Docs</Link><Link href="/updates">Updates</Link><Link href="/blog">Writing</Link><Link href="https://github.com/haroon0x/DeadDrop/blob/main/SECURITY.md">Security</Link></div><p className="footer-meta">GPL-3.0 · Built in the open</p></footer></body></html>;
}
