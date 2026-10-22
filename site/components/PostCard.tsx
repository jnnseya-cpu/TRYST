import { minutes, type Post, TOPICS } from "@/lib/posts";

export function PostCard({ p }: { p: Post }) {
  return (
    <li className="card">
      <a href={`/blog/${p.slug}/`}>
        <p className="eyebrow">{TOPICS[p.topic].name}{p.status === "coming" && " · Coming later"}</p>
        <h3>{p.title}</h3>
        <p>{p.description}</p>
        <p className="note">{minutes(p)} min read</p>
      </a>
    </li>
  );
}
