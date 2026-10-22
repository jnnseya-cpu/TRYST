import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { allPosts, getPost, minutes, related, TOPICS } from "@/lib/posts";
import { APP_URL, SITE_NAME, SITE_URL } from "@/lib/site";
import { PostCard } from "@/components/PostCard";

export const dynamicParams = false;
export function generateStaticParams() {
  return allPosts().map((p) => ({ slug: p.slug }));
}

type Props = { params: Promise<{ slug: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const p = getPost((await params).slug);
  if (!p) return {};
  return {
    title: p.title,
    description: p.description,
    alternates: { canonical: `/blog/${p.slug}/` },
    openGraph: { title: p.title, description: p.description, type: "article", publishedTime: p.date, url: `/blog/${p.slug}/` },
  };
}

export default async function PostPage({ params }: Props) {
  const p = getPost((await params).slug);
  if (!p) notFound();
  const ld = {
    "@context": "https://schema.org",
    "@type": "BlogPosting",
    headline: p.title,
    description: p.description,
    datePublished: p.date,
    articleSection: TOPICS[p.topic].name,
    mainEntityOfPage: `${SITE_URL}/blog/${p.slug}/`,
    publisher: { "@type": "Organization", name: SITE_NAME, logo: { "@type": "ImageObject", url: `${SITE_URL}/brand/tryst-logo.png` } },
  };
  return (
    <article className="post">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(ld) }} />
      <p className="eyebrow">
        <a href={`/topics/${p.topic}/`}>{TOPICS[p.topic].name}</a> · {minutes(p)} min read
        {p.status === "coming" && <span className="badge">Coming later</span>}
      </p>
      <h1>{p.title}</h1>
      <p className="lede">{p.description}</p>
      <aside className="usp" aria-label="In one line">
        <p className="eyebrow">The difference</p>
        <p>{p.usp}</p>
      </aside>
      <div className="prose" dangerouslySetInnerHTML={{ __html: p.html }} />
      <aside className="cta">
        {p.status === "live" ? (
          <>
            <h2>Try it yourself</h2>
            <p>Verified membership is free: every feature in this journal marked as available is included, with three intents a week. TRYST+ makes intents unlimited.</p>
            <a className="button" href={`${APP_URL}/join`}>Become a member</a>
          </>
        ) : (
          <>
            <h2>Not available yet</h2>
            <p>This is built but not switched on. We&rsquo;ll say so here, and in the app, the day it is. Nothing about it can be bought before then.</p>
            <a className="button ghost" href="/">Read about what&rsquo;s available now</a>
          </>
        )}
      </aside>
      <section aria-labelledby="more-h" className="more">
        <h2 id="more-h">Read next</h2>
        <ul className="cards">{related(p).map((r) => <PostCard key={r.slug} p={r} />)}</ul>
      </section>
    </article>
  );
}
