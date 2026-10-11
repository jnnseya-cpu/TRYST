import { test, expect } from "@playwright/test";
import { consentToDiscovery, member } from "./helpers";

// Tonight end to end with two real members (dev services).
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
  // Discovery needs the separate Article 9 consent first.
  await expect(ana.getByTestId("art9-consent")).toBeVisible();
  await consentToDiscovery(ana);
  await ana.getByLabel("For", { exact: true }).selectOption("4");
  await ana.getByRole("button", { name: /I.m free tonight/ }).click();
  await expect(ana.getByTestId("tonight-on")).toBeVisible();
  await expect(anaSees(B)).toHaveCount(0);
  await ben.goto("/tonight");
  await consentToDiscovery(ben);
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

  // Withdrawing the Article 9 consent takes Ben out of discovery at once.
  await ana.getByRole("button", { name: /I.m free tonight/ }).click();
  await expect(anaSees(B)).toHaveCount(1);
  await ben.goto("/account");
  await ben.getByTestId("consent-art9_processing").getByRole("button", { name: "Withdraw" }).click();
  await expect(ben.getByTestId("consent-art9_processing")).toContainText("not given");
  await ana.reload();
  await expect(anaSees(B)).toHaveCount(0);
  await ben.goto("/tonight");
  await expect(ben.getByTestId("art9-consent")).toBeVisible();
  await ana.getByRole("button", { name: "Stop now" }).click(); // leave the shared service clean
});
