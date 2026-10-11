import { describe, expect, it } from "vitest";
import { doorWords } from "../lib/doorwords";

describe("door-check words (FR-099)", () => {
  it("match the Go implementation exactly", async () => {
    const secret = new Uint8Array(new TextEncoder().encode("tryst-vector-secret-0123456789ab"));
    expect(await doorWords(secret, "mt_vector")).toEqual(["fern", "lantern", "birch"]);
  });
  it("differ for another meet or another secret", async () => {
    const s = new Uint8Array(new TextEncoder().encode("tryst-vector-secret-0123456789ab"));
    const other = new Uint8Array(new TextEncoder().encode("another-secret-xxxxxxxxxxxxxxxxxxx"));
    expect(await doorWords(s, "mt_other")).not.toEqual(await doorWords(s, "mt_vector"));
    expect(await doorWords(other, "mt_vector")).not.toEqual(await doorWords(s, "mt_vector"));
  });
});
