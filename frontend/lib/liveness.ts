import { api, problemTitle } from "@/lib/api";

// The live face check (B2) runs on the provider's own page, in a new tab, so the sign-in or
// step-up in this tab keeps its place. The provider sends the member to /liveness-done, which
// tells this tab over a same-origin channel; the server then checks the provider session it
// opened itself. The browser never handles the capture or its result.
export const LIVENESS_CHANNEL = "tryst-liveness";

/** Opens the hosted check. Call straight from the button's click so the tab isn't blocked. */
export async function openLiveCheck(loginId: string): Promise<{ ok: true; url: string; opened: boolean } | { ok: false; msg: string }> {
  const tab = window.open("", "_blank");
  const r = await api<{ url: string }>("/v1/auth/login/liveness/begin", { method: "POST", body: { login_id: loginId } });
  if (r.status !== 200) {
    tab?.close();
    return { ok: false, msg: problemTitle(r.data) };
  }
  if (tab) {
    tab.opener = null; // the provider's page gets no handle on this one
    tab.location.href = r.data.url;
  }
  return { ok: true, url: r.data.url, opened: !!tab };
}

/** Resolves when the hosted check reports it has finished (or the member says so). */
export function whenLiveCheckDone(): { done: Promise<void>; cancel: () => void } {
  let ch: BroadcastChannel | null = null;
  let resolve!: () => void;
  const done = new Promise<void>((res) => { resolve = res; });
  if (typeof BroadcastChannel !== "undefined") {
    ch = new BroadcastChannel(LIVENESS_CHANNEL);
    ch.onmessage = () => { ch?.close(); resolve(); };
  }
  return { done, cancel: () => ch?.close() };
}
