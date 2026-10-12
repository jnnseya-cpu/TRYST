# frontend

Next.js 16 web app. It holds no secrets and talks to the backend only over HTTP:
`/v1/*` is proxied to `IDENTITY_URL` (reports, appeals and standing to `SAFETY_URL`), so the
browser only ever calls its own origin and the CSP stays `connect-src 'self'`.

```sh
npm ci
npm run dev                  # expects the backend on :8081 and :8082
npm test                     # unit tests
npm run e2e                  # builds and starts the backend, then runs Playwright
```

| Setting | Where | Meaning |
|---|---|---|
| `IDENTITY_URL` | build time | Backend member API, e.g. `https://api.example.com` |
| `SAFETY_URL` | build time | Safety service, e.g. `https://safety.example.com` |
| `OPS_CONSOLE=1` | build time | Build the staff console instead (separate host, behind Google Identity-Aware Proxy) |

Brand assets come from `../shared/brand` at build time (`scripts/sync-brand.mjs`).
Deploying: [docs/runbooks/go-live.md](../docs/runbooks/go-live.md).
