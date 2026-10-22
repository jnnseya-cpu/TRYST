// Editorial rules every post must pass (01 §3.3, §15.1; ToS Q6; CPUT and the DMCC Act).
// They keep the journal selling only what is true today.
import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { allPosts, rawPost, TOPICS } from "@/lib/posts";

const posts = allPosts();
const text = (slug: string) => rawPost(slug);

// Prices that exist in the launch catalogue (backend/internal/commercesvc Catalogue, D-60).
const PRICES = new Set(["£24.99", "£14.99", "£179.88"]);
// Built but not on sale (D-60): may be mentioned only as not available yet.
const NOT_ON_SALE = /\b(ENVOY|DUO|GHOST|Keys?)\b/;
const SUPERLATIVE = /\b(the only (service|app|platform|site|discreet|one)|nobody else (does|offers)|no other (app|service|platform|site)|first[- ]ever|the best|world.s first|unlike any)\b/i;
// Not built on the web app: never claimed.
const NOT_BUILT = /\b(decoy|screenshots? (are )?blocked|travel mode|read receipts?|video calls?)\b/i;

describe("journal content", () => {
  it("has a post for every topic and enough posts", () => {
    expect(posts.length).toBeGreaterThanOrEqual(20);
    for (const t of Object.keys(TOPICS)) expect(posts.some((p) => p.topic === t), t).toBe(true);
  });

  it("titles, slugs and orders are unique", () => {
    for (const k of ["title", "slug", "order"] as const) {
      expect(new Set(posts.map((p) => p[k])).size, k).toBe(posts.length);
    }
  });

  for (const p of posts) {
    describe(p.slug, () => {
      const src = text(p.slug);
      const body = src.split("\n---\n").slice(1).join("\n---\n");

      it("has a search-friendly description and a real date", () => {
        expect(p.description.length).toBeGreaterThanOrEqual(70);
        expect(p.description.length).toBeLessThanOrEqual(160);
        expect(p.date).toMatch(/^\d{4}-\d{2}-\d{2}$/);
        expect(p.usp.length).toBeLessThanOrEqual(140);
      });

      it("sells the difference and states the limits", () => {
        expect(body).toMatch(/^## Why it.s different$/m);
        expect(body).toMatch(/^## What it doesn.t do$/m);
        expect(p.words).toBeGreaterThanOrEqual(200);
      });

      it("never says AI (01 §3.3)", () => {
        expect(src).not.toMatch(/\bAI\b|\bA\.I\.|artificial intelligence/i);
      });

      it("keeps to the claims discipline (01 §15.1)", () => {
        expect(src).not.toMatch(/\b(vetted|background[- ]checked|guaranteed?|100%|bullet-?proof|unhackable|risk-free|no fakes?)\b/i);
        // "safe" is never a promise; "safety" and the "home safe" check-in are fine.
        expect(src.replace(/home safe/gi, "")).not.toMatch(/\bsafe(r|st|ly)?\b/i);
        expect(src).not.toMatch(NOT_BUILT);
        // Comparative claims need evidence about every competitor (CPUT): don't make them.
        expect(src).not.toMatch(SUPERLATIVE);
      });

      it("only mentions real prices", () => {
        for (const m of src.match(/£\d+(\.\d\d)?/g) ?? []) expect(PRICES.has(m), m).toBe(true);
      });

      it("presents unreleased plans only as not available yet", () => {
        if (p.status === "coming") return;
        for (const line of body.split("\n")) {
          if (NOT_ON_SALE.test(line)) expect(line, line).toMatch(/\b(not|later|yet|isn.t|aren.t)\b/i);
        }
      });

      it("links only to pages that exist", () => {
        for (const [, slug] of body.matchAll(/\]\(\/blog\/([a-z0-9-]+)\/\)/g)) {
          expect(posts.some((x) => x.slug === slug), slug).toBe(true);
        }
      });

      it("uses British spelling", () => {
        expect(src).not.toMatch(/\b(color|favorite|center|organize|realize|recognize|apologize|license\b(?= to)|program\b)/i);
      });
    });
  }
});

describe("site source", () => {
  it("never says AI", () => {
    const walk = (d: string): string[] =>
      readdirSync(d, { withFileTypes: true }).flatMap((e) =>
        e.isDirectory() ? walk(join(d, e.name)) : /\.(tsx?|css)$/.test(e.name) ? [join(d, e.name)] : []);
    for (const f of [...walk(join(__dirname, "..", "app")), ...walk(join(__dirname, "..", "components")), ...walk(join(__dirname, "..", "lib"))]) {
      const src = readFileSync(f, "utf8");
      expect(src, f).not.toMatch(/\bAI\b|artificial intelligence/i);
      if (!f.endsWith(".test.ts")) expect(src, f).not.toMatch(SUPERLATIVE);
    }
  });
});
