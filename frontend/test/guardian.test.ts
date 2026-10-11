import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { check } from "../lib/guardian";

// The same labelled vectors the Go reference implementation must pass (FR-110).
const vectors = JSON.parse(readFileSync(join(__dirname, "..", "..", "shared", "contracts", "vocab", "guardian_vectors.json"), "utf8")) as {
  vectors: { text: string; expect: string[] }[];
};

describe("on-device message safety matches the shared vectors", () => {
  for (const v of vectors.vectors) {
    it(`${v.expect.length ? v.expect.join("+") : "benign"}: ${v.text.slice(0, 50)}`, () => {
      const got = check(v.text).map((s) => s.kind);
      for (const want of v.expect) expect(got).toContain(want);
      if (v.expect.length === 0) expect(got).toEqual([]);
    });
  }
});
