#!/usr/bin/env bash
# Provision the safety-service secrets in AWS Secrets Manager or Google Secret Manager
# (D-59; docs/runbooks/deploy.md, docs/runbooks/go-live.md).
# Values are generated here and written straight to Secrets Manager through private
# temporary files; nothing is printed, logged, or written to the repository.
#
#   ENV=prod scripts/provision-safety-secrets.sh --roster roster.txt   first time, or roster change
#   ENV=prod scripts/provision-safety-secrets.sh --rotate               new internal secrets
#   ENV=prod scripts/provision-safety-secrets.sh --finish-rotation      after both services redeployed
#
# SECRETS_BACKEND=aws (default): names "tryst-<env>/<NAME>". SECRETS_BACKEND=gcp: names
# "tryst-<env>-<NAME>" in the current gcloud project, stored in europe-west2 only.
#
# roster.txt: one person per line, "email role[,role]" (roles: moderator, lead). Lines
# starting with # are ignored. At least three people and one lead: a permanent removal
# needs two different moderators and an appeal needs a third who wasn't involved.
set -euo pipefail
env_name="${ENV:?set ENV (e.g. prod)}"
backend="${SECRETS_BACKEND:-aws}"
roster="" rotate=0 finish=0
while [ $# -gt 0 ]; do
  case "$1" in
    --roster) roster="${2:?--roster needs a file}"; shift 2 ;;
    --rotate) rotate=1; shift ;;
    --finish-rotation) finish=1; shift ;;
    *) echo "unknown option $1" >&2; exit 2 ;;
  esac
done
umask 077
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT

case "$backend" in
aws)
  command -v aws >/dev/null || { echo "aws CLI not found" >&2; exit 2; }
  prefix="tryst-$env_name"
  exists() { aws secretsmanager get-secret-value --secret-id "$prefix/$1" --query SecretString --output text >/dev/null 2>&1; }
  put() { # name, file
    if aws secretsmanager describe-secret --secret-id "$prefix/$1" >/dev/null 2>&1; then
      aws secretsmanager put-secret-value --secret-id "$prefix/$1" --secret-string "file://$2" >/dev/null
    else
      aws secretsmanager create-secret --name "$prefix/$1" --secret-string "file://$2" >/dev/null
    fi
    echo "set $prefix/$1"
  }
  current() { aws secretsmanager get-secret-value --secret-id "$prefix/$1" --query SecretString --output text; }
  ;;
gcp)
  command -v gcloud >/dev/null || { echo "gcloud CLI not found" >&2; exit 2; }
  prefix="tryst-$env_name"
  exists() { gcloud secrets versions access latest --secret="$prefix-$1" >/dev/null 2>&1; }
  put() { # name, file
    if gcloud secrets describe "$prefix-$1" >/dev/null 2>&1; then
      gcloud secrets versions add "$prefix-$1" --data-file="$2" >/dev/null
    else
      gcloud secrets create "$prefix-$1" --data-file="$2" --replication-policy=user-managed --locations=europe-west2 >/dev/null
    fi
    echo "set $prefix-$1"
  }
  current() { gcloud secrets versions access latest --secret="$prefix-$1"; }
  ;;
*) echo "SECRETS_BACKEND must be aws or gcp" >&2; exit 2 ;;
esac
new_secret() { openssl rand -hex 32 | tr -d '\n' > "$1"; }

secrets=(INTERNAL_SECRET_SAFETY INTERNAL_SECRET_PLATFORM)

if [ "$finish" = 1 ]; then
  for s in "${secrets[@]}"; do printf none > "$tmp/p"; put "${s}_PREVIOUS" "$tmp/p"; done
  echo "rotation finished: redeploy both services so they stop accepting the old secrets"
  exit 0
fi

if [ "$rotate" = 1 ]; then
  for s in "${secrets[@]}"; do
    current "$s" | tr -d '\n' > "$tmp/old"
    new_secret "$tmp/new"
    put "${s}_PREVIOUS" "$tmp/old"
    put "$s" "$tmp/new"
  done
  echo "rotated: redeploy the safety service and the API (each accepts old and new),"
  echo "then run with --finish-rotation and redeploy both again"
else
  for s in "${secrets[@]}"; do
    if exists "$s"; then
      echo "kept $prefix/$s (already set; use --rotate to replace)"
    else
      new_secret "$tmp/$s"
      put "$s" "$tmp/$s"
    fi
    if ! exists "${s}_PREVIOUS"; then printf none > "$tmp/p"; put "${s}_PREVIOUS" "$tmp/p"; fi
  done
  if cmp -s <(current INTERNAL_SECRET_SAFETY) <(current INTERNAL_SECRET_PLATFORM); then
    echo "the two internal secrets are identical; run --rotate" >&2; exit 1
  fi
fi

if [ -n "$roster" ]; then
  entries=() staff=0 leads=0
  declare -A seen=()
  while IFS= read -r line || [ -n "$line" ]; do
    line="${line%%#*}"; line="$(echo "$line" | xargs)"
    [ -z "$line" ] && continue
    email="${line%% *}"; roles="${line#* }"
    email="$(echo "$email" | tr '[:upper:]' '[:lower:]')"
    [[ "$email" =~ ^[^@[:space:]]+@[^@[:space:]]+\.[a-z]{2,}$ ]] || { echo "roster: not an email: $email" >&2; exit 1; }
    [[ "$roles" =~ ^(moderator|lead)(,(moderator|lead))*$ ]] || { echo "roster: roles for $email must be moderator and/or lead" >&2; exit 1; }
    [ -z "${seen[$email]:-}" ] || { echo "roster: $email listed twice" >&2; exit 1; }
    seen[$email]=1; staff=$((staff+1)); [[ "$roles" == *lead* ]] && leads=$((leads+1))
    entries+=("$email:$roles")
  done < "$roster"
  [ "$staff" -ge 3 ] && [ "$leads" -ge 1 ] || { echo "roster: need at least 3 people and 1 lead (have $staff, $leads leads)" >&2; exit 1; }
  (IFS=';'; printf '%s' "${entries[*]}") > "$tmp/roster"
  put STAFF_ROSTER "$tmp/roster"
  echo "roster: $staff people, $leads leads. Redeploy the safety service to apply."
fi
