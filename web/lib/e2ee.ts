// End-to-end encryption for threads (FR-114, D-51). Runs only on the member's device.
//
// Each device holds an ECDH P-256 key pair; the private key is created non-extractable and
// stays in this browser's IndexedDB. A message is encrypted separately for every device in
// the thread with a fresh ephemeral key: ECDH -> HKDF-SHA256 -> AES-256-GCM. The associated
// data binds each envelope to its thread, message and device, so envelopes can't be moved.
// The server only ever sees ciphertext.

export type PublicDevice = { id: string; jwk: JsonWebKey };
export type Envelope = { device_id: string; epk: string; iv: string; ct: string };

const subtle = () => globalThis.crypto.subtle;
const enc = new TextEncoder();
const dec = new TextDecoder();

export function b64(buf: ArrayBuffer | Uint8Array): string {
  const bytes = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s);
}

export function unb64(s: string): Uint8Array<ArrayBuffer> {
  const bin = atob(s);
  const out = new Uint8Array(new ArrayBuffer(bin.length));
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

export async function generateDeviceKeys(): Promise<CryptoKeyPair> {
  return subtle().generateKey({ name: "ECDH", namedCurve: "P-256" }, false, ["deriveBits"]) as Promise<CryptoKeyPair>;
}

export async function publicJwk(keys: CryptoKeyPair): Promise<JsonWebKey> {
  const jwk = await subtle().exportKey("jwk", keys.publicKey);
  return { kty: jwk.kty, crv: jwk.crv, x: jwk.x, y: jwk.y };
}

function aad(threadId: string, clientId: string, deviceId: string): Uint8Array<ArrayBuffer> {
  return new Uint8Array(enc.encode(`tryst/msg/v1|${threadId}|${clientId}|${deviceId}`));
}

async function messageKey(shared: ArrayBuffer, salt: Uint8Array<ArrayBuffer>, usage: KeyUsage): Promise<CryptoKey> {
  const ikm = await subtle().importKey("raw", shared, "HKDF", false, ["deriveKey"]);
  return subtle().deriveKey(
    { name: "HKDF", hash: "SHA-256", salt, info: enc.encode("tryst/msg/v1") },
    ikm,
    { name: "AES-GCM", length: 256 },
    false,
    [usage],
  );
}

/** Encrypts one message for every device given. */
export async function seal(threadId: string, clientId: string, plaintext: string, devices: PublicDevice[]): Promise<Envelope[]> {
  const out: Envelope[] = [];
  for (const d of devices) {
    const recipient = await subtle().importKey("jwk", { ...d.jwk, ext: true }, { name: "ECDH", namedCurve: "P-256" }, false, []);
    const eph = (await subtle().generateKey({ name: "ECDH", namedCurve: "P-256" }, true, ["deriveBits"])) as CryptoKeyPair;
    const epk = new Uint8Array(await subtle().exportKey("raw", eph.publicKey));
    const shared = await subtle().deriveBits({ name: "ECDH", public: recipient }, eph.privateKey, 256);
    const key = await messageKey(shared, epk, "encrypt");
    const iv = crypto.getRandomValues(new Uint8Array(12));
    const ct = await subtle().encrypt({ name: "AES-GCM", iv, additionalData: aad(threadId, clientId, d.id) }, key, new Uint8Array(enc.encode(plaintext)));
    out.push({ device_id: d.id, epk: b64(epk), iv: b64(iv), ct: b64(ct) });
  }
  return out;
}

/** Decrypts this device's envelope; throws if it was not meant for this device. */
export async function open(threadId: string, clientId: string, env: Envelope, myKeys: CryptoKeyPair): Promise<string> {
  const epk = unb64(env.epk);
  const ephPub = await subtle().importKey("raw", epk, { name: "ECDH", namedCurve: "P-256" }, false, []);
  const shared = await subtle().deriveBits({ name: "ECDH", public: ephPub }, myKeys.privateKey, 256);
  const key = await messageKey(shared, epk, "decrypt");
  const pt = await subtle().decrypt({ name: "AES-GCM", iv: unb64(env.iv), additionalData: aad(threadId, clientId, env.device_id) }, key, unb64(env.ct));
  return dec.decode(pt);
}

// ---- device key storage (IndexedDB; CryptoKey objects are stored without exporting them)

const DB = "tryst-device";
const STORE = "keys";

function idb(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB, 1);
    req.onupgradeneeded = () => req.result.createObjectStore(STORE);
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

export type DeviceIdentity = { id: string; keys: CryptoKeyPair };

/** Returns this device's keys, creating them on first use. */
export async function deviceIdentity(): Promise<DeviceIdentity> {
  const db = await idb();
  const existing = await new Promise<DeviceIdentity | undefined>((resolve) => {
    const r = db.transaction(STORE).objectStore(STORE).get("device");
    r.onsuccess = () => resolve(r.result as DeviceIdentity | undefined);
    r.onerror = () => resolve(undefined);
  });
  if (existing) return existing;
  const id = "dev_" + b64(crypto.getRandomValues(new Uint8Array(12))).replace(/[^a-zA-Z0-9]/g, "");
  const created: DeviceIdentity = { id, keys: await generateDeviceKeys() };
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE, "readwrite");
    tx.objectStore(STORE).put(created, "device");
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
  });
  return created;
}

/** Burn and sign-out: forget this device's keys (old messages become unreadable here). */
export async function forgetDevice(): Promise<void> {
  try {
    indexedDB.deleteDatabase(DB);
  } catch {
    /* ignore */
  }
}
