import { test, expect } from "@playwright/test";

const MOD1 = "e2e-mod-one-token-0001";
const LEAD = "e2e-lead-token-00000003";

async function staff(browser: import("@playwright/test").Browser, token: string) {
  const page = await (await browser.newContext()).newPage();
  await page.goto("/ops");
  await page.getByLabel("Staff token").fill(token);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Case queue" })).toBeVisible();
  await page.getByRole("link", { name: "Communications" }).click();
  await expect(page.getByRole("heading", { name: "Communications" })).toBeVisible();
  return page;
}

test("communications: catalogue, coverage, neutral previews, lead-only test sends to self", async ({ browser }) => {
  const mod = await staff(browser, MOD1);
  await expect(mod.getByTestId("comms-stats")).toContainText("Catalogue messages");
  await expect(mod.getByTestId("coverage-email")).toBeVisible();
  await expect(mod.getByText(/WhatsApp messages\./)).toBeVisible();
  await mod.getByRole("button", { name: "Send test to me" }).click();
  await expect(mod.getByTestId("test-result")).toContainText("Only a safety lead");

  const lead = await staff(browser, LEAD);
  // The sign-up code email: neutral subject, plain text, never the brand name.
  const preview = lead.getByTestId("preview");
  await expect(preview).toContainText("Subject: Your verification code");
  await expect(preview).not.toContainText("TRYST is");
  // A push never carries the message.
  await lead.getByLabel("Message").selectOption("message.new");
  await expect(preview).toContainText("You have an update");

  await lead.getByLabel("Message").selectOption("account.signup_code");
  await lead.getByRole("button", { name: "Send test to me" }).click();
  await expect(lead.getByTestId("test-result")).toContainText("Email: recorded in sandbox");
  await expect(lead.getByTestId("deliveries")).toContainText("account.signup_code (test)");
});
