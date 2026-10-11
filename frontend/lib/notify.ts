// Notices and notification settings (FR-026, FR-119).
import { api, getToken } from "./api";

export type Notice = { id: string; kind: string; text: string; link?: string; at: string; read: boolean };
export type NotifySettings = { push: boolean; text: string; devices: number; vapid_public_key: string; push_available: boolean };

export const notices = () => api<{ notices: Notice[]; unread: number }>("/v1/notices", { token: getToken() });
export const markRead = () => api("/v1/notices/read", { method: "POST", token: getToken() });
export const settings = () => api<NotifySettings>("/v1/notifications/settings", { token: getToken() });
export const saveSettings = (push: boolean, text: string) =>
  api<NotifySettings>("/v1/notifications/settings", { method: "PUT", token: getToken(), body: { push, text } });

/** Saves the wording where the service worker can read it (it can't see page storage). */
export async function saveWording(text: string) {
  if (!("caches" in window)) return;
  const c = await caches.open("notify-settings");
  await c.put("/__notify_text", new Response(text || "You have an update"));
}

function keyBytes(b64url: string): Uint8Array<ArrayBuffer> {
  const pad = "=".repeat((4 - (b64url.length % 4)) % 4);
  const raw = atob((b64url + pad).replace(/-/g, "+").replace(/_/g, "/"));
  const out = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}

/** Turns push on for this device: permission, service worker, subscription. */
export async function enablePush(vapidKey: string): Promise<string | null> {
  if (!("serviceWorker" in navigator) || !("PushManager" in window)) return "This browser can't show notifications.";
  const perm = await Notification.requestPermission();
  if (perm !== "granted") return "Notifications are blocked in this browser's settings.";
  const reg = await navigator.serviceWorker.register("/sw.js");
  await navigator.serviceWorker.ready;
  const sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: keyBytes(vapidKey) });
  const r = await api("/v1/notifications/push", { method: "PUT", token: getToken(), body: { endpoint: sub.endpoint } });
  return r.status === 200 ? null : "This browser's push service isn't supported.";
}

export async function disablePushHere() {
  if (!("serviceWorker" in navigator)) return;
  const reg = await navigator.serviceWorker.getRegistration("/sw.js");
  const sub = await reg?.pushManager.getSubscription();
  if (sub) {
    await api("/v1/notifications/push", { method: "DELETE", token: getToken(), body: { endpoint: sub.endpoint } });
    await sub.unsubscribe();
  }
}
