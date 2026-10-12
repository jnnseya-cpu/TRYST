#!/usr/bin/env bash
# Outside check from the Hostinger VPS: if the site stops answering, email on-call through
# the Hostinger mailbox. Catches an outage of Google Cloud itself, which Google's own
# monitoring might not report. Install once:
#   sudo cp uptime-check.sh /usr/local/bin/ && sudo install -m 600 uptime.env /etc/tryst-uptime.env
#   echo '* * * * * root /usr/local/bin/uptime-check.sh' | sudo tee /etc/cron.d/tryst-uptime
# /etc/tryst-uptime.env: SITE=https://tryst.example  ALERT_TO=oncall@...  SMTP_URL=smtps://smtp.hostinger.com:465
#                        SMTP_USER=alerts@...  SMTP_PASSWORD=...
set -euo pipefail
# shellcheck disable=SC1091
. /etc/tryst-uptime.env
state=/var/tmp/tryst-uptime.state
code="$(curl -s -o /dev/null -m 15 -w '%{http_code}' "$SITE/" || echo 000)"
prev="$(cat "$state" 2>/dev/null || echo up)"
send() {
  printf 'From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n' "$SMTP_USER" "$ALERT_TO" "$1" "$2" |
    curl -s --ssl-reqd --url "$SMTP_URL" --user "$SMTP_USER:$SMTP_PASSWORD" --mail-from "$SMTP_USER" --mail-rcpt "$ALERT_TO" -T - || true
}
if [ "$code" = 200 ]; then
  [ "$prev" = down ] && send "RECOVERED: $SITE" "The site answers again ($(date -u))."
  echo up >"$state"
else
  # Two failures in a row before paging, to ride out a single slow response.
  if [ "$prev" = up ]; then echo maybe >"$state"
  elif [ "$prev" = maybe ]; then send "DOWN: $SITE" "The site answered $code at $(date -u). Check Google Cloud and the VM."; echo down >"$state"
  fi
fi
