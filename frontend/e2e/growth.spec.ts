import { test, expect, type Browser } from "@playwright/test";

// Growth intelligence (D-73): staff console only, invisible to members.
const ANALYST = "e2e-growth-analyst-0001";
const EDITOR = "e2e-growth-editor-0001";
const MOD = "e2e-mod-one-token-0001";
const GUIDES = "http://localhost:8084";

async function signIn(browser: Browser, token: string) {
  const page = await (await browser.newContext()).newPage();
  await page.goto("/ops/growth");
  await page.getByLabel("Staff token").fill(token);
  await page.getByRole("button", { name: "Sign in" }).click();
  return page;
}

/** Nine complete weeks of Search Console clicks, the last one a spike, as CSV. */
function csv(): string {
  const now = new Date();
  const monday = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate() - ((now.getUTCDay() + 6) % 7)));
  const values = [60, 64, 58, 61, 63, 59, 62, 60, 240];
  return values.map((v, i) => {
    const start = new Date(monday.getTime() - (values.length - i) * 7 * 86400000).toISOString().slice(0, 10);
    return `search_console,discreet dating london,${start},${v}`;
  }).join("\n");
}

test("growth: staff only, imports and scores, pages exist only when an editor creates them, members see nothing", async ({ browser, page }) => {
  // A moderator has no growth role: the console asks for a different sign-in.
  const mod = await signIn(browser, MOD);
  await expect(mod.getByRole("heading", { name: "Staff sign-in" })).toBeVisible();

  const analyst = await signIn(browser, ANALYST);
  await expect(analyst.getByRole("heading", { name: "Growth" })).toBeVisible();
  await expect(analyst.getByTestId("growth-state")).toContainText("Templates");
  await expect(analyst.getByRole("button", { name: "Create page" })).toHaveCount(0); // editors only
  await analyst.getByLabel(/Import weekly search data/).fill(csv());
  await analyst.getByRole("button", { name: "Import" }).click();
  await expect(analyst.getByTestId("growth-note")).toHaveText("Imported.");
  await analyst.getByRole("button", { name: "Refresh scores" }).click();
  // One source family alone can't reach a brief: confidence is capped at 69.
  const row = analyst.getByTestId("opportunity-row").filter({ hasText: "discreet-dating-london" });
  await expect(row).toContainText("watching");
  await expect(row.getByRole("button", { name: "Draft brief" })).toHaveCount(0);

  // No guide page exists until an editor creates one.
  expect((await page.request.get(`${GUIDES}/guides/discreet-dating-london`)).status()).toBe(404);
  const editor = await signIn(browser, EDITOR);
  await editor.getByTestId("pages").getByRole("row").filter({ hasText: "discreet-dating-london" }).getByRole("button", { name: "Create page" }).click();
  await expect(editor.getByTestId("growth-note")).toHaveText("Page created.");
  const guide = await page.request.get(`${GUIDES}/guides/discreet-dating-london`);
  expect(guide.status()).toBe(200);
  const html = await guide.text();
  expect(html).toContain("<title>Meeting discreetly in London</title>");
  expect(guide.headers()["set-cookie"]).toBeUndefined();
  expect(html.toLowerCase()).not.toContain("<script");
  expect(html.toLowerCase()).not.toMatch(/growth|brief|generated|algorithm/);
  // The public address serves guide pages only, never the staff API.
  expect((await page.request.get(`${GUIDES}/ops/v1/growth/overview`, { headers: { Authorization: `Staff ${EDITOR}` } })).status()).toBe(404);

  // Freeze: no new change starts.
  await editor.getByRole("button", { name: "Freeze all changes" }).click();
  await expect(editor.getByTestId("growth-state")).toContainText("Frozen");
  await editor.getByRole("button", { name: "Unfreeze" }).click();
  await expect(editor.getByTestId("growth-state")).toContainText("Running");

  // Members see nothing of it.
  await page.goto("/");
  await expect(page.locator("body")).not.toContainText(/growth|guides/i);
  expect((await page.request.get("/ops/v1/growth/overview")).status()).toBe(401);
});
