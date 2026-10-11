// Thread client: device registration, sending and reading end-to-end encrypted messages.
import { api, getToken } from "./api";
import { deviceIdentity, open, seal, type Envelope, type PublicDevice } from "./e2ee";

export type ThreadView = { id: string; name: string; age: number | null; state: "normal" | "frozen" | "closed"; retention_days: number; last_seq: number; last_at: string; created: string };
type DeviceKey = { id: string; jwk: JsonWebKey };
export type Payload = { v: 1; type: "text"; text: string } | { v: 1; type: "meet"; place: string; time: string; note?: string };

export type Shown = { seq: number; mine: boolean; at: string; payload: Payload | null };

export async function registerDevice(): Promise<string> {
  const me = await deviceIdentity();
  const pub = await crypto.subtle.exportKey("jwk", me.keys.publicKey);
  await api("/v1/devices/key", { method: "PUT", token: getToken(), body: { device_id: me.id, public_jwk: { kty: pub.kty, crv: pub.crv, x: pub.x, y: pub.y } } });
  return me.id;
}

export async function threadDetail(id: string) {
  return api<{ thread: ThreadView; my_devices: DeviceKey[] | null; their_devices: DeviceKey[] | null }>(`/v1/threads/${id}`, { token: getToken() });
}

export async function sendPayload(threadId: string, payload: Payload): Promise<{ ok: boolean; error?: string }> {
  const me = await deviceIdentity();
  const d = await threadDetail(threadId);
  if (d.status !== 200) return { ok: false, error: "This conversation isn't available." };
  const devices: PublicDevice[] = [...(d.data.their_devices ?? []), ...(d.data.my_devices ?? [])].map((x) => ({ id: x.id, jwk: x.jwk }));
  if (!(d.data.their_devices ?? []).length) return { ok: false, error: "They haven't opened their messages on a device yet. Try again shortly." };
  const clientId = "m_" + crypto.randomUUID();
  const envelopes = await seal(threadId, clientId, JSON.stringify(payload), devices);
  const r = await api(`/v1/threads/${threadId}/messages`, { method: "POST", token: getToken(), body: { client_id: clientId, sender_device: me.id, envelopes } });
  if (r.status === 201 || r.status === 200) return { ok: true };
  return { ok: false, error: (r.data as { title?: string }).title ?? "Not sent. Try again." };
}

export async function readSince(threadId: string, after: number): Promise<Shown[]> {
  const me = await deviceIdentity();
  const r = await api<{ messages: { seq: number; client_id: string; mine: boolean; at: string; envelope: Envelope | null }[] }>(
    `/v1/threads/${threadId}/messages?device_id=${encodeURIComponent(me.id)}&after=${after}`,
    { token: getToken() },
  );
  if (r.status !== 200) return [];
  const out: Shown[] = [];
  for (const m of r.data.messages) {
    let payload: Payload | null = null;
    if (m.envelope) {
      try {
        payload = JSON.parse(await open(threadId, m.client_id, m.envelope, me.keys)) as Payload;
      } catch {
        payload = null;
      }
    }
    out.push({ seq: m.seq, mine: m.mine, at: m.at, payload });
  }
  return out;
}
