import { test, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { join } from "node:path";

// Captures every screen of the platform at desktop and phone sizes, driving real flows
// (real passkeys on virtual authenticators, real identity service). Output: docs/screenshots/.
const OUT = join(__dirname, "..", "..", "docs", "screenshots");
mkdirSync(OUT, { recursive: true });

const IPHONE_UA = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148";
const DESKTOP_UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/537.36 Chrome/140.0 Safari/537.36";

let n = 0;
async function shot(page: Page, name: string, fullPage = false) {
  n += 1;
  await page.waitForTimeout(250);
  await page.screenshot({ path: join(OUT, `${String(n).padStart(2, "0")}-${name}.png`), fullPage });
}

async function device(browser: Browser, kind: "desktop" | "phone"): Promise<{ ctx: BrowserContext; page: Page }> {
  const ctx = await browser.newContext(
    kind === "desktop"
      ? { userAgent: DESKTOP_UA, viewport: { width: 1440, height: 900 }, deviceScaleFactor: 2, reducedMotion: "reduce" }
      : { userAgent: IPHONE_UA, viewport: { width: 390, height: 844 }, deviceScaleFactor: 3, isMobile: true, hasTouch: true, reducedMotion: "reduce" },
  );
  const page = await ctx.newPage();
  const cdp = await ctx.newCDPSession(page);
  await cdp.send("WebAuthn.enable");
  await cdp.send("WebAuthn.addVirtualAuthenticator", {
    options: { protocol: "ctap2", transport: "internal", hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true },
  });
  return { ctx, page };
}

test.describe.configure({ mode: "serial" });
test.setTimeout(240_000);

test("landing page", async ({ browser }) => {
  const { page } = await device(browser, "desktop");
  await page.goto("/");
  await shot(page, "landing-hero-desktop");
  for (const [sel, name] of [
    [".lp-manifesto", "landing-manifesto"], [".lp-grid3", "landing-principles"], ["#how", "landing-how-it-works"],
    [".lp-modes", "landing-modes"], [".lp-split", "landing-discretion"], [".lp-signin", "landing-sign-in-security"],
    ["#membership", "landing-membership"], [".lp-safety", "landing-before-you-meet"], [".lp-final", "landing-close"],
  ] as const) {
    await page.locator(sel).first().scrollIntoViewIfNeeded();
    await page.locator(sel).first().evaluate((el) => (el.closest("section") ?? el).scrollIntoView({ block: "start" }));
    await shot(page, `${name}-desktop`);
  }
  await page.evaluate(() => window.scrollTo(0, 0));
  await shot(page, "landing-full-desktop", true);

  const phone = await device(browser, "phone");
  await phone.page.goto("/");
  await shot(phone.page, "landing-hero-phone");
  await shot(phone.page, "landing-full-phone", true);
});

test("computer: join, link phone, sign in with phone approval", async ({ browser }) => {
  const laptop = (await device(browser, "desktop")).page;
  const phone = (await device(browser, "phone")).page;
  const contact = `screens${Date.now()}@example.com`;

  await laptop.goto("/join");
  await laptop.getByLabel(/Email or mobile/).fill(contact);
  await shot(laptop, "join-contact-desktop");
  await laptop.getByRole("button", { name: "Send code" }).click();
  const code = (await laptop.getByTestId("dev-otp").textContent())!.match(/\d{6}/)![0];
  await laptop.getByLabel(/6-digit code/).fill(code);
  await shot(laptop, "join-code-desktop");
  await laptop.getByRole("button", { name: "Continue" }).click();
  await laptop.getByRole("heading", { name: "Create your passkey" }).waitFor();
  await shot(laptop, "join-create-passkey-desktop");
  await laptop.getByRole("button", { name: "Create passkey" }).click();
  await laptop.getByRole("heading", { name: "Add your second check" }).waitFor();
  await shot(laptop, "join-second-check-desktop");
  await laptop.getByRole("button", { name: "Link my phone" }).click();
  await laptop.getByTestId("enrol-link").waitFor();
  await shot(laptop, "join-link-phone-qr-desktop");

  await phone.goto((await laptop.getByTestId("enrol-link").getAttribute("href"))!);
  await shot(phone, "enrol-phone-ready-phone");
  await phone.getByRole("button", { name: "Link this phone" }).click();
  await phone.getByRole("heading", { name: "Phone linked" }).waitFor();
  await shot(phone, "enrol-phone-linked-phone");

  await laptop.getByRole("button", { name: "I've linked my phone" }).click();
  await laptop.getByRole("button", { name: "Sign in with passkey" }).waitFor();
  await shot(laptop, "sign-in-start-desktop");
  await laptop.getByRole("button", { name: "Sign in with passkey" }).click();
  await laptop.getByRole("button", { name: "Show code for my phone" }).waitFor();
  await shot(laptop, "sign-in-second-check-desktop");
  await laptop.getByRole("button", { name: "Show code for my phone" }).click();
  await laptop.getByTestId("approve-link").waitFor();
  await shot(laptop, "sign-in-waiting-for-phone-desktop");

  await phone.goto((await laptop.getByTestId("approve-link").getAttribute("href"))!);
  await shot(phone, "approve-sign-in-phone");
  await phone.getByRole("button", { name: "Approve" }).click();
  await phone.getByRole("heading", { name: "Approved" }).waitFor();
  await shot(phone, "approve-approved-phone");

  await laptop.getByTestId("session").waitFor();
  await shot(laptop, "account-signed-in-desktop");
});

test("phone: join and sign in with two biometric checks", async ({ browser }) => {
  const phone = (await device(browser, "phone")).page;
  await phone.goto("/join");
  await phone.getByLabel(/Email or mobile/).fill(`phone${Date.now()}@example.com`);
  await shot(phone, "join-contact-phone");
  await phone.getByRole("button", { name: "Send code" }).click();
  const code = (await phone.getByTestId("dev-otp").textContent())!.match(/\d{6}/)![0];
  await phone.getByLabel(/6-digit code/).fill(code);
  await phone.getByRole("button", { name: "Continue" }).click();
  await phone.getByRole("button", { name: "Create passkey" }).click();
  await phone.getByRole("heading", { name: "You're set up" }).waitFor();
  await shot(phone, "join-done-phone");
  await phone.getByRole("button", { name: "Sign in" }).click();
  await phone.getByRole("button", { name: "Sign in with passkey" }).click();
  await phone.getByRole("button", { name: "Start live check" }).waitFor();
  await shot(phone, "sign-in-live-face-check-phone");
  await phone.getByRole("button", { name: "Start live check" }).click();
  await phone.getByTestId("session").waitFor();
  await shot(phone, "account-signed-in-phone");
  await phone.goto("/account");
  await phone.evaluate(() => sessionStorage.clear());
  await phone.reload();
  await phone.getByRole("heading", { name: "Signed out" }).waitFor();
  await shot(phone, "signed-out-phone");
});
