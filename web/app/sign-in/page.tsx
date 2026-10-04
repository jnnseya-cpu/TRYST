"use client";

import { useEffect, useState } from "react";
import { FACTOR_COPY, GENERIC_FAILURE, guessSurface, nextFactor, requiredFactors, type Factor, type Surface } from "@/lib/login";

// Two-factor sign-in (02 §3.3). Factor ceremonies (WebAuthn, liveness SDK, QR approval) are
// stubbed until the identity service exists in P1; the step sequence is the real one.
export default function SignIn() {
  const [surface, setSurface] = useState<Surface>("desktop");
  const [passed, setPassed] = useState<Factor[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const standalone = window.matchMedia?.("(display-mode: standalone)").matches ?? false;
    setSurface(guessSurface(navigator.userAgent, standalone));
  }, []);

  const factors = requiredFactors(surface);
  const current = nextFactor({ surface, passed });

  async function runFactor(f: Factor) {
    setError(null);
    try {
      // P1: call /v1/auth/login/{passkey|liveness|cross-device} per contracts/openapi/tryst.v1.yaml
      setPassed((p) => [...p, f]);
    } catch {
      setError(GENERIC_FAILURE);
    }
  }

  return (
    <main>
      <h1>Sign in</h1>
      <p className="note">Only the account holder can sign in. Two checks are always required.</p>
      <ol className="steps">
        {factors.map((f, i) => (
          <li key={f} aria-current={current === f ? "step" : undefined}>
            {i + 1}. {FACTOR_COPY[f].title} {passed.includes(f) ? "✓" : ""}
          </li>
        ))}
      </ol>
      {current ? (
        <section className="card">
          <h2>{FACTOR_COPY[current].title}</h2>
          <p>{FACTOR_COPY[current].body}</p>
          <button type="button" className="button" onClick={() => runFactor(current)}>
            Continue
          </button>
        </section>
      ) : (
        <section className="card" role="status">
          <h2>Both checks complete</h2>
          <p>Your session would be issued now.</p>
        </section>
      )}
      {error && <p role="alert">{error}</p>}
      <p className="note">We never sign you in with a text message or email code, and our staff cannot sign in for you.</p>
    </main>
  );
}
