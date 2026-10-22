import { allPosts, TOPICS, type Topic } from "@/lib/posts";
import { APP_URL, TAGLINE } from "@/lib/site";
import { PostCard } from "@/components/PostCard";

export default function Home() {
  const posts = allPosts();
  return (
    <>
      <section className="hero">
        <p className="eyebrow">The TRYST journal</p>
        <h1>{TAGLINE}</h1>
        <p className="lede">Every feature of TRYST, explained plainly: what it does, what makes it different, and what it can&rsquo;t do. No hype, no promises we can&rsquo;t keep.</p>
        <a className="button" href={`${APP_URL}/join`}>Join free</a>
      </section>

      <section aria-labelledby="why-h" className="usp-grid-wrap">
        <h2 id="why-h">Why TRYST, in one line each</h2>
        <ul className="usp-grid">
          {posts.filter((p) => p.status === "live").map((p) => (
            <li key={p.slug}><a href={`/blog/${p.slug}/`}><strong>{p.title}</strong><span>{p.usp}</span></a></li>
          ))}
        </ul>
      </section>

      {(Object.keys(TOPICS) as Topic[]).map((t) => {
        const list = posts.filter((p) => p.topic === t);
        if (list.length === 0) return null;
        return (
          <section key={t} className="topic" aria-labelledby={`t-${t}`}>
            <div className="topic-head">
              <h2 id={`t-${t}`}><a href={`/topics/${t}/`}>{TOPICS[t].name}</a></h2>
              <p>{TOPICS[t].blurb}</p>
            </div>
            <ul className="cards">{list.map((p) => <PostCard key={p.slug} p={p} />)}</ul>
          </section>
        );
      })}
    </>
  );
}
