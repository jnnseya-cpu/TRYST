#!/usr/bin/env bash
# Rolls out new images by digest on the VM.
#
#   deploy/gcp/deploy.sh <api@sha256:...> <web@sha256:...> <ops@sha256:...>
#
# api and safety keep their working state in memory and must never run twice: the old
# container is stopped before the new one starts (a database lock also enforces it, and
# the new one waits for it). The web containers are stateless and restart in place.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
images=/etc/tryst/images.env
[ $# -eq 3 ] || { echo "usage: $0 API_IMAGE WEB_IMAGE OPS_IMAGE (pinned by digest)"; exit 2; }
for i in "$@"; do [[ "$i" == *@sha256:* ]] || { echo "pin images by digest: $i"; exit 2; }; done
install -d -m 0755 /etc/tryst
cp -f "$images" "$images.previous" 2>/dev/null || true
{ grep -E '^(LB_IP|SITE_HOST|CONSOLE_HOST)=' "$images" 2>/dev/null || true
  echo "API_IMAGE=$1"; echo "WEB_IMAGE=$2"; echo "OPS_IMAGE=$3"; } >"$images.new"
mv "$images.new" "$images"
dc() { docker compose -f "$here/compose.yml" --env-file "$images" "$@"; }
dc pull api web ops
for svc in safety api; do      # one copy only: stop, then start
  dc stop "$svc"
  dc up -d --no-deps "$svc"
done
dc up -d --wait
dc ps
echo "deployed. To roll back, run this script again with the three digests in $images.previous"
