/* eslint-disable @next/next/no-img-element */
import type { Metadata, Viewport } from "next";
import "./brand-tokens.css";
import "./globals.css";
import { QuickExit } from "@/components/QuickExit";
import { SITE_NAME, SITE_URL, TAGLINE } from "@/lib/site";
import { JoinButton } from "@/components/JoinButton";
import { TOPICS } from "@/lib/posts";

export const metadata: Metadata = {
  metadataBase: new URL(SITE_URL),
  icons: { icon: "/brand/tryst-logo-192.png", apple: "/brand/tryst-logo-180.png" },
  title: { default: `${SITE_NAME}: ${TAGLINE}`, template: `%s · ${SITE_NAME}` },
  description: "How TRYST works, feature by feature: discreet introductions for verified adults, with the privacy and safety tools behind them explained plainly.",
  referrer: "no-referrer",
  alternates: { types: { "application/rss+xml": "/feed.xml" } },
  openGraph: { siteName: SITE_NAME, type: "website", locale: "en_GB" },
};

export const viewport: Viewport = { themeColor: "#0B0607", colorScheme: "dark" };

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en-GB">
      <body>
        <QuickExit />
        <header className="site-head">
          <a href="/" className="brand" aria-label="TRYST home">
            <img src="/brand/tryst-logo-96.png" alt="" width={44} height={44} />
            <span>TRYST</span>
          </a>
          <nav aria-label="Topics">
            {Object.entries(TOPICS).map(([k, t]) => (
              <a key={k} href={`/topics/${k}/`}>{t.name}</a>
            ))}
          </nav>
        </header>
        <main>{children}</main>
        <footer className="site-foot">
          <p className="tag">{TAGLINE}</p>
          <p>For verified adults over 18 in the UK. Verified membership is free.</p>
          <p><JoinButton /></p>
          <p className="note">This site sets no cookies and runs no trackers. Press Esc twice, or Leave, to exit instantly; it can&rsquo;t clear your browser history, so use a private window if that matters.</p>
          <p className="note"><a href="/feed.xml">RSS</a></p>
        </footer>
      </body>
    </html>
  );
}
