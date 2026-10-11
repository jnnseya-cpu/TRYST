"use client";

import { useState } from "react";
import { Shell } from "@/components/Shell";
import { api, getToken, problemTitle } from "@/lib/api";

/** Development-only stand-in for the processor's hosted checkout. No money moves. */
export default function DevCheckout() {
  const [error, setError] = useState("");

  async function pay() {
    const session = new URLSearchParams(window.location.search).get("session") ?? "";
    const r = await api("/v1/billing/dev/complete", { method: "POST", token: getToken(), body: { session } });
    if (r.status !== 200) return setError(problemTitle(r.data));
    window.location.assign("/billing?done=1");
  }

  return (
    <Shell>
      <p className="eyebrow">Test payment</p>
      <h1>Development checkout</h1>
      <p>This stands in for the card payment page in development. No money moves.</p>
      <button type="button" className="button" onClick={pay}>Pay (test)</button>
      <a className="button ghost" href="/billing">Cancel</a>
      {error && <p role="alert">{error}</p>}
    </Shell>
  );
}
