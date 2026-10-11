"use client";

import { useCallback, useEffect, useState } from "react";
import { startRegistration } from "@simplewebauthn/browser";
import { Shell } from "@/components/Shell";
import { Action } from "@/components/Action";
import { api, problemTitle } from "@/lib/api";
import "../billing/billing.css";

const KEY = "tryst.recovery";

type Status = { state: string; liveness_done: boolean; id_done: boolean; hold_until?: string };

function saved(): string | null {
  try {
    return sessionStorage.getItem(KEY);
  } catch {
    return null;
  }
}
function save(id: string | null) {
  try {
    if (id) sessionStorage.setItem(KEY, id);
    else sessionStorage.removeItem(KEY);
  } catch {
    /* storage unavailable */
  }
}

const CLOSED: Record<string, string> = {
  failed: "Recovery didn't go through. The ID must be the same document this account was verified with.",
  cancelled: "This recovery was cancelled from a signed-in device.",
  expired: "This recovery has expired.",
};

/**
 * Account recovery (FR-061): the same identity checks as verification, on the same ID
 * document, then a 24-hour hold that any signed-in device can cancel. Nobody at TRYST can
 * recover an account for you.
 */
export default function Recover() {
  const [id, setId] = useState<string | null>(null);
  const [st, setSt] = useState<Status | null>(null);
  const [contact, setContact] = useState("");
  const [token, setTok] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [msg, setMsg] = useState("");
  const [bio, setBio] = useState(false);

  const load = useCallback(async (rid: string) => {
    const r = await api<Status>(`/v1/auth/recovery/${encodeURIComponent(rid)}`);
    if (r.status !== 200) {
      save(null);
      setId(null);
      return;
    }
    setId(rid);
    setSt(r.data);
  }, []);

  useEffect(() => {
    const rid = saved();
    if (rid) load(rid);
  }, [load]);

  async function begin() {
    setMsg("");
    const r = await api<{ recovery_id: string }>("/v1/auth/recovery/begin", { method: "POST", body: { contact } });
    if (r.status !== 202) return setMsg(problemTitle(r.data));
    save(r.data.recovery_id);
    load(r.data.recovery_id);
  }

  async function check(kind: "liveness" | "age") {
    setMsg("");
    const r = await api<{ url: string }>(`/v1/auth/recovery/${encodeURIComponent(id!)}/check`, { method: "POST", body: { kind } });
    if (r.status !== 201) return setMsg(problemTitle(r.data));
    window.location.assign(r.data.url);
  }

  async function finish() {
    setMsg("");
    const r = await api<{ access_token: string }>(`/v1/auth/recovery/${encodeURIComponent(id!)}/finish`, { method: "POST" });
    if (r.status !== 200) return setMsg(problemTitle(r.data));
    save(null);
    setTok(r.data.access_token);
  }

  async function addPasskey() {
    setMsg("");
    try {
      const b = await api<{ publicKey: Parameters<typeof startRegistration>[0]["optionsJSON"] }>("/v1/auth/passkeys/register/begin", { method: "POST", token, body: { attachment: "platform" } });
      if (b.status !== 200) return setMsg(problemTitle(b.data));
      const cred = await startRegistration({ optionsJSON: b.data.publicKey });
      const f = await api("/v1/auth/passkeys/register/finish", { method: "POST", token, body: cred });
      if (f.status !== 201) return setMsg(problemTitle(f.data));
      setDone(true);
    } catch {
      setMsg("Passkey creation was cancelled or isn't supported on this device.");
    }
  }

  function restart() {
    save(null);
    setId(null);
    setSt(null);
  }

  let body: React.ReactNode;
  if (done) {
    body = (
      <section className="card" role="status" data-testid="recovered">
        <h2>Your account is back</h2>
        <p>Every old device has been signed out. Sign in with your new passkey; you can add a second device from your account.</p>
        <a className="button" href="/sign-in">Sign in</a>
      </section>
    );
  } else if (token) {
    body = (
      <section className="card">
        <h2>Create your new passkey</h2>
        <p>This window lasts 15 minutes.</p>
        <Action className="button" onClick={addPasskey}>Create passkey</Action>
      </section>
    );
  } else if (!id || !st) {
    body = (
      <section className="card">
        <p>Use this only if you&rsquo;ve lost every device that can sign in. You&rsquo;ll repeat the face check and the ID check with the same ID you verified with. Then there&rsquo;s a 24-hour wait, and we tell your account, so you can stop it if it isn&rsquo;t you.</p>
        <label htmlFor="contact">Email or mobile number on your account</label>
        <input id="contact" className="input" value={contact} onChange={(e) => setContact(e.target.value)} autoComplete="off" />
        <Action className="button" onClick={begin}>Start recovery</Action>
      </section>
    );
  } else if (CLOSED[st.state]) {
    body = (
      <section className="card" data-testid="recovery-closed">
        <p>{CLOSED[st.state]}</p>
        <Action className="button ghost" onClick={restart}>Start again</Action>
      </section>
    );
  } else if (st.state === "hold") {
    body = (
      <section className="card" data-testid="recovery-hold">
        <h2>Checks done: now the 24-hour wait</h2>
        <p>Come back after {st.hold_until ? new Date(st.hold_until).toLocaleString("en-GB") : "the wait"} on this device to finish. If someone signed in to your account cancels it, it stops.</p>
        <Action className="button ghost" onClick={() => load(id)}>Check again</Action>
      </section>
    );
  } else if (st.state === "ready") {
    body = (
      <section className="card">
        <h2>Ready to finish</h2>
        <p>Finishing signs out every old device and removes their passkeys.</p>
        <Action className="button" onClick={finish}>Finish recovery</Action>
      </section>
    );
  } else {
    body = (
      <section className="card" data-testid="recovery-checks">
        <h2>{st.liveness_done ? "2. ID check" : "1. Face check"}</h2>
        <p>{st.liveness_done ? "Use the same ID document you verified this account with." : "A quick live check with our verification provider."}</p>
        {st.liveness_done ? <Action className="button" onClick={() => check("age")}>Start ID check</Action> : (
          <>
            <label className="bl-check"><input type="checkbox" checked={bio} onChange={(e) => setBio(e.target.checked)} /> I consent to a face check by the verification provider</label>
            <Action className="button" disabled={!bio} onClick={() => check("liveness")}>Start face check</Action>
          </>
        )}
      </section>
    );
  }

  return (
    <Shell>
      <p className="eyebrow">Members</p>
      <h1>Recover your account</h1>
      {body}
      {msg && <p role="alert">{msg}</p>}
      <p className="note">No one at TRYST can recover an account for you or skip these steps.</p>
    </Shell>
  );
}
