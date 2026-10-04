# prod environment root. Wires modules and passes values; declares no
# resource of its own without a comment saying why.

provider "google" {
  project = var.project
  region  = var.region
}

locals {
  env = "prod"
}

module "network" {
  source = "../../modules/network"

  name    = "catalift-infra-${local.env}"
  project = var.project
  region  = var.region

  subnets = {
    "catalift-infra-${local.env}-apps" = {
      region = var.region
      cidr   = var.apps_cidr
      secondary_ranges = {
        pods     = var.pods_cidr
        services = var.services_cidr
      }
    }
  }

  enable_nat        = true
  flow_log_sampling = 0.5
}

module "app_service_account" {
  source = "../../modules/service-account"

  account_id   = "catalift-infra-${local.env}-app"
  display_name = "CataliftInfra app (${local.env})"
  project      = var.project
  roles = [
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
  ]
  workload_identity_user = "serviceAccount:${var.project}.svc.id.goog[catalift-infra-${local.env}/catalift-infra]"
}

# Declared in the root rather than a module because `prevent_destroy` must
# be a literal (Terraform forbids variables in lifecycle blocks) and only
# prod carries it. This comment is the reason the "no resource without a
# module" rule asks for. Versioned, private, deletion-protected.
resource "google_storage_bucket" "data" {
  name                        = "${var.project}-catalift-infra-data"
  project                     = var.project
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  versioning {
    enabled = true
  }

  lifecycle {
    prevent_destroy = true
  }
}
