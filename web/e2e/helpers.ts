import { expect, type Browser, type Page } from "@playwright/test";
import { join as pathJoin } from "node:path";

export const IPHONE_UA = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148";
export const fx = (n: string) => pathJoin(__dirname, "fixtures", n);

/** A signed-in member with a complete profile (dev services). */
export async function member(browser: Browser, name: string, district: string, photo: string, opts: { verify?: boolean } = {}): Promise<Page> {
  const ctx = await browser.newContext({ userAgent: IPHONE_UA, viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  const cdp = await ctx.newCDPSession(page);
  await cdp.send("WebAuthn.enable");
  await cdp.send("WebAuthn.addVirtualAuthenticator", {
    options: { protocol: "ctap2", transport: "internal", hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true },
  });
  await page.goto("/join");
  await page.getByLabel(/Email or mobile/).fill(`${name.toLowerCase()}${Date.now()}@example.com`);
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
  if (opts.verify !== false) {
    await verify(page, `DOC-${name}-${Date.now()}`);
    await expect(page.getByTestId("verified")).toBeVisible();
  }
  return page;
}

/** Verification through the development stand-in provider: face check, declaration, ID. */
export async function verify(page: Page, doc: string, birth = "1990-05") {
  await page.goto("/verify");
  await page.getByRole("button", { name: "Start face check" }).click();
  await page.getByRole("button", { name: "Pass" }).click();
  await page.getByLabel("I make this declaration").check();
  await page.getByRole("button", { name: "Confirm declaration" }).click();
  await page.getByRole("button", { name: "Start ID check" }).click();
  await page.getByLabel(/Birth month/).fill(birth);
  await page.getByLabel("Document number").fill(doc);
  await page.getByRole("button", { name: "Pass" }).click();
}

/** Makes two members a mutual Tonight match and returns the thread URL. */
export async function matched(a: Page, aName: string, b: Page, bName: string): Promise<string> {
  for (const p of [a, b]) {
    await p.goto("/tonight");
    await p.getByRole("button", { name: /I.m free tonight/ }).click();
    await expect(p.getByTestId("tonight-on")).toBeVisible();
  }
  await a.reload();
  await a.getByTestId("person").filter({ hasText: bName }).getByRole("button", { name: "Interested" }).click();
  await b.reload();
  await b.getByTestId("person").filter({ hasText: aName }).getByRole("button", { name: "Interested" }).click();
  const link = b.getByTestId("mutual").filter({ hasText: aName }).getByRole("link", { name: "Message" });
  await expect(link).toBeVisible();
  return (await link.getAttribute("href"))!;
}
