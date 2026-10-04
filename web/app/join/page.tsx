"use client";

import { useState } from "react";
import { startRegistration } from "@simplewebauthn/browser";
import QRCode from "qrcode";
import { Shell } from "@/components/Shell";
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

  async function sendCode() {
    setMsg(null);
    const r = await api("/v1/auth/start", { method: "POST", body: { contact } });
    if (r.status !== 202) return setMsg(problemTitle(r.data));
    setDevCode(r.headers.get("X-Dev-OTP"));
    setStep("code");
  }

  async function verify() {
    setMsg(null);
    const r = await api<{ access_token: string }>("/v1/auth/verify", { method: "POST", body: { contact, otp: code } });
    if (r.status !== 200 && r.status !== 201) return setMsg(problemTitle(r.data));
    setLocalToken(r.data.access_token);
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
      <p className="lede">Open to verified adults in London, Manchester, Birmingham and Brighton. Matching opens city by city, starting with London; we&rsquo;ll show you how close yours is. Once you&rsquo;re set up, only you can ever sign in.</p>
      <p className="note">To keep every city balanced, we sometimes hold places on a short, first-come waitlist. You&rsquo;ll see your position, and you&rsquo;re let in automatically. Couples are never kept waiting.</p>

      {step === "contact" && (
        <section className="card">
          <label htmlFor="contact">Email or mobile number (international format)</label>
          <input id="contact" className="input" value={contact} onChange={(e) => setContact(e.target.value)} autoComplete="off" />
          <button type="button" className="button" onClick={sendCode}>Send code</button>
        </section>
      )}

      {step === "code" && (
        <section className="card">
          <label htmlFor="code">Enter the 6-digit code</label>
          <input id="code" className="input" inputMode="numeric" value={code} onChange={(e) => setCode(e.target.value)} autoComplete="one-time-code" />
          {devCode && <p className="note" data-testid="dev-otp">Development code: {devCode}</p>}
          <button type="button" className="button" onClick={verify}>Continue</button>
        </section>
      )}

      {step === "passkey" && (
        <section className="card">
          <h2>Create your passkey</h2>
          <p>Your device will ask for your fingerprint, face or screen lock. TRYST never sees it.</p>
          <button type="button" className="button" onClick={() => addPasskey("platform")}>Create passkey</button>
        </section>
      )}

      {step === "second" && (
        <section className="card">
          <h2>Add your second check</h2>
          <p>Signing in on a computer needs two checks: this passkey, plus approval from your phone or a second security key.</p>
          <button type="button" className="button" onClick={linkPhone}>Link my phone</button>
          {enrolLink && (
            <div>
              {qr && <img className="qr" src={qr} alt="Scan with your phone to link it" width={220} height={220} />}
              <p className="note">Or open this one-time link on your phone (valid 10 minutes):</p>
              <p className="note"><a className="code-link" data-testid="enrol-link" href={enrolLink}>{enrolLink}</a></p>
              <button type="button" className="button" onClick={finish}>I've linked my phone</button>
            </div>
          )}
          <p className="note">No phone? Use a second security key instead.</p>
          <button type="button" className="button ghost" onClick={() => addPasskey("cross-platform")}>Add a security key</button>
        </section>
      )}

      {step === "done" && (
        <section className="card" role="status">
          <h2>You're set up</h2>
          <p>{surface === "desktop" ? "Sign in with your passkey and your second check." : "Sign in with your fingerprint or face, then a live face check."}</p>
          <button type="button" className="button" onClick={finish}>Sign in</button>
        </section>
      )}

      {msg && <p role="alert">{msg}</p>}
    </Shell>
  );
}
