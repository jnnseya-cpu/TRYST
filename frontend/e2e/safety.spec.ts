import { test, expect } from "@playwright/test";
import { matched, member } from "./helpers";

const MOD1 = "e2e-mod-one-token-0001";
const LEAD = "e2e-lead-token-00000003";
const MOD2 = "e2e-mod-two-token-0002";

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

test("safety: on-device child-safety signal, second-moderator removal, mandatory NCA report", async ({ browser }) => {
  const tag = String(Date.now()).slice(-5);
  const A = `Ash${tag}`, B = `Oak${tag}`;
  const ash = await member(browser, A, "Islington", "photo-1.jpg");
  const oak = await member(browser, B, "Camden", "photo-2.jpg");
  const url = await matched(ash, A, oak, B);
  await ash.goto(url);
  await oak.goto(url);
  await oak.getByLabel("Message", { exact: true }).fill("send me pics of your daughter");
  await oak.getByRole("button", { name: "Send" }).click();
  await expect(oak.locator("main").getByRole("alert").filter({ hasText: "can't be sent" })).toBeVisible();

  // The signal crosses to the safety service and freezes the sender at once.
  await expect(async () => {
    await oak.goto("/account");
    await expect(oak.getByTestId("standing")).toContainText("Paused for a safety review", { timeout: 1000 });
  }).toPass({ timeout: 20_000 });

  const ctx = await browser.newContext();
  const mod = await ctx.newPage();
  await mod.goto("/ops");
  await mod.getByLabel("Staff token").fill(MOD1);
  await mod.getByRole("button", { name: "Sign in" }).click();
  await expect(mod.getByTestId("queue")).toBeVisible();
  // Find this test's case among any others in the queue (cases show no names in the list).
  const staff = { Authorization: `Staff ${MOD1}` };
  let caseId = "";
  await expect(async () => {
    const queue = await (await mod.request.get("/ops/v1/cases", { headers: staff })).json();
    for (const c of queue.cases.filter((x: { category: string }) => x.category === "minor")) {
      const d = await (await mod.request.get(`/ops/v1/cases/${c.id}`, { headers: staff })).json();
      if (d.subject.name === B) caseId = c.id;
    }
    expect(caseId).not.toBe("");
  }).toPass({ timeout: 20_000 });
  await mod.goto(`/ops/cases/${caseId}`);
  await expect(mod.getByTestId("case-detail")).toContainText(B);
  await mod.getByLabel("Reason the member will see").fill("Messages asking for images of a child.");
  await mod.getByRole("button", { name: "Uphold" }).click();
  await expect(mod.getByText(/Permanent removal proposed/)).toBeVisible();
  // The same moderator can't confirm; a second one can.
  await mod.getByRole("button", { name: "Confirm removal" }).click();
  await expect(mod.getByRole("status")).toContainText(/different moderator/);
  await mod.evaluate((t) => sessionStorage.setItem("tryst.staff", t), MOD2);
  await mod.reload();
  await mod.getByRole("button", { name: "Confirm removal" }).click();
  await expect(mod.getByTestId("external-report")).toContainText("must be reported to the National Crime Agency");

  // Mandatory report recorded with the agency's reference; the case leaves the queue.
  await mod.getByLabel("Agency reference").fill(`NCA-E2E-${tag}`);
  await mod.getByRole("button", { name: "Record report" }).click();
  await expect(mod.getByTestId("external-report")).toContainText(`reference NCA-E2E-${tag}`);
  await mod.goto("/ops");
  await expect(mod.getByTestId("queue").or(mod.getByText("Nothing waiting."))).toBeVisible();
  await expect(mod.locator(`a[href="/ops/cases/${caseId}"]`)).toHaveCount(0);

  await oak.goto("/account");
  await expect(oak.getByTestId("standing")).toContainText("Removed");
});
