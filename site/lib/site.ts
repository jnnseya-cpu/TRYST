// Where things live. Set at build time (Vercel project settings → Environment Variables).
export const SITE_URL = (process.env.NEXT_PUBLIC_SITE_URL ?? "https://www.tryst.example").replace(/\/$/, "");
export const APP_URL = (process.env.NEXT_PUBLIC_APP_URL ?? "https://tryst.example").replace(/\/$/, "");
export const SITE_NAME = "TRYST";
export const TAGLINE = "Private chemistry. Intelligent discretion.";
// Launch switch: until the app takes members, every "join" button says it's opening soon
// instead of sending people to a sign-up that isn't open. Set NEXT_PUBLIC_APP_OPEN=true in
// Vercel and redeploy at launch.
export const APP_OPEN = process.env.NEXT_PUBLIC_APP_OPEN === "true";
