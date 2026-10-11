#!/usr/bin/env bash
# Logical backup of every TRYST database (D-57). Production also runs the managed
# database's point-in-time recovery; this is the portable, encrypted copy and the input to
# the restore drill (scripts/restore-drill.sh, docs/runbooks/restore.md).
#
#   BACKUP_DIR=/backups DATABASE_URL_IDENTITY=... (all six) scripts/backup.sh
#
# Optional:
#   BACKUP_KEY_FILE   passphrase file; every dump is encrypted (AES-256, PBKDF2) and the
#                     plaintext removed
#   KEYS_BACKUP_DIR   separate destination for the key database, which has its own,
#                     shorter retention (7 days) so crypto-erasure reaches backups quickly
# The erasure ledger is backed up too, but restores always use the live ledger.
set -euo pipefail
stamp=$(date -u +%Y%m%dT%H%M%SZ)
out="${BACKUP_DIR:?BACKUP_DIR is required}/$stamp"
keys_out="${KEYS_BACKUP_DIR:-$BACKUP_DIR}/$stamp"
mkdir -p "$out" "$keys_out"
for db in IDENTITY PROFILE KEYS SAFETY COMMERCE LEDGER; do
  var="DATABASE_URL_$db"
  dsn="${!var:?$var is required}"
  name=$(echo "$db" | tr '[:upper:]' '[:lower:]')
  dir="$out"; [ "$db" = KEYS ] && dir="$keys_out"
  pg_dump --format=custom --no-owner --no-privileges --dbname="$dsn" --file="$dir/$name.dump"
  if [ -n "${BACKUP_KEY_FILE:-}" ]; then
    openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt -pass "file:$BACKUP_KEY_FILE" \
      -in "$dir/$name.dump" -out "$dir/$name.dump.enc"
    rm -f "$dir/$name.dump"
  fi
done
( cd "$out" && sha256sum ./* > MANIFEST.sha256 )
[ "$keys_out" != "$out" ] && ( cd "$keys_out" && sha256sum ./* > MANIFEST.sha256 )
echo "$out"
