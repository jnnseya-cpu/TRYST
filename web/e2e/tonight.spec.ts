import { test, expect, type Browser, type Page } from "@playwright/test";
import { join as pathJoin } from "node:path";

// Tonight end to end with two real members (dev services).
const IPHONE_UA = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148";
const fx = (n: string) => pathJoin(__dirname, "fixtures", n);

async function member(browser: Browser, name: string, district: string, photo: string): Promise<Page> {
  const ctx = await browser.newContext({ userAgent: IPHONE_UA, viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  const cdp = await ctx.newCDPSession(page);
  await cdp.send("WebAuthn.enable");
  await cdp.send("WebAuthn.addVirtualAuthenticator", {
    options: { protocol: "ctap2", transport: "internal", hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true },
  });
  await page.goto("/join");
  await page.getByLabel(/Email or mobile/).fill(`${name}${Date.now()}@example.com`);
  await page.getByRole("button", { name: "Send code" }).click();
  const code = (await page.getByTestId("dev-otp").textContent())!.match(/\d{6}/)![0];
  await page.getByLabel(/6-digit code/).fill(code);
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("button", { name: "Create passkey" }).click();
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.getByRole("button", { name: "Sign in with passkey" }).click();
  await page.getByRole("button", { name: "Start live check" }).click();
  await expect(page.getByTestId("session")).toBeVisible();
  await page.goto("/profile");
  await page.getByTestId("file-input").setInputFiles([fx(photo)]);
  await expect(page.getByTestId("photo")).toHaveCount(1);
  await page.getByLabel("Name shown to members").fill(name);
  await page.getByLabel("Name shown to members").blur();
  await page.getByLabel("Area").selectOption({ label: district });
  await expect(page.getByTestId("ready")).toBeVisible();
  return page;
}

test("tonight: reciprocal visibility and private interest until it's mutual", async ({ browser }) => {
  // Unique names: other test runs may have members free in the same dev service.
  const tag = String(Date.now()).slice(-5);
  const A = `Ana${tag}`, B = `Ben${tag}`;
  const ana = await member(browser, A, "Islington", "photo-1.jpg");
  const ben = await member(browser, B, "Camden", "photo-2.jpg");
  const anaSees = (n: string) => ana.getByTestId("person").filter({ hasText: n });
  const benSees = (n: string) => ben.getByTestId("person").filter({ hasText: n });

  // Ana goes free; Ben, who is not free, sees nothing of her.
  await ana.goto("/tonight");
  await ana.getByLabel("For", { exact: true }).selectOption("4");
  await ana.getByRole("button", { name: /I.m free tonight/ }).click();
  await expect(ana.getByTestId("tonight-on")).toBeVisible();
  await expect(anaSees(B)).toHaveCount(0);
  await ben.goto("/tonight");
  await expect(ben.getByRole("button", { name: /I.m free tonight/ })).toBeVisible();
  await expect(ben.getByTestId("person")).toHaveCount(0); // not free: sees nobody at all

  // Ben goes free: each now sees the other.
  await ben.getByRole("button", { name: /I.m free tonight/ }).click();
  await expect(benSees(A)).toHaveCount(1);
  await expect(benSees(A)).toContainText("Islington, London");
  await ana.reload();
  await expect(anaSees(B)).toHaveCount(1);

  // Ana's interest is private.
  await anaSees(B).getByRole("button", { name: "Interested" }).click();
  await expect(anaSees(B)).toContainText("Interest sent");
  await ben.reload();
  await expect(ben.getByTestId("mutual").filter({ hasText: A })).toHaveCount(0);
  await expect(benSees(A).getByRole("button", { name: "Interested" })).toBeVisible();

  // Ben says yes too: mutual for both.
  await benSees(A).getByRole("button", { name: "Interested" }).click();
  await expect(ben.getByTestId("mutual").filter({ hasText: A })).toHaveCount(1);
  await ana.reload();
  await expect(ana.getByTestId("mutual").filter({ hasText: B })).toHaveCount(1);

  // Stopping hides Ana at once.
  await ana.getByRole("button", { name: "Stop now" }).click();
  await ben.reload();
  await expect(ben.getByTestId("mutual").filter({ hasText: A })).toHaveCount(0);
  await expect(benSees(A)).toHaveCount(0);
  await ben.getByRole("button", { name: "Stop now" }).click(); // leave the shared service clean
});
