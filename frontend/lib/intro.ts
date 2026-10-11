// Introductions and couples (02 §5.3–5.6; FR-128).
import { api, getToken } from "./api";

export type Vector = { version?: number; identity: string; seeking: string[]; intent_shape: string[]; hard_limits: string[]; interests: string[] };
export type Vocab = { identity: string[]; seeking: string[]; intent_shape: string[]; hard_limits: string[]; interests: string[] };
export type Card = {
  handle: string; display_name: string; age: number | null; area: string; distance: string; couple: boolean;
  shared_intent: string[]; shared_interests: string[]; photos: number; explanation_codes: string[];
  paid_label: boolean; exploration: boolean; saved: boolean; intent_sent: boolean;
};
export type HandshakeView = { id: string; state: string; your_decision?: string; partner_decision?: string; thread_id?: string; brief?: Partial<Card> & { handle: string } };
export type Intents = { inbound: (Card & { id: string })[]; outbound: { id: string; to: string; state: string; keys: number; via: string }[]; handshakes: HandshakeView[]; free_left: number; unlimited: boolean };

const t = () => getToken();
export const vector = () => api<{ vector: Vector | null; history: number; vocab: Vocab }>("/v1/intake/vector", { token: t() });
export const confirmVector = (v: Vector) => api<{ vector: Vector }>("/v1/intake/vector/confirm", { method: "POST", token: t(), body: v });
export const slate = () => api<{ cards: Card[]; free_left: number; unlimited: boolean }>("/v1/slate", { token: t() });
export const sendIntent = (handle: string) => api<{ state: string; handshake?: string; free_left: number }>("/v1/intent", { method: "POST", token: t(), body: { target: handle } });
export const mark = (kind: "pass" | "save", handle: string) => api(`/v1/${kind}`, { method: "POST", token: t(), body: { handle } });
export const intents = () => api<Intents>("/v1/intents", { token: t() });
export const reply = (id: string, decision: "accept" | "decline") => api(`/v1/intents/${id}/reply`, { method: "POST", token: t(), body: { decision } });
export const decide = (id: string, decision: "accept" | "decline") => api<HandshakeView>(`/v1/handshake/${id}/decision`, { method: "POST", token: t(), body: { decision } });
export const envoy = () => api<{ proposals: { card: Card; rationale: string[] }[] }>("/v1/envoy/proposals", { token: t() });
export const approve = (handle: string) => api<{ state: string }>(`/v1/envoy/proposals/${handle}/approve`, { method: "POST", token: t() });

export const LABELS: Record<string, string> = {
  woman: "Woman", man: "Man", nonbinary: "Non-binary",
  women: "Women", men: "Men", couples: "Couples",
  one_off: "One-off", recurring: "Recurring", ongoing: "Ongoing", exploratory: "Exploring", third: "Joining a couple", couple: "As a couple",
  dinner: "Dinner", drinks: "Drinks", dancing: "Dancing", art: "Art", music: "Music", outdoors: "Outdoors", travel: "Travel",
  wellness: "Wellness", kink_curious: "Kink-curious", slow_burn: "Slow burn", conversation: "Conversation", spontaneous: "Spontaneous",
  intent_match: "You want the same kind of thing", shared_interests: "You share interests", nearby: "Close by",
  same_city: "In your city", both_verified: "Both verified",
};
export const label = (k: string) => LABELS[k] ?? k;

/** A stable, random id for this browser (couple co-signing needs two different devices). */
export function deviceId(): string {
  try {
    let id = localStorage.getItem("tryst.device");
    if (!id) {
      id = crypto.randomUUID();
      localStorage.setItem("tryst.device", id);
    }
    return id;
  } catch {
    return crypto.randomUUID();
  }
}
