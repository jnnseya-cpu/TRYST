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
const SAFETY_URL = process.env.SAFETY_URL ?? "http://localhost:8082";

const nextConfig: NextConfig = {
  poweredByHeader: false,
  output: "standalone", // container image (web/Dockerfile)
  async rewrites() {
    return [
      // Reports, appeals and standing are served by the standalone safety service (D-59).
      { source: "/v1/reports", destination: `${SAFETY_URL}/v1/reports` },
      { source: "/v1/reports/:path*", destination: `${SAFETY_URL}/v1/reports/:path*` },
      { source: "/v1/appeals", destination: `${SAFETY_URL}/v1/appeals` },
      { source: "/v1/me/standing", destination: `${SAFETY_URL}/v1/me/standing` },
      { source: "/v1/:path*", destination: `${IDENTITY_URL}/v1/:path*` },
      // Moderator console API. Production serves it on a separate staff-only host behind the
      // company sign-in proxy; the member site never routes it.
      ...(process.env.OPS_CONSOLE === "1"
        ? [
            { source: "/ops/v1/vouchers", destination: `${IDENTITY_URL}/ops/v1/vouchers` },
            { source: "/ops/v1/:path*", destination: `${SAFETY_URL}/ops/v1/:path*` },
          ]
        : []),
    ];
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
