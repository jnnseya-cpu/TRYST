// Identity and age verification (02 §3.1; D-61).
import { api, getToken } from "./api";

export type VerifyStatus = { tier: string; liveness_done: boolean; conduct_accepted: boolean; conduct_version: string; id_checked: boolean; age_check: string; refused: string };

export const status = () => api<VerifyStatus>("/v1/assurance/status", { token: getToken() });
export const start = (kind: "liveness" | "age") => api<{ session_id: string; url: string }>("/v1/assurance/session", { method: "POST", token: getToken(), body: { kind } });
export const acceptConduct = (version: string) => api<VerifyStatus>("/v1/attestations/conduct", { method: "POST", token: getToken(), body: { version, accept: true } });

export const ageMessage: Record<string, string> = {
  failed: "The ID check didn't go through. You can try again.",
  duplicate: "This ID is already linked to a TRYST account. Each person may hold one account: recover that one instead.",
  document_mismatch: "Use the same ID document you verified with before.",
  refused: "TRYST is for adults only.",
  not_eligible: "We can't verify this account.",
};
