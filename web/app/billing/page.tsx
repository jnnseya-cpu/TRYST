"use client";

import { useCallback, useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { problemTitle } from "@/lib/api";
import { buyKeys, cancel, entitlements, money, payDeposit, redeem, subscribe, type CatalogueItem, type Entitlements } from "@/lib/billing";
import "../profile/profile.css";
import "./billing.css";

export default function Billing() {
  const [e, setE] = useState<Entitlements | null>(null);
  const [signedOut, setSignedOut] = useState(false);
  const [choosing, setChoosing] = useState<CatalogueItem | null>(null);
  const [read, setRead] = useState(false);
  const [startNow, setStartNow] = useState(false);
  const [code, setCode] = useState("");
  const [msg, setMsg] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    const r = await entitlements();
    if (r.status !== 200) return setSignedOut(true);
    setE(r.data);
  }, []);

  useEffect(() => {
    load();
    if (new URLSearchParams(window.location.search).get("done")) setMsg("Payment received. Thank you.");
  }, [load]);

  async function go(r: { status: number; data: { checkout_url?: string } }) {
    if (r.status === 201 && r.data.checkout_url) return window.location.assign(r.data.checkout_url);
    setError(problemTitle(r.data));
  }

  async function doCancel(item: string) {
    setError("");
    const r = await cancel(item);
    if (r.status !== 200) return setError(problemTitle(r.data));
    setMsg(r.data.message + (r.data.refund_minor > 0 ? ` Refund: ${money(r.data.refund_minor)}.` : ""));
    load();
  }

  async function doRedeem() {
    setError("");
    const r = await redeem(code.trim());
    if (r.status !== 200) return setError(problemTitle(r.data));
    setCode("");
    setMsg(r.data.message);
    load();
  }

  if (signedOut) return <Shell><h1>Signed out</h1><a className="button inline" href="/sign-in">Sign in</a></Shell>;
  if (!e) return <Shell><p>Loading…</p></Shell>;
  const plans = e.catalogue.filter((c) => c.kind === "plan" || c.kind === "addon");
  const current = e.catalogue.find((c) => c.key === e.plan);
  const live = e.subscriptions.filter((s) => s.status !== "canceled");

  return (
    <Shell wide>
      <p className="eyebrow">Membership</p>
      <h1>Plans and Keys</h1>
      <p className="lede">Statements show a neutral name, never TRYST. Cancel any time in one step.</p>
      {msg && <p role="status">{msg}</p>}
      {error && <p role="alert">{error}</p>}
      {!e.payments_available && <p role="status">Payments aren&rsquo;t available right now.</p>}

      <section className="pf-section" data-testid="current">
        <h2>Your plan</h2>
        <p>{current ? current.name : "Verified (free)"}{e.ghost && " · GHOST"}</p>
        {e.voucher_plan && <p className="note">From a gift code, until {new Date(e.voucher_plan.until).toLocaleDateString()}.</p>}
        {live.map((s) => (
          <div key={s.item} className="bl-row" data-testid="subscription">
            <span>{s.name}: {s.cancel_at_period_end ? `cancelled, ends ${new Date(s.period_end).toLocaleDateString()}` : `renews ${new Date(s.period_end).toLocaleDateString()}`}</span>
            {!s.cancel_at_period_end && (
              <button type="button" className="button ghost inline" onClick={() => doCancel(s.item)}>Cancel {s.name}</button>
            )}
            {!s.cancel_at_period_end && s.in_cooling_off && <span className="note">Within 14 days: cancelling ends it now and refunds the days you haven&rsquo;t used.</span>}
          </div>
        ))}
      </section>

      <section className="pf-section">
        <h2>Keys</h2>
        <p className="bl-keys" data-testid="keys">{e.keys}</p>
        <p className="note">An intent costs 5 Keys (15 from a couple to a single member). You get them all back when they reply. Keys don&rsquo;t expire.</p>
        <div className="bl-row">
          <button type="button" className="button inline" disabled={!e.payments_available} onClick={async () => go(await buyKeys("keys_60"))}>Buy 60 Keys · £9.99</button>
          {!e.deposit_paid && (
            <button type="button" className="button ghost inline" disabled={!e.payments_available} onClick={async () => go(await payDeposit())}>Pay the £9.99 deposit (credited as 60 Keys)</button>
          )}
        </div>
      </section>

      <section className="pf-section">
        <h2>Plans</h2>
        <ul className="bl-plans">
          {plans.map((p) => (
            <li key={p.key} className="bl-plan" data-testid="plan">
              <h3>{p.name}</h3>
              <span className="bl-price">{money(p.price_minor)} a {p.period}</span>
              <span className="note">{p.description}</span>
              <button type="button" className="button ghost inline" disabled={!e.payments_available || live.some((s) => s.item === p.key)}
                onClick={() => { setChoosing(p); setRead(false); setStartNow(false); setError(""); }}>
                {live.some((s) => s.item === p.key) ? "Current" : `Choose ${p.name}`}
              </button>
            </li>
          ))}
        </ul>
      </section>

      {choosing && (
        <section className="pf-section" aria-label="Before you subscribe">
          <h2>Before you subscribe to {choosing.name}</h2>
          <ul className="bl-terms">
            <li>{money(choosing.price_minor)} every {choosing.period}, charged now and then automatically each {choosing.period} until you cancel.</li>
            <li>What you get: {choosing.description}.</li>
            <li>We&rsquo;ll remind you at least 7 days before each yearly renewal.</li>
            <li>Cancel any time here, in one step. You keep the plan until the end of the period you paid for.</li>
            <li>You have 14 days to cancel. If you cancel in that time you pay only for the days you used and we refund the rest within 14 days.</li>
            <li>Your bank statement shows a neutral name, never TRYST.</li>
          </ul>
          <label className="bl-check"><input type="checkbox" checked={read} onChange={(x) => setRead(x.target.checked)} /> I&rsquo;ve read the plan details above</label>
          <label className="bl-check"><input type="checkbox" checked={startNow} onChange={(x) => setStartNow(x.target.checked)} /> Start my plan now. I understand that if I cancel within 14 days I pay for the days I&rsquo;ve used.</label>
          <div className="bl-row">
            <button type="button" className="button inline" disabled={!read || !startNow} onClick={async () => go(await subscribe(choosing.key, read, startNow))}>Continue to payment</button>
            <button type="button" className="ms-block" onClick={() => setChoosing(null)}>Not now</button>
          </div>
        </section>
      )}

      <section className="pf-section">
        <h2>Gift or voucher code</h2>
        <label htmlFor="code">Code</label>
        <input id="code" className="input" autoComplete="off" value={code} onChange={(x) => setCode(x.target.value)} />
        <button type="button" className="button ghost inline" onClick={doRedeem}>Apply code</button>
      </section>
      <p><a href="/account">Your account</a></p>
    </Shell>
  );
}
