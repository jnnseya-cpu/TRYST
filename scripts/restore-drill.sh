#!/usr/bin/env bash
# Restore drill (D-57; docs/runbooks/restore.md). Restores a backup set into scratch
# databases, checks it loads and decrypts, then re-applies every erasure in the LIVE
# ledger, so no account deleted after the backup comes back. Prints timings (the drill's
# recovery time) and counts only.
#
#   DRILL_ADMIN_DSN=postgres://.../postgres  DRILL_PREFIX=tryst_drill \
#   DRILL_LEDGER_URL=<live ledger DSN>  TRYST_BIN=./identity-svc  [SAFETY_BIN=./safety-svc] \
#   [BACKUP_KEY_FILE=...] [KEYS_BACKUP=<dir holding keys.dump>] \
#   (plus the service's secrets: DATA_KEK, DSR_SIGNING_KEY, ...) scripts/restore-drill.sh <backup-dir>
set -euo pipefail
src="${1:?backup directory}"
keys_src="${KEYS_BACKUP:-$src}"
prefix="${DRILL_PREFIX:-tryst_drill}"
admin="${DRILL_ADMIN_DSN:?DRILL_ADMIN_DSN is required}"
bin="${TRYST_BIN:?TRYST_BIN is required}"
start=$(date +%s)
( cd "$src" && sha256sum --quiet -c MANIFEST.sha256 )
dsn_for() { # replace the database name in the admin DSN
  echo "$admin" | sed -E "s#/[^/?]+(\?|$)#/$1\1#"
}
for db in identity profile keys safety commerce; do
  name="${prefix}_$db"
  psql "$admin" -qc "drop database if exists $name" -c "create database $name"
  dir="$src"; [ "$db" = keys ] && dir="$keys_src"
  file="$dir/$db.dump"
  if [ ! -f "$file" ] && [ -f "$file.enc" ]; then
    tmp=$(mktemp); openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000 -pass "file:${BACKUP_KEY_FILE:?}" -in "$file.enc" -out "$tmp"; file="$tmp"
  fi
  pg_restore --no-owner --no-privileges --dbname="$(dsn_for "$name")" "$file"
  export "DATABASE_URL_$(echo "$db" | tr '[:lower:]' '[:upper:]')=$(dsn_for "$name")"
done
export DATABASE_URL_LEDGER="${DRILL_LEDGER_URL:?DRILL_LEDGER_URL (the live ledger) is required}"
restored=$(date +%s)
echo "== verify (as restored)"
"$bin" admin verify
echo "== reapply erasures from the live ledger"
"$bin" admin reapply-erasures
echo "== verify (after reapply)"
"$bin" admin verify
if [ -n "${SAFETY_BIN:-}" ]; then
  echo "== safety service: verify, reapply erasures, verify"
  "$SAFETY_BIN" admin verify
  "$SAFETY_BIN" admin reapply-erasures
  "$SAFETY_BIN" admin verify
fi
echo "restore: $((restored - start)) s, total: $(( $(date +%s) - start )) s"
