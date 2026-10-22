import { APP_OPEN, APP_URL } from "@/lib/site";

/** "Join" while the app takes members; a plain "Opening soon" label before that. */
export function JoinButton({ label = "Become a member", ghost = false }: { label?: string; ghost?: boolean }) {
  if (!APP_OPEN) return <span className="button ghost" aria-disabled="true" data-testid="opening-soon">Opening soon</span>;
  return <a className={ghost ? "button ghost" : "button"} href={`${APP_URL}/join`}>{label}</a>;
}
