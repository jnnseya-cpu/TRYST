# Cycle 1: 24 October 2026

Start `8ff2274`, end `492819c`. Environment: the development container (Go services in dev
mode with PostgreSQL 16, Next.js production builds, Chromium). No deployed environment
exists yet.

## Fixed

| Defect | Sev | What | Commit |
|---|---|---|---|
| D19, D20 | P1, P2 | Next.js critical advisories; Go crypto/x509 | 8ff2274 |
| D14 to D17 | P2–P3 | Obsolete AWS path removed; unwired city gating no longer claimed; dev stand-in pages 404 outside dev; site refuses a production build without its addresses; restore runbook matches Terraform | 52ed8ee |
| D10 | P2 | Landing page 27 s on slow 3G → under 8 s | cf74891 |
| D1, D2, D5 | P1, P1, P2 | Phone sign-in face check; duress reaches responders; UK time in alerts | 24c658a |
| D3, D4, D6–D9, D18 | P1, P2, P3 | Landing page truth; check-ins survive restarts; email-only copy; deletion copy; retention sweep; abuse-image cases; copy corrections | 83ee2b6 |
| D11, D12, D13 | P2, P3, P2 | Check-in PIN race (failing CI); double-send; Protobuf CI path | 492819c |

## Newly broken during the cycle

* The dev-checkout page became server-rendered and its button could be clicked before the
  page was ready (found by the full browser suite, fixed in 52ed8ee before commit).
* None outstanding.

## Tests added

`TestLivenessUsesTheServerHeldProviderSession`, `TestLivenessStartOpensAHostedCapture`,
duress assertions in `TestDuressLooksNormalAndCannotEndTheMeet`, `TestMeetWindowOnTheUKClock`,
`TestCheckInsSurviveARestart`, `TestShorterRetentionAndTheSweepReachEveryMessage`,
`TestKnownAbuseImageOpensASafetyCase`, `TestKnownAbuseImageSignalIsASuspectedMinorCase`,
`e2e/a11y.spec.ts` (13 screens at 360 px, slow 3G).

## Score and gates

Score 62 / 100 (not measured before this cycle). Gates: 1 pass, 4 partial, 5 blocked, and payments N/A for a free-only launch (blocked if TRYST+ is sold).
See [../README.md](../README.md) §3 and §1.

## Next cycle starts when

Any of H1 (identity provider), H2 (ARC or a decision), H3 (Google Cloud projects) is done.
