# Go live

Where each part runs, how to set it up once, and how to release, using only the providers
TRYST already has: **Google Cloud, Hostinger, Vercel and Firebase**. Owner: engineering
lead. What the services refuse to start without: [deploy.md](deploy.md).

## 1. Recommendation

| Need | Use | Why |
|---|---|---|
| Backend (`api`, `safety`) | **Google Cloud, London (`europe-west2`)**: one Compute Engine VM running `deploy/gcp/compose.yml` | UK data residency; the services must run as exactly one copy each, which a VM with stop-then-start deploys gives directly |
| Member site and staff console | **Same VM** (`web` on :3000, `ops` on :3001) | The site proxies `/v1/*` to the API over the VM's private network, so the API is never on the internet |
| Database | **Cloud SQL for PostgreSQL 16**, private IP, three instances: `tryst-main`, `tryst-keys` (7-day backups, for crypto-erasure), `tryst-ledger` (erasure ledger) | Managed backups, point-in-time recovery, encryption, failover |
| TLS, firewall, rate limits | **Google Cloud external Application Load Balancer** with a Google-managed certificate and **Cloud Armor** | Replaces a CDN/WAF vendor: OWASP rules, per-client rate limits on sign-in and the emergency lock |
| Staff console sign-in | **Identity-Aware Proxy (IAP)** on the console only | Only the safety team's Google accounts get through; Google's 2-Step Verification policy requires security keys; the safety service verifies IAP's signed token itself |
| Secrets, images | **Secret Manager**, **Artifact Registry** (London) | Secrets reach the VM only in memory (tmpfs) |
| Staging | **A second, smaller Google Cloud project** with the same setup | The pen test and release checks need a true copy of production; a different host would test something else |
| **Hostinger VPS** | **Development and demo copy** (dev mode, stand-in vendors, no real data); **off-site encrypted backup copy**; **outside uptime check** | Useful where data isn't real or is encrypted, and as a second provider that notices if Google Cloud is down. Not for members' data: no managed database, point-in-time recovery or key management |
| **Vercel** | **Not used for the app at launch.** Later: a separate marketing site with no member data | Putting the app on Vercel would route every member request, including Article 9 data, through another processor; the API's per-member rate limits would see Vercel's shared addresses; and the API would have to be on the public internet |
| **Firebase** | **Not used at launch.** Later: Firebase Cloud Messaging for push to the native apps | Firebase Auth would conflict with the passkey-only two-factor sign-in (FR-056); Firestore can't do the per-member crypto-erasure that account deletion depends on |
| Domain DNS | Wherever the domain is today (for example Hostinger DNS): two A records to the load balancer's address | No new provider |

Capacity: one `e2-standard-4` VM (4 vCPU, 16 GB) is enough for a London launch; move to a
bigger machine type when CPU stays above 60%. Running more than one VM needs the services'
state moved out of memory first (a planned engineering item, not a launch blocker).

## 2. One-time setup (production; repeat in the staging project)

Run as a project owner. Replace `<project>`, `<domain>` and `console.<domain>`.

```sh
gcloud config set project <project>
gcloud services enable compute.googleapis.com sqladmin.googleapis.com secretmanager.googleapis.com \
  artifactregistry.googleapis.com servicenetworking.googleapis.com iap.googleapis.com
R=europe-west2; Z=$R-a

# Registry
gcloud artifacts repositories create tryst --repository-format=docker --location=$R

# Private network for Cloud SQL
gcloud compute addresses create google-managed-services --global --purpose=VPC_PEERING --prefix-length=20 --network=default
gcloud services vpc-peerings connect --service=servicenetworking.googleapis.com --ranges=google-managed-services --network=default

# Databases: private IP only, point-in-time recovery
gcloud sql instances create tryst-main   --database-version=POSTGRES_16 --region=$R --tier=db-custom-2-7680 \
  --availability-type=REGIONAL --no-assign-ip --network=default --enable-point-in-time-recovery --retained-backups-count=30
gcloud sql instances create tryst-keys   --database-version=POSTGRES_16 --region=$R --tier=db-custom-1-3840 \
  --no-assign-ip --network=default --enable-point-in-time-recovery --retained-backups-count=7 --retained-transaction-log-days=7
gcloud sql instances create tryst-ledger --database-version=POSTGRES_16 --region=$R --tier=db-custom-1-3840 \
  --no-assign-ip --network=default --enable-point-in-time-recovery --retained-backups-count=30
for db in tryst_identity tryst_profile tryst_safety tryst_commerce; do gcloud sql databases create $db --instance=tryst-main; done
gcloud sql databases create tryst_keys --instance=tryst-keys
gcloud sql databases create tryst_ledger --instance=tryst-ledger
# One database user per service; their passwords go straight into the DATABASE_URL_* secrets.

# VM: no public address, no public SSH (IAP tunnel), a service account that reads only its secrets
gcloud iam service-accounts create tryst-vm
gcloud compute instances create tryst-prod --zone=$Z --machine-type=e2-standard-4 --no-address \
  --image-family=debian-12 --image-project=debian-cloud --boot-disk-size=50GB --boot-disk-type=pd-ssd \
  --shielded-secure-boot --service-account=tryst-vm@<project>.iam.gserviceaccount.com --scopes=cloud-platform \
  --tags=tryst-web
gcloud compute routers create tryst-nat-router --network=default --region=$R          # outbound only, for vendors
gcloud compute routers nats create tryst-nat --router=tryst-nat-router --region=$R --auto-allocate-nat-external-ips --nat-all-subnet-ip-ranges
gcloud compute firewall-rules create tryst-lb --allow=tcp:3000,tcp:3001 --target-tags=tryst-web \
  --source-ranges=35.191.0.0/16,130.211.0.0/22                                         # Google's load balancers only
gcloud compute firewall-rules create tryst-iap-ssh --allow=tcp:22 --target-tags=tryst-web --source-ranges=35.235.240.0/20

# Load balancer: one address, two backends (member site, console), Google-managed certificate
gcloud compute addresses create tryst-ip --global
gcloud compute instance-groups unmanaged create tryst-ig --zone=$Z
gcloud compute instance-groups unmanaged add-instances tryst-ig --zone=$Z --instances=tryst-prod
gcloud compute instance-groups set-named-ports tryst-ig --zone=$Z --named-ports=web:3000,console:3001
gcloud compute health-checks create http tryst-web-hc --port=3000 --request-path=/
gcloud compute health-checks create http tryst-console-hc --port=3001 --request-path=/
for b in web console; do
  gcloud compute backend-services create tryst-$b --global --protocol=HTTP --port-name=$b \
    --health-checks=tryst-$b-hc --load-balancing-scheme=EXTERNAL_MANAGED --timeout=60s
  gcloud compute backend-services add-backend tryst-$b --global --instance-group=tryst-ig --instance-group-zone=$Z
done
gcloud compute url-maps create tryst-lb --default-service=tryst-web
gcloud compute url-maps add-path-matcher tryst-lb --path-matcher-name=console --default-service=tryst-console \
  --new-hosts=console.<domain>
gcloud compute ssl-certificates create tryst-cert --global --domains=<domain>,console.<domain>
gcloud compute ssl-policies create tryst-tls --profile=MODERN --min-tls-version=1.2
gcloud compute target-https-proxies create tryst-https --url-map=tryst-lb --ssl-certificates=tryst-cert --ssl-policy=tryst-tls
gcloud compute forwarding-rules create tryst-https --global --target-https-proxy=tryst-https --ports=443 \
  --address=tryst-ip --load-balancing-scheme=EXTERNAL_MANAGED
gcloud compute addresses describe tryst-ip --global --format='value(address)'   # = LB_IP
```

**Cloud Armor** (attach to both backend services):

```sh
gcloud compute security-policies create tryst-armor
gcloud compute security-policies rules create 1000 --security-policy=tryst-armor \
  --expression="evaluatePreconfiguredWaf('sqli-v33-stable') || evaluatePreconfiguredWaf('xss-v33-stable') || evaluatePreconfiguredWaf('lfi-v33-stable') || evaluatePreconfiguredWaf('rce-v33-stable')" --action=deny-403
gcloud compute security-policies rules create 2000 --security-policy=tryst-armor \
  --expression="request.path.startsWith('/v1/auth/') || request.path.startsWith('/v1/emergency-lock')" \
  --action=throttle --rate-limit-threshold-count=60 --rate-limit-threshold-interval-sec=60 \
  --conform-action=allow --exceed-action=deny-429 --enforce-on-key=IP
for b in web console; do gcloud compute backend-services update tryst-$b --global --security-policy=tryst-armor; done
```

Check the WAF rules in preview mode (`--preview`) on staging first; the encrypted message
envelopes and photo uploads must not trip them.

**IAP on the console** (Security → Identity-Aware Proxy): turn IAP on for the
`tryst-console` backend and grant **IAP-secured Web App User** to a Google group holding
only the safety team. In the Google Admin console, set that group's 2-Step Verification
to **security keys only**. Note the backend's audience,
`/projects/<project-number>/global/backendServices/<backend-id>`
(`gcloud compute backend-services describe tryst-console --global --format='value(id)'`).

**DNS** (at the current DNS host): `A <domain> → LB_IP` and `A console.<domain> → LB_IP`.
The Google-managed certificate becomes active within about an hour of the records resolving.

**On the VM** (`gcloud compute ssh tryst-prod --tunnel-through-iap`): install Docker
Engine and the compose plugin, clone the repository to `/opt/tryst`, then:

```sh
sudo install -d -m 0755 /etc/tryst
printf 'LB_IP=<LB_IP>\n' | sudo tee /etc/tryst/images.env
sudo cp /opt/tryst/deploy/gcp/tryst.service /etc/systemd/system/ && sudo systemctl enable tryst
```

### Secrets

Create each name in `deploy/gcp/fetch-secrets.sh` as `tryst-prod-<NAME>` and grant
`tryst-vm` access to those secrets only. Generate keys fresh (`openssl rand -hex 32` for
`DATA_KEK`, `DSR_SIGNING_KEY`, `CONTACT_PEPPER`, `ANCHOR_PEPPER`). Internal secrets and the
staff roster: `SECRETS_BACKEND=gcp ENV=prod scripts/provision-safety-secrets.sh --roster roster.txt`.
Two secrets hold non-secret settings as `KEY=value` lines:

* `API_CONFIG`: `RP_ID=<domain>`, `RP_ORIGINS=https://<domain>`, `PUBLIC_URL=https://<domain>`,
  `SALE_ITEMS=tryst_plus,tryst_plus_annual`, `ASSURANCE_URL`, `ASSURANCE_PROVIDER`,
  `ASSURANCE_CERTS`, `LIVENESS_URL`, `ARC_URL`, `SIGHTENGINE_USER`, `TWILIO_ACCOUNT_SID`,
  `TWILIO_MESSAGING_SERVICE`, `OTP_EMAIL_FROM`, `VAPID_SUBJECT`, `STRIPE_PRICE_TRYST_PLUS`,
  `STRIPE_PRICE_TRYST_PLUS_ANNUAL`.
* `SAFETY_CONFIG`: `STAFF_ACCESS_PROVIDER=iap` and
  `STAFF_ACCESS_AUDIENCE=/projects/<project-number>/global/backendServices/<backend-id>`, and,
  once the 2-Step Verification policy is set, `STAFF_ACCESS_KEYS_CONFIRMED=google-2sv-security-keys-only`
  (`safety-svc admin preflight` checks it).

`DATABASE_URL_*` use the Cloud SQL private IPs with `sslmode=require`.

### GitHub

Set repository variables `GCP_PROJECT`, `GCP_WIF_PROVIDER`, `GCP_DEPLOY_SA` (Workload
Identity Federation; the deploy account can only push to Artifact Registry).

## 3. Release

1. Merge to `main`; the four pipelines go green.
2. Run **release images** (Actions → release.yml). Its summary prints the deploy command
   with the three digests.
3. Staging first, then production:
   `sudo /opt/tryst/deploy/gcp/deploy.sh <api@sha256:…> <web@sha256:…> <ops@sha256:…>`.
   The API and safety service stop and start (about 10–20 seconds of "try again" for
   members); the web containers restart in place.
4. Roll back with the digests in `/etc/tryst/images.env.previous`.

Release at quiet hours, not on Friday or Saturday evenings: check-ins pause for those
seconds (their timers live on the server and resume).

## 4. Backups and restores

Cloud SQL takes daily backups with point-in-time recovery (keys: 7 days, so a deleted
member's key can't come back from a backup after a week). After any restore, re-apply the
erasure ledger (restore.md) before taking traffic. Run the restore drill on staging
monthly.

**Off-site copy on Hostinger.** Weekly, `scripts/backup.sh` (run from the VM, against the
Cloud SQL private IPs) writes encrypted dumps; copy the main and ledger dumps to the
Hostinger VPS with `rsync` over SSH. The backup key stays in Secret Manager and never goes
to Hostinger, so the copy is unreadable there. **Never copy the keys database off Google
Cloud**: its 7-day limit is what makes deletion permanent.

## 5. Monitoring

* Google Cloud Monitoring uptime checks on `https://<domain>/` and the console host, and an
  alert policy that pages on-call.
* Prometheus on the VM evaluates `deploy/monitoring/alerts.yml`. Route it to on-call
  (Alertmanager on the VM, or Google Cloud Managed Service for Prometheus with Cloud
  Monitoring alerting); without a route the rules page nobody.
* Ops Agent on the VM for disk, memory and container restarts.
* From the Hostinger VPS, an outside check of the site every minute (a cron job with
  `curl`, alerting by email), so an outage of Google Cloud itself is still noticed.

## 6. Hostinger: development and demo copy

```sh
git clone <repo> /opt/tryst && cd /opt/tryst/deploy/gcp
sudo install -d -m 0700 /run/tryst && sudo touch /run/tryst/api.env /run/tryst/safety.env
printf 'LB_IP=127.0.0.1\nSITE_HOST=demo.<domain>\nAPI_IMAGE=…\nWEB_IMAGE=…\nOPS_IMAGE=…\n' | sudo tee /etc/tryst/images.env
sudo TRYST_ENV=dev docker compose --env-file /etc/tryst/images.env --profile vps up -d
```

Dev mode uses stand-in vendors and refuses nothing, which is why it's only for demos and
design reviews, never for real people. Point `demo.<domain>` at the VPS.

## 7. Go-live checklist

- [ ] Legal sign-offs ([../launch/legal-review-pack.md](../launch/legal-review-pack.md) §5).
- [ ] Launch vendors contracted and tested on staging ([../launch/vendor-checklist.md](../launch/vendor-checklist.md)).
- [ ] Penetration test done on staging; high and critical findings fixed and retested.
- [ ] Production secrets created fresh; nothing copied from staging.
- [ ] Restore drill passed on staging.
- [ ] Uptime checks and alert routing page a real person.
- [ ] Smoke test on production with team accounts ([staging.md](staging.md) §5), then open
      sign-up.
