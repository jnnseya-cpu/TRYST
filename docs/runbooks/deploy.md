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

### Staff console host, roster and internal secrets

Do these in order. Nothing here involves putting a secret or an email address in the
repository or in Terraform.

**1. Cloudflare Access application (Cloudflare Zero Trust dashboard)**

1. Access → Applications → Add an application → **Self-hosted**. Application domain: the
   console host (`ops_host`, e.g. `ops.<your-domain>`). Session duration: 1 hour.
2. Identity provider: your company sign-in (Microsoft Entra ID, Okta or Google
   Workspace). Turn off every other login method, including one-time PIN.
3. Policy "T&S staff", action **Allow**:
   * Include: the identity provider group for the Trust & Safety team (not "everyone at
     the company").
   * Require: **Authentication method** = hardware key (`hwk`). This needs your identity
     provider to report how the person signed in. If it doesn't, enforce
     phishing-resistant security keys for that group inside the identity provider instead.
4. Note the application's **Audience (AUD) tag** (application overview) and your **team
   domain** (`<team>.cloudflareaccess.com`).
5. Point the console host's DNS at Cloudflare (proxied) so all traffic passes Access.
   Even if someone reached the load balancer directly, the safety service still refuses
   them: it verifies the Access token itself.

**2. Terraform**

```hcl
staff_access = {
  issuer      = "https://<team>.cloudflareaccess.com"
  audience    = "<AUD tag>"
  jwks_url    = "https://<team>.cloudflareaccess.com/cdn-cgi/access/certs"
  require_amr = ""   # set to "hwk" in step 5 once check-token shows it arrives
}
```

**3. Roster and internal secrets**

Write the roster in a file outside the repository, one person per line (`email role`, roles
`moderator` and/or `lead`; at least three people and one lead, because a permanent removal
needs two different moderators and an appeal needs a third who wasn't involved). Then:

```sh
ENV=prod scripts/provision-safety-secrets.sh --roster /secure/roster.txt
```

It generates the two internal secrets (one per direction), stores everything in Secrets
Manager under `tryst-prod/`, never prints a value, and never overwrites a secret that's
already set. Delete the roster file afterwards. To change who has access, edit the list and
run it again with `--roster`; removing someone takes effect on the next safety-service
deploy, even if Access still lets them in.

**4. Deploy, then check**

Deploy both services, then run inside each task (ECS Exec):

```sh
/safety-svc admin preflight     # roster size, proxy keys, hardware key, secrets, database, link to the API
/identity-svc admin preflight   # secrets distinct, link to the safety service
```

Both must exit 0. Preflight prints no secrets and no emails.

**5. Confirm a real sign-in**

Sign in to the console host with a security key. Copy the `CF_Authorization` cookie value
and run `/safety-svc admin check-token` with the value on stdin. It shows the email, the
roles TRYST would give and the sign-in methods in the token. If `hwk` is listed, set
`require_amr = "hwk"` and redeploy: from then on TRYST itself refuses staff tokens that
don't show a hardware key. Run preflight again.

**Rotating the internal secrets** (yearly, or at once if one may have leaked):
`--rotate`, redeploy both services (each accepts old and new), `--finish-rotation`,
redeploy both again.

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
