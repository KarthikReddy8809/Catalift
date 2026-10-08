#!/usr/bin/env bash
# backup.sh: dump the database to the dump bucket (run nightly by
# catalift-backup.timer). The bucket deletes dumps after 14 days; disk
# snapshots keep 7 days of everything, product images included (ADR-0013).
set -euo pipefail

meta() {
  curl -fsS -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/attributes/$1"
}
bucket=$(meta catalift-dump-bucket)
name="catalift-$(date -u +%Y%m%dT%H%M%SZ).dump"

cd /opt/catalift
docker compose exec -T postgres pg_dump -U catalift -d catalift --format=custom \
  | gcloud storage cp - "gs://$bucket/$name"
echo "backup: gs://$bucket/$name"
