// Reports, standing and appeals (member side) and the moderator console client.
import { api, getToken } from "./api";

export type Category = { key: string; label: string };

export async function categories(): Promise<Category[]> {
  const r = await api<{ categories: Category[] }>("/v1/reports/categories", { token: getToken() });
  return r.status === 200 ? r.data.categories : [];
}

/** Sends a report. `disclosed` holds only the messages the member chose to share. */
export async function report(threadId: string, category: string, disclosed: string[], note: string) {
  return api<{ reference: string; message: string }>("/v1/reports", {
    method: "POST",
    token: getToken(),
    body: { thread_id: threadId, category, disclosed, note },
  });
}

export type StandingDecision = {
  decision_id: string;
  action: string;
  reason: string;
  at: string;
  appeal?: { filed: string; due: string; outcome?: string };
};

export async function standing() {
  return api<{ standing: string; under_review: boolean; decisions: StandingDecision[] }>("/v1/me/standing", { token: getToken() });
}

export async function appeal(decisionId: string, text: string) {
  return api<{ due: string; message: string }>("/v1/appeals", { method: "POST", token: getToken(), body: { decision_id: decisionId, text } });
}

export const actionText: Record<string, string> = {
  warn: "Warning",
  shadow_limit: "Limited: you aren't shown in Tonight",
  suspend: "Suspended",
  permanent: "Removed",
};

// --- console (staff) ---

const STAFF_KEY = "tryst.staff";

export function staffToken(): string | null {
  try {
    return sessionStorage.getItem(STAFF_KEY);
  } catch {
    return null;
  }
}

export function setStaffToken(t: string | null) {
  try {
    if (t) sessionStorage.setItem(STAFF_KEY, t);
    else sessionStorage.removeItem(STAFF_KEY);
  } catch {
    /* private mode */
  }
}

/** Console request. In production the staff sign-in proxy adds identity; dev uses a token. */
export async function ops<T = Record<string, unknown>>(path: string, opts: { method?: string; body?: unknown } = {}) {
  const t = staffToken();
  return api<T>(path, { ...opts, headers: t ? { Authorization: `Staff ${t}` } : {} });
}
