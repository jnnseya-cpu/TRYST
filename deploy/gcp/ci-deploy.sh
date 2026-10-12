#!/usr/bin/env bash
# Run by .github/workflows/deploy.yml on the runner, after Google sign-in, in a directory
# holding images.tar.gz and deploy-files.tgz. Pushes the images to this environment's
# registry, ships deploy/ and scripts/ to the VM over IAP, runs deploy.sh there, and waits
# for the site to answer through the load balancer.
#   PROJECT=<project> ENV_NAME=prod|staging DOMAIN=<member site host> deploy/gcp/ci-deploy.sh
set -euo pipefail
: "${PROJECT:?}" "${ENV_NAME:?}" "${DOMAIN:?set the TRYST_DOMAIN_<ENV> variable}"
reg="europe-west2-docker.pkg.dev/$PROJECT/tryst"
vm="tryst-$ENV_NAME"; zone="europe-west2-a"
tag="${GITHUB_SHA::12}"
gcloud auth configure-docker europe-west2-docker.pkg.dev --quiet
gunzip -c images.tar.gz | docker load -q
for img in api web ops; do docker tag "tryst-$img" "$reg/$img:$tag"; docker push -q "$reg/$img:$tag"; done
digest() { docker inspect --format '{{index .RepoDigests 0}}' "$reg/$1:$tag"; }
lb_ip="$(gcloud compute addresses describe "$vm" --global --project "$PROJECT" --format='value(address)')"
gcloud compute scp deploy-files.tgz "$vm:/tmp/deploy-files.tgz" --zone "$zone" --project "$PROJECT" --tunnel-through-iap --quiet
gcloud compute ssh "$vm" --zone "$zone" --project "$PROJECT" --tunnel-through-iap --quiet --command \
  "sudo install -d /opt/tryst && sudo tar -xzf /tmp/deploy-files.tgz -C /opt/tryst && rm -f /tmp/deploy-files.tgz && \
   sudo LB_IP=$lb_ip /opt/tryst/deploy/gcp/deploy.sh $(digest api) $(digest web) $(digest ops)"
for _ in $(seq 1 30); do
  code="$(curl -sk -o /dev/null -w '%{http_code}' --resolve "$DOMAIN:443:$lb_ip" "https://$DOMAIN/" || true)"
  if [ "$code" = 200 ]; then echo "live: https://$DOMAIN ($tag)"; exit 0; fi
  sleep 10
done
echo "the site didn't answer 200 within 5 minutes of the deploy"; exit 1
