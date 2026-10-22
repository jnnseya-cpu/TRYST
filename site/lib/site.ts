// Where things live. Set at build time (Vercel project settings → Environment Variables).
export const SITE_URL = (process.env.NEXT_PUBLIC_SITE_URL ?? "https://www.tryst.example").replace(/\/$/, "");
export const APP_URL = (process.env.NEXT_PUBLIC_APP_URL ?? "https://tryst.example").replace(/\/$/, "");
export const SITE_NAME = "TRYST";
export const TAGLINE = "Private chemistry. Intelligent discretion.";
