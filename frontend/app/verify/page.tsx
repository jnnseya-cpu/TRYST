"use client";

import { useCallback, useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { problemTitle } from "@/lib/api";
import { acceptConduct, ageMessage, start, status, type VerifyStatus } from "@/lib/verify";
import { CONSENT_COPY, setConsent } from "@/lib/consent";
import "../profile/profile.css";
import "../billing/billing.css";

/** Verification: face check (V1), conduct declaration, ID and age check (V2). */
export default function Verify() {
  const [s, setS] = useState<VerifyStatus | null>(null);
  const [signedOut, setSignedOut] = useState(false);
  const [ticked, setTicked] = useState(false);
  const [bio, setBio] = useState(false);
  const [msg, setMsg] = useState("");

  const load = useCallback(async () => {
    const r = await status();
    if (r.status !== 200) return setSignedOut(true);
    setS(r.data);
  }, []);
  useEffect(() => {
    load();
  }, [load]);

  async function go(kind: "liveness" | "age") {
    setMsg("");
    if (kind === "liveness") {
      const c = await setConsent("biometric_login", true);
      if (c.status !== 201) return setMsg(problemTitle(c.data));
    }
    const r = await start(kind);
    if (r.status !== 201) return setMsg(problemTitle(r.data));
    window.location.assign(r.data.url);
  }

  async function conduct() {
    if (!s) return;
    const r = await acceptConduct(s.conduct_version);
    if (r.status !== 200) return setMsg(problemTitle(r.data));
    setS(r.data);
  }

  if (signedOut) return <Shell><h1>Signed out</h1><a className="button inline" href="/sign-in">Sign in</a></Shell>;
  if (!s) return <Shell><p>Loading…</p></Shell>;
  if (s.refused) return <Shell><h1>We can&rsquo;t verify this account</h1><p>TRYST is for adults only, and each person may hold one account.</p></Shell>;
  if (s.tier === "V2" || s.tier === "V3") {
    return (
      <Shell>
        <p className="eyebrow">Verification</p>
        <h1 data-testid="verified">You&rsquo;re verified</h1>
        <p>You can see other members, use Tonight and message the people you match with.</p>
        <a className="button" href="/account">Your account</a>
      </Shell>
    );
  }
  return (
    <Shell>
      <p className="eyebrow">Verification</p>
      <h1>Verify it&rsquo;s you</h1>
      <p className="lede">Every member is a verified adult. Your ID is checked by our verification provider; we keep only that you&rsquo;re over 18, your birth month and year, and a one-way code so the same ID can&rsquo;t open a second account. Never a copy of your ID.</p>

      <section className="pf-section" aria-label="Face check">
        <h2>1. Face check</h2>
        {s.liveness_done ? <p data-testid="step-done">Done.</p> : (
          <>
            <p>A short live check that you&rsquo;re a real person and match your photos.</p>
            <p className="note">{CONSENT_COPY.biometric_login.body} You can withdraw this in your account.</p>
            <label className="bl-check"><input type="checkbox" checked={bio} onChange={(e) => setBio(e.target.checked)} /> I consent to face checks</label>
            <button type="button" className="button inline" disabled={!bio} onClick={() => go("liveness")}>Start face check</button>
          </>
        )}
      </section>

      <section className="pf-section" aria-label="Conduct declaration">
        <h2>2. Declaration</h2>
        {s.conduct_accepted ? <p data-testid="step-done">Done.</p> : (
          <>
            <p className="note">I declare that I have no unspent convictions for sexual or violent offences, and that I am not subject to a Sexual Harm Prevention Order, a restraining order or a non-molestation order. I understand that a declaration I knew to be false is grounds for permanent removal (Terms, clause 3.5).</p>
            <label className="bl-check"><input type="checkbox" checked={ticked} onChange={(e) => setTicked(e.target.checked)} /> I make this declaration</label>
            <button type="button" className="button inline" disabled={!ticked} onClick={conduct}>Confirm declaration</button>
          </>
        )}
      </section>

      <section className="pf-section" aria-label="ID check">
        <h2>3. ID check</h2>
        {s.id_checked ? <p data-testid="step-done">Done.</p> : (
          <>
            <p>Your passport or driving licence, checked by our provider. TRYST is for adults only.</p>
            {s.age_check && ageMessage[s.age_check] && <p role="alert">{ageMessage[s.age_check]}</p>}
            <button type="button" className="button inline" disabled={!s.liveness_done} onClick={() => go("age")}>Start ID check</button>
          </>
        )}
      </section>
      {msg && <p role="alert">{msg}</p>}
    </Shell>
  );
}
