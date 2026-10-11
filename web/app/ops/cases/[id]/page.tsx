"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Shell } from "@/components/Shell";
import { problemTitle } from "@/lib/api";
import { actionText, ops, staffToken } from "@/lib/safety";
import "../../../profile/profile.css";
import "../../ops.css";

type Detail = {
  case: { id: string; label: string; source: string; opened: string; due: string; state: string; appeal_pending: boolean };
  subject: { name: string; area: string };
  disclosed: string[] | null;
  note: string;
  independent_reporters: number;
  prior_strikes: number;
  decision: { id: string; action: string; ground?: string; decided_by: string; confirmed_by?: string; reason: string } | null;
  appeal: { text: string; due: string; outcome?: string; reviewer?: string } | null;
};

const grounds = [
  ["", "None (report only)"],
  ["g2", "Corroborated by independent reports"],
  ["g5", "Confirmed signal"],
  ["g1", "Member declared it themselves"],
  ["g3", "Court order"],
  ["g4", "Police request"],
  ["g6", "False attestation"],
];

export default function CaseView() {
  const { id } = useParams<{ id: string }>();
  const [d, setD] = useState<Detail | null>(null);
  const [error, setError] = useState("");
  const [why, setWhy] = useState("");
  const [photo, setPhoto] = useState<string | null>(null);
  const [ground, setGround] = useState("");
  const [reason, setReason] = useState("");
  const [msg, setMsg] = useState("");

  const load = useCallback(async () => {
    const r = await ops<Detail>(`/ops/v1/cases/${id}`);
    if (r.status !== 200) return setError(problemTitle(r.data));
    setD(r.data);
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  async function reveal() {
    const t = staffToken();
    const res = await fetch(`/ops/v1/cases/${id}/photo?reason=${encodeURIComponent(why)}`, { headers: t ? { Authorization: `Staff ${t}` } : {} });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      return setMsg(problemTitle(body, "No photo."));
    }
    setPhoto(URL.createObjectURL(await res.blob()));
    setMsg("");
  }

  async function act(path: string, body: unknown) {
    const r = await ops(`/ops/v1/cases/${id}/${path}`, { method: "POST", body });
    if (r.status !== 200) return setMsg(problemTitle(r.data));
    setMsg("Saved.");
    setReason("");
    load();
  }

  if (error) return <Shell><h1>Case</h1><p role="alert">{error}</p><a href="/ops">Queue</a></Shell>;
  if (!d) return <Shell><p>Loading…</p></Shell>;
  const c = d.case;
  return (
    <Shell wide>
      <p className="eyebrow">Case {c.id}</p>
      <h1>{c.label}</h1>
      <p className="note">
        {c.source === "guardian" ? "On-device signal" : "Member report"} · opened {new Date(c.opened).toLocaleString()} · due {new Date(c.due).toLocaleString()}
      </p>
      <div className="op-grid pf-section">
        <div>
          <div className="op-photo">{photo ? <img src={photo} alt={`${d.subject.name}, main photo`} /> : <div className="pf-placeholder" />}</div>
          {!photo && (
            <div>
              <label htmlFor="why">Why do you need to see the photo?</label>
              <input id="why" className="input" value={why} onChange={(e) => setWhy(e.target.value)} />
              <button type="button" className="button ghost inline" onClick={reveal}>Show photo</button>
            </div>
          )}
        </div>
        <div data-testid="case-detail">
          <h2>{d.subject.name}</h2>
          <p className="note">{d.subject.area} · {d.independent_reporters} independent reporter{d.independent_reporters === 1 ? "" : "s"} · {d.prior_strikes} prior strike{d.prior_strikes === 1 ? "" : "s"}</p>
          <p className="note">Reporter: hidden</p>
          {(d.disclosed ?? []).length > 0 && (
            <>
              <h3>Messages the member shared</h3>
              {(d.disclosed ?? []).map((m, i) => <p key={i} className="op-quote">{m}</p>)}
            </>
          )}
          {d.note && <><h3>Note</h3><p className="op-quote">{d.note}</p></>}
        </div>
      </div>

      {c.state === "open" && (
        <section className="pf-section" aria-label="Decision">
          <h2>Decision</h2>
          <p className="note">The action follows the enforcement ladder from the conduct, prior strikes and corroboration. A removal needs a second moderator.</p>
          <label htmlFor="ground">Ground</label>
          <select id="ground" className="input pf-select" value={ground} onChange={(e) => setGround(e.target.value)}>
            {grounds.map(([k, l]) => <option key={k} value={k}>{l}</option>)}
          </select>
          <label htmlFor="reason">Reason the member will see</label>
          <textarea id="reason" className="input" rows={3} value={reason} onChange={(e) => setReason(e.target.value)} />
          <div className="op-row">
            <button type="button" className="button inline" onClick={() => act("decision", { outcome: "uphold", ground, reason })}>Uphold</button>
            <button type="button" className="button ghost inline" onClick={() => act("decision", { outcome: "dismiss", reason })}>Dismiss</button>
          </div>
        </section>
      )}

      {d.decision && (
        <section className="pf-section" aria-label="Recorded decision">
          <h2>{d.decision.action === "dismiss" ? "Dismissed" : actionText[d.decision.action] ?? d.decision.action}</h2>
          <p className="note">By {d.decision.decided_by}{d.decision.confirmed_by ? `, confirmed by ${d.decision.confirmed_by}` : ""}.</p>
          <p className="op-quote">{d.decision.reason}</p>
          {c.state === "pending_second" && (
            <div>
              <p>Permanent removal proposed. The member is suspended until a second moderator decides.</p>
              <label htmlFor="reason2">Your reason</label>
              <textarea id="reason2" className="input" rows={2} value={reason} onChange={(e) => setReason(e.target.value)} />
              <div className="op-row">
                <button type="button" className="button inline" onClick={() => act("confirm", { agree: true, reason })}>Confirm removal</button>
                <button type="button" className="button ghost inline" onClick={() => act("confirm", { agree: false, reason })}>Send back</button>
              </div>
            </div>
          )}
        </section>
      )}

      {d.appeal && (
        <section className="pf-section" aria-label="Appeal">
          <h2>Appeal</h2>
          <p className="op-quote">{d.appeal.text}</p>
          {d.appeal.outcome ? (
            <p>Appeal {d.appeal.outcome} by {d.appeal.reviewer}.</p>
          ) : (
            <div>
              <p className="note">Due {new Date(d.appeal.due).toLocaleDateString()}. Must be decided by someone not involved in the decision.</p>
              <label htmlFor="reason3">Reason the member will see</label>
              <textarea id="reason3" className="input" rows={2} value={reason} onChange={(e) => setReason(e.target.value)} />
              <div className="op-row">
                <button type="button" className="button inline" onClick={() => act("appeal", { member_wins: true, reason })}>Overturn</button>
                <button type="button" className="button ghost inline" onClick={() => act("appeal", { member_wins: false, reason })}>Uphold decision</button>
              </div>
            </div>
          )}
        </section>
      )}
      {msg && <p role="status">{msg}</p>}
      <p><a href="/ops">Back to the queue</a></p>
    </Shell>
  );
}
