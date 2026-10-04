import Link from "next/link";

/** Frame for app screens: quiet top bar, centred editorial column, footer. */
export function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="shell">
      <header className="topbar">
        <Link className="mark" href="/" aria-label="TRYST home">TRYST</Link>
        <nav aria-label="Account">
          <Link href="/sign-in">Sign in</Link>
          <Link href="/join">Join</Link>
        </nav>
      </header>
      <main className="page">{children}</main>
      <footer className="footer">
        <span>Adults only. Opening in London first.</span>
        <span>Press Esc twice, or Leave, to exit instantly.</span>
      </footer>
    </div>
  );
}
