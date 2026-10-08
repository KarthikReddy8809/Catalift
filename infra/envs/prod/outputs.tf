# Everything another repository consumes from prod is an output here.

output "network_id" {
  description = "VPC id for cluster and firewall configuration."
  value       = module.network.network_id
}

output "network_name" {
  description = "VPC name."
  value       = module.network.network_name
}

output "subnet_ids" {
  description = "Subnet ids keyed by name."
  value       = module.network.subnet_ids
}

output "subnet_secondary_ranges" {
  description = "Secondary range names per subnet, for GKE ip allocation."
  value       = module.network.subnet_secondary_ranges
}

output "app_service_account_email" {
  description = "Email of the application service account; the workload's Kubernetes service account annotates with it."
  value       = module.app_service_account.email
}

output "data_bucket" {
  description = "Name of the prod data bucket (versioned, deletion-protected)."
  value       = google_storage_bucket.data.name
}

output "registry_url" {
  description = "Image prefix CI pushes to: <registry_url>/server:<tag> and <registry_url>/migrate:<tag>."
  value       = module.registry.url
}

output "vm_name" {
  description = "Catalift VM, for gcloud compute ssh --tunnel-through-iap."
  value       = module.app_host.name
}

output "vm_zone" {
  description = "Zone of the Catalift VM."
  value       = module.app_host.zone
}

output "vm_ip" {
  description = "Static IP of the Catalift VM; a custom domain's A record points here."
  value       = module.app_host.ip_address
}

output "site_url" {
  description = "Where Catalift is served."
  value       = "https://${module.app_host.site_address}"
}

output "web_bucket" {
  description = "Bucket CI uploads web/<tag>.tar.gz to."
  value       = google_storage_bucket.web.name
}

output "dump_bucket" {
  description = "Bucket the nightly database dumps go to."
  value       = google_storage_bucket.dumps.name
}
