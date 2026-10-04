import type { NextConfig } from "next";

// App origin security headers (docs/spec/04_Frontend.md §2, §3; FR-076, NFR-09).
// No third-party scripts, no trackers; the app origin is never indexed.
const csp = [
  "default-src 'self'",
  "script-src 'self' 'unsafe-inline'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "connect-src 'self'",
  "frame-ancestors 'none'",
  "base-uri 'self'",
  "form-action 'self'",
].join("; ");

// Same-origin API: /v1/* is proxied to identity-svc, so the CSP can stay connect-src 'self'.
const IDENTITY_URL = process.env.IDENTITY_URL ?? "http://localhost:8081";

const nextConfig: NextConfig = {
  poweredByHeader: false,
  async rewrites() {
    return [{ source: "/v1/:path*", destination: `${IDENTITY_URL}/v1/:path*` }];
  },
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "Content-Security-Policy", value: csp },
          { key: "X-Robots-Tag", value: "noindex, nofollow, noarchive" },
          { key: "Referrer-Policy", value: "no-referrer" },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Permissions-Policy", value: "geolocation=(self), camera=(self), microphone=(), interest-cohort=()" },
          { key: "Cache-Control", value: "no-store" },
        ],
      },
    ];
  },
};

export default nextConfig;
