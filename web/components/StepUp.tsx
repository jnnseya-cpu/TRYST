"use client";

import { useEffect, useRef, useState } from "react";
import { startAuthentication } from "@simplewebauthn/browser";
import QRCode from "qrcode";
import { Action } from "@/components/Action";
import { api, getToken, problemTitle } from "@/lib/api";
import { FACTOR_COPY, GENERIC_FAILURE, type Factor } from "@/lib/login";

type AssertionOptions = Parameters<typeof startAuthentication>[0]["optionsJSON"];

/**
 * Step-up (02 §3.3): both sign-in checks again, inside this session. Sensitive actions are
 * then allowed for 5 minutes. Calls onDone once the server confirms.
 */
export function StepUp({ reason, onDone }: { reason: string; onDone: () => void }) {
  const [loginId, setLoginId] = useState<string | null>(null);
  const [next, setNext] = useState<Factor | null>(null);
  const [qr, setQr] = useState<string | null>(null);
  const [msg, setMsg] = useState("");
  const poller = useRef<ReturnType<typeof setInterval> | null>(null);
  useEffect(() => () => { if (poller.current) clearInterval(poller.current); }, []);

  function finished(status: number, data: unknown) {
    if (status === 200 && (data as { step_up?: boolean }).step_up) return onDone();
    setMsg(problemTitle(data, GENERIC_FAILURE));
    setLoginId(null);
    setNext(null);
  }

  async function first() {
    setMsg("");
    const b = await api<{ login_id: string; options: { publicKey: AssertionOptions } }>("/v1/auth/step-up", { method: "POST", token: getToken() });
    if (b.status !== 200) return setMsg(problemTitle(b.data));
    try {
      const cred = await startAuthentication({ optionsJSON: b.data.options.publicKey });
      const r = await api<{ next_factor: Factor }>("/v1/auth/login/passkey", { method: "POST", body: { login_id: b.data.login_id, credential: cred } });
      if (r.status !== 200) return setMsg(problemTitle(r.data, GENERIC_FAILURE));
      setLoginId(b.data.login_id);
      setNext(r.data.next_factor);
    } catch {
      setMsg("The check was cancelled.");
    }
  }

  async function liveness() {
    const r = await api("/v1/auth/login/liveness", { method: "POST", body: { login_id: loginId } });
    finished(r.status, r.data);
  }

  async function phone() {
    const r = await api<{ qr_token: string; approve_path: string }>("/v1/auth/login/cross-device", { method: "POST", body: { login_id: loginId } });
    if (r.status !== 200) return setMsg(problemTitle(r.data));
    setQr(await QRCode.toDataURL(`${window.location.origin}${r.data.approve_path}`, { margin: 1 }));
    const qrToken = r.data.qr_token;
    poller.current = setInterval(async () => {
      const p = await api<{ state?: string }>(`/v1/auth/login/cross-device/${encodeURIComponent(qrToken)}`, { headers: { "X-Login-Id": loginId ?? "" } });
      if (p.status === 200 && p.data.state === "approval_pending") return;
      if (poller.current) clearInterval(poller.current);
      finished(p.status, p.data);
    }, 1500);
  }

  async function secondKey() {
    const b = await api<{ options: { publicKey: AssertionOptions } }>("/v1/auth/login/second-key/begin", { method: "POST", body: { login_id: loginId } });
    if (b.status !== 200) return setMsg(problemTitle(b.data));
    try {
      const cred = await startAuthentication({ optionsJSON: b.data.options.publicKey });
      const r = await api("/v1/auth/login/second-key", { method: "POST", body: { login_id: loginId, credential: cred } });
      finished(r.status, r.data);
    } catch {
      setMsg("The check was cancelled.");
    }
  }

  return (
    <div className="card" data-testid="step-up">
      <h3>Confirm it&rsquo;s you</h3>
      <p className="note">{reason} You&rsquo;ll do both sign-in checks again; this lasts 5 minutes.</p>
      {!loginId && <Action className="button" onClick={first}>Confirm with passkey</Action>}
      {next === "B2" && <Action className="button" onClick={liveness}>Start live check</Action>}
      {next === "F2" && (
        <>
          <p className="note">{FACTOR_COPY.F2.body}</p>
          {qr ? <img className="qr" src={qr} alt="Scan with your registered phone" width={200} height={200} /> : <Action className="button" onClick={phone}>Show code for my phone</Action>}
          <Action className="button ghost" onClick={secondKey}>Use my second security key</Action>
        </>
      )}
      {msg && <p role="alert">{msg}</p>}
    </div>
  );
}
