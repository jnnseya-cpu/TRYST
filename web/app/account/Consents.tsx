"use client";

import { useEffect, useState } from "react";
import { problemTitle } from "@/lib/api";
import { CONSENT_COPY, consents, setConsent, type Consents as Ledger } from "@/lib/consent";

const LABEL: Record<string, string> = {
  art9_processing: "Profile in discovery",
  biometric_login: "Face checks",
  safety_training_use: "Safety tool training",
  conduct_attestation: "Conduct declaration",
};

/** The member's consents and their full history (append-only, FR-040). */
export function Consents() {
  const [l, setL] = useState<Ledger | null>(null);
  const [msg, setMsg] = useState("");
  const load = () => consents().then((r) => r.status === 200 && setL(r.data));
  useEffect(() => {
    load();
  }, []);

  async function change(kind: string, granted: boolean) {
    const r = await setConsent(kind, granted);
    if (r.status !== 201) return setMsg(problemTitle(r.data));
    setMsg(granted ? "Consent given." : CONSENT_COPY[kind].withdraw);
    load();
  }

  if (!l) return null;
  return (
    <section className="card" aria-label="Consents">
      <h2>Consents</h2>
      {Object.entries(CONSENT_COPY).map(([kind, c]) => {
        const on = !!l.current[kind];
        return (
          <div key={kind} data-testid={`consent-${kind}`}>
            <h3>{c.title}: {on ? "given" : "not given"}</h3>
            <p className="note">{c.body}</p>
            <button type="button" className="button ghost" onClick={() => change(kind, !on)}>{on ? "Withdraw" : "Give consent"}</button>
          </div>
        );
      })}
      <details>
        <summary>History ({l.entries.length})</summary>
        <ul>
          {[...l.entries].reverse().map((e) => (
            <li key={e.id}>{new Date(e.at).toLocaleString("en-GB")}: {LABEL[e.kind] ?? e.kind} {e.granted ? "given" : "withdrawn"}{e.scope ? ` (${e.scope})` : ""}</li>
          ))}
        </ul>
        <p className="note">{l.chain_ok ? "This record is sealed: no entry can be changed or removed." : "We found a problem with this record and are looking into it."}</p>
      </details>
      {msg && <p role="status">{msg}</p>}
    </section>
  );
}
