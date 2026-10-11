import { test, expect, type Page } from "@playwright/test";
import { consentToDiscovery, emergencyCodes, member, stepUp } from "./helpers";

async function free(p: Page) {
  await p.goto("/tonight");
  await consentToDiscovery(p);
  const start = p.getByRole("button", { name: /I.m free tonight/ });
  if (await start.isVisible()) await start.click();
  await expect(p.getByTestId("tonight-on")).toBeVisible();
}

test("discretion: pause, hidden areas and partner shield hide people both ways; resuming needs step-up", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const A = `Ash${tag}`, B = `Bea${tag}`;
  const aContact = `ash${tag}@example.com`;
  const ash = await member(browser, A, "Islington", "photo-1.jpg", { contact: aContact });
  const bea = await member(browser, B, "Camden", "photo-2.jpg");
  const sees = async (p: Page, name: string) => {
    await p.goto("/tonight");
    return p.getByTestId("person").filter({ hasText: name });
  };
  await free(ash);
  await free(bea);
  await expect(await sees(ash, B)).toHaveCount(1);

  // Pause: Bea disappears at once; coming back needs both checks again.
  await bea.goto("/account");
  await bea.getByTestId("discretion").getByRole("button", { name: "Pause" }).click();
  await expect(bea.getByTestId("discretion-status")).toContainText("Paused");
  await expect(await sees(ash, B)).toHaveCount(0);
  await bea.goto("/account");
  await bea.getByRole("button", { name: "Come back to discovery" }).click();
  await stepUp(bea);
  await expect(bea.getByTestId("discretion-status")).toContainText("in discovery");
  await free(bea);
  await expect(await sees(ash, B)).toHaveCount(1);

  // Bea hides Islington: she and Ash no longer see each other.
  await bea.goto("/account");
  await bea.getByLabel("Hide an area").selectOption({ label: "Islington, London" });
  await expect(bea.getByTestId("zones")).toContainText("Islington");
  await expect(await sees(ash, B)).toHaveCount(0);
  await expect(await sees(bea, A)).toHaveCount(0);
  await bea.goto("/account");
  await bea.getByTestId("zones").getByRole("button", { name: "Remove" }).click();
  // Removing a hidden area shows her to more people: allowed now only because her step-up
  // from resuming is still fresh (5 minutes).
  await expect(bea.getByTestId("zones")).not.toContainText("Islington");
  await expect(await sees(ash, B)).toHaveCount(1);

  // Partner shield: Bea adds Ash's email; neither sees the other, and nobody is told.
  await bea.goto("/account");
  await bea.getByLabel("Partner’s email or phone").fill(aContact.toUpperCase());
  await bea.getByRole("button", { name: "Add to shield" }).click();
  await expect(bea.getByTestId("discretion")).toContainText("Shielded contacts: 1");
  await expect(await sees(ash, B)).toHaveCount(0);
  await expect(await sees(bea, A)).toHaveCount(0);

  for (const p of [ash, bea]) {
    await p.goto("/tonight");
    await p.getByRole("button", { name: "Stop now" }).click();
  }
});

test("emergency lock: the code from sign-up locks the account from any browser, without signing in", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const contact = `cal${tag}@example.com`;
  const cal = await member(browser, `Cal${tag}`, "Hackney", "photo-3.jpg", { contact, verify: false });
  const code = emergencyCodes.get(cal)!;
  expect(code).toMatch(/^[A-Z2-9]{4}-[A-Z2-9]{4}-[A-Z2-9]{4}$/);

  const ctx = await browser.newContext();
  const other = await ctx.newPage();
  await other.goto("/lock");
  await other.getByLabel(/Email or mobile number on the account/).fill(contact);
  await other.getByLabel("Emergency code").fill(code.toLowerCase());
  await other.getByRole("button", { name: "Lock the account" }).click();
  await expect(other.getByTestId("lock-done")).toBeVisible();
  await ctx.close();

  await cal.goto("/account");
  await expect(cal.getByRole("heading", { name: "Signed out" })).toBeVisible();
  await cal.goto("/sign-in");
  await cal.getByRole("button", { name: "Sign in with passkey" }).click();
  await cal.getByRole("button", { name: "Start live check" }).click();
  await expect(cal.getByRole("main").getByRole("alert")).toContainText("locked");
});
