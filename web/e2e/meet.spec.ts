import { test, expect } from "@playwright/test";
import { matched, member } from "./helpers";

test("meet: propose, confirm, door check, check-ins with duress, home safe, panic", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const A = `Fern${tag}`, B = `Cedar${tag}`;
  const fern = await member(browser, A, "Islington", "photo-1.jpg");
  const cedar = await member(browser, B, "Camden", "photo-2.jpg");
  const url = await matched(fern, A, cedar, B);
  await cedar.goto(url); // registers Cedar's device
  await fern.goto(url);

  // Fern suggests a public place; the server never receives it.
  await fern.goto(url + "/meet");
  await fern.getByLabel("Where").fill("The Crown, Upper Street");
  const later = new Date(Date.now() + 2 * 3600_000);
  await fern.getByLabel("Day").fill(later.toISOString().slice(0, 10));
  await fern.getByLabel("From").fill(`${String(later.getHours()).padStart(2, "0")}:00`);
  await fern.getByLabel("Until").fill(`${String((later.getHours() + 3) % 24).padStart(2, "0")}:00`);
  await fern.getByRole("button", { name: "Send suggestion" }).click();
  await expect(fern.getByTestId("plan")).toContainText("Waiting for");

  // Cedar sees it in the conversation and confirms.
  await cedar.reload();
  await expect(cedar.getByTestId("message").last()).toContainText("The Crown, Upper Street", { timeout: 10_000 });
  await cedar.goto(url + "/meet");
  await cedar.getByRole("button", { name: "Confirm" }).click();
  await expect(cedar.getByTestId("plan")).toContainText("Confirmed by both of you");

  // Door check: both phones show the same three words.
  await fern.reload();
  await fern.getByRole("button", { name: /I.m here/ }).click();
  await cedar.getByRole("button", { name: /I.m here/ }).click();
  const wf = await fern.getByTestId("door").locator(".mt-words").textContent();
  const wc = await cedar.getByTestId("door").locator(".mt-words").textContent();
  expect(wf).toBeTruthy();
  expect(wf).toBe(wc);

  // Check-ins: duress PIN must look exactly like a normal one.
  await fern.getByLabel("Check-in PIN").fill("2468");
  await fern.getByLabel("Duress PIN").fill("1357");
  await fern.getByLabel(/Trusted contact/).fill("+447700900123");
  await fern.getByRole("button", { name: "Start check-ins" }).click();
  await expect(fern.getByTestId("checkins")).toBeVisible();
  await fern.getByLabel("PIN", { exact: true }).fill("2468");
  await fern.getByRole("button", { name: "Check in", exact: true }).click();
  const normal = await fern.getByRole("status").textContent();
  await fern.getByLabel("PIN", { exact: true }).fill("1357");
  await fern.getByRole("button", { name: "Check in", exact: true }).click();
  await expect(fern.getByRole("status")).toHaveText(normal!);
  await fern.getByLabel("PIN", { exact: true }).fill("2468");
  await fern.getByRole("button", { name: /home safe/ }).click();
  await expect(fern.getByRole("status")).toContainText("Glad you're safe");

  // Panic shows the 999 screen with the Silent Solution.
  await cedar.getByRole("button", { name: "Panic" }).click();
  await expect(cedar.getByTestId("panic")).toContainText("Call 999");
  await expect(cedar.getByTestId("panic")).toContainText("55");
  await expect(cedar.getByRole("link", { name: "Call 999" })).toHaveAttribute("href", "tel:999");
});
