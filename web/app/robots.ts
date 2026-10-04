import type { MetadataRoute } from "next";

// App origin: nothing is indexable (FR-076). The public marketing site lives on its own host.
export default function robots(): MetadataRoute.Robots {
  return { rules: [{ userAgent: "*", disallow: "/" }] };
}
