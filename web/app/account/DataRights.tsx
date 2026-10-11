"use client";

import { useState } from "react";
import { getToken, problemTitle, setToken } from "@/lib/api";
import { forgetDevice } from "@/lib/e2ee";

const PHRASE = "delete my account";

/** Export (UK GDPR Art. 15/20) and deletion by cryptographic erasure (ToS cl. 15). */
export function DataRights() {
  const [msg, setMsg] = useState("");
  const [busy, setBusy] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [typed, setTyped] = useState("");

  async function download() {
    setBusy(true);
    setMsg("");
    const res = await fetch("/v1/account/export", { method: "POST", headers: { Authorization: `Bearer ${getToken()}` }, cache: "no-store" });
    setBusy(false);
    if (!res.ok) return setMsg(problemTitle(await res.json().catch(() => ({}))));
    const url = URL.createObjectURL(await res.blob());
    const a = document.createElement("a");
    a.href = url;
    a.download = "member-data.json";
    a.click();
    URL.revokeObjectURL(url);
    setMsg("Your data has downloaded. Keep the file somewhere private.");
  }

  async function erase() {
    setBusy(true);
    const res = await fetch("/v1/account", {
      method: "DELETE",
      headers: { Authorization: `Bearer ${getToken()}`, "Content-Type": "application/json" },
      body: JSON.stringify({ confirm: typed }),
      cache: "no-store",
    });
    const body = await res.json().catch(() => ({}));
    if (res.status !== 200 && res.status !== 202) {
      setBusy(false);
      return setMsg(problemTitle(body));
    }
    await forgetDevice();
    setToken(null);
    try {
      localStorage.clear();
      sessionStorage.clear();
    } catch {
      /* ignore */
    }
    window.location.replace(`/deleted?job=${encodeURIComponent(body.deletion_job_id)}`);
  }

  return (
    <section className="card" aria-label="Your data">
      <h2>Your data</h2>
      <p className="note">Download everything we hold about you, or delete your account. Deleting destroys the key that protects your data, so it can never be read again. It can&rsquo;t be undone.</p>
      <button type="button" className="button ghost" disabled={busy} onClick={download}>Download my data</button>
      {!deleting ? (
        <button type="button" className="button ghost" onClick={() => setDeleting(true)}>Delete my account</button>
      ) : (
        <div>
          <p>Live plans end now; inside the first 14 days we refund the days you didn&rsquo;t use, and an unspent deposit is refunded.</p>
          <label htmlFor="confirm-delete">Type &ldquo;{PHRASE}&rdquo; to confirm</label>
          <input id="confirm-delete" className="input" autoComplete="off" value={typed} onChange={(e) => setTyped(e.target.value)} />
          <button type="button" className="button" disabled={busy || typed.trim().toLowerCase() !== PHRASE} onClick={erase}>Delete everything now</button>
          <button type="button" className="ms-block" onClick={() => { setDeleting(false); setTyped(""); }}>Keep my account</button>
        </div>
      )}
      {msg && <p role="status">{msg}</p>}
    </section>
  );
}
