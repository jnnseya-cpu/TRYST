# Security testing

Internal engineering record (FR-122, D-58). What runs on every change, what it found when
first run on 11 October 2026, and what is still needed before launch.

## Automated, on every change (CI gates)

| Check | Tool | Scope |
|---|---|---|
| Known vulnerabilities, reachability-aware | govulncheck | Go code and standard library |
| Static analysis | staticcheck, gosec (justified exceptions are annotated in place) | Go |
| Dependency advisories | npm audit (high and above fail) | Web app |
| Image vulnerabilities | Trivy (high and critical fail) | Both container images |
| Route authorisation | `TestEveryContractRouteRefusesSignedOutCallers` | Every operation in both OpenAPI contracts, called signed out and with forged tokens: no 2xx except four public-by-design routes, no 5xx, `no-store`, `nosniff` and a `default-src 'none'; sandbox` CSP on every API answer |
| Web headers | `e2e/security.spec.ts` | CSP with no third-party origins, no framing, `noindex`, `no-referrer`, no `X-Powered-By`; member data unreachable signed out |
| Restore safety | `TestRestoreDrill` | A restored backup can't bring back a deleted member, in either service's databases |
| Safety service surface | `cmd/safety-svc` route test, `TestProductionRefusesUnsafeConfig` | Every console and safety member operation refuses unauthenticated and forged callers; production refuses dev tokens, a missing roster, missing staff sign-in or missing internal secrets |
| Staff sign-in | `internal/staffauth` tests | Proxy token verification: wrong key, tampered signature, algorithm swap, expired, not-yet-valid, wrong issuer or audience, off-roster email; roles never taken from the request; key fetching rate-limited |
| Production setup | `admin preflight` (both services), `TestPreflight`, `TestProvisionScript` | Roster can run the two-person rule and independent appeals, proxy keys reachable, hardware key enforced, internal secrets distinct and rotatable, services reach each other; secrets generated and stored without being printed |
| Internal APIs | `internal/svcauth` and cross-service tests | Signature bound to caller, time, method, path and body; wrong secret, unknown caller, stale or replayed-elsewhere calls refused |
| Feature security | service tests and browser tests | Crypto-erasure, E2EE relay of ciphertext only, staff-only console, two-person removal, signed webhooks, SSRF-proof push endpoints, fail-closed vendors |

## First run: findings and fixes

| Finding | Severity | Fix |
|---|---|---|
| 28 Go standard-library vulnerabilities reachable from our code (net/http, crypto/x509, crypto/tls, net/url…) on Go 1.26.0 | High | Toolchain pinned to Go 1.26.9; govulncheck now reports 0 reachable |
| `golang.org/x/crypto` advisory GO-2026-5932 | — | Not reachable from our code; no fixed version yet. Re-check weekly |
| Vitest path traversal and Tinypool prototype pollution | Critical (dev only) | Vitest 5.0.3 |
| npm's own bundled packages in the web runtime image (brace-expansion, pacote, sigstore, ip-address…) | High | Package managers removed from the runtime image; Trivy now clean |
| `/v1/reports/categories` answered signed-out callers | Low | Requires a session |
| Rate-limit log line could be forged with an encoded newline in the path | Low | Path quoted |
| Invisible Unicode characters in source literals (Trojan Source risk) | Low | Replaced with escapes; repository scanned clean for bidi and zero-width characters |
| Safety alerts sent while holding the service lock: a slow alarm-receiving centre would stall every request | High (availability of safety features) | Delivered asynchronously with retry and a paging metric (D-57) |
| A child-safety message blocked on the sender's device named the recipient as the subject | High | The sender is the subject |
| No secrets found in the repository | — | Trivy secret scan |

## Still needed before launch (cannot be done from here)

* An independent penetration test by a CREST-accredited firm, covering the web app, API,
  staff console host, Stripe and vendor integrations, and the native apps when they exist.
  Scope and rules of engagement: [pentest-scope.md](pentest-scope.md); it runs on staging
  ([../runbooks/staging.md](../runbooks/staging.md)).
* A review of the cryptography (message envelopes, crypto-erasure, deletion proofs) by a
  specialist.
* Cloud configuration review of the deployed AWS account (the Terraform is validated, not
  deployed).
* A bug-bounty or vulnerability disclosure programme with a `security.txt`.
