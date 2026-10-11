// Discretion (02 §6.4; FR-005, FR-011, FR-046, FR-106, FR-127).
import { api, getToken } from "./api";

export type Policy = { version: number; visibility: "visible" | "incognito"; max_distance_km: number; min_counterparty_tier: "V2" | "V3"; zones: string[] };
export type Discretion = { status: "active" | "paused" | "vanished" | "locked"; policy: Policy; versions: number; shield: number };

const t = () => getToken();
export const discretion = () => api<Discretion>("/v1/discretion", { token: t() });
export const putPolicy = (p: Pick<Policy, "visibility" | "max_distance_km" | "min_counterparty_tier">) =>
  api<Discretion>("/v1/discretion/policy", { method: "PUT", token: t(), body: p });
export const putZones = (zones: string[]) => api<Discretion>("/v1/discretion/zones", { method: "PUT", token: t(), body: { zones } });
export const statusChange = (what: "pause" | "vanish" | "resume") => api<Discretion>(`/v1/account/${what}`, { method: "POST", token: t() });
export const clearShield = () => api<Discretion>("/v1/discretion/shield", { method: "DELETE", token: t() });

/** Same normalisation as the server (identity NormaliseContact). */
export function normaliseContact(c: string): string {
  c = c.trim();
  return c.includes("@") ? c.toLowerCase() : c.replace(/[\s\-()]/g, "");
}

/** The contact never leaves the device: only its SHA-256 is sent, and the server keys it. */
export async function shieldDigest(contact: string): Promise<string> {
  const d = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(normaliseContact(contact)));
  return Array.from(new Uint8Array(d), (b) => b.toString(16).padStart(2, "0")).join("");
}

export const addShield = async (contact: string) =>
  api<Discretion>("/v1/discretion/shield", { method: "POST", token: t(), body: { add: [await shieldDigest(contact)] } });
