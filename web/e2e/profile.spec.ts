import { test, expect, type Browser, type Page } from "@playwright/test";
import { join as pathJoin } from "node:path";
import { verify } from "./helpers";

// Profile and photo upload end to end: real Chromium, the real Go services in dev mode
// (moderation vendors simulated; provenance checks and metadata stripping are real).
const IPHONE_UA = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148";
const fx = (n: string) => pathJoin(__dirname, "fixtures", n);

async function signedInPhone(browser: Browser): Promise<Page> {
  const ctx = await browser.newContext({ userAgent: IPHONE_UA, viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  const cdp = await ctx.newCDPSession(page);
  await cdp.send("WebAuthn.enable");
  await cdp.send("WebAuthn.addVirtualAuthenticator", {
    options: { protocol: "ctap2", transport: "internal", hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true },
  });
  await page.goto("/join");
  await page.getByLabel(/Email or mobile/).fill(`p${Date.now()}@example.com`);
  await page.getByRole("button", { name: "Send code" }).click();
  const code = (await page.getByTestId("dev-otp").textContent())!.match(/\d{6}/)![0];
  await page.getByLabel(/6-digit code/).fill(code);
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("button", { name: "Create passkey" }).click();
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.getByRole("button", { name: "Sign in with passkey" }).click();
  await page.getByRole("button", { name: "Start live check" }).click();
  await expect(page.getByTestId("session")).toBeVisible();
  return page;
}

test("profile: upload photos, refuse generated images, set details, preview the card", async ({ browser }) => {
  const page = await signedInPhone(browser);
  await page.getByRole("link", { name: "Your profile and photos" }).click();
  await expect(page.getByRole("heading", { name: "How other members see you" })).toBeVisible();
  await expect(page.getByText("Add a main photo that clearly shows your face.")).toBeVisible();

  // Four real uploads plus one image labelled as generator output.
  await page.getByTestId("file-input").setInputFiles([fx("photo-1.jpg"), fx("photo-2.jpg"), fx("photo-3.jpg"), fx("generated.jpg"), fx("photo-4.jpg")]);
  await expect(page.getByTestId("photo")).toHaveCount(4);
  await expect(page.locator("main").getByRole("alert")).toContainText("generated.jpg: This image is marked as computer-generated");
  await expect(page.getByText("4 of 10")).toBeVisible();
  await expect(page.getByTestId("photo").first()).toContainText("Main photo");

  // The main photo can't be blurred; another can.
  await expect(page.getByTestId("photo").first().getByRole("button", { name: "Blur" })).toHaveCount(0);
  await page.getByTestId("photo").nth(2).getByRole("button", { name: "Blur" }).click();
  await expect(page.getByTestId("photo").nth(2)).toContainText("Blurred until you reveal it");

  // Details: a name with a phone number is refused, a clean one is kept.
  await page.getByLabel("Name shown to members").fill("Call 07700 900123");
  await page.getByLabel("Name shown to members").blur();
  await expect(page.locator("main").getByRole("alert")).toContainText("contact details");
  await page.getByLabel("Name shown to members").fill("Velvet");
  await page.getByLabel("Name shown to members").blur();
  await page.getByLabel("Area").selectOption({ label: "Islington" });
  await expect(page.getByTestId("ready")).toBeVisible();

  // Make photo 2 the main photo: it moves first.
  await page.getByTestId("photo").nth(1).getByRole("button", { name: "Make main" }).click();
  await expect(page.getByTestId("photo").first()).toContainText("Main photo");

  await page.getByRole("link", { name: "Preview your card" }).click();
  const card = page.getByTestId("card");
  await expect(card).toContainText("Velvet");
  // Until the ID check is done, no age is shown and nobody else can see the card.
  await expect(card).toContainText("ID check not finished yet");
  await verify(page, `PROFILE-${Date.now()}`, "1991-02");
  await page.goto("/profile/card");
  await expect(card).toContainText(/, \d{2}/); // age from the verified ID
  await expect(card).toContainText("Islington, London · Nearby");
  await expect(card).toContainText("4 photos");
  await expect(card).toContainText("Blurred");

  // Exact location is never shown; city only hides the district.
  await page.goto("/profile");
  await page.getByLabel("Show only my city").click();
  await expect(page.getByLabel("Show only my city")).toBeChecked();
  await page.goto("/profile/card");
  await expect(page.getByTestId("card")).toContainText("London · Nearby");
  await expect(page.getByTestId("card")).not.toContainText("Islington");
});

test("profile: signed-out visitors see nothing", async ({ page }) => {
  await page.goto("/profile");
  await expect(page.getByRole("heading", { name: "Signed out" })).toBeVisible();
  await page.goto("/profile/card");
  await expect(page.getByRole("heading", { name: "Signed out" })).toBeVisible();
});
