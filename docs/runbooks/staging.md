# Staging

Staging is an optional, full copy of production in its own Google Cloud project, built by
the same `deploy/gcp/setup.sh` from the same images, with every production guard on
(`TRYST_ENV=prod`). It is where the penetration test runs, where vendor sandboxes are wired
up, and where releases go before production. It never holds real members' data. Without
staging variables in GitHub, the pipeline deploys straight to production.

Owner: engineering lead. Before you start: [go-live.md](go-live.md) and [restore.md](restore.md).

## 1. Project and access

1. Create a second Google Cloud project (for example `tryst-staging`) on the same billing
   account.
2. Staging gets its own host names, so cookies and passkeys never cross over:
   `staging.<domain>` for members and `console.staging.<domain>` for staff.
3. A separate Google group for staging console access (it can be the same people).

## 2. Infrastructure

```sh
cp deploy/gcp/prod.conf.example deploy/gcp/staging.conf          # staging project, hosts, group
cp deploy/gcp/secrets.env.example deploy/gcp/staging.secrets.env # sandbox credentials only
deploy/gcp/setup.sh staging
```

Use smaller tiers (`MACHINE_TYPE`, `DB_TIER_MAIN`) in `staging.conf`.

## 3. Secrets

`setup.sh staging` generates fresh internal keys (`DATA_KEK`, `DSR_SIGNING_KEY`,
`CONTACT_PEPPER`, `ANCHOR_PEPPER`, the internal secrets) straight into Secret Manager as
`tryst-staging-<NAME>`. **Never copy a production value into staging, or the reverse.**
`staging.secrets.env` holds vendor sandbox credentials only: Stripe test keys, the identity
provider's sandbox, the ARC's test endpoint, Sightengine and PhotoDNA test keys.

## 4. First start

1. Merge to `main` (or run the deploy workflow); wait for every `/readyz`.
2. Run the restore drill against staging once before anything else ([restore.md](restore.md))
   and keep the timings it prints.

## 5. Smoke test

Run the browser suite against staging. It needs the dev stand-ins switched off, so use the
vendor sandboxes (identity provider test documents, Stripe test cards):

```sh
cd frontend && E2E_BASE_URL=https://staging.<domain> npx playwright test
```

Tests that rely on development-only helpers (the `X-Dev-OTP` header, `/verify/dev`) are
expected to be skipped or adapted for staging. Record which ones in the release notes;
do not turn dev mode on in staging to make them pass.

Then by hand, with two test members on two devices:

- [ ] Sign up by email (the Hostinger mailbox); a phone number is refused with `email_only`.
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
* Staging's databases are wiped after the pen test and before launch (delete and re-run
  `setup.sh staging`).

## 7. Release flow

CI builds images on every merge to `main` and deploys them to staging by digest. A release
is promoted to production only after the smoke test above passes on staging with the same
digests.
