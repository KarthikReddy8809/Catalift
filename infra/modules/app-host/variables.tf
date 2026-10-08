variable "project" {
  description = "Google Cloud project id."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.project))
    error_message = "project must be a valid Google Cloud project id (6 to 30 lowercase characters)."
  }
}

variable "region" {
  description = "Region of the address, the snapshots and the registry the VM pulls from."
  type        = string

  validation {
    condition     = can(regex("^[a-z]+-[a-z]+[0-9]$", var.region))
    error_message = "region must look like asia-south1."
  }
}

variable "zone" {
  description = "Zone of the VM; the data disk must be in the same zone."
  type        = string

  validation {
    condition     = can(regex("^[a-z]+-[a-z]+[0-9]-[a-z]$", var.zone))
    error_message = "zone must look like asia-south1-a."
  }
}

variable "name" {
  description = "VM name, also the prefix of its address, firewall rules and snapshot policy, and its network tag."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{0,40}[a-z0-9]$", var.name))
    error_message = "name must be lowercase letters, digits and hyphens, at most 42 characters."
  }
}

variable "machine_type" {
  description = "Machine type. e2-small fits about 10 users a minute plus Postgres (ADR-0002)."
  type        = string
  default     = "e2-small"

  validation {
    condition     = can(regex("^[a-z0-9]+-[a-z0-9-]+$", var.machine_type))
    error_message = "machine_type must look like e2-small."
  }
}

variable "boot_disk_gb" {
  description = "Boot disk size in GB; images and data live on the data disk."
  type        = number
  default     = 20

  validation {
    condition     = var.boot_disk_gb >= 10 && var.boot_disk_gb <= 100
    error_message = "boot_disk_gb must be between 10 and 100."
  }
}

variable "network" {
  description = "VPC name or self link the firewall rules apply to."
  type        = string

  validation {
    condition     = length(var.network) > 0
    error_message = "network must not be empty."
  }
}

variable "subnetwork" {
  description = "Subnet id or self link the VM's interface joins."
  type        = string

  validation {
    condition     = length(var.subnetwork) > 0
    error_message = "subnetwork must not be empty."
  }
}

variable "service_account_email" {
  description = "Service account the VM runs as (pulls images, reads its secrets, writes logs)."
  type        = string

  validation {
    condition     = can(regex("@.+\\.iam\\.gserviceaccount\\.com$", var.service_account_email))
    error_message = "service_account_email must be a service account email."
  }
}

variable "data_disk_self_link" {
  description = "Self link of the data disk (Postgres, product images, web builds, certificates)."
  type        = string

  validation {
    condition     = length(var.data_disk_self_link) > 0
    error_message = "data_disk_self_link must not be empty."
  }
}

variable "data_disk_name" {
  description = "Name of the data disk, for the snapshot schedule."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{0,61}[a-z0-9]$", var.data_disk_name))
    error_message = "data_disk_name must be lowercase letters, digits and hyphens."
  }
}

variable "snapshot_retention_days" {
  description = "Days a daily snapshot of the data disk is kept (decided 2026-10-08: 7)."
  type        = number
  default     = 7

  validation {
    condition     = var.snapshot_retention_days >= 1 && var.snapshot_retention_days <= 365
    error_message = "snapshot_retention_days must be between 1 and 365."
  }
}

variable "metadata" {
  description = "Extra instance metadata, such as the settings deploy.sh reads (catalift-registry and others)."
  type        = map(string)
  default     = {}

  validation {
    condition     = alltrue([for k in keys(var.metadata) : !contains(["startup-script", "enable-oslogin"], k)])
    error_message = "metadata may not override startup-script or enable-oslogin; the module sets them."
  }
}

variable "domain" {
  description = "Domain Caddy serves, with its A record on the static IP. Empty uses <ip>.sslip.io."
  type        = string
  default     = ""

  validation {
    condition     = var.domain == "" || can(regex("^([a-z0-9-]+\\.)+[a-z]{2,}$", var.domain))
    error_message = "domain must be empty or a lowercase host name such as catalift.example.com."
  }
}

variable "deletion_protection" {
  description = "Refuse to delete the VM until this is turned off. The data disk has its own protection."
  type        = bool
  default     = true
}
