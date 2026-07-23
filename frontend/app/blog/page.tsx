import type { Metadata } from "next";
import Image from "next/image";
import Link from "next/link";
import { articles } from "@/lib/articles";

export const metadata: Metadata = { title: "Blog" };

export default function Blog() {
  const [featured, ...rest] = articles;

  return (
    <div className="shell">
      <section className="page-hero">
        <p className="label label-acid bracket">Technical writing</p>
        <h1 className="d2">Notes on agent infrastructure you can inspect.</h1>
        <p className="lede">
          Worktrees, receipts, leases, and the unglamorous details between a
          prompt and a patch.
        </p>
      </section>

      <section className="band">
        <div className="split reveal">
          <Link href={`/blog/${featured.slug}`} className="figure-block plate">
            <Image
              src="/img/taxonomy.webp"
              alt="Machine components sorted into neat columns by type and size"
              width={1400}
              height={900}
            />
            <figcaption className="figure-tag">
              <span className="label label-acid">{featured.category}</span>
              <span className="label">{featured.minutes}</span>
            </figcaption>
          </Link>
          <div className="split-copy">
            <h2 className="d3">{featured.title}</h2>
            <p className="body">{featured.deck}</p>
            <Link className="btn" href={`/blog/${featured.slug}`}>
              Read article
            </Link>
          </div>
        </div>
      </section>

      <section className="band" style={{ paddingTop: 0 }}>
        <ul className="index-list reveal">
          {rest.map((article) => (
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
      </section>
    </div>
  );
}
