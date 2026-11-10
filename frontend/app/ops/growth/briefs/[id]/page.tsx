"use client";

import { use, useCallback, useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { growth, key, message } from "../../growth";
import "../../../../profile/profile.css";
import "../../../ops.css";

type Action = {
  id: string; brief_id: string; brief_version: number; action_kind: string; channel: string; target_id: string;
  expected_target_revision: number; patch: Record<string, string>; evidence_ids: string[]; approved_claim_ids: string[];
  payload_hash: string; state: string; live: boolean; author: string; approval?: { approver: string };
  previous_fields?: Record<string, string>; applied_revision?: number; last_error?: string; rollback_of?: string;
  inference?: { route: string; degraded: boolean; notes: string[] };
};
type View = {
  brief: { id: string; status: string; author: string; approver?: string; body: Record<string, unknown>; experiment?: { experiment_id: string } };
  actions: Action[]; pages: Record<string, { slug: string; active_revision: number; fields: Record<string, string> }>;
  claims: Record<string, string>;
};
type Report = { decision: string; reason?: string; label: string; status: string; before?: Record<string, unknown>; after?: Record<string, unknown> };

const KIND: Record<string, string> = { seo_metadata_patch: "Search title and description", owned_content_patch: "Headline and button", organic_social_draft: "Social post (export only)" };

/** One brief: what it says, the changes drafted from it, approval, apply, roll back, results. */
export default function BriefPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [v, setV] = useState<View | null>(null);
  const [roles, setRoles] = useState<string[]>([]);
  const [report, setReport] = useState<Report | null>(null);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    const r = await growth<View>(`/ops/v1/growth/briefs/${id}`);
    if (r.status !== 200) return setNote(r.status === 401 ? "Sign in on the Growth page first." : message(r.data));
    setV(r.data);
    const o = await growth<{ roles: string[] }>("/ops/v1/growth/overview");
    setRoles(o.data.roles ?? []);
    const exp = r.data.brief.experiment?.experiment_id;
    if (exp) setReport((await growth<Report>(`/ops/v1/growth/experiments/${exp}`)).data);
  }, [id]);

  useEffect(() => { load(); }, [load]);
  useEffect(() => { // follow the worker while a change is queued
    if (!v?.actions.some((a) => a.state === "queued" || a.state === "executing")) return;
    const t = setTimeout(load, 2000);
    return () => clearTimeout(t);
  }, [v, load]);

  async function post(path: string, body: unknown, ok: string, headers?: Record<string, string>) {
    setBusy(true);
    const r = await growth(path, { method: "POST", body, headers });
    setBusy(false);
    setNote(r.status < 300 ? ok : message(r.data));
    await load();
    return r;
  }

  if (!v) return <Shell><p>{note || "Loading…"}</p></Shell>;
  const b = v.brief.body;
  const editor = roles.includes("editor");
  const analyst = roles.includes("growth");
  const page = Object.values(v.pages)[0];

  return (
    <Shell wide>
      <p><a href="/ops/growth">Back to Growth</a></p>
      <p className="eyebrow">Growth brief · version {String(b.brief_version)} · {v.brief.status}</p>
      <h1>{String(b.topic_id)}</h1>
      {note && <p className="note" role="status" data-testid="brief-note">{note}</p>}
      <div className="cm-tiles">
        <div className="cm-tile"><span>Score</span><b>{Number(b.opportunity_score).toFixed(2)}</b></div>
        <div className="cm-tile"><span>Confidence</span><b>{Number(b.confidence).toFixed(2)}</b></div>
        <div className="cm-tile"><span>Expires</span><b>{new Date(String(b.expires_at)).toLocaleDateString()}</b></div>
        <div className="cm-tile"><span>Evidence</span><b>{(b.evidence_ids as string[]).length}</b><small>aggregate rows</small></div>
      </div>
      <p><b>Audience:</b> {String(b.audience)}<br /><b>Intent:</b> {String(b.intent)}<br /><b>Objective:</b> {String(b.objective)}</p>
      <p><b>Approved claims:</b></p>
      <ul>{Object.entries(v.claims).map(([k, t]) => <li key={k}>{t}</li>)}</ul>
      <p className="note">Never say: {(b.prohibited_claims as string[]).join(", ")}. Drafted by {v.brief.author}{v.brief.approver ? `, approved by ${v.brief.approver}` : ""}.</p>

      {v.brief.status === "draft" && editor && (
        <button type="button" className="button inline" disabled={busy}
          onClick={() => post(`/ops/v1/growth/briefs/${id}/approve`, {}, "Brief approved.", { "If-Match": String(b.brief_version) })}>Approve brief</button>
      )}
      {v.brief.status === "approved" && analyst && (
        <button type="button" className="button inline" disabled={busy}
          onClick={() => post(`/ops/v1/growth/briefs/${id}/drafts`, {}, "Changes drafted. An editor reviews each one.")}>Draft changes</button>
      )}

      <h2>Changes</h2>
      {v.actions.length === 0 ? <p className="pf-empty">No changes drafted yet.</p> : v.actions.map((a) => {
        const before = a.rollback_of ? {} : page?.fields ?? {};
        return (
          <section key={a.id} className="vk-row" data-testid="change">
            <p><b>{KIND[a.action_kind] ?? a.action_kind}</b>{a.rollback_of ? " (roll back)" : ""} on /guides/{page?.slug} · <span data-testid="change-state">{a.state}</span>{a.last_error ? ` (${a.last_error})` : ""}</p>
            <table className="op-table">
              <thead><tr><th>Field</th><th>Now</th><th>Proposed</th></tr></thead>
              <tbody>
                {Object.entries(a.patch).map(([f, val]) => (
                  <tr key={f}><td>{f}</td><td>{a.state === "applied" ? (a.previous_fields?.[f] ?? "") : (before[f] ?? "")}</td><td>{val}</td></tr>
                ))}
              </tbody>
            </table>
            <p className="note">
              Proposed by {a.author} against page revision {a.expected_target_revision}
              {a.inference ? ` · ${a.inference.route === "deterministic" ? "template" : "own model"}${a.inference.degraded ? " (fallback)" : ""}` : ""}
              {a.approval ? ` · approved by ${a.approval.approver}` : ""}
            </p>
            <div className="op-row">
              {editor && a.state === "pending_approval" && page && a.expected_target_revision !== page.active_revision && (
                <button type="button" className="ms-block" disabled={busy} onClick={() => post("/ops/v1/growth/actions", {
                  brief_id: a.brief_id, brief_version: a.brief_version, action_kind: a.action_kind, channel: a.channel, target_id: a.target_id,
                  expected_target_revision: page.active_revision, patch: a.patch, evidence_ids: a.evidence_ids, approved_claim_ids: a.approved_claim_ids,
                }, "Re-proposed against the current page.", { "Idempotency-Key": key() })}>Re-propose on current page</button>
              )}
              {editor && a.state === "pending_approval" && (!page || a.expected_target_revision === page.active_revision) && (
                <button type="button" className="ms-block" disabled={busy}
                  onClick={() => post(`/ops/v1/growth/actions/${a.id}/approve`, { payload_hash: a.payload_hash, target_revision: a.expected_target_revision }, "Approved.")}>Approve</button>
              )}
              {editor && a.state === "approved" && (
                <button type="button" className="ms-block" disabled={busy} onClick={() => post(`/ops/v1/growth/actions/${a.id}/execute`, {}, "Queued; applying shortly.")}>Apply</button>
              )}
              {editor && a.state === "applied" && a.live && !a.rollback_of && (
                <button type="button" className="ms-block" disabled={busy} onClick={() => post(`/ops/v1/growth/actions/${a.id}/rollback`, {}, "Roll back queued.")}>Roll back</button>
              )}
              {!a.live && (
                <button type="button" className="ms-block" onClick={async () => {
                  const r = await growth<{ copy: Record<string, string> }>(`/ops/v1/growth/actions/${a.id}/export`);
                  if (r.status === 200) { await navigator.clipboard?.writeText(Object.values(r.data.copy).join("\n")).catch(() => {}); setNote("Copied for posting by hand."); }
                  else setNote(message(r.data));
                }}>Copy for posting</button>
              )}
            </div>
          </section>
        );
      })}

      {report && (
        <>
          <h2>Results</h2>
          <p className="note">{report.label}</p>
          <p data-testid="report"><b>{report.decision.replaceAll("_", " ")}</b>{report.reason ? `: ${report.reason}` : ""}</p>
          {report.before && <p className="note">Before: {JSON.stringify(report.before)} · After: {JSON.stringify(report.after)}</p>}
        </>
      )}
    </Shell>
  );
}
