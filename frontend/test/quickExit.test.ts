import { describe, expect, it, vi } from "vitest";
import { NEUTRAL_EXIT_URL, quickExit } from "../lib/quickExit";

describe("quick exit (FR-029)", () => {
  it("hides content, clears session storage and replaces history", () => {
    const replace = vi.fn();
    const clear = vi.fn();
    const body = { style: { visibility: "visible" } };
    quickExit({ location: { replace }, sessionStorage: { clear }, document: { body } });
    expect(body.style.visibility).toBe("hidden");
    expect(clear).toHaveBeenCalled();
    expect(replace).toHaveBeenCalledWith(NEUTRAL_EXIT_URL);
  });
  it("still leaves if storage throws", () => {
    const replace = vi.fn();
    quickExit({ location: { replace }, sessionStorage: { clear: () => { throw new Error("blocked"); } } });
    expect(replace).toHaveBeenCalled();
  });
});
