import { test, expect } from "@playwright/test";
import { IPHONE_UA, member, stepUp, verify } from "./helpers";

test("devices need step-up; recovery needs the same ID and can be cancelled from a signed-in device", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const contact = `wren${tag}@example.com`;
  const doc = `DOC-Wren-${tag}`;
  const page = await member(browser, `Wren${tag}`, "Hackney", "photo-1.jpg", { verify: false, contact });
  await verify(page, doc);
  await expect(page.getByTestId("verified")).toBeVisible();

  // Devices: hidden until both checks are repeated.
  await page.goto("/account");
  await page.getByRole("button", { name: "Show my devices" }).click();
  await stepUp(page);
  await expect(page.getByTestId("device")).toHaveCount(1);
  await page.getByTestId("device").getByRole("button", { name: "Remove" }).click();
  await expect(page.getByRole("status").filter({ hasText: /only device/ })).toBeVisible();

  // Someone with the contact starts a recovery on another browser, with no passkey.
  const ctx = await browser.newContext({ userAgent: IPHONE_UA });
  const other = await ctx.newPage();
  await other.goto("/recover");
  await other.getByLabel(/Email or mobile number on your account/).fill(contact);
  await other.getByRole("button", { name: "Start recovery" }).click();
  await other.getByRole("button", { name: "Start face check" }).click();
  await other.getByRole("button", { name: "Pass" }).click();
  await other.waitForURL("**/recover");
  // A different document ends the attempt.
  await other.getByRole("button", { name: "Start ID check" }).click();
  await other.getByLabel("Document number").fill("SOMEONE-ELSE");
  await other.getByRole("button", { name: "Pass" }).click();
  await expect(other.getByTestId("recovery-closed")).toContainText("same document");

  await other.getByRole("button", { name: "Start again" }).click();
  await other.getByLabel(/Email or mobile number on your account/).fill(contact);
  await other.getByRole("button", { name: "Start recovery" }).click();
  await other.getByRole("button", { name: "Start face check" }).click();
  await other.getByRole("button", { name: "Pass" }).click();
  await other.waitForURL("**/recover");
  await other.getByRole("button", { name: "Start ID check" }).click();
  await other.getByLabel("Document number").fill(doc);
  await other.getByRole("button", { name: "Pass" }).click();
  await expect(other.getByTestId("recovery-hold")).toBeVisible();

  // The member sees it on their signed-in device and cancels.
  await page.reload();
  await expect(page.getByTestId("recovery-banner")).toBeVisible();
  await page.getByRole("button", { name: "Cancel recovery" }).click();
  await expect(page.getByTestId("recovery-banner")).toHaveCount(0);
  await other.getByRole("button", { name: "Check again" }).click();
  await expect(other.getByTestId("recovery-closed")).toContainText("cancelled");
  await ctx.close();
});
