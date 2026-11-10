"use client";

import { useCallback, useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { setStaffToken } from "@/lib/safety";
import { growth, key, message, parseCsv, type Opportunity, type Overview } from "./growth";
import "../../profile/profile.css";
import "../ops.css";

/**
 * Growth (staff console, D-73). Works in the background: members and visitors never see
 * any of this. Analysts import data, refresh scores and draft briefs; editors approve,
 * create guide pages, apply changes, roll back and freeze.
 */
export default function Growth() {
  const [ov, setOv] = useState<Overview | null>(null);
  const [opps, setOpps] = useState<Opportunity[]>([]);
  const [needsSignIn, setNeedsSignIn] = useState(false);
  const [token, setToken] = useState("");
  const [note, setNote] = useState("");
  const [csv, setCsv] = useState("");

  const load = useCallback(async () => {
    const r = await growth<Overview>("/ops/v1/growth/overview");
    if (r.status === 401) return setNeedsSignIn(true);
    setNeedsSignIn(false);
    if (r.status !== 200) return setNote(message(r.data));
    setOv(r.data);
    const o = await growth<{ items: Opportunity[] }>("/ops/v1/growth/opportunities");
    setOpps(o.data.items ?? []);
  }, []);

  useEffect(() => { load(); }, [load]);

  async function act(path: string, body?: unknown, ok = "Done.", headers?: Record<string, string>) {
    const r = await growth(path, { method: "POST", body: body ?? {}, headers });
    setNote(r.status < 300 ? ok : message(r.data));
    await load();
    return r;
  }

  if (needsSignIn) {
    return (
      <Shell>
        <p className="eyebrow">Growth</p>
        <h1>Staff sign-in</h1>
        <p className="note">In production the console sits behind the company sign-in. This development build takes a staff token.</p>
        <form onSubmit={(e) => { e.preventDefault(); setStaffToken(token.trim()); load(); }}>
          <label htmlFor="staff-token">Staff token</label>
          <input id="staff-token" className="input" type="password" autoComplete="off" value={token} onChange={(e) => setToken(e.target.value)} />
          <button type="submit" className="button inline">Sign in</button>
        </form>
      </Shell>
    );
  }
  if (!ov) return <Shell><p>Loading…</p></Shell>;
  const analyst = ov.roles.includes("growth");
  const editor = ov.roles.includes("editor");

  return (
    <Shell wide>
      <div className="op-bar">
        <div>
          <p className="eyebrow">Staff console</p>
          <h1>Growth</h1>
        </div>
        <div className="op-row">
          <a className="ms-block" href="/ops">Case queue</a>
          {(editor || ov.roles.includes("lead")) && (
            <button type="button" className="ms-block" onClick={() => act("/ops/v1/growth/freeze", { frozen: !ov.frozen }, ov.frozen ? "Unfrozen." : "Frozen: no new change will start.")}>
              {ov.frozen ? "Unfreeze" : "Freeze all changes"}
            </button>
          )}
          <button type="button" className="ms-block" onClick={() => { setStaffToken(null); setOv(null); load(); }}>Sign out</button>
        </div>
      </div>
      <p className="note">Runs in the background and is never shown to members. It reads weekly counts by city (groups under 50 are never sent) and imported search data, never a member. Every change to a guide page needs an editor, and every step is logged.</p>
      {note && <p className="note" role="status" data-testid="growth-note">{note}</p>}

      <div className="cm-tiles" data-testid="growth-state">
        <div className="cm-tile"><span>State</span><b>{ov.frozen ? "Frozen" : "Running"}</b><small>mode {ov.mode}</small></div>
        <div className="cm-tile"><span>Drafting</span><b>{ov.inference_route === "deterministic" ? "Templates" : "Own model"}</b><small>no outside provider</small></div>
        <div className="cm-tile"><span>Member platform</span><b>{ov.platform_connected ? "Connected" : "Off"}</b><small>counts by city only</small></div>
        <div className="cm-tile"><span>Profile</span><b>{ov.profile_revision.slice(0, 7)}</b><small>reviewed in the repository</small></div>
      </div>

      <h2>Opportunities</h2>
      {analyst && <button type="button" className="button inline" onClick={() => act("/ops/v1/growth/refresh", undefined, "Scores refreshed.")}>Refresh scores</button>}
      {opps.length === 0 ? <p className="pf-empty">Nothing scored yet.</p> : (
        <table className="op-table" data-testid="opportunities">
          <thead><tr><th>Topic</th><th>Score</th><th>Confidence</th><th>Status</th><th>Why</th><th></th></tr></thead>
          <tbody>
            {opps.map((o) => (
              <tr key={o.id} data-testid="opportunity-row">
                <td>{o.topic_id}</td>
                <td>{o.score.toFixed(2)}</td>
                <td>{o.confidence.toFixed(2)}</td>
                <td>{o.status}</td>
                <td className="note">{[...o.vetoes.map((v) => `veto: ${v}`), ...o.reasons, ...o.notes].join("; ")}</td>
                <td>{analyst && o.status === "eligible" && (
                  <button type="button" className="ms-block" onClick={async () => {
                    const r = await act("/ops/v1/growth/briefs", { snapshot_id: o.id }, "Brief drafted.");
                    const id = (r.data as { id?: string }).id;
                    if (id) window.location.href = `/ops/growth/briefs/${id}`;
                  }}>Draft brief</button>
                )}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <h2>Briefs</h2>
      {ov.briefs.length === 0 ? <p className="pf-empty">No briefs yet.</p> : (
        <table className="op-table" data-testid="briefs">
          <thead><tr><th>Topic</th><th>Version</th><th>Status</th><th>Expires</th></tr></thead>
          <tbody>
            {ov.briefs.map((b) => (
              <tr key={b.id}>
                <td><a href={`/ops/growth/briefs/${b.id}`}>{b.topic_id}</a></td>
                <td>{b.brief_version}</td>
                <td>{b.status}</td>
                <td>{new Date(b.expires_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <h2>Guide pages</h2>
      <p className="note">No guide page exists until an editor creates it from its reviewed starting wording.</p>
      <table className="op-table" data-testid="pages">
        <thead><tr><th>Page</th><th>Revision</th><th>Title</th><th></th></tr></thead>
        <tbody>
          {ov.pages.map((p) => (
            <tr key={p.asset_id}>
              <td>/guides/{p.slug}</td>
              <td>{p.created ? p.active_revision : "not created"}</td>
              <td>{p.fields?.title ?? ""}</td>
              <td>{editor && !p.created && (
                <button type="button" className="ms-block" onClick={() => act("/ops/v1/growth/pages", { asset_id: p.asset_id }, "Page created.")}>Create page</button>
              )}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <h2>Sources</h2>
      <table className="op-table" data-testid="sources">
        <thead><tr><th>Source</th><th>How</th><th>Rows</th><th>Latest week</th></tr></thead>
        <tbody>
          {ov.sources.map((s) => (
            <tr key={s.source_id}>
              <td>{s.source_id}{s.context_only ? " (context only)" : ""}</td>
              <td>{s.access_path === "first_party" ? "Member platform counts" : "Imported by staff"}</td>
              <td>{s.observations}</td>
              <td className={s.stale ? "op-due-late" : ""}>{s.last_window_end ? `${s.last_window_end.slice(0, 10)}${s.stale ? " (stale)" : ""}` : "none"}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {analyst && (
        <form onSubmit={async (e) => {
          e.preventDefault();
          const { items, errors } = parseCsv(csv, ov.sources);
          if (errors.length || items.length === 0) return setNote(errors.join(" ") || "Nothing to import.");
          const r = await act("/ops/v1/growth/observations", { items }, "Imported.", { "Idempotency-Key": key() });
          if (r.status === 200) setCsv("");
        }}>
          <label htmlFor="csv">Import weekly search data (CSV: source_id,term,week_start,value)</label>
          <textarea id="csv" className="input" rows={4} value={csv} onChange={(e) => setCsv(e.target.value)} placeholder="search_console,discreet dating london,2026-11-02,220" />
          <button type="submit" className="button inline">Import</button>
        </form>
      )}

      <h2>Approved claims</h2>
      <ul>
        {ov.claims.map((c) => <li key={c.claim_id}><b>{c.claim_id}</b>: {c.text} <span className="note">({c.source})</span></li>)}
      </ul>
    </Shell>
  );
}
