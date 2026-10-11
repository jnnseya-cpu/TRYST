import { test, expect, type Page } from "@playwright/test";
import { member } from "./helpers";

async function intake(p: Page, me: string, meet: string, shapes: string[]) {
  await p.goto("/introductions");
  const form = p.getByTestId("intake");
  await form.getByRole("radio", { name: me, exact: true }).check();
  await form.getByRole("group", { name: "I want to meet" }).getByLabel(meet, { exact: true }).check();
  for (const s of shapes) await form.getByRole("group", { name: "I'm looking for" }).getByLabel(s, { exact: true }).check();
  await form.getByRole("group", { name: /Interests/ }).getByLabel("Art", { exact: true }).check();
  await form.getByRole("button", { name: "Confirm my answers" }).click();
  await expect(form).toHaveCount(0);
  const consent = p.getByTestId("art9-consent");
  if (await consent.isVisible()) {
    await consent.getByLabel("I give my explicit consent").check();
    await consent.getByRole("button", { name: "Continue" }).click();
  }
}

test("introductions: a private intent, a reply, then both accept and a conversation opens", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const I = `Ivy${tag}`, J = `Jon${tag}`;
  const ivy = await member(browser, I, "Islington", "photo-1.jpg");
  const jon = await member(browser, J, "Camden", "photo-2.jpg");
  await intake(ivy, "Woman", "Men", ["Ongoing"]);
  await intake(jon, "Man", "Women", ["Ongoing", "One-off"]);

  await ivy.reload();
  const card = ivy.getByTestId("card").filter({ hasText: J });
  await expect(card).toContainText("You want the same kind of thing");
  await expect(card).toContainText("Both into: Art");
  await card.getByRole("button", { name: "I’d like to meet" }).click();
  await expect(ivy.getByTestId("card").filter({ hasText: J })).toContainText("Sent.");
  await expect(ivy.getByTestId("free-left")).toContainText("2 free intents left this week");

  await jon.reload();
  const asks = jon.getByRole("region", { name: "Someone would like to meet you" }).getByTestId("card").filter({ hasText: I });
  await asks.getByRole("button", { name: "Yes, introduce us" }).click();
  const hs = jon.getByTestId("handshake").filter({ hasText: I });
  await hs.getByRole("button", { name: "Accept" }).click();
  await expect(hs).toContainText("Waiting for the other side");

  await ivy.reload();
  const mine = ivy.getByTestId("handshake").filter({ hasText: J });
  await mine.getByRole("button", { name: "Accept" }).click();
  await expect(ivy.getByTestId("handshake").filter({ hasText: J }).getByRole("link", { name: "Message" })).toBeVisible();
});

test("couples: link from two devices, veto mode needs both, either can end it", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const K = `Kit${tag}`, L = `Lou${tag}`;
  const kit = await member(browser, K, "Hackney", "photo-3.jpg");
  const lou = await member(browser, L, "Shoreditch", "photo-4.jpg");
  await intake(kit, "Woman", "Women", ["Joining a couple"]);
  await intake(lou, "Man", "Women", ["Joining a couple"]);

  await kit.goto("/couple");
  await kit.getByRole("button", { name: "Create an invitation" }).click();
  const link = (await kit.getByTestId("invite-link").getAttribute("href"))!;
  await lou.goto(link);
  await lou.getByRole("button", { name: "Look at the invitation" }).click();
  await expect(lou.getByTestId("preview")).toContainText(K);
  await lou.getByRole("button", { name: "Confirm and link" }).click();
  await expect(lou.getByTestId("couple")).toContainText(`Linked with ${K}`);

  await kit.goto("/couple");
  await kit.getByRole("button", { name: "Ask for “both”" }).click();
  await expect(kit.getByTestId("veto")).toContainText("Now: either · you asked for “both”");
  await lou.reload();
  await lou.getByRole("button", { name: "Ask for “both”" }).click();
  await expect(lou.getByTestId("veto")).toContainText("Now: both");

  await lou.getByRole("button", { name: "End our couple link" }).click();
  await expect(lou.getByRole("heading", { name: "Invite your partner" })).toBeVisible();
  await kit.reload();
  await expect(kit.getByRole("heading", { name: "Invite your partner" })).toBeVisible();
});
