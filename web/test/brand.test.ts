import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// Brand rule (01 §3.3, D-33): TRYST is never named, branded or marketed as AI.
// Scans every user-facing source file for "AI", "A.I." and "artificial intelligence".
const roots = ["app", "components", "lib"];
const files: string[] = [];
function walk(dir: string) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p);
    else if (/\.(tsx?|css|json)$/.test(name)) files.push(p);
  }
}
roots.forEach((r) => walk(join(__dirname, "..", r)));

const banned = [/\bAI\b/, /\bA\.I\./, /artificial intelligence/i];

describe("brand: no AI in TRYST's name, brand or copy (D-33)", () => {
  it("scans the user-facing sources", () => expect(files.length).toBeGreaterThan(5));
  for (const f of files) {
    it(f.split("/web/")[1] ?? f, () => {
      const text = readFileSync(f, "utf8");
      for (const re of banned) expect(text, `${re} in ${f}`).not.toMatch(re);
    });
  }
  it("brand tokens and metadata stay AI-free", () => {
    const tokens = readFileSync(join(__dirname, "..", "..", "assets", "brand", "tokens.json"), "utf8");
    for (const re of banned) expect(tokens).not.toMatch(re);
  });
});
