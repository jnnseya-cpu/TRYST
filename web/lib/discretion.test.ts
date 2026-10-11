import { describe, expect, it } from "vitest";
import { normaliseContact, shieldDigest } from "./discretion";

describe("partner shield hashing", () => {
  it("normalises like the server", () => {
    expect(normaliseContact("  Sam@Example.COM ")).toBe("sam@example.com");
    expect(normaliseContact("+44 (0)7700-900 123")).toBe("+4407700900123");
  });
  it("sends only a SHA-256 of the normalised contact", async () => {
    // sha256("sam@example.com")
    const d = await shieldDigest("SAM@example.com");
    expect(d).toMatch(/^[0-9a-f]{64}$/);
    expect(d).toBe(await shieldDigest("sam@example.com "));
    expect(d).not.toContain("sam");
  });
});
