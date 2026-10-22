import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { byTopic, TOPICS, type Topic } from "@/lib/posts";
import { PostCard } from "@/components/PostCard";

export const dynamicParams = false;
export function generateStaticParams() {
  return (Object.keys(TOPICS) as Topic[]).filter((t) => byTopic(t).length > 0).map((topic) => ({ topic }));
}

type Props = { params: Promise<{ topic: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const t = (await params).topic as Topic;
  if (!(t in TOPICS)) return {};
  return { title: TOPICS[t].name, description: TOPICS[t].blurb, alternates: { canonical: `/topics/${t}/` } };
}

export default async function TopicPage({ params }: Props) {
  const t = (await params).topic as Topic;
  if (!(t in TOPICS)) notFound();
  return (
    <section className="topic">
      <p className="eyebrow">Topic</p>
      <h1>{TOPICS[t].name}</h1>
      <p className="lede">{TOPICS[t].blurb}</p>
      <ul className="cards">{byTopic(t).map((p) => <PostCard key={p.slug} p={p} />)}</ul>
    </section>
  );
}
