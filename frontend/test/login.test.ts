import { describe, expect, it } from "vitest";
import { guessSurface, nextFactor, requiredFactors } from "../lib/login";

describe("login factors (02 §3.3)", () => {
  it("mobile surfaces need two biometric factors", () => {
    expect(requiredFactors("mobile_app")).toEqual(["B1", "B2"]);
    expect(requiredFactors("mobile_pwa")).toEqual(["B1", "B2"]);
  });
  it("desktop needs passkey plus phone approval", () => {
    expect(requiredFactors("desktop")).toEqual(["F1", "F2"]);
  });
  it("is complete only after both factors", () => {
    expect(nextFactor({ surface: "desktop", passed: [] })).toBe("F1");
    expect(nextFactor({ surface: "desktop", passed: ["F1"] })).toBe("F2");
    expect(nextFactor({ surface: "desktop", passed: ["F1", "F2"] })).toBeNull();
  });
  it("guesses surface from the user agent (UI hint only)", () => {
    expect(guessSurface("Mozilla/5.0 (iPhone; CPU iPhone OS 18_0)", false)).toBe("mobile_pwa");
    expect(guessSurface("Mozilla/5.0 (Windows NT 10.0; Win64; x64)", false)).toBe("desktop");
  });
});
