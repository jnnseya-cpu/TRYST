# Go live

Where each part runs, how to set it up once, and how to release. Owner: engineering lead.
Production settings and what the services refuse to start without: [deploy.md](deploy.md).

## 1. Where things run

| Part | Production | Staging |
|---|---|---|
| Backend: `api` and `safety` | **Google Cloud, London (`europe-west2`)**: one Compute Engine VM running `deploy/gcp/compose.yml` | Hostinger VPS (choose a UK or EU data centre), same compose file, `localdb` profile, test data only |
| Frontend: member site and staff console | Same VM, behind Caddy (`web` and `ops` containers) | Same VPS |
| PostgreSQL | **Cloud SQL for PostgreSQL 16**, private IP: `tryst-main` (identity, profile, safety, commerce), `tryst-keys` (member data keys, 7-day backups for crypto-erasure), `tryst-ledger` (erasure ledger, kept apart for restores) | Container on the VPS |
| Secrets | Secret Manager (`tryst-prod-<NAME>`), fetched into tmpfs at boot | Secret Manager (`tryst-staging-<NAME>`) or a root-only env file |
| Images | Artifact Registry `europe-west2-docker.pkg.dev/<project>/tryst` | Same registry |
| Edge | Cloudflare: DNS, TLS, WAF, rate limiting, **Access** (hardware keys) for the console | Cloudflare, separate Access app |

### Why one VM and not Cloud Run, Vercel or Firebase

* **The backend runs as exactly one copy of each service.** They keep working state in
  memory and write through to PostgreSQL, and a database lock stops a second copy
  (backend/README.md). Cloud Run starts copies freely and overlaps old and new copies on
  every deploy, so the new one would just wait for the lock. A VM with a stop-then-start
  deploy fits exactly. A VM also gives the internal ports the two services use to talk
  to each other, which Cloud Run can't.
* **Vercel** is a good host for Next.js, but not for this app at launch. Every member request
  (including Article 9 data) would pass through another processor. The API's per-member rate
  limits would see Vercel's shared addresses instead of members'. And the frontend proxies
  to a backend that should never be on the public internet. Keeping both on the same private
  network avoids all three. Vercel stays an option later for a separate marketing site.
* **Firebase** isn't needed. Firebase Auth would break the passkey-only, two-factor sign-in
  (FR-056), and Firestore can't do the per-member crypto-erasure the deletion promise relies
  on. Firebase Cloud Messaging becomes useful when the native apps ship (push to phones).
* **Hostinger** has no managed database, point-in-time recovery or key management, so it's
  right for staging, not for production members' data.

Capacity: one `e2-standard-4` VM (4 vCPU, 16 GB) comfortably serves a London launch; scale
up the machine type when CPU stays above 60%. More than one VM needs the services' state
moved out of memory first (a planned engineering item, not a launch blocker).

## 2. One-time setup (production)

Run with a project owner account. Replace `<project>` and the domain names.

```sh
gcloud config set project <project>
gcloud services enable compute.googleapis.com sqladmin.googleapis.com secretmanager.googleapis.com \
  artifactregistry.googleapis.com servicenetworking.googleapis.com iap.googleapis.com
R=europe-west2

# Registry
gcloud artifacts repositories create tryst --repository-format=docker --location=$R

# Private network for Cloud SQL
gcloud compute addresses create google-managed-services --global --purpose=VPC_PEERING --prefix-length=20 --network=default
gcloud services vpc-peerings connect --service=servicenetworking.googleapis.com --ranges=google-managed-services --network=default

# Databases (private IP only, encrypted, point-in-time recovery)
gcloud sql instances create tryst-main   --database-version=POSTGRES_16 --region=$R --tier=db-custom-2-7680 \
  --availability-type=REGIONAL --no-assign-ip --network=default --enable-point-in-time-recovery --retained-backups-count=30
gcloud sql instances create tryst-keys   --database-version=POSTGRES_16 --region=$R --tier=db-custom-1-3840 \
  --no-assign-ip --network=default --enable-point-in-time-recovery --retained-backups-count=7 --retained-transaction-log-days=7
gcloud sql instances create tryst-ledger --database-version=POSTGRES_16 --region=$R --tier=db-custom-1-3840 \
  --no-assign-ip --network=default --enable-point-in-time-recovery --retained-backups-count=30
for db in tryst_identity tryst_profile tryst_safety tryst_commerce; do gcloud sql databases create $db --instance=tryst-main; done
gcloud sql databases create tryst_keys --instance=tryst-keys
gcloud sql databases create tryst_ledger --instance=tryst-ledger
# One user per service; passwords go straight into the DATABASE_URL_* secrets below.

# VM: no public SSH (use IAP), service account that can only read its secrets
gcloud iam service-accounts create tryst-vm
gcloud compute instances create tryst-prod --zone=$R-a --machine-type=e2-standard-4 \
  --image-family=debian-12 --image-project=debian-cloud --boot-disk-size=50GB --boot-disk-type=pd-ssd \
  --shielded-secure-boot --service-account=tryst-vm@<project>.iam.gserviceaccount.com --scopes=cloud-platform \
  --tags=tryst-web
# 443 only from Cloudflare; SSH only through IAP
gcloud compute firewall-rules create tryst-cloudflare-443 --allow=tcp:443 --target-tags=tryst-web \
  --source-ranges=173.245.48.0/20,103.21.244.0/22,103.22.200.0/22,103.31.4.0/22,141.101.64.0/18,108.162.192.0/18,190.93.240.0/20,188.114.96.0/20,197.234.240.0/22,198.41.128.0/17,162.158.0.0/15,104.16.0.0/13,104.24.0.0/14,172.64.0.0/13,131.0.72.0/22
gcloud compute firewall-rules create tryst-iap-ssh --allow=tcp:22 --target-tags=tryst-web --source-ranges=35.235.240.0/20
```

On the VM (`gcloud compute ssh tryst-prod --tunnel-through-iap`): install Docker Engine and
the compose plugin, clone the repository to `/opt/tryst`, then:

```sh
sudo install -d -m 0755 /etc/tryst /etc/tryst/origin
printf 'SITE_HOST=<domain>\nCONSOLE_HOST=console.<domain>\n' | sudo tee /etc/tryst/images.env
# Cloudflare origin certificate (SSL/TLS → Origin Server) into /etc/tryst/origin/{cert,key}.pem, key mode 0600
sudo cp /opt/tryst/deploy/gcp/tryst.service /etc/systemd/system/ && sudo systemctl enable tryst
```

### Secrets

Create each name from `deploy/gcp/fetch-secrets.sh` as `tryst-prod-<NAME>` and grant
`tryst-vm` access to those secrets only. Generate keys fresh (`openssl rand -hex 32` for
`DATA_KEK`, `DSR_SIGNING_KEY`, `CONTACT_PEPPER`, `ANCHOR_PEPPER`); internal secrets and the
staff roster with `SECRETS_BACKEND=gcp ENV=prod scripts/provision-safety-secrets.sh --roster roster.txt`. Two secrets hold non-secret settings as `KEY=value` lines:

* `API_CONFIG`: `RP_ID=<domain>`, `RP_ORIGINS=https://<domain>`, `PUBLIC_URL=https://<domain>`,
  `SALE_ITEMS=tryst_plus,tryst_plus_annual`, `ASSURANCE_URL`, `ASSURANCE_PROVIDER`,
  `ASSURANCE_CERTS`, `LIVENESS_URL`, `ARC_URL`, `SIGHTENGINE_USER`, `TWILIO_ACCOUNT_SID`,
  `TWILIO_MESSAGING_SERVICE`, `OTP_EMAIL_FROM`, `VAPID_SUBJECT`, `STRIPE_PRICE_TRYST_PLUS`,
  `STRIPE_PRICE_TRYST_PLUS_ANNUAL`.
* `SAFETY_CONFIG`: `STAFF_ACCESS_ISSUER`, `STAFF_ACCESS_AUDIENCE`, `STAFF_ACCESS_JWKS_URL`,
  `STAFF_ACCESS_REQUIRE_AMR=hwk`.

`DATABASE_URL_*` use the Cloud SQL private IPs with `sslmode=require`.

### Cloudflare

DNS for `<domain>` and `console.<domain>`, proxied; SSL mode **Full (strict)**; WAF managed
rules on; a rate-limiting rule on `/v1/auth/*` and `/v1/emergency-lock`. An Access
application on `console.<domain>` limited to the safety team with hardware-key sign-in
(deploy.md). Turn off Cloudflare features that inject scripts (Rocket Loader, analytics),
since the site's CSP forbids them.

### GitHub

Set repository variables `GCP_PROJECT`, `GCP_WIF_PROVIDER`, `GCP_DEPLOY_SA` (Workload
Identity Federation; the deploy account can only push to Artifact Registry).

## 3. Release

1. Merge to `main`; the four pipelines go green.
2. Run **release images** (Actions → release.yml). Its summary prints the deploy command
   with the three digests.
3. On staging first, then production:
   `sudo /opt/tryst/deploy/gcp/deploy.sh <api@sha256:…> <web@sha256:…> <ops@sha256:…>`.
   The API and safety service stop and start (about 10–20 seconds of "try again" for
   members); the web containers restart in place.
4. Roll back by running the same command with the digests in `/etc/tryst/images.env.previous`.

Releases go out at quiet hours, not during an active meet window peak (Friday and Saturday
evenings), because check-ins pause for those seconds. Timers live on the server and resume.

## 4. Backups and restores

Cloud SQL takes daily backups with point-in-time recovery (keys: 7 days, so a deleted
member's key can't come back from a backup after a week). After any restore, run the
erasure ledger re-apply (restore.md) before taking traffic. Run the restore drill on
staging monthly.

## 5. Monitoring

Prometheus runs on the VM with the alert rules in `deploy/monitoring/alerts.yml`. Before
launch, also set up:

* Google Cloud Monitoring uptime checks on `https://<domain>/` and the console host, paging
  on-call.
* An Alertmanager (or Google Managed Prometheus) route for the rules in `alerts.yml` to the
  on-call rota. Without it, the rules evaluate but page nobody.
* Ops Agent on the VM for disk, memory and container restarts.

## 6. Go-live checklist

- [ ] Legal sign-offs ([../launch/legal-review-pack.md](../launch/legal-review-pack.md) §5).
- [ ] Launch vendors contracted and tested on staging ([../launch/vendor-checklist.md](../launch/vendor-checklist.md)).
- [ ] Penetration test done on staging; high and critical findings fixed and retested.
- [ ] Production secrets created fresh; nothing copied from staging.
- [ ] Restore drill passed against production-shaped staging.
- [ ] Uptime checks and alert routing page a real person.
- [ ] Smoke test on production with team accounts ([staging.md](staging.md) §5), then open
      sign-up.
