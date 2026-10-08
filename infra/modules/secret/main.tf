# A Secret Manager secret with no value: the operator adds the value once by
# hand (gcloud secrets versions add), so it never passes through Terraform
# state. Replicas stay in one region (data residency); only the named
# members may read it.

resource "google_secret_manager_secret" "this" {
  project   = var.project
  secret_id = var.secret_id

  replication {
    user_managed {
      replicas {
        location = var.region
      }
    }
  }

  labels = {
    managed-by = "terraform"
  }
}

resource "google_secret_manager_secret_iam_member" "accessors" {
  for_each = toset(var.accessors)

  project   = var.project
  secret_id = google_secret_manager_secret.this.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = each.value
}
