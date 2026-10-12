#!/usr/bin/env bash
# One command builds (or brings up to date) a whole TRYST environment on Google Cloud.
# Safe to run again at any time: it only creates what's missing and never replaces a key.
#
#   deploy/gcp/setup.sh prod        (reads deploy/gcp/prod.conf and prod.secrets.env)
#   deploy/gcp/setup.sh staging
#
# Needs: gcloud (signed in as a project owner), terraform >= 1.6, openssl.
set -euo pipefail
env_name="${1:?usage: setup.sh prod|staging}"
here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
conf="$here/$env_name.conf"
secrets_file="$here/$env_name.secrets.env"
[ -f "$conf" ] || { echo "missing $conf (copy prod.conf.example)"; exit 2; }
# shellcheck disable=SC1090
. "$conf"
for v in PROJECT_ID DOMAIN CONSOLE_DOMAIN STAFF_GROUP ALERT_EMAIL GITHUB_REPO; do
  [ -n "${!v:-}" ] || { echo "$v is required in $conf"; exit 2; }
done
for t in gcloud terraform openssl; do command -v "$t" >/dev/null || { echo "$t not found"; exit 2; }; done
umask 077
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
g() { gcloud --project="$PROJECT_ID" "$@"; }
name="tryst-$env_name"
say() { printf '\n== %s\n' "$*"; }

say "Terraform state bucket"
bucket="$PROJECT_ID-tryst-tfstate"
g services enable storage.googleapis.com cloudresourcemanager.googleapis.com >/dev/null
if ! g storage buckets describe "gs://$bucket" >/dev/null 2>&1; then
  g storage buckets create "gs://$bucket" --location=europe-west2 --uniform-bucket-level-access --public-access-prevention
  g storage buckets update "gs://$bucket" --versioning
fi

say "Infrastructure (Terraform)"
cd "$here/terraform"
terraform init -input=false -reconfigure -backend-config="bucket=$bucket" -backend-config="prefix=$env_name" >/dev/null
tfvars=(-var "project_id=$PROJECT_ID" -var "env=$env_name" -var "domain=$DOMAIN" -var "console_domain=$CONSOLE_DOMAIN"
  -var "staff_group=$STAFF_GROUP" -var "alert_email=$ALERT_EMAIL" -var "github_repo=$GITHUB_REPO"
  -var "machine_type=${MACHINE_TYPE:-e2-standard-4}" -var "db_tier_main=${DB_TIER_MAIN:-db-custom-2-7680}"
  -var "db_high_availability=${DB_HA:-true}")
terraform plan -input=false -out="$tmp/plan" "${tfvars[@]}"
if [ "${YES:-}" != 1 ]; then
  read -r -p "Apply this plan to $PROJECT_ID? [yes/N] " ok
  [ "$ok" = yes ] || { echo "stopped"; exit 1; }
fi
terraform apply -input=false "$tmp/plan"
out() { terraform output -raw "$1"; }
lb_ip="$(out lb_ip)"; aud="$(out console_iap_audience)"
declare -A sql_ip=() sql_name=()
for k in main keys ledger; do
  sql_ip[$k]="$(terraform output -json sql_private_ip | python3 -c "import json,sys;print(json.load(sys.stdin)['$k'])")"
  sql_name[$k]="$(terraform output -json sql_instances | python3 -c "import json,sys;print(json.load(sys.stdin)['$k'])")"
done
wif="$(out github_wif_provider)"; deploy_sa="$(out github_deploy_sa)"
cd "$repo"

has() { g secrets versions access latest --secret="$name-$1" >/dev/null 2>&1; }
put() { # name, value-file: adds a version only when the value changed
  if has "$1" && cmp -s "$2" <(g secrets versions access latest --secret="$name-$1"); then return; fi
  g secrets versions add "$name-$1" --data-file="$2" >/dev/null && echo "set $name-$1"
}
gen() { # name, generator: only when missing; never replaces a key
  has "$1" && return
  "$2" >"$tmp/v"; put "$1" "$tmp/v"
}
hex32() { openssl rand -hex 32 | tr -d '\n'; }
vapid() { # base64url P-256 private scalar
  openssl ecparam -name prime256v1 -genkey -noout | openssl ec -no_public -outform DER 2>/dev/null |
    tail -c +8 | head -c 32 | openssl base64 -A | tr '+/' '-_' | tr -d '='
}

say "Generated keys (kept forever; never regenerated)"
for k in DATA_KEK DSR_SIGNING_KEY CONTACT_PEPPER ANCHOR_PEPPER BACKUP_KEY; do gen "$k" hex32; done
gen VAPID_PRIVATE_KEY vapid

say "Database users"
dsn_secret() { # secret, instance key, database, user
  has "$1" && return
  local pass inst="${sql_name[$2]}"; pass="$(openssl rand -hex 24)"
  if g sql users describe "$4" --instance="$inst" >/dev/null 2>&1; then
    g sql users set-password "$4" --instance="$inst" --password="$pass" >/dev/null
  else
    g sql users create "$4" --instance="$inst" --password="$pass" >/dev/null
  fi
  printf 'postgres://%s:%s@%s:5432/%s?sslmode=require' "$4" "$pass" "${sql_ip[$2]}" "$3" >"$tmp/v"
  put "$1" "$tmp/v"
}
dsn_secret DATABASE_URL_IDENTITY main tryst_identity identity_svc
dsn_secret DATABASE_URL_PROFILE main tryst_profile profile_svc
dsn_secret DATABASE_URL_SAFETY main tryst_safety safety_svc
dsn_secret DATABASE_URL_COMMERCE main tryst_commerce commerce_svc
dsn_secret DATABASE_URL_KEYS keys tryst_keys keys_svc
dsn_secret DATABASE_URL_LEDGER ledger tryst_ledger ledger_svc

say "Internal secrets and staff roster"
if [ -n "${ROSTER_FILE:-}" ] && [ -f "$ROSTER_FILE" ]; then
  CLOUDSDK_CORE_PROJECT="$PROJECT_ID" SECRETS_BACKEND=gcp ENV="$env_name" scripts/provision-safety-secrets.sh --roster "$ROSTER_FILE"
elif ! has STAFF_ROSTER; then
  echo "WARNING: no roster yet (set ROSTER_FILE): the safety service won't start without one"
fi

say "Vendor credentials ($secrets_file)"
declare -A vendor=()
if [ -f "$secrets_file" ]; then
  while IFS='=' read -r k v; do
    [[ -z "$k" || "$k" == \#* || -z "$v" ]] && continue
    vendor[$k]="$v"
  done <"$secrets_file"
fi
for k in SMTP_PASSWORD ASSURANCE_SECRET ASSURANCE_API_KEY ASSURANCE_CALLBACK_SECRET LIVENESS_SECRET LIVENESS_API_KEY \
  ARC_SECRET ARC_API_KEY STRIPE_SECRET_KEY STRIPE_WEBHOOK_SECRET PHOTODNA_KEY SIGHTENGINE_SECRET OFFSITE_TARGET; do
  if [ -n "${vendor[$k]:-}" ]; then printf '%s' "${vendor[$k]}" >"$tmp/v"; put "$k" "$tmp/v"; fi
done
if [ -n "${vendor[OFFSITE_SSH_KEY_FILE]:-}" ]; then put OFFSITE_SSH_KEY "${vendor[OFFSITE_SSH_KEY_FILE]}"; fi

say "Settings"
{
  echo "RP_ID=$DOMAIN"; echo "RP_ORIGINS=https://$DOMAIN"; echo "PUBLIC_URL=https://$DOMAIN"
  echo "SALE_ITEMS=${SALE_ITEMS:-tryst_plus,tryst_plus_annual}"; echo "VAPID_SUBJECT=mailto:$ALERT_EMAIL"
  [ -n "${SMTP_HOST:-}" ] && { echo "SMTP_HOST=$SMTP_HOST"; echo "SMTP_PORT=${SMTP_PORT:-465}"; echo "SMTP_USER=$SMTP_USER"; }
  [ -n "${OTP_EMAIL_FROM:-}" ] && echo "OTP_EMAIL_FROM=$OTP_EMAIL_FROM"
  for k in ASSURANCE_URL ASSURANCE_PROVIDER ASSURANCE_CERTS LIVENESS_URL ARC_URL SIGHTENGINE_USER \
    STRIPE_PRICE_TRYST_PLUS STRIPE_PRICE_TRYST_PLUS_ANNUAL; do
    [ -n "${vendor[$k]:-}" ] && echo "$k=${vendor[$k]}"
  done
  true
} >"$tmp/api"
put API_CONFIG "$tmp/api"
{
  echo "STAFF_ACCESS_PROVIDER=iap"; echo "STAFF_ACCESS_AUDIENCE=$aud"
  [ -n "${STAFF_ACCESS_KEYS_CONFIRMED:-}" ] && echo "STAFF_ACCESS_KEYS_CONFIRMED=$STAFF_ACCESS_KEYS_CONFIRMED"
  true
} >"$tmp/safety"
put SAFETY_CONFIG "$tmp/safety"

say "Done. Once, by hand:"
cat <<MSG
1. DNS (at Hostinger, where the domain is):
     A  $DOMAIN          -> $lb_ip
     A  $CONSOLE_DOMAIN  -> $lb_ip
   The certificate turns active by itself within about an hour of these resolving.
2. GitHub → Settings → Secrets and variables → Actions → Variables:
     GCP_PROJECT_${env_name^^}      = $PROJECT_ID
     GCP_WIF_PROVIDER_${env_name^^} = $wif
     GCP_DEPLOY_SA_${env_name^^}    = $deploy_sa
     TRYST_DOMAIN_${env_name^^}     = $DOMAIN
   and (production only) Settings → Environments → production → Required reviewers.
3. Google Admin: 2-Step Verification for $STAFF_GROUP = security keys only; then set
   STAFF_ACCESS_KEYS_CONFIRMED=google-2sv-security-keys-only in $conf and run this again.
After that, every merge to main deploys itself (.github/workflows/deploy.yml).
MSG
