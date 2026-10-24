"use client";

import { useState } from "react";
import { startRegistration } from "@simplewebauthn/browser";
import QRCode from "qrcode";
import { Shell } from "@/components/Shell";
import { Action } from "@/components/Action";
import { api, problemTitle, setToken } from "@/lib/api";

type Step = "contact" | "code" | "passkey" | "second" | "done";

// Sign-up (02 §3.3): contact OTP proves the handle (V0), then passkeys. On a computer the
// member also links a phone or adds a second security key, because desktop login needs two.
export default function Join() {
  const [step, setStep] = useState<Step>("contact");
  const [contact, setContact] = useState("");
  const [code, setCode] = useState("");
  const [devCode, setDevCode] = useState<string | null>(null);
  const [token, setLocalToken] = useState<string | null>(null);
  const [surface, setSurface] = useState<string>("");
  const [enrolLink, setEnrolLink] = useState<string | null>(null);
  const [qr, setQr] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [emergency, setEmergency] = useState<string | null>(null);

  async function sendCode() {
    setMsg(null);
    const r = await api("/v1/auth/start", { method: "POST", body: { contact } });
    if (r.status !== 202) return setMsg(problemTitle(r.data));
    setDevCode(r.headers.get("X-Dev-OTP"));
    setStep("code");
  }

  async function verify() {
    setMsg(null);
    const r = await api<{ access_token: string; emergency_code?: string }>("/v1/auth/verify", { method: "POST", body: { contact, otp: code } });
    if (r.status !== 200 && r.status !== 201) return setMsg(problemTitle(r.data));
    setLocalToken(r.data.access_token);
    if (r.data.emergency_code) setEmergency(r.data.emergency_code);
    setStep("passkey");
  }

  async function addPasskey(attachment: "platform" | "cross-platform") {
    setMsg(null);
    try {
      const begin = await api<{ publicKey: Parameters<typeof startRegistration>[0]["optionsJSON"] }>(
        "/v1/auth/passkeys/register/begin",
        { method: "POST", body: { attachment }, token },
      );
      if (begin.status !== 200) return setMsg(problemTitle(begin.data));
      const cred = await startRegistration({ optionsJSON: begin.data.publicKey });
      const fin = await api<{ surface: string }>("/v1/auth/passkeys/register/finish", { method: "POST", body: cred, token });
      if (fin.status !== 201) return setMsg(problemTitle(fin.data));
      if (step === "passkey") {
        setSurface(fin.data.surface);
        setStep(fin.data.surface === "desktop" ? "second" : "done");
      } else {
        setStep("done");
      }
    } catch {
      setMsg("Passkey creation was cancelled or isn't supported on this device.");
    }
  }

  async function linkPhone() {
    setMsg(null);
    const r = await api<{ enrol_path: string }>("/v1/auth/enrol-device", { method: "POST", token });
    if (r.status !== 201) return setMsg(problemTitle(r.data));
    const link = `${window.location.origin}${r.data.enrol_path}`;
    setEnrolLink(link);
    setQr(await QRCode.toDataURL(link, { margin: 1, color: { dark: "#0B0607", light: "#FCF2DD" } }));
  }

  function finish() {
    setToken(null); // onboarding sessions are not kept; members now sign in with two factors
    window.location.assign("/sign-in");
  }

  return (
    <Shell>
      <p className="eyebrow">Membership</p>
      <h1>Become a member</h1>
      <p className="lede">Open to verified adults in London, Manchester, Birmingham and Brighton. Once you&rsquo;re set up, only you can ever sign in.</p>

      {step === "contact" && (
        <section className="card">
          <label htmlFor="contact">Email or mobile number (international format)</label>
          <input id="contact" className="input" value={contact} onChange={(e) => setContact(e.target.value)} autoComplete="off" />
          <Action className="button" onClick={sendCode}>Send code</Action>
        </section>
      )}

      {step === "code" && (
        <section className="card">
          <label htmlFor="code">Enter the 6-digit code</label>
          <input id="code" className="input" inputMode="numeric" value={code} onChange={(e) => setCode(e.target.value)} autoComplete="one-time-code" />
          {devCode && <p className="note" data-testid="dev-otp">Development code: {devCode}</p>}
          <Action className="button" onClick={verify}>Continue</Action>
        </section>
      )}

      {step === "passkey" && emergency && (
        <section className="card" data-testid="emergency-code">
          <h2>Your emergency code</h2>
          <p className="code-link" style={{ fontSize: "1.4rem", letterSpacing: "0.1em" }}>{emergency}</p>
          <p className="note">Keep it somewhere only you can reach. If someone takes your phone, use it with your email or number at /lock to lock the account from any browser, without signing in. It works once. We show it only now.</p>
        </section>
      )}

      {step === "passkey" && (
        <section className="card">
          <h2>Create your passkey</h2>
          <p>Your device will ask for your fingerprint, face or screen lock. TRYST never sees it.</p>
          <Action className="button" onClick={() => addPasskey("platform")}>Create passkey</Action>
        </section>
      )}

      {step === "second" && (
        <section className="card">
          <h2>Add your second check</h2>
          <p>Signing in on a computer needs two checks: this passkey, plus approval from your phone or a second security key.</p>
          <Action className="button" onClick={linkPhone}>Link my phone</Action>
          {enrolLink && (
            <div>
              {qr && <img className="qr" src={qr} alt="Scan with your phone to link it" width={220} height={220} />}
              <p className="note">Or open this one-time link on your phone (valid 10 minutes):</p>
              <p className="note"><a className="code-link" data-testid="enrol-link" href={enrolLink}>{enrolLink}</a></p>
              <Action className="button" onClick={finish}>I've linked my phone</Action>
            </div>
          )}
          <p className="note">No phone? Use a second security key instead.</p>
          <Action className="button ghost" onClick={() => addPasskey("cross-platform")}>Add a security key</Action>
        </section>
      )}

      {step === "done" && (
        <section className="card" role="status">
          <h2>You're set up</h2>
          <p>{surface === "desktop" ? "Sign in with your passkey and your second check." : "Sign in with your fingerprint or face, then a live face check."}</p>
          <Action className="button" onClick={finish}>Sign in</Action>
        </section>
      )}

      {msg && <p role="alert">{msg}</p>}
    </Shell>
  );
}
