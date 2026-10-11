"use client";

import { useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { api, getToken, setToken } from "@/lib/api";
import { forgetDevice } from "@/lib/e2ee";
import { DataRights } from "./DataRights";
import { Standing } from "./Standing";

type Me = { session_kind: string; surface: string; tier: string; passkeys: number; expires_at: string };

export default function Account() {
  const [me, setMe] = useState<Me | null>(null);
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    api<Me>("/v1/me", { token: getToken() }).then((r) => {
      if (r.status === 200) setMe(r.data);
      setChecked(true);
    });
  }, []);

  async function signOut() {
    await api("/v1/auth/logout", { method: "POST", token: getToken() });
    setToken(null);
    window.location.assign("/");
  }

  async function burn() {
    await api("/v1/burn", { method: "POST", token: getToken() });
    await forgetDevice();
    setToken(null);
    try {
      localStorage.clear();
      sessionStorage.clear();
    } catch {
      /* ignore */
    }
    window.location.replace("/");
  }

  if (!checked) return <Shell><p>Loading…</p></Shell>;
  if (!me) {
    return (
      <Shell>
        <h1>Signed out</h1>
        <a className="button" href="/sign-in">Sign in</a>
      </Shell>
    );
  }
  return (
    <Shell>
      <p className="eyebrow">Your account</p>
      <h1>Signed in</h1>
      <section className="card" data-testid="session">
        <p>Signed in with two checks ({me.surface === "desktop" ? "computer" : "phone"}).</p>
        <p className="note">Passkeys registered: {me.passkeys} · Verification: {me.tier}</p>
      </section>
      <Standing />
      <a className="button" href="/tonight" style={{ marginBottom: "0.8rem" }}>Free tonight?</a>
      <a className="button ghost" href="/messages" style={{ marginBottom: "0.8rem" }}>Messages</a>
      <a className="button ghost" href="/profile" style={{ marginBottom: "0.8rem" }}>Your profile and photos</a>
      <a className="button ghost" href="/billing" style={{ marginBottom: "0.8rem" }}>Plans and Keys</a>
      <button type="button" className="button ghost" onClick={signOut}>Sign out</button>
      <p />
      <button type="button" className="button ghost" onClick={burn}>Burn: sign out everywhere and wipe this device</button>
      <DataRights />
    </Shell>
  );
}
