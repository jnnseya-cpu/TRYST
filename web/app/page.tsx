/* eslint-disable @next/next/no-img-element */
// Logo is rendered exactly as supplied (no Next image optimisation or re-encoding).
export default function Home() {
  return (
    <main>
      <img className="logo" src="/brand/tryst-logo.png" alt="TRYST" width={1254} height={1254} />
      <p className="tagline">Private chemistry. Intelligent discretion.</p>
      <section className="card">
        <h2>Discretion is the product</h2>
        <p>
          Verified adults only. Every person on TRYST has passed liveness and age checks, and only you can sign in
          to your account.
        </p>
        <a className="button" href="/sign-in">
          Sign in
        </a>
      </section>
      <p className="note">
        The Exit button leaves this site instantly. It cannot hide your browser or network history — use a private
        window on shared devices.
      </p>
    </main>
  );
}
