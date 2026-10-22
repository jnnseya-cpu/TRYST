import type { MetadataRoute } from "next";
import { allPosts, byTopic, TOPICS, type Topic } from "@/lib/posts";
import { SITE_URL } from "@/lib/site";

export const dynamic = "force-static";

export default function sitemap(): MetadataRoute.Sitemap {
  return [
    { url: `${SITE_URL}/`, changeFrequency: "weekly", priority: 1 },
    ...(Object.keys(TOPICS) as Topic[]).filter((t) => byTopic(t).length).map((t) => ({ url: `${SITE_URL}/topics/${t}/`, priority: 0.6 })),
    ...allPosts().map((p) => ({ url: `${SITE_URL}/blog/${p.slug}/`, lastModified: p.date, priority: p.status === "live" ? 0.8 : 0.4 })),
  ];
}
