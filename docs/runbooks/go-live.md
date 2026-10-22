# Go live

TRYST is set up **once**; after that it deploys, updates, backs itself up and reports
problems without anyone touching it. It uses only the providers TRYST already has:
**Google Cloud, Hostinger, Vercel and Firebase** (D-66). Owner: engineering lead.

## 1. Where everything runs

| Provider | Runs | Why |
|---|---|---|
| **Google Cloud, London** | Production and staging, one project each. Per project: one VM running the member site, staff console, API and safety service; three Cloud SQL databases (main, keys, ledger) on private addresses; Secret Manager; Artifact Registry; the load balancer with a Google-managed certificate and Cloud Armor; Identity-Aware Proxy on the console; Cloud Monitoring alerts by email | UK data residency, managed databases with point-in-time recovery, and everything else from one provider. The backend must run as exactly one copy, which one VM with stop-then-start deploys guarantees |
| **Hostinger** | The mailbox that sends sign-up codes and trusted-contact alerts (SMTP); the domain's DNS; a VPS with an outside uptime check, the encrypted off-site backup copy, and an optional dev-mode demo copy | Email without another vendor; a second provider that notices if Google Cloud is down. Never members' data in clear |
| **Vercel** | The public website and journal (`site/`, `www.<domain>`): static pages, no member data, no cookies | Fast and free of operations for pages that should be found by search engines. The app itself stays off Vercel: it would pass members' Article 9 traffic through another processor and hide members' addresses from the rate limits |
| **Firebase** | Not used at launch. Later Firebase Cloud Messaging for the native apps | Firebase Auth would break passkey-only sign-in; Firestore can't do per-member crypto-erasure |

Launch is **email only**: sign-up codes and trusted-contact alerts go by email through the
Hostinger mailbox. Phone numbers are refused with a clear message until an SMS provider
is added (no code change needed then: set `TWILIO_*`).

## 2. Set it up once

You need: a Google Cloud billing account, two empty projects (staging and production),
`gcloud` signed in as their owner, `terraform` ≥ 1.6 and `openssl` on your computer.

1. **Fill in the settings** (no secrets in these):
   ```sh
   cp deploy/gcp/prod.conf.example deploy/gcp/prod.conf          # and staging.conf
   cp deploy/gcp/secrets.env.example deploy/gcp/prod.secrets.env # never committed
   ```
   `prod.conf`: project, domains, the safety team's Google group, the alert email, the
   repository, the Hostinger mailbox (`SMTP_USER`) and the staff roster file.
   `prod.secrets.env`: the mailbox password and each vendor's credentials as you get them.
2. **Build each environment**: `deploy/gcp/setup.sh staging`, then `deploy/gcp/setup.sh prod`.
   It shows the Terraform plan and asks before applying, then creates every key and
   database password straight into Secret Manager (never on disk, never in Terraform).
3. **Do the three things it prints**, once per environment:
   * add the two DNS A records at Hostinger;
   * add four GitHub Actions variables, and for production a **required reviewer**;
   * in Google Admin, set the safety group's 2-Step Verification to **security keys
     only**, put `STAFF_ACCESS_KEYS_CONFIRMED=google-2sv-security-keys-only` in the conf,
     and run `setup.sh` once more.
4. **Hostinger VPS**: install `deploy/hostinger/uptime-check.sh` (instructions at the top
   of the file), and create a `backup` user with an SSH key whose private half goes in
   `OFFSITE_SSH_KEY_FILE` / `OFFSITE_TARGET` in `prod.secrets.env` (run `setup.sh prod`
   again to load it).

## 3. What then happens by itself

| What | How |
|---|---|
| **Deploys** | Every merge to `main` that passes the integration tests is built once, deployed to staging, checked, and deployed to production after one click of approval (`.github/workflows/deploy.yml`). Secrets are re-read and the systemd units refreshed on every deploy. Roll back by re-running an earlier deploy from the Actions tab |
| **Never two copies** | A database lock and stop-then-start deploys keep the API and safety service single |
| **Certificates** | Google-managed, renewed automatically |
| **Security updates** | Daily on the VM; a reboot, if needed, Tuesday 04:30. Cloud SQL maintenance Tuesday 03:00 |
| **Backups** | Cloud SQL daily with point-in-time recovery (keys: 7 days, main and ledger: 30); VM disk snapshots daily; a weekly encrypted copy to Hostinger (never the keys database) |
| **Monitoring** | Google checks the site every minute from three regions and emails on-call; CPU and database disk alerts; the Hostinger VPS emails if Google Cloud itself is unreachable |
| **Housekeeping** | Container logs rotate; old images are pruned weekly on the VM and after 90 days in the registry |
| **Firewall** | Cloud Armor blocks common attacks and limits sign-in, recovery and emergency-lock requests per address |

## 4. The few things that stay manual

* Adding or removing safety staff: edit the roster file, run `setup.sh prod`.
* A new vendor credential or rotated key: put it in `prod.secrets.env`, run `setup.sh prod`;
  the next deploy picks it up.
* Rotating the internal secrets: `SECRETS_BACKEND=gcp ENV=prod scripts/provision-safety-secrets.sh --rotate`,
  deploy, then `--finish-rotation`, deploy.
* Restores: [restore.md](restore.md). Run the restore drill on staging monthly.
* Growing: raise `MACHINE_TYPE` or `DB_TIER_MAIN` in the conf and run `setup.sh`.
* A Node.js security release (the member site and console run on Node 22): raise
  `NODE_MIN` in `frontend/Dockerfile` to the fixed version and merge. The next deploy
  rebuilds on a freshly pulled base image, and the build fails on anything older. The API
  and safety service are Go and unaffected; the public site on Vercel is static.

## 5. Before opening sign-up

- [ ] Legal sign-offs ([../launch/legal-review-pack.md](../launch/legal-review-pack.md) §5).
- [ ] Vendors contracted and tested on staging ([../launch/vendor-checklist.md](../launch/vendor-checklist.md)).
- [ ] Penetration test on staging; high and critical findings fixed and retested.
- [ ] Restore drill passed on staging.
- [ ] An alert email received from a test outage (stop staging's VM for three minutes).
- [ ] Smoke test on production with team accounts ([staging.md](staging.md) §5).

## 6. Optional: a dev-mode demo on the Hostinger VPS

For design reviews and demos with stand-in vendors and no real people:

```sh
git clone <repo> /opt/tryst && cd /opt/tryst/deploy/gcp
sudo install -d -m 0700 /run/tryst && sudo touch /run/tryst/api.env /run/tryst/safety.env
printf 'LB_IP=127.0.0.1\nSITE_HOST=demo.<domain>\nAPI_IMAGE=…\nWEB_IMAGE=…\nOPS_IMAGE=…\n' | sudo tee /etc/tryst/images.env
sudo TRYST_ENV=dev docker compose --env-file /etc/tryst/images.env --profile vps up -d
```
