"use client";

import { useEffect, useRef, useState } from "react";
import { startAuthentication } from "@simplewebauthn/browser";
import QRCode from "qrcode";
import { FACTOR_COPY, GENERIC_FAILURE, type Factor } from "@/lib/login";
import { Shell } from "@/components/Shell";
import { api, problemTitle, setToken } from "@/lib/api";

type AssertionOptions = Parameters<typeof startAuthentication>[0]["optionsJSON"];
type Session = { access_token: string };

// Account-holder-only sign-in (02 §3.3). The server decides the path from the passkey used:
// phone → B1 + B2 (live face check); computer → F1 + F2 (phone approval or second key).
export default function SignIn() {
  const [loginId, setLoginId] = useState<string | null>(null);
  const [required, setRequired] = useState<Factor[]>([]);
  const [passed, setPassed] = useState<Factor[]>([]);
  const [qr, setQr] = useState<{ img: string; link: string } | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const poller = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => () => { if (poller.current) clearInterval(poller.current); }, []);

  function done(s: Session) {
    if (poller.current) clearInterval(poller.current);
    setToken(s.access_token);
    window.location.assign("/account");
  }

  function failed(data?: unknown) {
    setMsg(problemTitle(data, GENERIC_FAILURE));
  }

  async function firstFactor() {
    setMsg(null);
    const begin = await api<{ login_id: string; options: { publicKey: AssertionOptions } }>("/v1/auth/login/begin", { method: "POST" });
    if (begin.status !== 200) return failed(begin.data);
    try {
      const cred = await startAuthentication({ optionsJSON: begin.data.options.publicKey });
      const r = await api<{ required_factors: Factor[] }>("/v1/auth/login/passkey", {
        method: "POST",
        body: { login_id: begin.data.login_id, credential: cred },
      });
      if (r.status !== 200) return failed(r.data);
      setLoginId(begin.data.login_id);
      setRequired(r.data.required_factors);
      setPassed([r.data.required_factors[0]]);
    } catch {
      setMsg("Sign-in was cancelled.");
    }
  }

  async function liveness() {
    setMsg(null);
    const r = await api<Session>("/v1/auth/login/liveness", { method: "POST", body: { login_id: loginId } });
    if (r.status !== 200) return failed(r.data);
    done(r.data);
  }

  async function phoneApproval() {
    setMsg(null);
    const r = await api<{ qr_token: string; approve_path: string }>("/v1/auth/login/cross-device", { method: "POST", body: { login_id: loginId } });
    if (r.status !== 200) return failed(r.data);
    const link = `${window.location.origin}${r.data.approve_path}`;
    setQr({ img: await QRCode.toDataURL(link, { margin: 1, color: { dark: "#0B0607", light: "#FCF2DD" } }), link });
    const qrToken = r.data.qr_token;
    if (poller.current) clearInterval(poller.current);
    poller.current = setInterval(async () => {
      const p = await api<Session & { state?: string }>(`/v1/auth/login/cross-device/${encodeURIComponent(qrToken)}`, {
        headers: { "X-Login-Id": loginId ?? "" },
      });
      if (p.status === 200 && p.data.access_token) done(p.data);
      else if (p.status !== 200) {
        if (poller.current) clearInterval(poller.current);
        failed(p.data);
      }
    }, 1500);
  }

  async function secondKey() {
    setMsg(null);
    const b = await api<{ options: { publicKey: AssertionOptions } }>("/v1/auth/login/second-key/begin", { method: "POST", body: { login_id: loginId } });
    if (b.status !== 200) return failed(b.data);
    try {
      const cred = await startAuthentication({ optionsJSON: b.data.options.publicKey });
      const r = await api<Session>("/v1/auth/login/second-key", { method: "POST", body: { login_id: loginId, credential: cred } });
      if (r.status !== 200) return failed(r.data);
      done(r.data);
    } catch {
      setMsg("Sign-in was cancelled.");
    }
  }

  const next = required.find((f) => !passed.includes(f));

  return (
    <Shell>
      <p className="eyebrow">Members</p>
      <h1>Sign in</h1>
      <p className="lede">Two checks, every time. Only you can open this account.</p>
      {required.length > 0 && (
        <ol className="steps">
          {required.map((f, i) => (
            <li key={f} aria-current={next === f ? "step" : undefined}>
              <span className="num">{["I", "II"][i]}</span>
              {FACTOR_COPY[f].title}
              <span className="state">{passed.includes(f) ? "Done" : next === f ? "Now" : ""}</span>
            </li>
          ))}
        </ol>
      )}

      {!loginId && (
        <section className="card">
          <h2>Use your passkey</h2>
          <p>Your device will ask for your fingerprint, face, or screen lock.</p>
          <button type="button" className="button" onClick={firstFactor}>Sign in with passkey</button>
        </section>
      )}

      {next === "B2" && (
        <section className="card">
          <h2>{FACTOR_COPY.B2.title}</h2>
          <p>{FACTOR_COPY.B2.body}</p>
          <button type="button" className="button" onClick={liveness}>Start live check</button>
        </section>
      )}

      {next === "F2" && (
        <section className="card">
          <h2>{FACTOR_COPY.F2.title}</h2>
          {!qr ? (
            <button type="button" className="button" onClick={phoneApproval}>Show code for my phone</button>
          ) : (
            <div>
              <img className="qr" src={qr.img} alt="Scan with your registered phone" width={220} height={220} />
              <p className="note">Or open on your phone: <a className="code-link" data-testid="approve-link" href={qr.link}>{qr.link}</a></p>
              <p className="note" role="status">Waiting for approval on your phone…</p>
            </div>
          )}
          <p className="note">No phone?</p>
          <button type="button" className="button ghost" onClick={secondKey}>Use my second security key</button>
        </section>
      )}

      {msg && <p role="alert">{msg}</p>}
      <hr className="rule" />
      <p className="note">We never sign you in with a text message or an email code, and no one at TRYST can sign in for you.</p>
      <p className="note">Not a member yet? <a href="/join">Request an invitation</a></p>
    </Shell>
  );
}
