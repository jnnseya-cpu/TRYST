#!/usr/bin/env bash
# Every 2 minutes (tryst-settings.timer): when a safety lead presses "Apply" on the staff
# console's Vendor keys page, the console adds a version to SETTINGS_REVISION. This notices
# the new version and restarts TRYST, which re-reads every secret at start (fetch-secrets.sh).
# Nothing else restarts the services, so saving keys one by one never causes downtime.
set -euo pipefail
env_name="${ENV:?}"
state=/var/lib/tryst/settings-revision
install -d -m 0700 /var/lib/tryst
latest="$(gcloud secrets versions list "tryst-$env_name-SETTINGS_REVISION" --filter=state:ENABLED \
  --sort-by=~createTime --limit=1 --format='value(name)' 2>/dev/null || true)"
[ -n "$latest" ] || exit 0                       # never applied yet
if [ ! -f "$state" ]; then echo "$latest" >"$state"; exit 0; fi  # first run: just remember it
[ "$(cat "$state")" = "$latest" ] && exit 0
echo "$latest" >"$state"
echo "vendor keys changed (revision $latest): restarting TRYST"
systemctl restart tryst
