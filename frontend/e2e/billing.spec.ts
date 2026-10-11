import { test, expect } from "@playwright/test";
import { member } from "./helpers";

const LEAD = "e2e-lead-token-00000003";

test("billing: launch catalogue, subscribe with pre-contract notice, one-step cooling-off cancel, voucher", async ({ browser, request }) => {
  const tag = String(Date.now()).slice(-5);
  const page = await member(browser, `Sky${tag}`, "Islington", "photo-1.jpg");
  await page.goto("/billing");

  // Only what exists is sold: TRYST+ (monthly, annual). No Keys, deposit, DUO, ENVOY or GHOST.
  await expect(page.getByTestId("plan")).toHaveCount(2);
  await expect(page.getByRole("button", { name: /Buy 60 Keys/ })).toHaveCount(0);
  await expect(page.getByText("DUO")).toHaveCount(0);
  const refused = await page.evaluate(async () => {
    const r = await fetch("/v1/subscription", { method: "POST", headers: { Authorization: `Bearer ${sessionStorage.getItem("tryst.session")}`, "Content-Type": "application/json" },
      body: JSON.stringify({ plan: "duo", pre_contract_ack: true, start_now: true }) });
    return r.status;
  });
  expect(refused).toBe(409);

  // Subscribing needs the plan details read and a separate request to start now.
  await page.getByTestId("plan").filter({ hasText: "TRYST+" }).first().getByRole("button", { name: "Choose TRYST+" }).click();
  await expect(page.getByText(/£24.99 every month, charged now/)).toBeVisible();
  const go = page.getByRole("button", { name: "Continue to payment" });
  await expect(go).toBeDisabled();
  await page.getByLabel(/read the plan details/).check();
  await expect(go).toBeDisabled();
  await page.getByLabel(/Start my plan now/).check();
  await go.click();
  await page.getByRole("button", { name: "Pay (test)" }).click();
  await expect(page.getByTestId("current")).toContainText("TRYST+");
  await expect(page.getByTestId("subscription")).toContainText("Within 14 days");

  // One step, no questions: ends now with a refund for unused days.
  await page.getByRole("button", { name: "Cancel TRYST+" }).click();
  await expect(page.getByRole("status")).toContainText(/Cancelled\. Your plan has ended.*Refund: £/);
  await expect(page.getByTestId("current")).toContainText("Verified (free)");

  // A lead can issue TRYST+ gift codes, but not codes for things not on sale.
  const keysCode = await request.post("/ops/v1/vouchers", { headers: { Authorization: `Staff ${LEAD}` }, data: { grant: "keys:60", count: 1 } });
  expect(keysCode.status()).toBe(422);
  const issued = await request.post("/ops/v1/vouchers", { headers: { Authorization: `Staff ${LEAD}` }, data: { grant: "plan:tryst_plus:30", count: 1 } });
  expect(issued.status()).toBe(201);
  const code = (await issued.json()).codes[0] as string;
  await page.getByLabel("Code").fill(code);
  await page.getByRole("button", { name: "Apply code" }).click();
  await expect(page.getByRole("status")).toContainText("Code applied");
  await expect(page.getByTestId("current")).toContainText("TRYST+");
  await page.getByLabel("Code").fill(code);
  await page.getByRole("button", { name: "Apply code" }).click();
  await expect(page.locator("main").getByRole("alert")).toContainText(/isn't valid or has already been used/);
});
