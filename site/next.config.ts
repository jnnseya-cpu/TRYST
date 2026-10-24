import type { NextConfig } from "next";

// The public website and blog: fully static (HTML files), so it can be served by Vercel or
// any static host with no server and no member data (FR-075). Security headers for the host
// are in vercel.json.
// A production deploy on Vercel must know its real addresses, or the canonical links, sitemap
// and feed would point at the placeholder domain.
if (process.env.VERCEL_ENV === "production") {
  for (const name of ["NEXT_PUBLIC_SITE_URL", "NEXT_PUBLIC_APP_URL"]) {
    if (!process.env[name]) throw new Error(`${name} must be set for the production site (Vercel → Settings → Environment Variables)`);
  }
}

const nextConfig: NextConfig = {
  output: "export",
  trailingSlash: true,
  poweredByHeader: false,
  images: { unoptimized: true },
};

export default nextConfig;
