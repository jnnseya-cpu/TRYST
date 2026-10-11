"use client";

import { useEffect, useState } from "react";
import { useGuarded } from "@/components/StepUp";
import { api, getToken } from "@/lib/api";
import { addShield, clearShield, discretion, putPolicy, putZones, statusChange, type Discretion as D } from "@/lib/discretion";

type Area = { id: string; district: string; city: string };

const STATUS: Record<string, string> = {
  active: "You're in discovery.",
  paused: "Paused: nobody can find you. Your conversations carry on.",
  vanished: "Vanished: nobody can find you and your conversations are closed.",
  locked: "Locked.",
};

/** Pause, vanish, the Discretion Policy, hidden areas and the partner shield. */
export function Discretion() {
  const [d, setD] = useState<D | null>(null);
  const [areas, setAreas] = useState<Area[]>([]);
  const [partner, setPartner] = useState("");
  const [msg, setMsg] = useState("");
  const { guarded, prompt } = useGuarded("Showing yourself to more people is protected.", setMsg);

  useEffect(() => {
    discretion().then((r) => r.status === 200 && setD(r.data));
    api<{ areas: Area[] }>("/v1/profile/areas", { token: getToken() }).then((r) => r.status === 200 && setAreas(r.data.areas));
  }, []);

  const apply = (run: () => Promise<{ status: number; data: unknown }>, done: string) => {
    setMsg("");
    return guarded(run, (data) => {
      setD(data as D);
      setMsg(done);
    });
  };

  if (!d) return null;
  const p = d.policy;
  const zoneLabel = (id: string) => {
    const a = areas.find((x) => x.id === id);
    return a ? `${a.district}, ${a.city}` : id;
  };

  return (
    <section className="card" aria-label="Discretion" data-testid="discretion">
      <h2>Discretion</h2>
      <p data-testid="discretion-status">{STATUS[d.status]}</p>
      {d.status === "active" ? (
        <>
          <button type="button" className="button ghost" onClick={() => apply(() => statusChange("pause"), "Paused.")}>Pause</button>
          <button type="button" className="button ghost" onClick={() => apply(() => statusChange("vanish"), "Vanished. Your conversations are closed.")}>Vanish</button>
        </>
      ) : (
        <button type="button" className="button ghost" onClick={() => apply(() => statusChange("resume"), "You're back in discovery.")}>Come back to discovery</button>
      )}

      <h3>Who can see you</h3>
      <label htmlFor="visibility">Visibility</label>
      <select id="visibility" className="input" value={p.visibility}
        onChange={(e) => apply(() => putPolicy({ ...p, visibility: e.target.value as "visible" | "incognito" }), "Saved.")}>
        <option value="visible">Visible to people free nearby</option>
        <option value="incognito">Incognito: only people I&rsquo;m interested in</option>
      </select>
      <label htmlFor="max-distance">Furthest away</label>
      <select id="max-distance" className="input" value={p.max_distance_km}
        onChange={(e) => apply(() => putPolicy({ ...p, max_distance_km: Number(e.target.value) }), "Saved.")}>
        {[5, 10, 25, 50].map((k) => <option key={k} value={k}>about {k} km</option>)}
      </select>
      <label htmlFor="min-tier">Only members verified to at least</label>
      <select id="min-tier" className="input" value={p.min_counterparty_tier}
        onChange={(e) => apply(() => putPolicy({ ...p, min_counterparty_tier: e.target.value as "V2" | "V3" }), "Saved.")}>
        <option value="V2">ID-checked</option>
        <option value="V3">ID-checked and vouched for</option>
      </select>

      <h3>Hidden areas</h3>
      <p className="note">People in these areas won&rsquo;t see you and you won&rsquo;t see them, for example around home or work.</p>
      <ul data-testid="zones">
        {p.zones.map((z) => (
          <li key={z}>{zoneLabel(z)} <button type="button" className="ms-block" onClick={() => apply(() => putZones(p.zones.filter((x) => x !== z)), "Area removed.")}>Remove</button></li>
        ))}
      </ul>
      {p.zones.length < 4 && (
        <>
          <label htmlFor="add-zone">Hide an area</label>
          <select id="add-zone" className="input" value="" onChange={(e) => e.target.value && apply(() => putZones([...p.zones, e.target.value]), "Area hidden.")}>
            <option value="">Choose…</option>
            {areas.filter((a) => !p.zones.includes(a.id)).map((a) => <option key={a.id} value={a.id}>{a.district}, {a.city}</option>)}
          </select>
        </>
      )}

      <h3>Partner shield</h3>
      <p className="note">Add a partner&rsquo;s email or phone number. It&rsquo;s scrambled on this device before it&rsquo;s sent, and we can&rsquo;t turn it back. If they ever join, neither of you will see the other. Nobody is told. Shielded contacts: {d.shield}.</p>
      <label htmlFor="partner">Partner&rsquo;s email or phone</label>
      <input id="partner" className="input" autoComplete="off" value={partner} onChange={(e) => setPartner(e.target.value)} />
      <button type="button" className="button ghost" disabled={!partner.trim()} onClick={() => apply(() => addShield(partner), "Added to your shield.").then(() => setPartner(""))}>Add to shield</button>
      {d.shield > 0 && <button type="button" className="ms-block" onClick={() => apply(clearShield, "Shield cleared.")}>Clear shield</button>}

      {prompt}
      {msg && <p role="status">{msg}</p>}
    </section>
  );
}
