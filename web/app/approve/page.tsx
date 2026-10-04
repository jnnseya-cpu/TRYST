"use client";

import { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";
import { startAuthentication } from "@simplewebauthn/browser";
import { Shell } from "@/components/Shell";
import { api, problemTitle } from "@/lib/api";

// F2: the registered phone approves a desktop sign-in with its own biometric passkey (B1).
function ApproveInner() {
  const qr = useSearchParams().get("qr") ?? "";
  const [state, setState] = useState<"ready" | "approved" | "denied">("ready");
  const [msg, setMsg] = useState<string | null>(null);

  async function decide(decision: "approve" | "deny") {
    setMsg(null);
    const begin = await api<{ options: { publicKey: Parameters<typeof startAuthentication>[0]["optionsJSON"] } }>(
      `/v1/auth/approvals/${encodeURIComponent(qr)}/begin`,
      { method: "POST" },
    );
    if (begin.status !== 200) return setMsg(problemTitle(begin.data, "This code has expired."));
    try {
      const cred = await startAuthentication({ optionsJSON: begin.data.options.publicKey });
      const fin = await api(`/v1/auth/approvals/${encodeURIComponent(qr)}`, { method: "POST", body: { decision, credential: cred } });
      if (fin.status !== 200) return setMsg(problemTitle(fin.data));
      setState(decision === "approve" ? "approved" : "denied");
    } catch {
      setMsg("Approval was cancelled.");
    }
  }

  return (
    <Shell>
      <p className="eyebrow">Approve sign-in</p>
      <h1>Is this you?</h1>
      {state === "ready" && (
        <section className="card">
          <p>Someone is signing in to your account on a computer. Approve only if it's you.</p>
          <button type="button" className="button" onClick={() => decide("approve")}>Approve</button>
          <p />
          <button type="button" className="button ghost" onClick={() => decide("deny")}>It wasn't me</button>
        </section>
      )}
      {state === "approved" && <section className="card" role="status"><h2>Approved</h2><p>Return to your computer.</p></section>}
      {state === "denied" && <section className="card" role="status"><h2>Declined</h2><p>That sign-in has been blocked.</p></section>}
      {msg && <p role="alert">{msg}</p>}
    </Shell>
  );
}

export default function Approve() {
  return (
    <Suspense>
      <ApproveInner />
    </Suspense>
  );
}
