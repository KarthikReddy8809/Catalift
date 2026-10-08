# Deploying Catalift

One VM in `asia-south1` runs everything with docker compose (ADR-0002,
ADR-0013). Only the owner applies Terraform and approves deploys.

```
Browser ──HTTPS──▶ Caddy (VM, :443) ──/v1──▶ api ──▶ postgres
                     │                         worker ──▶ OpenRouter
                     └── /srv/web/current (web build on the data disk)

GitHub Actions ──push──▶ Artifact Registry: server:<tag>, migrate:<tag>
               ──upload─▶ gs://<project>-catalift-web/web/<tag>.tar.gz
               ──IAP SSH─▶ VM: deploy.sh <tag>
```

| Piece | Where | Made by |
| --- | --- | --- |
| VM, IP, firewall, data disk, snapshots | `infra/modules/app-host`, `infra/envs/prod/app.tf` | Terraform |
| Image registry, buckets, secrets, service accounts | `infra/envs/prod/app.tf` | Terraform |
| Compose file, Caddyfile, deploy and backup scripts | `deploy/` | copied to `/opt/catalift` by each deploy |
| Server and migrate images, web build | `server/Dockerfile`, `server/Dockerfile.migrate`, `apps/web` | `.github/workflows/release.yml` |

On the VM the data disk is mounted at `/mnt/data`: `postgres/` (database),
`app/` (product images, exports), `web/releases/<tag>` with `web/current`
pointing at the live build, and `caddy/` (certificates).

## First time only

### 1. Bootstrap: state bucket, CI account, Workload Identity Federation

These sit outside the Terraform they enable (ADR-0011). Run once, as the
project owner, with your project id in `PROJECT`:

```bash
PROJECT=my-gcp-project-prod
REPO=KarthikReddy8809/catalift
gcloud config set project "$PROJECT"
gcloud services enable iamcredentials.googleapis.com sts.googleapis.com

# Terraform state (versioned, private).
gcloud storage buckets create "gs://$PROJECT-tfstate" --location=asia-south1 --uniform-bucket-level-access --public-access-prevention
gcloud storage buckets update "gs://$PROJECT-tfstate" --versioning

# The CI service account. Terraform grants it only what deploys need.
gcloud iam service-accounts create catalift-ci --display-name="Catalift CI"

# Trust GitHub Actions from this repository's main branch only.
gcloud iam workload-identity-pools create github --location=global
gcloud iam workload-identity-pools providers create-oidc github \
  --location=global --workload-identity-pool=github \
  --issuer-uri=https://token.actions.githubusercontent.com \
  --attribute-mapping=google.subject=assertion.sub,attribute.repository=assertion.repository,attribute.ref=assertion.ref \
  --attribute-condition="assertion.repository=='$REPO' && assertion.ref=='refs/heads/main'"
NUM=$(gcloud projects describe "$PROJECT" --format='value(projectNumber)')
gcloud iam service-accounts add-iam-policy-binding "catalift-ci@$PROJECT.iam.gserviceaccount.com" \
  --role=roles/iam.workloadIdentityUser \
  --member="principalSet://iam.googleapis.com/projects/$NUM/locations/global/workloadIdentityPools/github/attribute.repository/$REPO"
```

### 2. Terraform plan and apply

```bash
cd infra
cp .env.example .env              # TF_BACKEND_BUCKET=<project>-tfstate, project, region
cp envs/prod/terraform.tfvars.example envs/prod/terraform.tfvars
#   set project, and ci_service_account = "catalift-ci@<project>.iam.gserviceaccount.com"
make init
make plan                         # read .plans/prod.txt: it must destroy nothing
```

Apply the reviewed plan (you, never CI on its own):

```bash
terraform -chdir=envs/prod apply ../../.plans/prod.tfplan
terraform -chdir=envs/prod output
```

The VM boots and its startup script installs Docker, the compose plugin and
the Ops Agent, and formats and mounts the data disk (only when blank).

### 3. Put the two secret values in

Terraform creates the secrets empty, so the values never enter its state.
Run each command and paste the value when it waits for input:

```bash
gcloud secrets versions add catalift-openrouter-key --data-file=-
openssl rand -hex 24 | gcloud secrets versions add catalift-postgres-password --data-file=-
```

The Postgres password is hex so it is safe inside the connection URL. Set it
before the first deploy: Postgres keeps the password it was first started with.

### 4. GitHub settings

- **Environments > prod**: add yourself as required reviewer.
- **Secrets and variables > Actions > Variables**, from `terraform output`:

| Variable | Value |
| --- | --- |
| `GCP_PROJECT` | project id |
| `GCP_REGION` | `asia-south1` |
| `GCP_ZONE` | `vm_zone` |
| `GCP_WIF_PROVIDER` | `projects/<number>/locations/global/workloadIdentityPools/github/providers/github` |
| `GCP_CI_SERVICE_ACCOUNT` | `catalift-ci@<project>.iam.gserviceaccount.com` |
| `CATALIFT_VM` | `vm_name` |
| `CATALIFT_REGISTRY` | `registry_url` |
| `CATALIFT_WEB_BUCKET` | `web_bucket` |

### 5. First deploy and the demo accounts

Actions > **release** > Run workflow on `main`, leave **tag** empty, approve
the `prod` deployment. Then create the two accounts once:

```bash
gcloud compute ssh catalift-prod --zone asia-south1-a --tunnel-through-iap
cd /opt/catalift
sudo docker compose run --rm --entrypoint /admin api create-user seller@example.com seller
sudo docker compose run --rm --entrypoint /admin api create-user reviewer@example.com reviewer
```

Each command reads the password from standard input. Open `site_url` from
`terraform output`.

## Every deploy

1. Merge to `main`. The **release** workflow builds `server:<sha>`,
   `migrate:<sha>` and the web build, and pushes them.
2. Actions > **release** > Run workflow (tag empty) and approve `prod`. It copies
   `deploy/` to the VM and runs `deploy.sh <sha>`:
   secrets to `.env`, pull, migrations, web switch, restart.

A failed migration stops the deploy before the API or the worker restarts.
The API and the worker also refuse to start against a database missing a
migration. Expect a few seconds of downtime; the worker finishes in-flight
AI calls first (up to 75 seconds).

## Rollback

Actions > **release** > Run workflow with **tag** set to the previous short
sha (from `/opt/catalift/deployed-tag` history or the registry). It skips the
build and deploys that tag; the last five web builds stay on the VM, older
ones come back from the bucket (kept 90 days). Migrations only go forward: a
rollback across a migration needs that migration's Down run by hand first
(`make migrate-down` against the VM's database through an IAP tunnel).

## Backups and restore

| What | How often | Kept | Holds |
| --- | --- | --- | --- |
| Data disk snapshot | daily, 02:30 IST | 7 days | database, product images, web builds, certificates |
| `pg_dump` to `gs://<project>-catalift-dumps` | nightly, 03:00 IST | 14 days | database only |

Restore the database from a dump:

```bash
gcloud storage cp gs://<project>-catalift-dumps/catalift-<time>.dump /tmp/restore.dump   # on the VM
cd /opt/catalift && sudo docker compose stop api worker
sudo docker compose exec -T postgres pg_restore -U catalift -d catalift --clean --if-exists < /tmp/restore.dump
sudo docker compose start api worker
```

Restore everything (images too) from a snapshot: create a disk from the
snapshot in the console or with `gcloud compute disks create ... --source-snapshot`,
then import it in place of `google_compute_disk.data` in a planned change.
No restore has been rehearsed yet; do one before the first launch.

## Logs and health

- Container logs: Cloud Logging, filter `logName:"gcplogs-docker-driver"`.
- VM metrics and system logs: the Ops Agent (ADR-0010).
- `https://<site>/healthz` (process up) and `/readyz` (database reachable).
- On the VM: `cd /opt/catalift && sudo docker compose ps` and `sudo docker compose logs -f api worker`.
