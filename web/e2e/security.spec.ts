import { test, expect } from "@playwright/test";

// Security headers on the member site and on API responses through it (04 §2–3, NFR-09).
test("security: page and API headers, no fingerprinting, no framing", async ({ request }) => {
  for (const path of ["/", "/join", "/sign-in", "/account", "/messages", "/billing"]) {
    const r = await request.get(path);
    expect(r.status(), path).toBeLessThan(400);
    const h = r.headers();
    const csp = h["content-security-policy"] ?? "";
    expect(csp, path).toContain("default-src 'self'");
    expect(csp, path).toContain("frame-ancestors 'none'");
    expect(csp, path).toContain("connect-src 'self'");
    expect(csp, path).not.toMatch(/https?:\/\//); // no third-party origins at all
    expect(h["x-robots-tag"], path).toContain("noindex");
    expect(h["referrer-policy"], path).toBe("no-referrer");
    expect(h["x-content-type-options"], path).toBe("nosniff");
    expect(h["cache-control"], path).toContain("no-store");
    expect(h["x-powered-by"], path).toBeUndefined();
  }
  // API through the same origin: locked down, never cached.
  const api = await request.get("/v1/profile");
  expect(api.status()).toBe(401);
  const ah = api.headers();
  expect(ah["content-security-policy"]).toContain("default-src 'none'");
  expect(ah["cache-control"]).toContain("no-store");
  expect(ah["x-content-type-options"]).toBe("nosniff");
  // Nothing about a member is reachable signed out.
  for (const path of ["/v1/threads", "/v1/entitlements", "/v1/notices", "/v1/me/standing", "/v1/reports/categories"]) {
    expect((await request.get(path)).status(), path).toBe(401);
  }
  const exp = await request.post("/v1/account/export");
  expect(exp.status()).toBe(401);
  const del = await request.delete("/v1/account", { data: { confirm: "delete my account" } });
  expect(del.status()).toBe(401);
});
