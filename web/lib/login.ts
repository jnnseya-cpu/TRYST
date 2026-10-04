// Client mirror of the login rules in docs/spec/02_Shared_Contracts.md §3.3 (FR-057, FR-058).
// The server decides the surface and the required factors; the client only renders steps.

export type Surface = "mobile_app" | "mobile_pwa" | "desktop";
export type Factor = "B1" | "B2" | "F1" | "F2";

export const FACTOR_COPY: Record<Factor, { title: string; body: string }> = {
  B1: { title: "Unlock with your phone", body: "Use your phone's fingerprint or face unlock." },
  B2: { title: "Live face check", body: "A quick live check that it's really you. Nothing is stored by TRYST." },
  F1: { title: "Use your passkey", body: "Confirm with Windows Hello, Touch ID or your security key." },
  F2: { title: "Approve on your phone", body: "Open TRYST on your registered phone, choose Approve sign-in and scan this code." },
};

export function requiredFactors(surface: Surface): [Factor, Factor] {
  return surface === "desktop" ? ["F1", "F2"] : ["B1", "B2"];
}

/** UI hint only; the server re-decides from attestation and authenticator properties. */
export function guessSurface(userAgent: string, standalone: boolean): Surface {
  const mobile = /Android|iPhone|iPad|iPod|Mobile/i.test(userAgent);
  if (!mobile) return "desktop";
  return standalone ? "mobile_pwa" : "mobile_pwa";
}

export type LoginProgress = { surface: Surface; passed: Factor[] };

/** Next factor to show, or null when both are done (a session may then be issued). */
export function nextFactor(p: LoginProgress): Factor | null {
  const req = requiredFactors(p.surface);
  return p.passed.length >= req.length ? null : req[p.passed.length];
}

/** Errors never reveal which factor failed. */
export const GENERIC_FAILURE = "We couldn't sign you in. Please try again.";
