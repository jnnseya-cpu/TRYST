"use client";

import { useState } from "react";
import { Shell } from "@/components/Shell";
import { Action } from "@/components/Action";
import { api, problemTitle } from "@/lib/api";

/**
 * Emergency lock (FR-064). No sign-in: the contact on the account and the one-time
 * emergency code lock it from any browser. The answer is the same whatever was entered.
 */
export default function Lock() {
  const [contact, setContact] = useState("");
  const [code, setCode] = useState("");
  const [done, setDone] = useState(false);
  const [msg, setMsg] = useState("");

  async function lock() {
    setMsg("");
    const r = await api("/v1/emergency-lock", { method: "POST", body: { contact, code } });
    if (r.status !== 202) return setMsg(problemTitle(r.data));
    setDone(true);
  }

  return (
    <Shell>
      <p className="eyebrow">Emergency</p>
      <h1>Lock an account</h1>
      {done ? (
        <section className="card" role="status" data-testid="lock-done">
          <p>If the details matched, the account is now locked: every device is signed out, nobody can sign in, and the profile and photos are hidden.</p>
          <p>To unlock it later, <a href="/recover">recover the account</a>. You&rsquo;ll get a new emergency code then.</p>
        </section>
      ) : (
        <section className="card">
          <p>Use this if your phone has been taken or someone else can get into your account. You need the emergency code you were given when you joined.</p>
          <label htmlFor="contact">Email or mobile number on the account</label>
          <input id="contact" className="input" autoComplete="off" value={contact} onChange={(e) => setContact(e.target.value)} />
          <label htmlFor="code">Emergency code</label>
          <input id="code" className="input" autoComplete="off" value={code} onChange={(e) => setCode(e.target.value)} />
          <Action className="button" onClick={lock}>Lock the account</Action>
        </section>
      )}
      {msg && <p role="alert">{msg}</p>}
    </Shell>
  );
}
