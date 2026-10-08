# The Catalift deployment (ADR-0002, ADR-0013): one VM running docker compose
# behind Caddy, images from Artifact Registry, secrets from Secret Manager,
# the web build and the nightly database dumps in Cloud Storage.

# The APIs this file needs. Declared in the root because it is one for_each
# over service names, with nothing a module would add; disable_on_destroy is
# off so a destroy here never switches an API off for other users.
resource "google_project_service" "this" {
  for_each = toset([
    "compute.googleapis.com",
    "artifactregistry.googleapis.com",
    "secretmanager.googleapis.com",
    "iap.googleapis.com",
    "logging.googleapis.com",
    "monitoring.googleapis.com",
  ])

  project            = var.project
  service            = each.value
  disable_on_destroy = false
}

locals {
  ci_member = var.ci_service_account == "" ? [] : ["serviceAccount:${var.ci_service_account}"]
  vm_member = "serviceAccount:${module.vm_service_account.email}"
}

module "vm_service_account" {
  source = "../../modules/service-account"

  account_id   = "catalift-${local.env}-vm"
  display_name = "Catalift VM (${local.env})"
  project      = var.project
  roles = [
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
  ]
}

module "registry" {
  source = "../../modules/artifact-registry"

  project = var.project
  region  = var.region
  name    = "catalift"
  readers = [local.vm_member]
  writers = local.ci_member

  depends_on = [google_project_service.this]
}

# Values are added by hand (docs/deploy/DEPLOYMENT.md), never through state.
module "secrets" {
  source   = "../../modules/secret"
  for_each = toset(["catalift-openrouter-key", "catalift-postgres-password"])

  project   = var.project
  region    = var.region
  secret_id = each.value
  accessors = [local.vm_member]

  depends_on = [google_project_service.this]
}

# Declared in the root because prevent_destroy must be a literal. Postgres,
# product images, the web builds and Caddy's certificates live here; the
# daily snapshot schedule is attached in module.app_host.
resource "google_compute_disk" "data" {
  name    = "catalift-${local.env}-data"
  project = var.project
  zone    = var.zone
  type    = "pd-balanced"
  size    = var.data_disk_gb

  labels = {
    managed-by = "terraform"
  }

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.this]
}

# Nightly pg_dump files, deleted after 14 days (decided 2026-10-08). In the
# root for the literal prevent_destroy.
resource "google_storage_bucket" "dumps" {
  name                        = "${var.project}-catalift-dumps"
  project                     = var.project
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  versioning {
    enabled = false
  }

  lifecycle_rule {
    condition {
      age = var.dump_retention_days
    }
    action {
      type = "Delete"
    }
  }

  lifecycle {
    prevent_destroy = true
  }
}

# Web builds CI uploads as web/<tag>.tar.gz; deploy.sh unpacks one on the VM.
# Kept 90 days so any recent tag can be rolled back to. In the root with the
# dump bucket, as the two buckets differ only in names and rules.
resource "google_storage_bucket" "web" {
  name                        = "${var.project}-catalift-web"
  project                     = var.project
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false

  versioning {
    enabled = false
  }

  lifecycle_rule {
    condition {
      age = 90
    }
    action {
      type = "Delete"
    }
  }
}

resource "google_storage_bucket_iam_member" "vm_reads_web" {
  bucket = google_storage_bucket.web.name
  role   = "roles/storage.objectViewer"
  member = local.vm_member
}

resource "google_storage_bucket_iam_member" "vm_writes_dumps" {
  bucket = google_storage_bucket.dumps.name
  role   = "roles/storage.objectCreator"
  member = local.vm_member
}

resource "google_storage_bucket_iam_member" "ci_uploads_web" {
  for_each = toset(local.ci_member)

  bucket = google_storage_bucket.web.name
  role   = "roles/storage.objectCreator"
  member = each.value
}

module "app_host" {
  source = "../../modules/app-host"

  project               = var.project
  region                = var.region
  zone                  = var.zone
  name                  = "catalift-${local.env}"
  machine_type          = var.machine_type
  network               = module.network.network_name
  subnetwork            = module.network.subnet_ids["catalift-infra-${local.env}-apps"]
  service_account_email = module.vm_service_account.email
  data_disk_self_link   = google_compute_disk.data.self_link
  data_disk_name        = google_compute_disk.data.name
  domain                = var.domain

  # What deploy.sh and backup.sh read from the metadata server.
  metadata = {
    catalift-registry    = module.registry.url
    catalift-web-bucket  = google_storage_bucket.web.name
    catalift-dump-bucket = google_storage_bucket.dumps.name
  }

  depends_on = [google_project_service.this]
}

# The CI account deploys over IAP SSH: it may open a tunnel to this VM, log in
# with sudo through OS Login, and act as the VM's service account. Granted on
# the VM and the account only, never on the project.
resource "google_iap_tunnel_instance_iam_member" "ci" {
  for_each = toset(local.ci_member)

  project  = var.project
  zone     = var.zone
  instance = module.app_host.name
  role     = "roles/iap.tunnelResourceAccessor"
  member   = each.value
}

resource "google_compute_instance_iam_member" "ci_login" {
  for_each = toset(local.ci_member)

  project       = var.project
  zone          = var.zone
  instance_name = module.app_host.name
  role          = "roles/compute.osAdminLogin"
  member        = each.value
}

resource "google_service_account_iam_member" "ci_acts_as_vm" {
  for_each = toset(local.ci_member)

  service_account_id = module.vm_service_account.id
  role               = "roles/iam.serviceAccountUser"
  member             = each.value
}
