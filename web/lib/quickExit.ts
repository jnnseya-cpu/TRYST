// Quick exit (FR-029): hide content and leave in < 500 ms, replacing history so Back does
// not return here. It cannot hide network or device history, and onboarding says so.

export const NEUTRAL_EXIT_URL = "https://www.bbc.co.uk/weather";

export interface ExitTarget {
  location: { replace(url: string): void };
  sessionStorage?: { clear(): void };
  document?: { body?: { style: { visibility: string } } };
}

export function quickExit(w: ExitTarget, url: string = NEUTRAL_EXIT_URL): void {
  if (w.document?.body) w.document.body.style.visibility = "hidden";
  try {
    w.sessionStorage?.clear();
  } catch {
    /* storage may be unavailable; leaving matters more */
  }
  w.location.replace(url);
}
