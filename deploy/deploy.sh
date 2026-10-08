#!/usr/bin/env bash
# deploy.sh TAG: put one build of Catalift live on this VM (run as root).
#
#   1. read settings from the instance metadata and secrets from Secret Manager
#   2. write /opt/catalift/.env (root only)
#   3. pull the server and migrate images for TAG from Artifact Registry
#   4. unpack the web build for TAG from Cloud Storage and switch to it
#   5. run the migrations, then restart the API, the worker and Caddy
#
# Rollback is the same command with the previous tag. A failed migration
# stops the deploy before the API or the worker restarts.
set -euo pipefail

tag="${1:?usage: deploy.sh TAG (the commit short sha CI built)}"
[[ "$tag" =~ ^[A-Za-z0-9._-]{1,64}$ ]] || { echo "deploy: bad tag: $tag" >&2; exit 2; }
[[ $EUID -eq 0 ]] || { echo "deploy: run as root (sudo)" >&2; exit 2; }

dir=/opt/catalift
web=/mnt/data/web
cd "$dir"

meta() {
  curl -fsS -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/attributes/$1"
}
registry=$(meta catalift-registry)
web_bucket=$(meta catalift-web-bucket)
site=$(meta catalift-site-address)
secret() { gcloud secrets versions access latest --secret="$1"; }

echo "deploy: $tag to $site"
umask 077
{
  echo "TAG=$tag"
  echo "REGISTRY=$registry"
  echo "SITE_ADDRESS=$site"
  echo "POSTGRES_PASSWORD=$(secret catalift-postgres-password)"
  echo "OPENROUTER_API_KEY=$(secret catalift-openrouter-key)"
} > .env.next
mv .env.next .env
umask 022

docker compose pull --quiet migrate api worker

# The web build: one folder per tag, then an atomic switch of the link.
release="$web/releases/$tag"
if [[ ! -d "$release" ]]; then
  tmp=$(mktemp -d)
  gcloud storage cp "gs://$web_bucket/web/$tag.tar.gz" "$tmp/web.tar.gz"
  mkdir -p "$release.partial"
  tar -xzf "$tmp/web.tar.gz" -C "$release.partial"
  [[ -f "$release.partial/index.html" ]] || { echo "deploy: web build has no index.html" >&2; exit 1; }
  mv "$release.partial" "$release"
  rm -rf "$tmp"
fi

docker compose up -d postgres
docker compose run --rm migrate
ln -sfn "releases/$tag" "$web/current.next"
mv -T "$web/current.next" "$web/current"
touch "$release"  # newest by time, so the prune below never removes what is live
docker compose up -d --remove-orphans api worker caddy

# Keep the five newest web builds for rollback.
mapfile -t old < <(ls -1dt "$web"/releases/*/ 2>/dev/null | tail -n +6)
((${#old[@]})) && rm -rf "${old[@]}"
docker image prune -f >/dev/null

echo "$tag" > "$dir/deployed-tag"
echo "deploy: $tag live"
