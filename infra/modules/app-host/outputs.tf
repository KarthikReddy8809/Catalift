output "ip_address" {
  description = "Static external IP; point the domain's A record here."
  value       = google_compute_address.this.address
}

output "site_address" {
  description = "Where Catalift is served: the domain, or <ip>.sslip.io."
  value       = local.site_address
}

output "name" {
  description = "VM name, for gcloud compute ssh --tunnel-through-iap."
  value       = google_compute_instance.this.name
}

output "zone" {
  description = "VM zone."
  value       = google_compute_instance.this.zone
}
