import type { NextConfig } from "next";

// The public website and blog: fully static (HTML files), so it can be served by Vercel or
// any static host with no server and no member data (FR-075). Security headers for the host
// are in vercel.json.
const nextConfig: NextConfig = {
  output: "export",
  trailingSlash: true,
  poweredByHeader: false,
  images: { unoptimized: true },
};

export default nextConfig;
