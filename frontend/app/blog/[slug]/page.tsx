import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { articles } from "@/lib/articles";

export function generateStaticParams() { return articles.map(({ slug }) => ({ slug })); }
export function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> { return params.then(({slug}) => ({ title: articles.find(a=>a.slug===slug)?.title ?? "Article" })); }

export default async function ArticlePage({ params }: { params: Promise<{ slug: string }> }) { const {slug}=await params; const article=articles.find(a=>a.slug===slug); if(!article) notFound(); return <article className="prose"><header><Link href="/blog">← All writing</Link><p className="eyebrow">{article.category} · {article.minutes}</p><h1>{article.title}</h1><p>{article.deck}</p></header>{article.sections.map((section,i)=><section key={i}>{section.heading&&<h2>{section.heading}</h2>}{section.body.map((p,j)=><p key={j}>{p}</p>)}{section.code&&<pre><code>{section.code}</code></pre>}{section.aside&&<aside>{section.aside}</aside>}</section>)}<footer><Link href={article.next.href}>{article.next.label} →</Link></footer></article> }
