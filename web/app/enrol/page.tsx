"use client";

import { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";
import { startRegistration } from "@simplewebauthn/browser";
import { api, problemTitle } from "@/lib/api";

function EnrolInner() {
  const t = useSearchParams().get("t") ?? "";
  const [state, setState] = useState<"ready" | "done">("ready");
  const [msg, setMsg] = useState<string | null>(null);

  async function enrol() {
    setMsg(null);
    const r = await api<{ access_token: string }>("/v1/auth/enrol-device/redeem", { method: "POST", body: { enrol_token: t } });
    if (r.status !== 200) return setMsg(problemTitle(r.data));
    const token = r.data.access_token;
    try {
      const begin = await api<{ publicKey: Parameters<typeof startRegistration>[0]["optionsJSON"] }>(
        "/v1/auth/passkeys/register/begin",
        { method: "POST", body: { attachment: "platform" }, token },
      );
      if (begin.status !== 200) return setMsg(problemTitle(begin.data));
      const cred = await startRegistration({ optionsJSON: begin.data.publicKey });
      const fin = await api("/v1/auth/passkeys/register/finish", { method: "POST", body: cred, token });
      if (fin.status !== 201) return setMsg(problemTitle(fin.data));
      setState("done");
    } catch {
      setMsg("Passkey creation was cancelled or isn't supported on this device.");
    }
  }

  return (
    <main>
      <h1>Link this phone</h1>
      {state === "ready" ? (
        <section className="card">
          <p>This phone will approve sign-ins on your computer using its fingerprint or face unlock.</p>
          <button type="button" className="button" onClick={enrol}>Link this phone</button>
        </section>
      ) : (
        <section className="card" role="status">
          <h2>Phone linked</h2>
          <p>You can now approve sign-ins on your computer from this phone.</p>
        </section>
      )}
      {msg && <p role="alert">{msg}</p>}
    </main>
  );
}

export default function Enrol() {
  return (
    <Suspense>
      <EnrolInner />
    </Suspense>
  );
}
