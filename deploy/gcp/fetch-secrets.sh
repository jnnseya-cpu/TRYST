#!/usr/bin/env bash
# Writes /run/tryst/api.env and /run/tryst/safety.env from Google Secret Manager at boot.
# /run is tmpfs: secrets never touch the VM's disk. The VM's service account needs
# roles/secretmanager.secretAccessor on these secrets only.
#
#   ENV=prod deploy/gcp/fetch-secrets.sh
set -euo pipefail
env_name="${ENV:?set ENV (prod or staging)}"
project="${PROJECT:-$(gcloud config get-value project 2>/dev/null)}"
dir=/run/tryst
install -d -m 0700 "$dir"

# Names as in deploy/gcp/terraform/secrets.tf.
api=(DATA_KEK DSR_SIGNING_KEY CONTACT_PEPPER ANCHOR_PEPPER VAPID_PRIVATE_KEY SMTP_PASSWORD POSTMARK_SERVER_TOKEN
  TWILIO_AUTH_TOKEN STRIPE_SECRET_KEY STRIPE_WEBHOOK_SECRET LIVENESS_SECRET LIVENESS_API_KEY
  ASSURANCE_SECRET ASSURANCE_API_KEY ASSURANCE_CALLBACK_SECRET SIGHTENGINE_SECRET PHOTODNA_KEY
  ARC_SECRET ARC_API_KEY DATABASE_URL_IDENTITY DATABASE_URL_PROFILE DATABASE_URL_KEYS
  DATABASE_URL_COMMERCE DATABASE_URL_LEDGER INTERNAL_SECRET_SAFETY INTERNAL_SECRET_PLATFORM
  INTERNAL_SECRET_SAFETY_PREVIOUS INTERNAL_SECRET_PLATFORM_PREVIOUS API_CONFIG)
safety=(DATABASE_URL_SAFETY DATABASE_URL_LEDGER INTERNAL_SECRET_SAFETY INTERNAL_SECRET_PLATFORM
  INTERNAL_SECRET_SAFETY_PREVIOUS INTERNAL_SECRET_PLATFORM_PREVIOUS STAFF_ROSTER SAFETY_CONFIG)

write() { # file, names...
  local out="$1"; shift
  local tmp; tmp="$(mktemp -p "$dir")"
  chmod 0600 "$tmp"
  for n in "$@"; do
    if ! v="$(gcloud secrets versions access latest --secret="tryst-$env_name-$n" --project="$project" 2>/dev/null)"; then
      continue # optional secrets (an API key a vendor doesn't issue, a *_PREVIOUS outside a rotation)
    fi
    case "$n" in
      API_CONFIG|SAFETY_CONFIG) printf '%s\n' "$v" >>"$tmp" ;; # KEY=value lines of non-secret settings
      *) printf '%s=%s\n' "$n" "$(printf '%s' "$v" | tr -d '\n')" >>"$tmp" ;;
    esac
  done
  mv "$tmp" "$dir/$out"
}
write api.env "${api[@]}"
write safety.env "${safety[@]}"
echo "secrets written to $dir (tmpfs)"
