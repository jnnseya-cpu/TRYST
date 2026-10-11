import { test, expect } from "@playwright/test";
import { readFileSync } from "node:fs";
import { member } from "./helpers";

test("data rights: download everything, then delete by cryptographic erasure with a signed proof", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const name = `Lark${tag}`;
  const page = await member(browser, name, "Islington", "photo-1.jpg");
  await page.goto("/account");

  const [download] = await Promise.all([page.waitForEvent("download"), page.getByRole("button", { name: "Download my data" }).click()]);
  expect(download.suggestedFilename()).toBe("member-data.json");
  const data = JSON.parse(readFileSync((await download.path())!, "utf8"));
  expect(data.profile.profile.display_name).toBe(name);
  expect(data.profile.photos).toHaveLength(1);
  expect(data.profile.photos[0].data_base64.length).toBeGreaterThan(1000);
  expect(data.identity.passkeys.length).toBeGreaterThan(0);
  expect(JSON.stringify(data)).not.toContain("@example.com"); // the contact is only a one-way code

  await page.getByRole("button", { name: "Delete my account" }).click();
  const go = page.getByRole("button", { name: "Delete everything now" });
  await expect(go).toBeDisabled();
  await page.getByLabel(/Type .delete my account. to confirm/).fill("delete my account");
  await go.click();
  await expect(page.getByRole("heading", { name: "Your account is gone" })).toBeVisible();
  await expect(page.getByTestId("deletion-ref")).toHaveText(/^del_[0-9a-f]{24}$/);

  // Nothing is left to sign in to; the profile and its photo are gone.
  await page.goto("/account");
  await expect(page.getByRole("heading", { name: "Signed out" })).toBeVisible();
});
