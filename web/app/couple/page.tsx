"use client";

import { useCallback, useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { Action } from "@/components/Action";
import { api, getToken, problemTitle } from "@/lib/api";
import { deviceId, label, type Vector } from "@/lib/intro";

type Couple = { id: string; partner: string; veto_mode: string; pending_veto_mode?: string; your_pending_veto_mode?: string; reconfirm_needed: boolean; reconfirm_due: string; in_discovery: boolean };

/** Couple link (02 §5.4): optional; joining alone needs nothing from a partner. */
export default function CouplePage() {
  const [c, setC] = useState<Couple | null | undefined>(undefined);
  const [invite, setInvite] = useState("");
  const [token, setTokenIn] = useState("");
  const [preview, setPreview] = useState<{ inviter: string; version: number; vector: Vector } | null>(null);
  const [msg, setMsg] = useState("");
  const tok = () => getToken();

  const load = useCallback(async () => {
    const r = await api<{ couple: Couple | null }>("/v1/couple", { token: tok() });
    if (r.status === 200) setC(r.data.couple);
    else setC(null);
  }, []);
  useEffect(() => {
    load();
    const t = new URLSearchParams(window.location.search).get("t");
    if (t) setTokenIn(t);
  }, [load]);

  async function call(path: string, method: string, body?: unknown) {
    setMsg("");
    const r = await api<{ couple: Couple | null }>(path, { method, token: tok(), body });
    if (r.status >= 300) return setMsg(problemTitle(r.data));
    setC(r.data.couple);
  }

  async function makeInvite() {
    setMsg("");
    const r = await api<{ token: string }>("/v1/couple/invite", { method: "POST", token: tok(), body: { device_id: deviceId() } });
    if (r.status !== 201) return setMsg(problemTitle(r.data));
    setInvite(`${window.location.origin}/couple?t=${r.data.token}`);
  }

  async function look() {
    setMsg("");
    const r = await api<{ inviter: string; version: number; vector: Vector }>(`/v1/couple/invite-preview?token=${encodeURIComponent(token)}`, { token: tok() });
    if (r.status !== 200) return setMsg(problemTitle(r.data));
    setPreview(r.data);
  }

  if (c === undefined) return <Shell><p>Loading…</p></Shell>;
  return (
    <Shell>
      <p className="eyebrow">Couples</p>
      <h1>Couple link</h1>
      {c ? (
        <section className="card" data-testid="couple">
          <p>Linked with <strong>{c.partner}</strong>.</p>
          {!c.in_discovery && <p role="alert">You&rsquo;re out of introductions until you both reconfirm.</p>}
          {c.reconfirm_needed && <Action className="button" onClick={() => call(`/v1/couple/${c.id}/reconfirm`, "POST")}>Reconfirm we&rsquo;re still together</Action>}
          <h2>Veto</h2>
          <p className="note">With &ldquo;either&rdquo;, one of you can say yes to an introduction unless the other says no. With &ldquo;both&rdquo;, you both have to say yes. Changing it needs both of you.</p>
          <p data-testid="veto">Now: {c.veto_mode}{c.your_pending_veto_mode && ` · you asked for “${c.your_pending_veto_mode}”`}{c.pending_veto_mode && ` · your partner asked for “${c.pending_veto_mode}”`}</p>
          {["either", "both"].filter((m) => m !== c.veto_mode).map((m) => (
            <Action key={m} className="button ghost" onClick={() => call(`/v1/couple/${c.id}/veto-mode`, "PUT", { veto_mode: m })}>Ask for &ldquo;{m}&rdquo;</Action>
          ))}
          <h2>End the link</h2>
          <p className="note">Either of you can end it now. Your partner is told only that it ended.</p>
          <Action className="button ghost" onClick={() => call(`/v1/couple/${c.id}`, "DELETE")}>End our couple link</Action>
        </section>
      ) : (
        <>
          <section className="card">
            <h2>Invite your partner</h2>
            <p className="note">Your partner confirms on their own phone or computer, within 24 hours, after seeing your answers.</p>
            {invite ? <p className="note">Send this to your partner: <a className="code-link" data-testid="invite-link" href={invite}>{invite}</a></p>
              : <Action className="button" onClick={makeInvite}>Create an invitation</Action>}
          </section>
          <section className="card">
            <h2>Accept an invitation</h2>
            <label htmlFor="token">Invitation</label>
            <input id="token" className="input" value={token} onChange={(e) => setTokenIn(e.target.value)} />
            {!preview ? <Action className="button ghost" onClick={look}>Look at the invitation</Action> : (
              <div data-testid="preview">
                <p>{preview.inviter} wants to link as a couple. Their answers: {preview.vector.intent_shape.map(label).join(", ")}; meeting {preview.vector.seeking.map(label).join(", ")}.</p>
                <Action className="button" onClick={() => call("/v1/couple/cosign", "POST", { token, device_id: deviceId(), approved_version: preview.version })}>Confirm and link</Action>
              </div>
            )}
          </section>
        </>
      )}
      {msg && <p role="alert">{msg}</p>}
    </Shell>
  );
}
