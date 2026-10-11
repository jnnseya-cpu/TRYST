"use client";

import { useState } from "react";
import { startRegistration } from "@simplewebauthn/browser";
import { Action } from "@/components/Action";
import { StepUp } from "@/components/StepUp";
import { api, getToken, problemTitle } from "@/lib/api";

type Device = { id: string; surface: string; kind: string; added: string; revoked: boolean };

const KIND: Record<string, string> = { platform_uv: "Passkey on this kind of device", platform_biometric: "Phone app", roaming_key: "Security key" };

/** Devices (passkeys) on the account. Viewing, adding and removing need step-up. */
export function Devices() {
  const [list, setList] = useState<Device[] | null>(null);
  const [need, setNeed] = useState<null | (() => void)>(null);
  const [msg, setMsg] = useState("");

  async function guarded(run: () => Promise<{ status: number; data: unknown }>, then: (data: unknown) => void) {
    setMsg("");
    const r = await run();
    if (r.status === 403 && (r.data as { code?: string }).code === "step_up_required") {
      setNeed(() => async () => {
        setNeed(null);
        const again = await run();
        if (again.status >= 300) return setMsg(problemTitle(again.data));
        then(again.data);
      });
      return;
    }
    if (r.status >= 300) return setMsg(problemTitle(r.data));
    then(r.data);
  }

  const load = () => guarded(() => api<{ devices: Device[] }>("/v1/auth/devices", { token: getToken() }), (d) => setList((d as { devices: Device[] }).devices));

  const remove = (id: string) => guarded(() => api(`/v1/auth/devices/${encodeURIComponent(id)}`, { method: "DELETE", token: getToken() }), () => load());

  const add = (attachment: "platform" | "cross-platform") =>
    guarded(
      () => api("/v1/auth/passkeys/register/begin", { method: "POST", token: getToken(), body: { attachment } }),
      async (opts) => {
        try {
          const cred = await startRegistration({ optionsJSON: (opts as { publicKey: Parameters<typeof startRegistration>[0]["optionsJSON"] }).publicKey });
          const fin = await api("/v1/auth/passkeys/register/finish", { method: "POST", token: getToken(), body: cred });
          if (fin.status !== 201) return setMsg(problemTitle(fin.data));
          setMsg("Device added. Your other devices have been told.");
          load();
        } catch {
          setMsg("Adding the device was cancelled.");
        }
      },
    );

  return (
    <section className="card" aria-label="Your devices">
      <h2>Your devices</h2>
      <p className="note">The passkeys that can sign in to this account. Seeing or changing them needs both sign-in checks again.</p>
      {need ? (
        <StepUp reason="Your devices are protected." onDone={need} />
      ) : list === null ? (
        <Action className="button ghost" onClick={load}>Show my devices</Action>
      ) : (
        <>
          <ul data-testid="devices">
            {list.filter((d) => !d.revoked).map((d) => (
              <li key={d.id} data-testid="device">
                {KIND[d.kind] ?? d.kind} · {d.surface === "desktop" ? "computer" : "phone"} · added {new Date(d.added).toLocaleDateString("en-GB")}{" "}
                <button type="button" className="ms-block" onClick={() => remove(d.id)}>Remove</button>
              </li>
            ))}
          </ul>
          <Action className="button ghost" onClick={() => add("platform")}>Add a passkey on this device</Action>
          <Action className="button ghost" onClick={() => add("cross-platform")}>Add a security key</Action>
        </>
      )}
      {msg && <p role="status">{msg}</p>}
    </section>
  );
}
