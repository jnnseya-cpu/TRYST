# Deploying TRYST

Internal engineering runbook (D-57). Infrastructure: `deploy/terraform` (AWS, London).
Images: `backend/Dockerfile` (API, distroless, non-root) and `web/Dockerfile` (Next.js
standalone, non-root). Local stack: `deploy/docker-compose.yml`.

## What production refuses to run without

The API exits at start-up, with a message naming the missing setting, unless all of these
are present (see the header of `backend/cmd/identity-svc/main.go`):

| Setting | Why |
|---|---|
| `DATABASE_URL_IDENTITY`, `_PROFILE`, `_KEYS`, `_SAFETY`, `_COMMERCE`, `_LEDGER` | One database per domain; the ledger apart from the rest (restores) |
| `DATA_KEK` | Wraps every member data key (crypto-erasure) |
| `DSR_SIGNING_KEY` | Signs deletion completion proofs |
| `CONTACT_PEPPER` (≥ 32 bytes) | Contact hashes |
| `POSTMARK_SERVER_TOKEN` and/or `TWILIO_*` | Sign-up codes must be deliverable |
| `ARC_URL`, `ARC_SECRET` (+ a message channel) | Meet safety can't run without the safety line (D-48) |
| `SAFETY_INTERNAL_URL`, `INTERNAL_SECRET_SAFETY`, `INTERNAL_SECRET_PLATFORM` | The API reaches the standalone safety service (D-59) |

The safety service (`--entrypoint /safety-svc`, same image) refuses to start without
`DATABASE_URL_SAFETY`, `PLATFORM_INTERNAL_URL`, both `INTERNAL_SECRET_*` (the same values as
the API), staff sign-in (`STAFF_ACCESS_ISSUER`, `STAFF_ACCESS_AUDIENCE`,
`STAFF_ACCESS_JWKS_URL`, or `STAFF_TRUSTED_PROXIES`) and `STAFF_ROSTER`. Dev staff tokens
are refused in production.

### Staff console host

1. Create a Cloudflare Access application for the console host (`ops_host`), limited to the
   T&S group, with hardware-key MFA. Note its AUD tag and team domain.
2. Set `staff_access` in Terraform (issuer `https://<team>.cloudflareaccess.com`, the AUD
   tag, JWKS `https://<team>.cloudflareaccess.com/cdn-cgi/access/certs`).
3. Put the roster in `tryst-prod/STAFF_ROSTER` (`email:moderator` or `email:lead,moderator`).
   Removing someone from the roster removes their console access at once, even if Access
   still lets them in.
4. Build the console image with `OPS_CONSOLE=1` (`ops_image`).

Without these, features degrade safely rather than stop the service: no `STRIPE_*` →
purchases answer 503; no `VAPID_PRIVATE_KEY` → no push (in-app notices still work); no
photo vendors → every photo waits for a person; no `LIVENESS_URL` → login face checks fail
closed.

## First deployment

1. `terraform apply` with `api_image`, `web_image` (pinned by digest) and `certificate_arn`.
2. Put every secret in Secrets Manager (`tryst-prod/<NAME>`); values never go in Terraform.
3. Create the databases on the three instances: identity, profile, safety, commerce on
   `main`; keys on `keys`; ledger on `ledger`.
4. Stripe: create Prices, set `STRIPE_PRICE_<KEY>`, point the webhook at
   `/v1/billing/stripe/webhook`, enable renewal reminder emails at least 7 days ahead, and
   set a neutral statement descriptor.
5. Run the restore drill once against the empty stack (below) before taking traffic.

## Every release

CI builds both images and runs Go (including the restore drill against PostgreSQL), web
unit, and browser tests. Deploy by changing the image digests; ECS rolls back
automatically if the new tasks fail their health checks. The API drains for up to 25 s on
stop so in-flight safety alerts finish.

## Metrics

Each API task serves `/metrics` on port 9090, reachable only from the monitoring security
group. Alert rules: `deploy/monitoring/alerts.yml`; what to do: `oncall.md`.
