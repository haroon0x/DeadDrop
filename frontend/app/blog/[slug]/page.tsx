import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft, ArrowRight } from "@phosphor-icons/react/dist/ssr";
import { articles } from "@/lib/articles";

export function generateStaticParams() {
  return articles.map(({ slug }) => ({ slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const article = articles.find((a) => a.slug === slug);
  return {
    title: article?.title ?? "Article",
    description: article?.deck,
  };
}

export default async function ArticlePage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const article = articles.find((a) => a.slug === slug);
  if (!article) notFound();

  return (
    <div className="shell">
      <article className="prose">
        <header>
          <Link className="tlink" href="/blog">
            <ArrowLeft size={13} weight="bold" />
            All writing
          </Link>
          <p className="label label-acid">
            {article.category} · {article.minutes}
          </p>
          <h1>{article.title}</h1>
          <p>{article.deck}</p>
        </header>

        {article.sections.map((section, i) => (
          <section key={i}>
            {section.heading ? <h2>{section.heading}</h2> : null}
            {section.body.map((paragraph, j) => (
              <p key={j}>{paragraph}</p>
            ))}
            {section.code ? (
              <pre>
                <code>{section.code}</code>
              </pre>
            ) : null}
            {section.aside ? <aside>{section.aside}</aside> : null}
          </section>
        ))}

        <footer>
          <Link className="tlink" href={article.next.href}>
            {article.next.label}
            <ArrowRight size={13} weight="bold" />
          </Link>
        </footer>
      </article>
    </div>
  );
}
