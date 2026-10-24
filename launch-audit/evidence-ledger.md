# Evidence ledger

Cycle 1, 24 October 2026, in the development container (no deployed environment exists).
Release candidate: `492819c` on `claude/gracious-rubin-bp7b7q`. Statuses: PASS · FAIL ·
PARTIAL · BLOCKED · NOT TESTED · N/A. A PASS here means the check ran and passed **in this
environment**; nothing below was run against a deployed system.

## Build and code

| Test | How | Result | Status |
|---|---|---|---|
| E1 Clean install | `npm ci` in `frontend/` and `site/` | Installs from lockfiles | PASS |
| E2 Dependency audit | `npm audit` both apps | 0 vulnerabilities (after Next.js 16.4.2) | PASS |
| E3 Go vulnerabilities | govulncheck | GO-2026-6809 found, fixed (go1.26.10); re-scan blocked by the container's network, runs in CI | PARTIAL |
| E4 Static analysis | `go vet`, staticcheck, gosec, gofmt | Clean | PASS |
| E5 Secret scan, full history | gitleaks v8.28.0 over all 53 commits | No leaks | PASS |
| E6 Go tests | `go test -race ./...` with PostgreSQL 16 | 31 packages pass | PASS |
| E7 Restore drill | `TestRestoreDrill` with PostgreSQL | Passes (6.5 s) | PASS (local PostgreSQL, not Cloud SQL) |
| E8 Web unit and types | `tsc --noEmit`, vitest | 122 pass | PASS |
| E9 Browser end-to-end | Playwright, Chromium, virtual authenticators, real Go services in dev mode | 24/24 pass, including the new hosted face check | PASS (dev stand-ins for vendors) |
| E10 Site | content rules, typecheck, static build | 211 pass, build OK | PASS |
| E11 Contracts | Redocly lint, Protobuf compile, spec check | Valid; D=69 | PASS |
| E12 Terraform | `validate` both stacks (provider mirror) | Valid (AWS stack since removed) | PASS |
| E13 CI on GitHub | Actions runs for this branch | integration and shared were **red** (D11, D13); on `492819c` backend, frontend, shared and integration (browser tests, image scan, Terraform) all pass ([run 40121556041](https://github.com/jnnseya-cpu/TRYST/actions/runs/40121556041)) | PASS |

## Adversarial and non-functional

| Test | How | Result | Status |
|---|---|---|---|
| E14 API fuzz, unauthenticated | 4,353 requests across every route in `tryst.v1.yaml`: malformed, null, deep and 3 MB bodies, injection strings, traversal ids, forged and oversized tokens | 0 server errors, 0 stack traces or internals leaked; 1 flag (`POST /v1/burn` uniform 204) is by design | PASS |
| E15 API fuzz, authenticated | — | Needs passkey sessions; covered only by the contract-driven route-authorisation tests in Go | NOT TESTED |
| E16 Rate limits | `hey` 5,000 sign-up code requests, one contact | 5 accepted, 4,995 refused (429) | PASS |
| E17 Load baseline | `hey` 20,000 health requests, 100 concurrent, local | 17.9k req/s, p99 16 ms, 22 MB memory | PARTIAL (local only, not a load test of the real VM) |
| E18 Accessibility | axe (WCAG 2.2 AA rules) on 5 public and 8 member screens at 360 px | 0 violations, no sideways scroll | PASS (automated rules only) |
| E19 Slow connection | Landing page first heading on emulated slow 3G (400 kbps, 400 ms) | 27 s → under 8 s after D10 | PASS |
| E20 Truth audit | Every claim in 25 site posts and the app's copy traced to code (independent agent, then verified by hand) | 3 P1, 6 P2, 9 P3 mismatches; all fixed or copy corrected | PASS after fixes |
| E21 Deletion | Browser test: export with step-up, deletion, signed proof | Passes | PASS (dev) |

## Not possible here (need H1 to H7)

| Test | Status | Blocked on |
|---|---|---|
| Real identity provider: sign-up, V1, V2, phone sign-in, recovery | BLOCKED | H1 |
| Safety line: missed check-in, duress, Panic reaching an ARC | BLOCKED | H2 |
| Deployed smoke test, TLS, DNS, IAP console sign-in | BLOCKED | H3, H4 |
| Deploy, rollback, redeploy | BLOCKED | H3 |
| Cloud SQL restore drill with timings | BLOCKED | H3 |
| Alerts firing to a person | BLOCKED | H3 |
| Load, stress, soak on the real VM | BLOCKED | H3 |
| Email deliverability (SPF, DKIM, DMARC) | BLOCKED | H5 |
| Payments in Stripe test mode end to end | BLOCKED (optional at launch) | H9 |
| Penetration test | BLOCKED | H7 |
| Manual screen-reader and keyboard-only pass; real phones (Safari iOS, Android Chrome) | NOT TESTED | A person with devices |
