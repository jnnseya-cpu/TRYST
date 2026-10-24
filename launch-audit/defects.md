# Defects

One record per defect found in cycle 1 (24 October 2026). Severity: P0 catastrophic · P1
critical · P2 high · P3 medium · P4 low. Every fixed defect has a regression test unless
noted, and was retested with the full suite (Go with PostgreSQL and the race detector, 122
web unit tests, 24 browser tests, 211 site tests).

| ID | Sev | Area | Defect | Root cause | Fix | Test | Commit | Status |
|---|---|---|---|---|---|---|---|---|
| D1 | P1 | Sign-in | Phone sign-in and step-up couldn't complete in production: everyone whose passkey is on a phone would be locked out | The app never obtained face-check evidence; the provider adapter refuses empty evidence; the dev stand-in approves everything, so every test passed | Hosted provider session: `POST /v1/auth/login/liveness/begin` opens it, the server keeps the provider session, the app opens the capture in a new tab, `/liveness-done` signals completion, `POST /v1/auth/login/liveness` checks that server-held session once | `TestLivenessUsesTheServerHeldProviderSession`, `TestLivenessStartOpensAHostedCapture`, 24 browser tests through the new flow | 24c658a | Fixed; **real-provider retest pending** (H1) |
| D2 | P1 | Meet safety | A duress check-in reached the safety line as "missed_check_in" and the trusted contact got the non-urgent wording | `duress := false` was never set | Reason `duress`, urgent wording | `TestDuressLooksNormalAndCannotEndTheMeet` (fails without the fix) | 24c658a | Fixed |
| D3 | P1 | Truth | Landing page sold what doesn't exist: negotiating agents, a £9.99 deposit, ENVOY/DUO/GHOST/Keys plans, app disguise and decoy PIN, screenshot blocking, fingerprint-change sign-out, TRYST+ extras | Copy written for the full roadmap, not the launch catalogue (D-60) | Rewritten to Verified (free) and TRYST+ only, real features only | Truth audit re-run by grep; browser suite | 83ee2b6 | Fixed |
| D4 | P2 | Meet safety | A deploy or restart mid-meet silently dropped every running check-in plan | Plans were memory-only | Plans saved (`meet.State`/`Resume`), trusted contact sealed under the member's key, resumed at start, deleted when ended and on erasure | `TestCheckInsSurviveARestart` | 83ee2b6 | Fixed |
| D5 | P2 | Meet safety | Meet windows in alerts were UTC with no label: an hour early in summer | `Format("15:04")` on UTC times | UK time, labelled, zone data compiled in | `TestMeetWindowOnTheUKClock` | 24c658a | Fixed |
| D6 | P2 | Novice | Join, lock and recovery invited a mobile number that production refuses | Copy predates email-only launch (D-67) | "Email address", email keyboard | Browser suite | 83ee2b6 | Fixed |
| D7 | P2 | Truth/privacy | Deletion copy said all data is cryptographically erased, including in backups; only photos are | Copy overstated `store/vault.go` coverage | Post states what happens to each kind of record; at most one key-database dump kept, never over 7 days | Site content tests | 83ee2b6 | Fixed (copy); wider sealing is a follow-up (R5) |
| D8 | P2 | Privacy | Expired messages were deleted only when someone read the thread; shortening retention didn't reach messages already sent | Purge on read only; expiry fixed at send | Hourly `SweepMessages`; expiry is the earlier of send-time expiry and current retention | `TestShorterRetentionAndTheSweepReachEveryMessage` | 83ee2b6 | Fixed |
| D9 | P2 | Child safety | A known child-abuse image hash match was refused and forgotten: no case, no route to an NCA report | Upload path only returned 422 | Opens a suspected-minor case (15-minute deadline, interim suspension, mandatory NCA report) | `TestKnownAbuseImageOpensASafetyCase`, `TestKnownAbuseImageSignalIsASuspectedMinorCase` | 83ee2b6 | Fixed |
| D10 | P2 | Performance | Landing page took 27 s to show its heading on a slow mobile connection | The 1.1 MB original logo loaded on every page of both sites | Scaled copies (58 KB WebP); original untouched | `e2e/a11y.spec.ts` (< 8 s on slow 3G) | cf74891 | Fixed |
| D11 | P2 | Novice/meet | On a slow connection a late check-in reply wiped the PIN typed next, so "home safe" failed | PIN cleared on reply, not on send | Cleared as sent; buttons wait for the answer | `meet.spec.ts` 5/5 repeated (failed twice in CI before) | 492819c | Fixed |
| D12 | P3 | Messages | Double tap sent a message twice; a late reply wiped text typed next | Same class as D11 (pattern sweep) | Send once; clear as sent; restore on failure | Browser suite | 492819c | Fixed |
| D13 | P2 | CI | The shared-contracts workflow always failed | Protobuf include path relative to the wrong directory | `-Ishared/contracts/proto` | Ran locally | 492819c | Fixed; CI confirmation pending |
| D14 | P2 | Truth | App claimed city-by-city matching and a waitlist that aren't wired | `backend/internal/launch` not imported by any service | Copy removed (D-69) | — | 52ed8ee | Fixed (copy); wiring is a founder decision (H8) |
| D15 | P3 | Exposure | Development stand-in pages (fake ID check, fake checkout) were reachable in production builds | Pages had no environment guard | 404 unless `TRYST_ENV=dev` | Browser suite (dev) | 52ed8ee | Fixed |
| D16 | P3 | Ops | Public site would publish `tryst.example` URLs if its addresses weren't set on Vercel | Fallback default | Vercel production build fails without them | Ran with `VERCEL_ENV=production` | 52ed8ee | Fixed |
| D17 | P3 | Docs | Restore runbook said 35 days of point-in-time recovery; Terraform has 7 days PITR and 30 daily backups | Doc drift | Runbook matches `sql.tf` | — | 52ed8ee | Fixed |
| D18 | P3 | Truth | Eight smaller over-claims (quick exit and Back, consent checks, policy versions, sign-in history, ID number handling, Tonight cap, couple device, partner shield phones) | Copy ahead of code | Copy corrected | Site tests | 83ee2b6 | Fixed |
| D19 | P1 | Security (supply chain) | Next.js 16.3.8 had critical advisories (GHSA-2464-fhr4-w9xw, GHSA-673q-mjm6-r5xr) | Dependency age | 16.4.2 in app and site; `npm audit` 0 | Builds and tests | 8ff2274 | Fixed |
| D20 | P2 | Security | GO-2026-6809 (crypto/x509) reachable via the SMTP TLS dial | Toolchain go1.26.9 | go1.26.10 | govulncheck in CI | 8ff2274 | Fixed |

## Known, not fixed (non-blocking, recorded)

| ID | Sev | Defect | Why not fixed now |
|---|---|---|---|
| R1 | P3 | Tonight's 10-interest cap resets if a member stops and starts again | Copy now says "up to 10 at a time"; a per-evening count is a product choice |
| R2 | P3 | Couple-link "device" is a client-supplied id | Two separately verified accounts are still required; copy corrected |
| R3 | P3 | Consent-chain monitor re-verifies the copy loaded at start, not the database | Copy corrected; reading the database each time is a follow-up |
| R4 | P3 | Quick exit replaces only the current history entry | Browser limitation; copy says so |
| R5 | P2 | Only photos are sealed under the per-member key; other records rely on deletion plus 30-day backup expiry and re-erasure on restore | Sealing every record type is a larger change; copy now accurate |
