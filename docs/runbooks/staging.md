# Staging

Staging is a full copy of production in its own AWS account, built from the same Terraform
and the same images, and run with every production guard on (`TRYST_ENV=prod`). It is where
the penetration test runs, where vendor sandboxes are wired up, and where every release
goes before production. It never holds real members' data.

Owner: engineering lead. Before you start: [deploy.md](deploy.md) and [restore.md](restore.md).

**Recommended: a second Google Cloud project** (`tryst-staging`) set up exactly as
production in [go-live.md](go-live.md) §2, with smaller machine and database tiers,
`ENV=staging` secrets (`tryst-staging-<NAME>`), the vendor sandboxes, its own IAP group and
the hosts `staging.<domain>` and `console.staging.<domain>`. The Hostinger VPS is for the
dev-mode demo copy, not staging (go-live.md §6). The AWS route below stays available if
production goes to AWS instead; sections 3, 5, 6 and 7 apply either way.

## 1. Account and access

1. Create a separate AWS account `tryst-staging` in the organisation, with the same SCPs as
   production (region locked to `eu-west-2`, no public S3, CloudTrail on).
2. Engineers get read access by default and deploy access through the CI role only.
3. Staging gets its own domain names, so cookies and passkeys never cross over:
   `staging.<domain>` for members and `console.staging.<domain>` for staff. Request the ACM
   certificate for both in `eu-west-2`.

## 2. Infrastructure

```sh
cd deploy/terraform
terraform workspace new staging
terraform apply \
  -var env=staging \
  -var api_image=<ecr>/tryst-api@sha256:... \
  -var web_image=<ecr>/tryst-web@sha256:... \
  -var ops_image=<ecr>/tryst-web-ops@sha256:... \
  -var certificate_arn=arn:aws:acm:eu-west-2:...:certificate/... \
  -var ops_host=console.staging.<domain> \
  -var 'staff_access={issuer="https://<team>.cloudflareaccess.com",audience="<staging AUD tag>",jwks_url="https://<team>.cloudflareaccess.com/cdn-cgi/access/certs",require_amr="hwk"}' \
  -var 'api_config={
      RP_ID="staging.<domain>", RP_ORIGINS="https://staging.<domain>", PUBLIC_URL="https://staging.<domain>",
      SALE_ITEMS="tryst_plus,tryst_plus_annual",
      ASSURANCE_URL="https://<provider sandbox>", ASSURANCE_PROVIDER="<provider>",
      ASSURANCE_CERTS="[{\"name\":\"<provider>\",\"certs\":[...]}]",
      LIVENESS_URL="https://<provider sandbox>", ARC_URL="https://<ARC test endpoint>",
      SIGHTENGINE_USER="<id>", TWILIO_ACCOUNT_SID="<test SID>", OTP_EMAIL_FROM="<sender>",
      VAPID_SUBJECT="mailto:security@<domain>", STRIPE_PRICE_TRYST_PLUS="price_...", STRIPE_PRICE_TRYST_PLUS_ANNUAL="price_..."
    }'
```

Use a staging Cloudflare Access application (its own AUD tag) with the same rules as
production: safety team group, hardware key required.

## 3. Secrets

Every name in `deploy/terraform/data.tf` (`api_secrets`, `safety_secrets`) is created empty
under `tryst-staging/<NAME>`. ECS won't start a task while a referenced secret has no value,
so give each one a value before the first deployment.

* Generate fresh values: `DATA_KEK`, `DSR_SIGNING_KEY`, `CONTACT_PEPPER`, `ANCHOR_PEPPER`
  (`openssl rand -hex 32` each), `VAPID_PRIVATE_KEY`. **Never copy a production value into
  staging, or the reverse.**
* Internal secrets and the roster: `scripts/provision-safety-secrets.sh` with
  `ENV=staging`.
* Vendor sandbox credentials only: Stripe test mode keys and webhook secret, the identity
  provider's sandbox secret and callback secret, Postmark sandbox server token, Twilio test
  credentials, Sightengine and PhotoDNA test keys, the ARC's test endpoint secret.
* `DATABASE_URL_*`: the RDS endpoints from `terraform output`, one user per database.

## 4. Databases and first start

1. Create the databases (identity, profile, safety, commerce on `main`; keys on `keys`;
   ledger on `ledger`). The services create their own tables at start.
2. Deploy; wait for every service's `/readyz`.
3. Run the restore drill against staging once before anything else (restore.md) and keep
   the timings it prints.

## 5. Smoke test

Run the browser suite against staging. It needs the dev stand-ins switched off, so use the
vendor sandboxes (identity provider test documents, Stripe test cards):

```sh
cd web && E2E_BASE_URL=https://staging.<domain> npx playwright test
```

Tests that rely on development-only helpers (the `X-Dev-OTP` header, `/verify/dev`) are
expected to be skipped or adapted for staging. Record which ones in the release notes;
do not turn dev mode on in staging to make them pass.

Then by hand, with two test members on two devices:

- [ ] Sign up by email and by SMS (Postmark, Twilio test numbers).
- [ ] Face check and ID check through the provider sandbox; an under-18 test document is
      refused and signed out; a second account on the same document is refused.
- [ ] Introductions and Tonight between the two members; a conversation opens.
- [ ] A meet with check-ins; a missed check-in reaches the ARC test endpoint.
- [ ] TRYST+ checkout with a Stripe test card; cancel inside 14 days and see the refund.
- [ ] A report, a moderator decision in the console (hardware key sign-in), an appeal.
- [ ] Data export (step-up), account deletion and its signed proof.
- [ ] Emergency lock from a third browser; recovery with the same test document.

## 6. Data rules

* Test members only, created by the team or the pen-test firm. No production data, no
  production backups restored into staging, no real ID documents.
* Staging is wiped and rebuilt (`terraform destroy` + apply) after the pen test and before
  launch.

## 7. Release flow

CI builds images on every merge to `main` and deploys them to staging by digest. A release
is promoted to production only after the smoke test above passes on staging with the same
digests.
