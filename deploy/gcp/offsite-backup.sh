#!/usr/bin/env bash
# Weekly: encrypted logical backups of every database (scripts/backup.sh). The main and
# ledger dumps are copied to the Hostinger VPS (OFFSITE_TARGET, e.g.
# backup@vps.example:/srv/tryst-backups) with the key in OFFSITE_SSH_KEY. The keys database
# dump stays on Google Cloud and is deleted after 7 days: its short life is what makes a
# member's deletion permanent. Cloud SQL's own backups stay the primary restore path.
# Does nothing until OFFSITE_TARGET is set.
set -euo pipefail
env_name="${ENV:?}"
secret() { gcloud secrets versions access latest --secret="tryst-$env_name-$1" 2>/dev/null; }
target="$(secret OFFSITE_TARGET || true)"
if [ -z "$target" ]; then echo "off-site backup not configured (OFFSITE_TARGET)"; exit 0; fi
umask 077
work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
secret BACKUP_KEY >"$work/key"
secret OFFSITE_SSH_KEY >"$work/ssh"
set -a
# shellcheck disable=SC1091
. /run/tryst/api.env
. /run/tryst/safety.env
set +a
# Keep at most one key-database dump: this week's replaces last week's before it is made, so
# none is ever older than the weekly timer's 7 days.
find /var/backups/tryst-keys -mindepth 1 -maxdepth 1 -exec rm -rf {} + 2>/dev/null || true
out="$(BACKUP_DIR=/var/backups/tryst KEYS_BACKUP_DIR=/var/backups/tryst-keys BACKUP_KEY_FILE="$work/key" \
  /opt/tryst/scripts/backup.sh)"
rsync -a -e "ssh -i $work/ssh -o StrictHostKeyChecking=accept-new -o BatchMode=yes" "$out" "$target/"
find /var/backups/tryst -mindepth 1 -maxdepth 1 -mtime +7 -exec rm -rf {} +
echo "off-site copy done: $(basename "$out")"
