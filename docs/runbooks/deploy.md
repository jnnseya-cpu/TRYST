# Deploying TRYST

Internal engineering runbook (D-57). Hosting: Google Cloud, London, one VM per environment
plus Cloud SQL, set up with `deploy/gcp/setup.sh` ([go-live.md](go-live.md)). This is the only
supported target (D-66).
Images: `backend/Dockerfile` (API, distroless, non-root) and `frontend/Dockerfile` (Next.js
standalone, non-root). They run on the VM from `deploy/gcp/compose.yml`. Local development:
[backend/README.md](../../backend/README.md) and [frontend/README.md](../../frontend/README.md).

## What production refuses to run without

The API exits at start-up, with a message naming the missing setting, unless all of these
are present (see the header of `backend/cmd/identity-svc/main.go`):

| Setting | Why |
|---|---|
| `DATABASE_URL_IDENTITY`, `_PROFILE`, `_KEYS`, `_SAFETY`, `_COMMERCE`, `_LEDGER` | One database per domain; the ledger apart from the rest (restores) |
| `DATA_KEK` | Wraps every member data key (crypto-erasure) |
| `DSR_SIGNING_KEY` | Signs deletion completion proofs |
| `CONTACT_PEPPER` (≥ 32 bytes) | Contact hashes |
| `SMTP_*` (the Hostinger mailbox), `POSTMARK_SERVER_TOKEN` or `TWILIO_*` | Sign-up codes must be deliverable; without `TWILIO_*` sign-up and trusted contacts are email only |
| `ARC_URL`, `ARC_SECRET` (+ a message channel) | Meet safety can't run without the safety line (D-48) |
| `SAFETY_INTERNAL_URL`, `INTERNAL_SECRET_SAFETY`, `INTERNAL_SECRET_PLATFORM` | The API reaches the standalone safety service (D-59) |
| `ASSURANCE_URL`, `ASSURANCE_SECRET`, `ASSURANCE_CALLBACK_SECRET` (≥ 32 chars) | Nobody can be verified without the identity provider (D-61) |
| `ANCHOR_PEPPER` (hex, ≥ 32 bytes) | One-way document codes: one account per ID, bans, recovery (D-61, D-62) |

Non-secret settings (`RP_ID`, `RP_ORIGINS`, `PUBLIC_URL`, `SALE_ITEMS`, `ASSURANCE_CERTS`, the
vendor URLs and `STRIPE_PRICE_<KEY>`) come from `deploy/gcp/<env>.conf`; `setup.sh` stores
them in Secret Manager as `API_CONFIG` and secrets as their own entries. `ASSURANCE_CERTS` without a current certificate for web
capture keeps every check refused (G-NG-4). Staging: [staging.md](staging.md).

The safety service (`--entrypoint /safety-svc`, same image) refuses to start without
`DATABASE_URL_SAFETY`, `PLATFORM_INTERNAL_URL`, both `INTERNAL_SECRET_*` (the same values as
the API), staff sign-in (`STAFF_ACCESS_ISSUER`, `STAFF_ACCESS_AUDIENCE`,
`STAFF_ACCESS_JWKS_URL`, or `STAFF_TRUSTED_PROXIES`) and `STAFF_ROSTER`. Dev staff tokens
are refused in production.

### Staff console host, roster and internal secrets

Do these in order. Nothing here involves putting a secret or an email address in the
repository or in Terraform.

**1. Sign-in proxy for the console host**

Recommended (only Google Cloud, D-66): **Identity-Aware Proxy** on the console's backend
service, set up as in [go-live.md](go-live.md) §2, with `STAFF_ACCESS_PROVIDER=iap` and
`STAFF_ACCESS_AUDIENCE` in the safety service's settings. Security keys are enforced by the
safety team's Google 2-Step Verification policy (IAP tokens don't report how someone signed
in, so `STAFF_ACCESS_REQUIRE_AMR` stays unset). Then go to step 3.

**3. Roster and internal secrets**

Write the roster in a file outside the repository, one person per line (`email role`, roles
`moderator` and/or `lead`; at least three people and one lead, because a permanent removal
needs two different moderators and an appeal needs a third who wasn't involved). Then:

Set `ROSTER_FILE=/secure/roster.txt` in `prod.conf` and run `deploy/gcp/setup.sh prod`. It
calls `scripts/provision-safety-secrets.sh`, which generates the two internal secrets (one
per direction), stores everything in Secret Manager as `tryst-prod-<NAME>`, never prints a
value, and never overwrites a secret that's already set. Keep the roster file off shared
drives. To change who has access, edit the list and run `setup.sh prod` again; removing
someone takes effect on the next deploy, even if IAP still lets them in.

**4. Deploy, then check**

Deploy, then on the VM (`gcloud compute ssh tryst-prod --zone europe-west2-a --tunnel-through-iap`):

```sh
cd /opt/tryst/deploy/gcp
sudo docker compose --env-file /etc/tryst/images.env exec safety /safety-svc admin preflight   # roster, IAP keys, secrets, database, link to the API
sudo docker compose --env-file /etc/tryst/images.env exec api /identity-svc admin preflight    # secrets distinct, link to the safety service
```

Both must exit 0. Preflight prints no secrets and no emails.

**5. Confirm a real sign-in**

Sign in to the console host with a team member's Google account. A person outside the
safety group must get Google's "You don't have access" page.

**Rotating the internal secrets** (yearly, or at once if one may have leaked):
`ENV=prod scripts/provision-safety-secrets.sh --rotate`, deploy (each
service accepts old and new), `--finish-rotation`, deploy again.

## First deployment

1. `deploy/gcp/setup.sh prod` ([go-live.md](go-live.md) §2): network, the three Cloud SQL
   instances and their databases, the VM, the load balancer, IAP and every secret.
2. Stripe (optional at launch): create Prices, set `STRIPE_PRICE_<KEY>`, point the webhook
   at `/v1/billing/stripe/webhook`, enable renewal reminder emails at least 7 days ahead,
   and set a neutral statement descriptor.
3. Run the restore drill once against the empty stack before taking traffic
   ([restore.md](restore.md)).

## Every release

CI builds the images and runs Go (including the restore drill against PostgreSQL), web unit
and browser tests. A merge to `main` deploys by image digest (`.github/workflows/deploy.yml`):
the VM stops the old API and safety service, then starts the new ones, and the pipeline
fails if the site doesn't answer within 5 minutes. Roll back by re-running an earlier
successful deploy. The API drains for up to 25 s on stop so in-flight safety alerts finish.

## Metrics

The API serves `/metrics` on port 9090 inside the VM; Prometheus on the VM scrapes it and
listens on `127.0.0.1:9091` only (reach it through an IAP tunnel). Alert rules: `deploy/monitoring/alerts.yml`; what to do: `oncall.md`.
