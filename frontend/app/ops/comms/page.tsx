"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Shell } from "@/components/Shell";
import { problemTitle } from "@/lib/api";
import { ops } from "@/lib/safety";
import "../../profile/profile.css";
import "../ops.css";

type Event = { key: string; category: string; label: string; severity: string; audience: string; mandatory: boolean; channels: string[]; subject?: string; text: string; trigger: string };
type Delivery = { at: string; event: string; channel: string; status: string; provider: string; recipient: string; test?: boolean };
type Data = {
  catalogue: Event[]; categories: string[]; not_sent: { what: string; why: string }[]; coverage: Record<string, number>;
  log: { recent: Delivery[]; totals: Record<string, Record<string, number>> }; email_provider?: string; log_unavailable?: boolean;
};

const CHANNEL: Record<string, string> = { inapp: "In-app", push: "Push", email: "Email", sms: "SMS", safety_line: "Safety line" };
const AUDIENCE: Record<string, string> = { member: "Member", trusted_contact: "Trusted contact", responders: "Alarm centre", staff: "Staff" };

function time(iso: string) {
  return new Date(iso).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

/**
 * Communications (staff console): one catalogue of everything TRYST sends, by channel, with
 * its exact wording; what is never sent and why; the delivery log; and test sends (leads).
 * The log never holds an address, a name or any content.
 */
export default function Communications() {
  const [data, setData] = useState<Data | null>(null);
  const [error, setError] = useState("");
  const [pick, setPick] = useState("account.signup_code");
  const [result, setResult] = useState("");

  const load = useCallback(async () => {
    const r = await ops<Data>("/ops/v1/comms");
    if (r.status !== 200) return setError(r.status === 401 ? "Sign in to the console first." : problemTitle(r.data));
    setError("");
    setData(r.data);
  }, []);
  useEffect(() => {
    load();
    const t = setInterval(load, 15000);
    return () => clearInterval(t);
  }, [load]);

  const stats = useMemo(() => {
    if (!data) return null;
    let sent = 0, attempted = 0;
    for (const s of Object.values(data.log.totals ?? {})) {
      for (const [k, v] of Object.entries(s)) {
        if (k === "suppressed") continue;
        attempted += v;
        if (k === "sent" || k === "sandbox") sent += v;
      }
    }
    const sentBy: Record<string, number> = {};
    for (const d of data.log.recent) if (d.status === "sent" || d.status === "sandbox") sentBy[d.channel] = (sentBy[d.channel] ?? 0) + 1;
    return { mandatory: data.catalogue.filter((e) => e.mandatory).length, sent, attempted, sentBy };
  }, [data]);

  async function sendTest() {
    setResult("");
    const r = await ops<{ results: { channel: string; status: string; provider: string }[] }>("/ops/v1/comms/test", { method: "POST", body: { event: pick } });
    if (r.status !== 200) return setResult(problemTitle(r.data));
    setResult(r.data.results.map((x) => `${CHANNEL[x.channel] ?? x.channel}: ${x.status === "sandbox" ? "recorded in sandbox" : `${x.status} via ${x.provider}`}`).join(" · "));
    load();
  }

  if (error) {
    return (
      <Shell>
        <p className="eyebrow">Console</p>
        <h1>Communications</h1>
        <p role="alert">{error}</p>
        <a className="button ghost" href="/ops">Back to the console</a>
      </Shell>
    );
  }
  if (!data || !stats) return <Shell><p>Loading…</p></Shell>;
  const chosen = data.catalogue.find((e) => e.key === pick) ?? data.catalogue[0];
  const channels = Object.keys(data.coverage);

  return (
    <Shell wide>
      <div className="op-bar">
        <div>
          <p className="eyebrow">Console</p>
          <h1>Communications</h1>
        </div>
        <div className="op-row"><a className="ms-block" href="/ops">Case queue</a><a className="ms-block" href="/ops/settings">Vendor keys</a></div>
      </div>
      <p className="note">
        One catalogue: {data.catalogue.length} messages across {channels.map((c) => CHANNEL[c] ?? c).join(", ")}. Every email is plain text from a neutral
        sender, pushes carry no content, and in-app notices name nobody.
      </p>

      <div className="cm-tiles" data-testid="comms-stats">
        <div className="cm-tile"><span>Catalogue messages</span><b>{data.catalogue.length}</b><small>{data.categories.length} categories</small></div>
        <div className="cm-tile"><span>Required</span><b>{stats.mandatory}</b><small>security, safety or legal: always sent</small></div>
        <div className="cm-tile"><span>Delivered</span><b>{stats.sent}</b><small>of {stats.attempted} attempted since the last restart</small></div>
        <div className="cm-tile"><span>Email</span><b>{data.email_provider ? "Live" : "Sandbox"}</b><small>{data.email_provider || "no mailbox set: Vendor keys"}</small></div>
      </div>

      <section className="card">
        <h2>Channel coverage</h2>
        <p className="note">How many catalogue messages use each channel, and how many went out recently.</p>
        {channels.map((c) => (
          <div key={c} className="cm-bar" data-testid={`coverage-${c}`}>
            <span className="cm-bar-label">{CHANNEL[c] ?? c}</span>
            <span className="cm-bar-track"><span className="cm-bar-fill" style={{ width: `${(100 * data.coverage[c]) / data.catalogue.length}%` }} /></span>
            <span className="note">{data.coverage[c]} messages{stats.sentBy[c] ? ` · ${stats.sentBy[c]} sent` : ""}</span>
          </div>
        ))}
      </section>

      <section className="card">
        <h2>Template check</h2>
        <p className="note">See exactly what a recipient receives, or send it to yourself. Email goes for real to your staff address when a mailbox is set; every other channel is recorded in sandbox. A test never reaches a member, a trusted contact or the alarm centre.</p>
        <label htmlFor="cm-pick">Message</label>
        <select id="cm-pick" className="input" value={pick} onChange={(e) => { setPick(e.target.value); setResult(""); }}>
          {data.categories.map((cat) => (
            <optgroup key={cat} label={cat}>
              {data.catalogue.filter((e) => e.category === cat).map((e) => <option key={e.key} value={e.key}>{e.label} — {e.key}</option>)}
            </optgroup>
          ))}
        </select>
        <div className="cm-preview" data-testid="preview">
          {chosen.channels.includes("email") && (
            <div className="cm-mail">
              <p className="note">From: a neutral sender (no logo, never &ldquo;TRYST&rdquo;)</p>
              <p><b>Subject:</b> {chosen.subject}</p>
              <p>{chosen.text}</p>
            </div>
          )}
          {chosen.channels.includes("inapp") && <div className="cm-notice"><span className="note">In-app notice</span><p>{chosen.text}</p></div>}
          {chosen.channels.includes("push") && <div className="cm-notice"><span className="note">Push (only if the member turned it on)</span><p>You have an update</p></div>}
          {chosen.channels.includes("safety_line") && <div className="cm-notice"><span className="note">Alarm-receiving centre</span><p>{chosen.text}</p></div>}
        </div>
        <button type="button" className="button inline" onClick={sendTest}>Send test to me</button>
        {result && <p role="status" data-testid="test-result">{result}</p>}
      </section>

      <section className="card">
        <h2>Recent deliveries</h2>
        <p className="note">Every message × channel × recipient. Recipients are pseudonyms that change on every restart; no address or content is kept.</p>
        {data.log_unavailable && <p role="alert">The delivery log is unavailable right now.</p>}
        {data.log.recent.length === 0 ? <p className="pf-empty">Nothing sent yet.</p> : (
          <table className="op-table" data-testid="deliveries">
            <thead><tr><th>Channel</th><th>Message</th><th>Status</th><th>Via</th><th>To</th></tr></thead>
            <tbody>
              {data.log.recent.slice(0, 50).map((d, i) => (
                <tr key={i}>
                  <td>{CHANNEL[d.channel] ?? d.channel}</td>
                  <td>{d.event}{d.test ? " (test)" : ""}</td>
                  <td className={d.status === "failed" ? "op-due-late" : ""}>{d.status}</td>
                  <td>{d.provider} · {time(d.at)}</td>
                  <td>{d.recipient}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      {data.categories.map((cat) => {
        const list = data.catalogue.filter((e) => e.category === cat);
        return (
          <section key={cat} className="card" data-testid={`category-${cat}`}>
            <h2>{cat} <span className="note">{list.length}</span></h2>
            {list.map((e) => (
              <div key={e.key} className="vk-row">
                <p><b>{e.label}</b> <code>{e.key}</code> {e.mandatory && <span className="cm-chip cm-req">required</span>} <span className={`cm-chip cm-${e.severity}`}>{e.severity}</span></p>
                <p>{e.subject ? <><b>{e.subject}:</b> </> : null}{e.text}</p>
                <p className="note">{e.channels.map((c) => CHANNEL[c] ?? c).join(" · ")} — to {AUDIENCE[e.audience] ?? e.audience}. When: {e.trigger}.</p>
              </div>
            ))}
          </section>
        );
      })}

      <section className="card">
        <h2>Never sent, by design</h2>
        {data.not_sent.map((n) => <p key={n.what}><b>{n.what}.</b> {n.why}</p>)}
      </section>
    </Shell>
  );
}
