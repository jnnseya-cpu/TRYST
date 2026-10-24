import { test, expect, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { member } from "./helpers";

// Accessibility (WCAG 2.2 AA rules axe can check automatically) and phone fit at 360 px
// on a slow mobile connection, for the screens a first-time member meets. Automated rules
// catch only part of WCAG; keyboard and screen-reader passes stay manual.
const PHONE = { width: 360, height: 740 };
const TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"];

async function slow3g(page: Page) {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send("Network.emulateNetworkConditions", {
    offline: false, latency: 400, downloadThroughput: (400 * 1024) / 8, uploadThroughput: (400 * 1024) / 8,
  });
}

async function check(page: Page, path: string) {
  await page.goto(path);
  await page.waitForLoadState("networkidle");
  const { violations } = await new AxeBuilder({ page }).withTags(TAGS).analyze();
  const summary = violations.map((v) => `${v.id} (${v.impact}): ${v.nodes.slice(0, 3).map((n) => n.target.join(" ")).join(", ")}`);
  expect(summary, `${path} accessibility`).toEqual([]);
  // Nothing wider than the screen: a sideways scroll on a phone is a broken layout.
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow, `${path} horizontal overflow (px)`).toBeLessThanOrEqual(0);
}

test("a11y: public screens on a 360 px phone over a slow connection", async ({ browser }) => {
  const ctx = await browser.newContext({ viewport: PHONE, isMobile: true, hasTouch: true });
  const page = await ctx.newPage();
  await slow3g(page);
  const started = Date.now();
  await page.goto("/");
  await page.getByRole("heading", { level: 1 }).waitFor();
  expect(Date.now() - started, "first screen visible on slow 3G (ms)").toBeLessThan(8000);
  for (const path of ["/", "/join", "/sign-in", "/recover", "/lock"]) await check(page, path);
  await ctx.close();
});

test("a11y: a new member's screens on a 360 px phone", async ({ browser }) => {
  const page = await member(browser, `Ash${String(Date.now()).slice(-5)}`, "Islington", "photo-1.jpg");
  await page.setViewportSize(PHONE);
  for (const path of ["/introductions", "/tonight", "/messages", "/notices", "/profile", "/billing", "/account", "/couple"]) {
    await check(page, path);
  }
  await page.context().close();
});
