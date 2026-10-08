output "url" {
  description = "Image path prefix, such as asia-south1-docker.pkg.dev/project/catalift."
  value       = "${var.region}-docker.pkg.dev/${var.project}/${google_artifact_registry_repository.this.repository_id}"
}

output "name" {
  description = "Repository id."
  value       = google_artifact_registry_repository.this.repository_id
}
