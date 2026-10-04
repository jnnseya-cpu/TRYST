import type { Metadata, Viewport } from "next";
import "./brand-tokens.css";
import "./globals.css";
import { QuickExit } from "@/components/QuickExit";

export const metadata: Metadata = {
  title: "TRYST",
  description: "Private chemistry. Intelligent discretion.",
  robots: { index: false, follow: false, nocache: true }, // app origin is never indexed (FR-076)
  manifest: "/manifest.webmanifest",
  referrer: "no-referrer",
};

export const viewport: Viewport = { themeColor: "#0B0607", colorScheme: "dark" };

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en-GB">
      <body>
        <QuickExit />
        {children}
      </body>
    </html>
  );
}
