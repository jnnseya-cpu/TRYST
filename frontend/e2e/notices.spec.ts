import { test, expect } from "@playwright/test";
import { matched, member } from "./helpers";

test("notices: masked in-app notices for a match and a message; notification wording rules", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const A = `Fern${tag}`, B = `Reed${tag}`;
  const fern = await member(browser, A, "Islington", "photo-1.jpg");
  const reed = await member(browser, B, "Camden", "photo-2.jpg");
  const url = await matched(fern, A, reed, B);

  await reed.goto(url);
  await fern.goto(url);
  await fern.getByLabel("Message", { exact: true }).fill("Hi there");
  await fern.getByRole("button", { name: "Send" }).click();
  await expect(fern.getByTestId("message").last()).toContainText("Hi there");

  await reed.goto("/account");
  await expect(reed.getByTestId("notices-link")).toHaveText("Notices (2)");
  await reed.getByTestId("notices-link").click();
  const items = reed.getByTestId("notice");
  await expect(items).toHaveCount(2);
  await expect(items.nth(0)).toContainText("You have a new message.");
  await expect(items.nth(1)).toContainText("You have a new match.");
  // Masked: no names anywhere in the notices.
  await expect(reed.locator("main")).not.toContainText(A);
  await reed.goto("/account");
  await expect(reed.getByTestId("notices-link")).toHaveText("Notices");

  // Wording can't name the app; without browser permission nothing is left half-on.
  const box = reed.getByTestId("notify-settings");
  await box.getByLabel("Words to show").fill("New TRYST match");
  await box.getByRole("button", { name: "Turn notifications on" }).click();
  await expect(box.getByRole("status")).toContainText(/don't mention the app/);
  await box.getByLabel("Words to show").fill("Reminder");
  await box.getByRole("button", { name: "Turn notifications on" }).click();
  await expect(box.getByRole("status")).toContainText(/blocked|can't show/);
  await expect(box.getByRole("button", { name: "Turn notifications on" })).toBeVisible();
});
