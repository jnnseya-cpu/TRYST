# Launch audit: final report

Working files: [evidence-ledger.md](evidence-ledger.md) · [defects.md](defects.md) ·
[truth-audit.md](truth-audit.md) · [novice-walkthrough.md](novice-walkthrough.md) ·
[human-actions.md](human-actions.md) · [cycle-reports/](cycle-reports/).

## 1. Executive verdict

**NO-GO.** Confidence score **62 / 100**. Release candidate `492819c` (branch
`claude/gracious-rubin-bp7b7q`). Environment: development container only; nothing is
deployed. Test period: 24 October 2026, 1 cycle. Risk level: high until H1 to H7 are done.

The code is in much better shape than at the start of the cycle. Twenty defects were found
and fixed with tests, including three P1s: phone sign-in that could never have worked in
production, duress check-ins reported as ordinary missed check-ins, and a landing page that
sold plans and features that don't exist. Every public claim is now either real or removed.

But a platform that has never been deployed, never verified a real person through a real
provider, never been tested by an outside party, and has no signed legal basis for
processing sex-life data, can't be approved for members. What's left is not mostly code:
it's contracts, accounts, a pen test and legal sign-off.

## 2. Immediate launch blockers

| ID | Sev | Area | Blocker | Required | Status |
|---|---|---|---|---|---|
| H1 | P0 | Verification | No identity and age provider; production won't start, and the new phone face check is untested against a real provider | Contract plus sandbox keys, then a gateway mapping (~1 day) | Open (human) |
| H2 | P0 | Safety | No alarm-receiving centre; production won't start (D-48) | Contract, or approve making the safety line optional | Open (human) |
| H3 | P0 | Deployment | Nothing deployed: no smoke, restore, alert, load or rollback test on the real system | Google Cloud projects; `setup.sh` | Open (human) |
| H6 | P0 | Legal | No counsel opinion, DPIA, Online Safety Act assessments or final Terms | Counsel and DPO | Open (human) |
| H7 | P1 | Security | No independent penetration test | CREST firm on staging | Open (human) |
| H4, H5 | P1 | Operations | No staff roster or security-key group; mailbox not authenticated (SPF, DKIM, DMARC) | Founder | Open (human) |

No code-level P0 or P1 defect is open.

## 3. Gate results

| Gate | Status | Evidence | Remaining risk |
|---|---|---|---|
| 1 Build integrity | PASS | E1–E13; all CI workflows green on `492819c` | — |
| 2 Truth and fitness | PARTIAL | Truth audit: all claims real or removed | No timed walkthrough by a person on real phones |
| 3 Critical functionality | PARTIAL | 24/24 browser journeys, dev stand-ins | Real provider, ARC, email and Stripe flows untested |
| 4 Security | BLOCKED | Scans clean, fuzz clean, no secrets in history | No pen test; authenticated fuzzing not done |
| 5 Data integrity | PARTIAL | Restore drill on PostgreSQL; transactional stores; erasure tests | No Cloud SQL restore; only photos crypto-erased (R5) |
| 6 Financial integrity | N/A free-only / BLOCKED with TRYST+ | Stripe adapter and webhook tests | Stripe not approved or configured |
| 7 Performance | BLOCKED | Local baseline only | No load test on the VM |
| 8 Reliability | PARTIAL | Fail-closed vendors, saved check-ins, outbox, single-instance lock | No failure injection on deployed system |
| 9 Observability | BLOCKED | Alert rules and uptime checks defined | Never fired to a person |
| 10 Privacy and compliance | BLOCKED | Consent ledger, deletion with proof, retention sweep | DPIA and privacy notice unsigned |
| 11 Operational readiness | BLOCKED | Runbooks written | Roster, on-call and account ownership unconfirmed |

### Score

| Category | Weight | Score | Why |
|---|---|---|---|
| Architecture and build | 8 | 80 | Deterministic builds, CI; never deployed |
| Truth and fitness | 10 | 75 | All claims real or removed; no human walkthrough |
| Core functional reliability | 14 | 70 | 24 journeys pass with stand-in vendors |
| Novice usability | 6 | 55 | Automated phone checks only |
| Authentication and authorisation | 11 | 75 | Route-auth tests, fuzz; real face check untested |
| Security | 13 | 60 | Scans clean; no pen test |
| Data integrity and recovery | 9 | 65 | Local restore drill; R5 |
| Payments | 8 | 50 | Not configured |
| Performance | 6 | 30 | Local baseline only |
| AI safety and quality | — | N/A | No model in the running services |
| Privacy | 4 | 50 | Mechanisms built; DPIA unsigned |
| Observability | 3 | 40 | Alerts defined, never fired |
| Deploy and rollback | 2 | 10 | Never deployed |
| Accessibility and devices | 2 | 70 | Automated rules pass; no manual or real-device pass |
| **Weighted (re-normalised over 96)** | | **62** | A score never overrides a failed gate |

## 4. Truth audit summary

126 claims real. Built to make claims true: D1, D2, D4, D5, D8, D9. Removed because not true at
launch: negotiating agents, deposit, ENVOY/DUO/GHOST/Keys plans, app disguise, screenshot
blocking, fingerprint-change sign-out, TRYST+ extras, city gating and waitlist. Details:
[truth-audit.md](truth-audit.md).

## 5. Novice-user findings

Landing page 27 s → under 8 s on slow 3G; join copy matched to email-only; PIN and message
races on slow connections fixed; phone sign-in made possible. Time to value with a real person
on a real phone: **not measured** ([novice-walkthrough.md](novice-walkthrough.md)).

## 6. Coverage

Automated: Go 31 packages (race, PostgreSQL), 122 web unit, 24 browser journeys, 13 screens
for accessibility at 360 px, 211 site content checks, 4,353 fuzz requests over every route.
Not covered: deployed system, real vendors, real devices, authenticated fuzzing, screen
readers. Browser coverage: Chromium only.

## 7. Architecture and dependencies

Go `identity-svc` and `safety-svc` (one image), Next.js app and staff console, PostgreSQL
(three Cloud SQL instances), static public site on Vercel, Google Cloud VM behind a load
balancer with Cloud Armor and IAP. Single points of failure: the one VM (by design, single
instance; Cloud SQL is managed) and each single-vendor dependency (identity provider until a
second is contracted, ARC, Hostinger mailbox). Undocumented components: none found; obsolete
AWS path removed. Ownership of the Google, Vercel, Hostinger and GitHub accounts by the
company (not a person) is **unconfirmed**.

## 8. Security findings

Fixed: D19 (Next.js critical), D20 (Go x509), D15 (dev pages in production), D1 (sign-in
evidence now server-held). Clean: gosec, staticcheck, govulncheck (CI), npm audit, gitleaks
(full history), header tests, unauthenticated fuzz. Not done: independent pen test,
authenticated fuzzing, cloud configuration review.

## 9. Functional and UX findings

D1–D6, D10–D12 in [defects.md](defects.md).

## 10. Data and database findings

Restore drill passes locally; erasure re-applied after restore; check-in plans now durable;
messages swept on expiry. Open: R5 (only photos sealed per member).

## 11. Payment and financial findings

Launch can be free-only. With TRYST+: server-side prices, signed and idempotent webhooks,
refunds and cooling-off are unit-tested against a Stripe-shaped server; nothing has run in
Stripe test mode (H9, H11). Financial invariants report: not producible without Stripe.

## 12. AI and agent findings

N/A at launch: no model is called anywhere in the running services (`agents/` and `aigov` are
not deployed). The on-device message-safety checks are deterministic rules with labelled
vectors.

## 13. Performance findings

Local only: 17.9k health requests per second, p99 16 ms; sign-up codes rate-limited per
contact. Real targets (p95 < 500 ms, LCP < 2.5 s on 4G) are untested on the VM.

## 14. Observability and incident response

Defined: Google uptime checks, CPU and database disk alerts, safety-alert failure metrics,
the Hostinger outside check, on-call runbook. Not proven: no alert has reached a person.

## 15. Fixes implemented

See [defects.md](defects.md) (D1–D20: root cause, change, test, commit).

## 16. Unresolved risks

R1–R5 (accepted as P3/P2 with accurate copy); single-vendor identity provider until a second
is contracted; the single VM; account ownership unconfirmed.

## 17. Human Action List

[human-actions.md](human-actions.md): H1–H12.

## 18. Pre-launch action plan

| When | Action | Owner | Evidence of done |
|---|---|---|---|
| Before any launch | H1, H2, H3, H4, H5, H6; merge to `main` (H12) | Founder | Production starts; smoke test passes |
| Before beta | Real-provider retest of sign-up, phone sign-in, recovery; alert test; Cloud SQL restore drill; deploy and rollback drill; manual phone walkthrough | Engineering | Ledger entries move from BLOCKED to PASS |
| Before public launch | H7 pen test with fixes; load test to expected peak; R5 decision | Founder, engineering | Report; load results within targets |
| Within 7 days of launch | Daily review of alerts and the safety queue; one restore drill | Engineering, safety lead | Drill timings |
| Within 30 days | Second identity provider; seal more record types (R5); city gating decision (H8) | Founder, engineering | Ledger update |

## 19. Launch configuration (when the blockers clear)

Enabled: sign-up by email, verification, introductions, intents (3 a week free), Tonight,
messages, meets with check-ins, reports and moderation, data rights. Sold: TRYST+ only if
Stripe is ready, otherwise nothing. Disabled: SMS, ENVOY, DUO, GHOST, Keys, deposit.
Public site: `NEXT_PUBLIC_APP_OPEN=false` until the doors open. Rollback trigger: any
safety-alert delivery failure, error rate over 0.5 % for 10 minutes, or a failed smoke test →
re-run the previous deploy. Kill switch: `NEXT_PUBLIC_APP_OPEN=false` stops new sign-ups in
two minutes.

## 20. Final decision

**This release is not approved for public launch.**
