import type { Metadata } from "next";
import Link from "next/link";
import { articles } from "@/lib/articles";

export const metadata: Metadata = { title: "Blog" };
export default function Blog() { const [first,...rest]=articles; return <><section className="page-hero"><p className="eyebrow">Technical writing</p><h1>Notes on agent infrastructure you can inspect.</h1><p>Worktrees, receipts, leases, and the unglamorous details between a prompt and a patch.</p></section><section className="blog-feature"><Link href={`/blog/${first.slug}`}><div><span>{first.category} · {first.minutes}</span><h2>{first.title}</h2><p>{first.deck}</p><b>Read article →</b></div><div className="feature-visual"><span>source/ <small>HEAD</small></span><i>→</i><span>job/ <small>detached</small></span></div></Link></section><section className="section article-grid">{rest.map(a=><Link key={a.slug} href={`/blog/${a.slug}`}><span>{a.category} · {a.minutes}</span><h3>{a.title}</h3><p>{a.deck}</p><small>Read article →</small></Link>)}</section></> }
