import { test, expect, type Browser, type Page } from "@playwright/test";

const IPHONE_UA = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148";
const DESKTOP_UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/140.0 Safari/537.36";

/** A browser context with a virtual platform authenticator that has biometric user verification. */
async function deviceWithPasskeys(browser: Browser, userAgent: string, transport: "internal" | "usb" = "internal"): Promise<Page> {
  const ctx = await browser.newContext({ userAgent });
  const page = await ctx.newPage();
  const cdp = await ctx.newCDPSession(page);
  await cdp.send("WebAuthn.enable");
  await cdp.send("WebAuthn.addVirtualAuthenticator", {
    options: { protocol: "ctap2", transport, hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true },
  });
  return page;
}

async function join(page: Page, contact: string) {
  await page.goto("/join");
  await page.getByLabel(/Email or mobile/).fill(contact);
  await page.getByRole("button", { name: "Send code" }).click();
  const dev = await page.getByTestId("dev-otp").textContent();
  const code = dev!.match(/\d{6}/)![0];
  await page.getByLabel(/6-digit code/).fill(code);
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("button", { name: "Create passkey" }).click();
}

test("phone: passkey with biometric + live face check, and nothing less", async ({ browser }) => {
  const phone = await deviceWithPasskeys(browser, IPHONE_UA);
  await join(phone, `m${Date.now()}@example.com`);
  await expect(phone.getByRole("heading", { name: "You're set up" })).toBeVisible();
  await phone.getByRole("button", { name: "Sign in" }).click();

  await phone.getByRole("button", { name: "Sign in with passkey" }).click();
  await expect(phone.getByRole("heading", { name: "Live face check" })).toBeVisible();
  // After the first factor there is still no session.
  await phone.goto("/account");
  await expect(phone.getByRole("heading", { name: "Signed out" })).toBeVisible();

  await phone.goto("/sign-in");
  await phone.getByRole("button", { name: "Sign in with passkey" }).click();
  await phone.getByRole("button", { name: "Start live check" }).click();
  await expect(phone.getByTestId("session")).toContainText("phone");
});

test("computer: passkey + approval from the registered phone", async ({ browser }) => {
  const laptop = await deviceWithPasskeys(browser, DESKTOP_UA);
  const phone = await deviceWithPasskeys(browser, IPHONE_UA);

  await join(laptop, `d${Date.now()}@example.com`);
  await expect(laptop.getByRole("heading", { name: "Add your second check" })).toBeVisible();
  await laptop.getByRole("button", { name: "Link my phone" }).click();
  const enrolLink = await laptop.getByTestId("enrol-link").getAttribute("href");

  await phone.goto(enrolLink!);
  await phone.getByRole("button", { name: "Link this phone" }).click();
  await expect(phone.getByRole("heading", { name: "Phone linked" })).toBeVisible();

  await laptop.getByRole("button", { name: "I've linked my phone" }).click();
  await laptop.getByRole("button", { name: "Sign in with passkey" }).click();
  await laptop.getByRole("button", { name: "Show code for my phone" }).click();
  const approveLink = await laptop.getByTestId("approve-link").getAttribute("href");

  await phone.goto(approveLink!);
  await phone.getByRole("button", { name: "Approve" }).click();
  await expect(phone.getByRole("heading", { name: "Approved" })).toBeVisible();

  await expect(laptop.getByTestId("session")).toContainText("computer");
});

test("computer: declining on the phone blocks the sign-in", async ({ browser }) => {
  const laptop = await deviceWithPasskeys(browser, DESKTOP_UA);
  const phone = await deviceWithPasskeys(browser, IPHONE_UA);
  await join(laptop, `x${Date.now()}@example.com`);
  await laptop.getByRole("button", { name: "Link my phone" }).click();
  await phone.goto((await laptop.getByTestId("enrol-link").getAttribute("href"))!);
  await phone.getByRole("button", { name: "Link this phone" }).click();
  await expect(phone.getByRole("heading", { name: "Phone linked" })).toBeVisible();

  await laptop.getByRole("button", { name: "I've linked my phone" }).click();
  await laptop.getByRole("button", { name: "Sign in with passkey" }).click();
  await laptop.getByRole("button", { name: "Show code for my phone" }).click();
  await phone.goto((await laptop.getByTestId("approve-link").getAttribute("href"))!);
  await phone.getByRole("button", { name: "It wasn't me" }).click();
  await expect(phone.getByRole("heading", { name: "Declined" })).toBeVisible();
  await expect(laptop.locator("main").getByRole("alert")).toContainText(/declined/i);
  await laptop.goto("/account");
  await expect(laptop.getByRole("heading", { name: "Signed out" })).toBeVisible();
});
