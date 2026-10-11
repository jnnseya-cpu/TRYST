"use client";

import { useEffect, useState } from "react";
import { problemTitle } from "@/lib/api";
import { disablePushHere, enablePush, saveSettings, saveWording, settings, type NotifySettings } from "@/lib/notify";

/** Notification settings: off by default; when on, pushes show only the member's own words. */
export function Notifications() {
  const [s, setS] = useState<NotifySettings | null>(null);
  const [text, setText] = useState("");
  const [msg, setMsg] = useState("");

  useEffect(() => {
    settings().then((r) => {
      if (r.status === 200) {
        setS(r.data);
        setText(r.data.text);
      }
    });
  }, []);

  async function save(push: boolean) {
    setMsg("");
    const r = await saveSettings(push, text);
    if (r.status !== 200) return setMsg(problemTitle(r.data));
    await saveWording(r.data.text);
    let err: string | null = null;
    if (push && !s?.push && r.data.vapid_public_key) {
      err = await enablePush(r.data.vapid_public_key);
      if (err) await saveSettings(false, text); // nothing half-on
    }
    if (!push) await disablePushHere();
    const fresh = await settings();
    if (fresh.status === 200) setS(fresh.data);
    setMsg(err ?? (push ? "Notifications are on for this device." : "Notifications are off."));
  }

  if (!s) return null;
  return (
    <section className="card" aria-label="Notifications" data-testid="notify-settings">
      <h2>Notifications</h2>
      <p className="note">Off unless you turn them on. A notification never shows a name, photo or message, only the words you choose below.</p>
      <label htmlFor="notify-text">Words to show</label>
      <input id="notify-text" className="input" maxLength={40} value={text} onChange={(e) => setText(e.target.value)} />
      {s.push ? (
        <>
          <p>On{s.devices ? ` for ${s.devices} device${s.devices === 1 ? "" : "s"}` : ""}.</p>
          <button type="button" className="button ghost" onClick={() => save(true)}>Save words</button>
          <button type="button" className="button ghost" onClick={() => save(false)}>Turn notifications off</button>
        </>
      ) : (
        <button type="button" className="button ghost" disabled={!s.push_available} onClick={() => save(true)}>Turn notifications on</button>
      )}
      {msg && <p role="status">{msg}</p>}
    </section>
  );
}
