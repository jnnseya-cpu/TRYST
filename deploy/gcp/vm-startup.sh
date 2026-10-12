#!/usr/bin/env bash
# Runs on every boot of the TRYST VM (Terraform startup-script). Idempotent. It keeps the
# machine maintaining itself: security updates with a weekly reboot window, Docker with log
# rotation, the Ops Agent for metrics and logs, Artifact Registry access, and the TRYST
# service, which fetches secrets into memory and starts the containers.
set -euo pipefail
exec > >(logger -t tryst-startup) 2>&1
md() { curl -sf -H 'Metadata-Flavor: Google' "http://metadata.google.internal/computeMetadata/v1/instance/attributes/$1"; }
ENV_NAME="$(md tryst-env || echo prod)"
REGION="europe-west2"
export DEBIAN_FRONTEND=noninteractive

if ! command -v docker >/dev/null; then
  apt-get update -q
  apt-get install -yq ca-certificates curl gnupg unattended-upgrades rsync
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
  echo "deb [signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian bookworm stable" >/etc/apt/sources.list.d/docker.list
  curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc -o /etc/apt/keyrings/pgdg.asc
  echo "deb [signed-by=/etc/apt/keyrings/pgdg.asc] https://apt.postgresql.org/pub/repos/apt bookworm-pgdg main" >/etc/apt/sources.list.d/pgdg.list
  apt-get update -q
  apt-get install -yq docker-ce docker-ce-cli containerd.io docker-compose-plugin postgresql-client-16
fi

if ! systemctl is-active -q google-cloud-ops-agent 2>/dev/null; then
  curl -sSO https://dl.google.com/cloudagents/add-google-cloud-ops-agent-repo.sh
  bash add-google-cloud-ops-agent-repo.sh --also-install && rm -f add-google-cloud-ops-agent-repo.sh
fi

# Security updates every day; if one needs a reboot, it happens Tuesday 04:30 (quiet hours).
cat >/etc/apt/apt.conf.d/52tryst-upgrades <<'CONF'
Unattended-Upgrade::Origins-Pattern { "origin=Debian,codename=${distro_codename},label=Debian-Security"; "origin=Docker"; };
Unattended-Upgrade::Automatic-Reboot "true";
Unattended-Upgrade::Automatic-Reboot-Time "04:30";
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
CONF
mkdir -p /etc/systemd/system/apt-daily-upgrade.timer.d
printf '[Timer]\nOnCalendar=\nOnCalendar=Tue *-*-* 04:00\nRandomizedDelaySec=0\n' >/etc/systemd/system/apt-daily-upgrade.timer.d/override.conf

# Container logs rotate; nothing grows without bound.
mkdir -p /etc/docker
cat >/etc/docker/daemon.json <<'CONF'
{ "log-driver": "json-file", "log-opts": { "max-size": "20m", "max-file": "5" }, "live-restore": false }
CONF
systemctl restart docker
gcloud auth configure-docker "$REGION-docker.pkg.dev" --quiet >/dev/null 2>&1 || true

install -d -m 0755 /opt/tryst /etc/tryst /var/backups/tryst /var/backups/tryst-keys
echo "ENV=$ENV_NAME" >/etc/tryst/env
touch /etc/tryst/images.env

# Weekly image clean-up.
cat >/etc/cron.weekly/tryst-docker-prune <<'CONF'
#!/bin/sh
docker image prune -af --filter "until=336h" >/dev/null 2>&1 || true
CONF
chmod +x /etc/cron.weekly/tryst-docker-prune

# The TRYST service and the off-site backup timer arrive with the first deploy
# (deploy/gcp is synced to /opt/tryst/deploy/gcp by the deploy workflow).
if [ -f /opt/tryst/deploy/gcp/tryst.service ]; then
  cp /opt/tryst/deploy/gcp/tryst.service /opt/tryst/deploy/gcp/tryst-offsite-backup.service \
     /opt/tryst/deploy/gcp/tryst-offsite-backup.timer /etc/systemd/system/
  systemctl daemon-reload
  systemctl enable --now tryst-offsite-backup.timer
  systemctl enable tryst
  if grep -q '^API_IMAGE=' /etc/tryst/images.env; then systemctl start tryst; fi
fi
