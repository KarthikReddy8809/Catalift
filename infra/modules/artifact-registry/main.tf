# Artifact Registry: one Docker repository for the server and migrate images.
# CI pushes (writer), the VM pulls (reader). Untagged images are cleaned up
# after 30 days and the 20 newest versions are always kept, so a rollback
# target is never deleted.

resource "google_artifact_registry_repository" "this" {
  project       = var.project
  location      = var.region
  repository_id = var.name
  format        = "DOCKER"
  description   = "Managed by Terraform (Catalift)"

  cleanup_policies {
    id     = "keep-newest"
    action = "KEEP"
    most_recent_versions {
      keep_count = 20
    }
  }

  cleanup_policies {
    id     = "drop-old-untagged"
    action = "DELETE"
    condition {
      tag_state  = "UNTAGGED"
      older_than = "2592000s"
    }
  }
}

resource "google_artifact_registry_repository_iam_member" "readers" {
  for_each = toset(var.readers)

  project    = var.project
  location   = var.region
  repository = google_artifact_registry_repository.this.name
  role       = "roles/artifactregistry.reader"
  member     = each.value
}

resource "google_artifact_registry_repository_iam_member" "writers" {
  for_each = toset(var.writers)

  project    = var.project
  location   = var.region
  repository = google_artifact_registry_repository.this.name
  role       = "roles/artifactregistry.writer"
  member     = each.value
}
