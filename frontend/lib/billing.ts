// Billing client (02 §5.9).
import { api, getToken } from "./api";

export type CatalogueItem = { key: string; name: string; price_minor: number; currency: string; period?: string; keys?: number; kind: "plan" | "addon" | "keys" | "deposit"; description: string };
export type SubView = { item: string; name: string; status: string; period_end: string; cancel_at_period_end: boolean; in_cooling_off: boolean };
export type Entitlements = {
  plan: string;
  ghost: boolean;
  keys: number;
  deposit_paid: boolean;
  subscriptions: SubView[];
  catalogue: CatalogueItem[];
  payments_available: boolean;
  voucher_plan?: { plan: string; until: string };
};

export const money = (minor: number) => `£${(minor / 100).toFixed(2)}`;

export function entitlements() {
  return api<Entitlements>("/v1/entitlements", { token: getToken() });
}

export function subscribe(plan: string, preContract: boolean, startNow: boolean) {
  return api<{ checkout_url: string }>("/v1/subscription", { method: "POST", token: getToken(), body: { plan, pre_contract_ack: preContract, start_now: startNow } });
}

export function buyKeys(pack: string) {
  return api<{ checkout_url: string }>("/v1/keys/purchase", { method: "POST", token: getToken(), body: { pack } });
}

export function payDeposit() {
  return api<{ checkout_url: string }>("/v1/deposit", { method: "POST", token: getToken() });
}

export function cancel(item: string) {
  return api<{ message: string; refund_minor: number }>("/v1/subscription/cancel", { method: "POST", token: getToken(), body: { item } });
}

export function redeem(code: string) {
  return api<{ message: string }>("/v1/vouchers/redeem", { method: "POST", token: getToken(), body: { code } });
}
