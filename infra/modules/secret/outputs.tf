output "secret_id" {
  description = "Secret name, for gcloud secrets versions add and access."
  value       = google_secret_manager_secret.this.secret_id
}
