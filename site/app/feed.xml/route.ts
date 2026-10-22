import { allPosts } from "@/lib/posts";
import { SITE_NAME, SITE_URL, TAGLINE } from "@/lib/site";

export const dynamic = "force-static";

const esc = (s: string) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

export function GET() {
  const items = [...allPosts()]
    .sort((a, b) => b.date.localeCompare(a.date) || a.order - b.order)
    .map((p) => `<item><title>${esc(p.title)}</title><link>${SITE_URL}/blog/${p.slug}/</link><guid>${SITE_URL}/blog/${p.slug}/</guid><pubDate>${new Date(p.date + "T09:00:00Z").toUTCString()}</pubDate><description>${esc(p.description)}</description></item>`)
    .join("");
  const xml = `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>${SITE_NAME} journal</title><link>${SITE_URL}/</link><description>${esc(TAGLINE)}</description><language>en-gb</language>${items}</channel></rss>`;
  return new Response(xml, { headers: { "Content-Type": "application/rss+xml; charset=utf-8" } });
}
