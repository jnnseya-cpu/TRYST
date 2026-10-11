// Consent ledger (02 §5.2; FR-040, FR-126). Grants and withdrawals are appended, never edited.
import { api, getToken } from "./api";

export type ConsentEntry = { id: string; kind: string; granted: boolean; scope?: string; at: string };
export type Consents = { entries: ConsentEntry[]; current: Record<string, boolean>; chain_ok: boolean };

export const consents = () => api<Consents>("/v1/consents", { token: getToken() });
export const setConsent = (kind: string, granted: boolean) =>
  api<ConsentEntry>("/v1/consents", { method: "POST", token: getToken(), body: { kind, granted } });

/** What each consent the member controls directly means, in plain words. */
export const CONSENT_COPY: Record<string, { title: string; body: string; withdraw: string }> = {
  art9_processing: {
    title: "Your profile in discovery",
    body: "Your profile says things about your sex life and who you're looking for. We need your explicit consent to process that and to show you to other members.",
    withdraw: "Withdrawing takes you out of Tonight and discovery straight away. Your account stays.",
  },
  biometric_login: {
    title: "Face checks",
    body: "Live face checks at our verification provider, to verify you and to sign you in on a phone. TRYST never holds your face data.",
    withdraw: "Withdrawing stops new face checks. Sign in on a computer with a security key instead.",
  },
  safety_training_use: {
    title: "Help train safety tools",
    body: "Reports you make may be used, without your name, to improve our safety tools. Optional; it never affects how your report is handled.",
    withdraw: "Withdrawing stops any future use.",
  },
};
