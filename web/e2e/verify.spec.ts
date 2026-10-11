import { test, expect } from "@playwright/test";
import { member, verify } from "./helpers";

test("verification: Tonight needs it; one account per ID document", async ({ browser }) => {
  const tag = String(Date.now()).slice(-6);
  const ana = await member(browser, `Ana${tag}`, "Islington", "photo-1.jpg", { verify: false });
  await ana.goto("/tonight");
  await expect(ana.getByText("Finish your ID check to use Tonight.")).toBeVisible();
  await ana.getByRole("link", { name: "Verify now" }).click();
  await verify(ana, `PASS-${tag}`);
  await expect(ana.getByTestId("verified")).toBeVisible();

  const ben = await member(browser, `Ben${tag}`, "Camden", "photo-2.jpg", { verify: false });
  await verify(ben, `pass-${tag} `); // same document, typed differently
  await expect(ben.locator("main").getByRole("alert")).toContainText("already linked to a TRYST account");
  await expect(ben.getByTestId("verified")).toHaveCount(0);
});

test("verification: under-18s are refused and signed out everywhere", async ({ browser }) => {
  const tag = String(Date.now()).slice(-6);
  const teen = await member(browser, `Kit${tag}`, "Islington", "photo-1.jpg", { verify: false });
  const now = new Date();
  const birth = `${now.getUTCFullYear() - 18}-${String(now.getUTCMonth() + 1).padStart(2, "0")}`; // 18 this month
  await verify(teen, `TEEN-${tag}`, birth);
  await teen.waitForURL("**/verify");
  await expect(teen.getByRole("heading", { name: "Signed out" })).toBeVisible();
  await teen.goto("/account");
  await expect(teen.getByRole("heading", { name: "Signed out" })).toBeVisible();
});
