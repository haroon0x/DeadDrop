import type { Metadata } from "next";
import { Archivo, JetBrains_Mono } from "next/font/google";
import Link from "next/link";
import { ArrowUpRight } from "@phosphor-icons/react/dist/ssr";
import "./globals.css";

const display = Archivo({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700", "800"],
  variable: "--font-display",
  display: "swap",
});

const mono = JetBrains_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  variable: "--font-mono",
  display: "swap",
});

export const metadata: Metadata = {
  metadataBase: new URL("https://deaddrop-dpk8.onrender.com"),
  title: {
    default: "DeadDrop · Verified coding tasks for local agents",
    template: "%s · DeadDrop",
  },
  description:
    "A self-hosted coding task inbox that runs local agents in disposable Git worktrees and returns evidence-backed patches.",
  openGraph: {
    title: "DeadDrop · Verified coding tasks for local agents",
    description:
      "Leave a task. Your local agent works in a disposable Git worktree. Come back to verified evidence and a patch you choose whether to apply.",
    type: "website",
  },
};

const nav: [string, string][] = [
  ["How it works", "/#how-it-works"],
  ["Docs", "/docs"],
  ["Architecture", "/docs/architecture"],
  ["Blog", "/blog"],
];

const footerLinks: [string, [string, string][]][] = [
  [
    "Read",
    [
      ["Documentation", "/docs"],
      ["Architecture", "/docs/architecture"],
      ["Writing", "/blog"],
      ["Updates", "/updates"],
    ],
  ],
  [
    "Project",
    [
      ["Source", "https://github.com/haroon0x/DeadDrop"],
      [
        "Security policy",
        "https://github.com/haroon0x/DeadDrop/blob/main/SECURITY.md",
      ],
      [
        "Contributing",
        "https://github.com/haroon0x/DeadDrop/blob/main/CONTRIBUTING.md",
      ],
      ["Demo receipt", "/demo"],
    ],
  ],
];

function Brand() {
  return (
    <Link className="brand" href="/">
      <i aria-hidden="true">DD</i>
      <span>DeadDrop</span>
    </Link>
  );
}

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${display.variable} ${mono.variable}`}>
      <body>
        <div className="grain" aria-hidden="true" />
        <div className="scan" aria-hidden="true" />

        <header className="head">
          <div className="shell head-inner">
            <Brand />
            <nav>
              {nav.map(([label, href]) => (
                <Link key={href} href={href}>
                  {label}
                </Link>
              ))}
            </nav>
            <Link className="btn btn-sm" href="https://github.com/haroon0x/DeadDrop">
              GitHub
              <ArrowUpRight size={13} weight="bold" />
            </Link>
          </div>
        </header>

        <main>{children}</main>

        <footer>
          <div className="shell">
            <div className="foot">
              <div>
                <Brand />
                <p className="body foot-blurb">
                  Local agents. Disposable worktrees. Evidence you can read
                  before you apply anything.
                </p>
              </div>
              {footerLinks.map(([heading, links]) => (
                <div key={heading}>
                  <h4>{heading}</h4>
                  <ul>
                    {links.map(([label, href]) => (
                      <li key={href}>
                        <Link href={href}>{label}</Link>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
              <div className="foot-note">
                <span className="label">GPL-3.0 · Built in the open</span>
                <span className="label">Self-hosted by design</span>
              </div>
            </div>
          </div>
        </footer>
      </body>
    </html>
  );
}
