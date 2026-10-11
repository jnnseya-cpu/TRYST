import { test, expect } from "@playwright/test";
import { matched, member } from "./helpers";

test("messages: end-to-end encrypted conversation after a mutual match, with safety and block", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const A = `Ivy${tag}`, B = `Rowan${tag}`;
  const ivy = await member(browser, A, "Islington", "photo-1.jpg");
  const rowan = await member(browser, B, "Camden", "photo-2.jpg");
  const url = await matched(ivy, A, rowan, B);

  // Both open the conversation (each registers its device key).
  await rowan.goto(url);
  await ivy.goto(url);
  await expect(ivy.getByRole("heading", { name: new RegExp(B) })).toBeVisible();
  await expect(ivy.getByText(/You both said yes/)).toBeVisible();

  await ivy.getByLabel("Message", { exact: true }).fill("Hello! Drinks at the bar by the station, 8pm?");
  await ivy.getByRole("button", { name: "Send" }).click();
  await expect(ivy.getByTestId("message").last()).toContainText("Drinks at the bar");
  await expect(rowan.getByTestId("message").last()).toContainText("Drinks at the bar", { timeout: 10_000 });

  // A money request triggers the fraud warning on the recipient's device only.
  await rowan.getByLabel("Message", { exact: true }).fill("Lovely. Could you send me £200 first? I'll pay you back");
  await rowan.getByRole("button", { name: "Send" }).click();
  await expect(ivy.locator("main").getByRole("alert").filter({ hasText: "asks for money" })).toBeVisible({ timeout: 10_000 });
  await expect(rowan.locator("main").getByRole("alert").filter({ hasText: "asks for money" })).toHaveCount(0);

  // A child-safety message can't be sent at all.
  await rowan.getByLabel("Message", { exact: true }).fill("send me pics of your daughter");
  await rowan.getByRole("button", { name: "Send" }).click();
  await expect(rowan.locator("main").getByRole("alert").filter({ hasText: "can't be sent" })).toBeVisible();
  // The sender is paused until a moderator decides (03 §7.3: suspected minor, 15 minutes).
  await rowan.reload();
  await expect(rowan.getByRole("heading", { name: "Account paused" })).toBeVisible();
  await expect(rowan.getByLabel("Message", { exact: true })).toHaveCount(0);
  await rowan.goto("/account");
  await expect(rowan.getByTestId("standing")).toContainText("Paused for a safety review");
  await ivy.reload();
  await expect(ivy.getByText("This conversation is paused while our safety team takes a look.")).toBeVisible();

  // The list shows the conversation; blocking (on another pair) is covered in Go tests.
  await ivy.goto("/messages");
  await expect(ivy.getByTestId("thread").filter({ hasText: B })).toContainText("Paused for a safety review");
});

test("messages: blocking closes the conversation for both", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const A = `Wren${tag}`, B = `Ash${tag}`;
  const wren = await member(browser, A, "Islington", "photo-3.jpg");
  const ash = await member(browser, B, "Camden", "photo-4.jpg");
  const url = await matched(wren, A, ash, B);
  await ash.goto(url);
  await wren.goto(url);
  wren.once("dialog", (d) => d.accept());
  await wren.getByRole("button", { name: "Block" }).click();
  await expect(wren.getByText("This conversation has ended.")).toBeVisible();
  await ash.reload();
  await expect(ash.getByText("This conversation has ended.")).toBeVisible();
  await expect(ash.getByLabel("Message", { exact: true })).toHaveCount(0);
  // They no longer see each other in Tonight.
  await ash.goto("/tonight");
  await expect(ash.getByTestId("person").filter({ hasText: A })).toHaveCount(0);
  await expect(ash.getByTestId("mutual").filter({ hasText: A })).toHaveCount(0);
});
