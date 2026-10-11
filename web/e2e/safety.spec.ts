import { test, expect } from "@playwright/test";
import { matched, member } from "./helpers";

const MOD1 = "e2e-mod-one-token-0001";
const LEAD = "e2e-lead-token-00000003";

test("safety: report with chosen messages, moderator decision, member appeal to a different reviewer", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const A = `Wren${tag}`, B = `Jude${tag}`;
  const wren = await member(browser, A, "Islington", "photo-1.jpg");
  const jude = await member(browser, B, "Camden", "photo-2.jpg");
  const url = await matched(wren, A, jude, B);
  await jude.goto(url);
  await wren.goto(url);
  await jude.getByLabel("Message", { exact: true }).fill("Answer me now. I know where you drink.");
  await jude.getByRole("button", { name: "Send" }).click();
  await expect(wren.getByTestId("message").last()).toContainText("I know where you drink", { timeout: 10_000 });

  // Wren reports, sharing only the message they tick, and blocks.
  await wren.getByRole("button", { name: "Report" }).click();
  await wren.getByLabel(/Harassment or pressure/).check();
  await wren.getByRole("checkbox", { name: /I know where you drink/ }).click();
  await wren.getByLabel(/Anything else/).fill("Keeps messaging after I said no.");
  await wren.getByRole("button", { name: "Send report" }).click();
  await expect(wren.getByText(/won't be told you reported them/)).toBeVisible();
  const ref = (await wren.getByTestId("report-ref").textContent())!.trim();
  await expect(wren.getByText("This conversation has ended.")).toBeVisible();

  // Moderator console: staff sign-in, queue, case, reveal photo with a reason, decide.
  const modCtx = await browser.newContext();
  const mod = await modCtx.newPage();
  await mod.goto("/ops");
  await mod.getByLabel("Staff token").fill(MOD1);
  await mod.getByRole("button", { name: "Sign in" }).click();
  await expect(mod.getByTestId("queue")).toBeVisible();
  await mod.goto(`/ops/cases/${ref}`);
  const detail = mod.getByTestId("case-detail");
  await expect(detail).toContainText(B);
  await expect(detail).toContainText("Reporter: hidden");
  await expect(detail).toContainText("I know where you drink");
  await expect(detail).not.toContainText(A);
  await mod.getByLabel(/Why do you need to see the photo/).fill("check");
  await mod.getByRole("button", { name: "Show photo" }).click();
  await expect(mod.getByRole("status")).toContainText(/at least 10 characters/);
  await mod.getByLabel(/Why do you need to see the photo/).fill("Confirm this is the reported profile");
  await mod.getByRole("button", { name: "Show photo" }).click();
  await expect(mod.getByRole("img", { name: new RegExp(B) })).toBeVisible();
  await mod.getByLabel("Reason the member will see").fill("You kept messaging a member after they asked you to stop.");
  await mod.getByRole("button", { name: "Uphold" }).click();
  await expect(mod.getByRole("heading", { name: "Warning" })).toBeVisible();

  // Jude sees the decision and appeals.
  await jude.goto("/account");
  const standing = jude.getByTestId("standing");
  await expect(standing).toContainText("kept messaging a member");
  await standing.getByRole("button", { name: "Appeal" }).click();
  await jude.getByLabel(/Why is this decision wrong/).fill("We'd agreed to meet at that bar; it wasn't a threat.");
  await jude.getByRole("button", { name: "Send appeal" }).click();
  await expect(standing).toContainText(/A different reviewer will decide/);

  // The deciding moderator can't hear the appeal; a lead can.
  await mod.reload();
  await mod.getByLabel("Reason the member will see").fill("Reviewing my own decision");
  await mod.getByRole("button", { name: "Overturn" }).click();
  await expect(mod.getByRole("status")).toContainText(/wasn't involved/);
  await mod.evaluate((t) => sessionStorage.setItem("tryst.staff", t), LEAD);
  await mod.reload();
  await mod.getByLabel("Reason the member will see").fill("The context shows an agreed meeting place.");
  await mod.getByRole("button", { name: "Overturn" }).click();
  await expect(mod.getByText(/Appeal overturned by lead1/)).toBeVisible();

  await jude.reload();
  await expect(jude.getByTestId("standing")).toContainText("Your appeal succeeded");
  await expect(jude.getByTestId("standing")).toContainText("in good standing");
});
