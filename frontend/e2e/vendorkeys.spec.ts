import { test, expect } from "@playwright/test";

const MOD1 = "e2e-mod-one-token-0001";
const LEAD = "e2e-lead-token-00000003";

async function console(browser: import("@playwright/test").Browser, token: string) {
  const page = await (await browser.newContext()).newPage();
  await page.goto("/ops");
  await page.getByLabel("Staff token").fill(token);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Case queue" })).toBeVisible();
  return page;
}

test("vendor keys: leads only, checked before saving, never shown again, applied on request", async ({ browser }) => {
  const mod = await console(browser, MOD1);
  await mod.getByRole("link", { name: "Vendor keys" }).click();
  await expect(mod.getByText(/Only a safety lead/)).toBeVisible();

  const lead = await console(browser, LEAD);
  await lead.getByRole("link", { name: "Vendor keys" }).click();
  await expect(lead.getByRole("heading", { name: "Vendor keys" })).toBeVisible();
  await expect(lead.getByTestId("launch-ready")).toContainText("Launch needs");

  // A wrong value is refused with a sentence the lead can act on, and nothing is stored.
  const url = lead.getByTestId("setting-ARC_URL");
  await url.getByRole("textbox").fill("http://arc.example");
  await url.getByRole("button", { name: "Save" }).click();
  await expect(url.getByRole("alert")).toContainText("https://");
  await expect(url).toContainText("Not set");

  // A secret is saved, cleared from the page and shown only as set.
  const secret = "arc-signing-secret-from-the-dashboard";
  const box = lead.getByTestId("setting-ARC_SECRET");
  await box.getByLabel(/Alarm centre signing secret/).fill(secret);
  await box.getByRole("button", { name: "Save" }).click();
  await expect(box.getByRole("status")).toHaveText("Saved.");
  await expect(box).toContainText("Set");
  await expect(box.getByLabel(/Alarm centre signing secret/)).toHaveValue("");
  expect(await lead.content()).not.toContain(secret);

  lead.once("dialog", (d) => d.accept());
  await lead.getByRole("button", { name: "Apply keys now" }).click();
  await expect(lead.getByTestId("applied")).toContainText("restarts");
});
