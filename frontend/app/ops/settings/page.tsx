"use client";

import { useCallback, useEffect, useState } from "react";
import { Shell } from "@/components/Shell";
import { problemTitle } from "@/lib/api";
import { ops } from "@/lib/safety";
import "../../profile/profile.css";
import "../ops.css";

type Setting = { name: string; label: string; secret: boolean; required: boolean; help: string; set: boolean; updated?: string };
type Group = { key: string; label: string; required: boolean; note: string; ready: boolean; missing: number; settings: Setting[] };

function when(iso?: string) {
  if (!iso || iso.startsWith("0001")) return "";
  return new Date(iso).toLocaleString([], { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
}

/**
 * Vendor keys (safety leads only): what each vendor needs, what's set, and a place to paste
 * each key. Values are write-only: once saved, a key is never shown again, here or anywhere.
 */
export default function VendorKeys() {
  const [groups, setGroups] = useState<Group[] | null>(null);
  const [ready, setReady] = useState(false);
  const [denied, setDenied] = useState("");
  const [values, setValues] = useState<Record<string, string>>({});
  const [notes, setNotes] = useState<Record<string, { ok: boolean; text: string }>>({});
  const [busy, setBusy] = useState("");
  const [applied, setApplied] = useState("");

  const load = useCallback(async () => {
    const r = await ops<{ groups: Group[]; launch_ready: boolean }>("/ops/v1/settings/vendor-keys");
    if (r.status !== 200) return setDenied(r.status === 401 ? "Sign in to the console first." : problemTitle(r.data));
    setDenied("");
    setGroups(r.data.groups);
    setReady(r.data.launch_ready);
  }, []);
  useEffect(() => { load(); }, [load]);

  async function save(name: string) {
    if (busy) return;
    const value = values[name] ?? "";
    setBusy(name);
    const r = await ops<{ message: string }>(`/ops/v1/settings/vendor-keys/${name}`, { method: "PUT", body: { value } });
    setBusy("");
    if (r.status !== 200) return setNotes((n) => ({ ...n, [name]: { ok: false, text: problemTitle(r.data) } }));
    setValues((v) => ({ ...v, [name]: "" }));
    setNotes((n) => ({ ...n, [name]: { ok: true, text: "Saved." } }));
    load();
  }

  async function apply() {
    if (!confirm("Apply the saved keys now? TRYST restarts within about two minutes and is offline for around 30 seconds.")) return;
    const r = await ops<{ message: string }>("/ops/v1/settings/vendor-keys/apply", { method: "POST" });
    setApplied(r.status === 202 ? r.data.message : problemTitle(r.data));
  }

  if (denied) {
    return (
      <Shell>
        <p className="eyebrow">Settings</p>
        <h1>Vendor keys</h1>
        <p role="alert">{denied}</p>
        <a className="button ghost" href="/ops">Back to the console</a>
      </Shell>
    );
  }
  if (!groups) return <Shell><p>Loading…</p></Shell>;
  return (
    <Shell wide>
      <div className="op-bar">
        <div>
          <p className="eyebrow">Settings · safety leads only</p>
          <h1>Vendor keys</h1>
        </div>
        <a className="ms-block" href="/ops">Case queue</a>
      </div>
      <p className="note">
        Paste each key from the vendor&rsquo;s dashboard and press Save. Keys are stored in Google Secret Manager and are
        never shown again. Every change is recorded in the audit log. When you&rsquo;ve entered what you have, press Apply.
      </p>
      <p className="ms-banner" role="status" data-testid="launch-ready">
        {ready ? "Everything launch needs is set." : "Launch needs every group marked Required."}
      </p>

      {groups.map((g) => (
        <section key={g.key} className="card" data-testid={`group-${g.key}`}>
          <div className="op-bar">
            <h2>{g.label}</h2>
            <span className="note">{g.ready ? "Ready" : `${g.missing} missing`}{g.required ? " · Required" : " · Optional"}</span>
          </div>
          <p className="note">{g.note}</p>
          {g.settings.map((s) => (
            <div key={s.name} className="vk-row" data-testid={`setting-${s.name}`}>
              <label htmlFor={`vk-${s.name}`}>
                {s.label} <span className="note">{s.set ? `Set${when(s.updated) ? ` · ${when(s.updated)}` : ""}` : "Not set"}</span>
              </label>
              <div className="op-row">
                <input id={`vk-${s.name}`} className="input" type={s.secret ? "password" : "text"} autoComplete="off" spellCheck={false}
                  placeholder={s.set ? "Enter a new value to replace it" : ""} value={values[s.name] ?? ""}
                  onChange={(e) => setValues((v) => ({ ...v, [s.name]: e.target.value }))} />
                <button type="button" className="button inline" disabled={!values[s.name] || busy === s.name} onClick={() => save(s.name)}>Save</button>
              </div>
              <p className="note">{s.help}</p>
              {notes[s.name] && <p role={notes[s.name].ok ? "status" : "alert"} className="note">{notes[s.name].text}</p>}
            </div>
          ))}
        </section>
      ))}

      <section className="card">
        <h2>Apply</h2>
        <p className="note">Saved keys take effect when TRYST restarts. Apply restarts it within about two minutes (around 30 seconds offline).</p>
        <button type="button" className="button" onClick={apply}>Apply keys now</button>
        {applied && <p role="status" data-testid="applied">{applied}</p>}
      </section>
    </Shell>
  );
}
