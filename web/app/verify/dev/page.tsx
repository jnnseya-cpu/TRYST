"use client";

import { useState } from "react";
import { Shell } from "@/components/Shell";
import { api, getToken, problemTitle } from "@/lib/api";
import "../../billing/billing.css";

/** Development-only stand-in for the verification provider's hosted check. */
export default function DevVerify() {
  const params = typeof window === "undefined" ? null : new URLSearchParams(window.location.search);
  const kind = params?.get("kind") ?? "liveness";
  const [birth, setBirth] = useState("1990-05");
  const [doc, setDoc] = useState("");
  const [adult, setAdult] = useState(true);
  const [error, setError] = useState("");

  async function finish(result: "pass" | "fail") {
    const r = await api<{ state: string }>("/v1/assurance/dev/complete", {
      method: "POST",
      token: getToken(),
      body: { session_id: params?.get("session"), result, is_adult: adult, birth_year_month: birth, document_number: doc },
    });
    if (r.status !== 200) return setError(problemTitle(r.data));
    window.location.assign("/verify");
  }

  return (
    <Shell>
      <p className="eyebrow">Test verification</p>
      <h1>Development {kind === "age" ? "ID check" : "face check"}</h1>
      <p>This stands in for the verification provider in development. Nothing is checked.</p>
      {kind === "age" && (
        <>
          <label htmlFor="birth">Birth month (YYYY-MM)</label>
          <input id="birth" className="input" value={birth} onChange={(e) => setBirth(e.target.value)} />
          <label htmlFor="doc">Document number</label>
          <input id="doc" className="input" value={doc} onChange={(e) => setDoc(e.target.value)} />
          <label className="bl-check"><input type="checkbox" checked={adult} onChange={(e) => setAdult(e.target.checked)} /> Provider says adult</label>
        </>
      )}
      <button type="button" className="button" onClick={() => finish("pass")}>Pass</button>
      <button type="button" className="button ghost" onClick={() => finish("fail")}>Fail</button>
      {error && <p role="alert">{error}</p>}
    </Shell>
  );
}
