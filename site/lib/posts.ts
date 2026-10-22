// The blog is programmatic: every Markdown file in content/posts becomes a page, a card on
// the home page, an entry in its topic page, the sitemap and the RSS feed. Front matter
// (between --- lines) carries the facts each template needs.
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { marked } from "marked";

export const TOPICS = {
  discretion: { name: "Discretion", blurb: "Who sees you, where, and when. Decided by you, enforced on every screen." },
  security: { name: "Sign-in and security", blurb: "Only you can open your account, and you can see everything that happens to it." },
  verification: { name: "Verification", blurb: "Every member is an identity-checked adult. Here's what that means and what it doesn't." },
  connections: { name: "Meeting people", blurb: "Fewer, better introductions, and nobody left wondering." },
  safety: { name: "Meet safety", blurb: "Tools for the evening itself, from the door to getting home." },
  data: { name: "Your data", blurb: "What we keep, what we can't read, and how you leave without a trace." },
  membership: { name: "Membership", blurb: "What's free, what isn't, and how to stop paying in one step." },
} as const;
export type Topic = keyof typeof TOPICS;

export type Post = {
  slug: string;
  title: string;
  description: string; // meta description, 70–160 characters
  date: string; // YYYY-MM-DD
  topic: Topic;
  usp: string; // the one line that sells it
  status: "live" | "coming";
  order: number;
  html: string;
  words: number;
};

const DIR = join(process.cwd(), "content", "posts");

function frontMatter(src: string): { data: Record<string, string>; body: string } {
  const m = /^---\n([\s\S]*?)\n---\n([\s\S]*)$/.exec(src);
  if (!m) throw new Error("missing front matter");
  const data: Record<string, string> = {};
  for (const line of m[1].split("\n")) {
    const i = line.indexOf(":");
    if (i > 0) data[line.slice(0, i).trim()] = line.slice(i + 1).trim().replace(/^"(.*)"$/, "$1");
  }
  return { data, body: m[2] };
}

export function parsePost(slug: string, src: string): Post {
  const { data, body } = frontMatter(src);
  for (const k of ["title", "description", "date", "topic", "usp", "status", "order"]) {
    if (!data[k]) throw new Error(`${slug}: front matter needs ${k}`);
  }
  if (!(data.topic in TOPICS)) throw new Error(`${slug}: unknown topic ${data.topic}`);
  if (data.status !== "live" && data.status !== "coming") throw new Error(`${slug}: status is live or coming`);
  return {
    slug,
    title: data.title,
    description: data.description,
    date: data.date,
    topic: data.topic as Topic,
    usp: data.usp,
    status: data.status,
    order: Number(data.order),
    html: marked.parse(body, { async: false }) as string,
    words: body.split(/\s+/).filter(Boolean).length,
  };
}

let cache: Post[] | null = null;

export function allPosts(): Post[] {
  if (!cache) {
    cache = readdirSync(DIR)
      .filter((f) => f.endsWith(".md"))
      .map((f) => parsePost(f.replace(/\.md$/, ""), readFileSync(join(DIR, f), "utf8")))
      .sort((a, b) => a.order - b.order);
  }
  return cache;
}

export function rawPost(slug: string): string {
  return readFileSync(join(DIR, `${slug}.md`), "utf8");
}

export const getPost = (slug: string) => allPosts().find((p) => p.slug === slug);
export const byTopic = (t: Topic) => allPosts().filter((p) => p.topic === t);

/** Up to three other posts: same topic first, then neighbours in reading order. */
export function related(p: Post): Post[] {
  const rest = allPosts().filter((x) => x.slug !== p.slug);
  const same = rest.filter((x) => x.topic === p.topic);
  const near = rest.filter((x) => x.topic !== p.topic).sort((a, b) => Math.abs(a.order - p.order) - Math.abs(b.order - p.order));
  return [...same, ...near].slice(0, 3);
}

export const minutes = (p: Post) => Math.max(1, Math.round(p.words / 220));
