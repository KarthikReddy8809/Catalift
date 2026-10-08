variable "project" {
  description = "Google Cloud project id for prod."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.project))
    error_message = "project must be a valid Google Cloud project id (6 to 30 lowercase characters)."
  }
}

variable "region" {
  description = "Default region for regional resources."
  type        = string
  default     = "asia-south1"

  validation {
    condition     = can(regex("^[a-z]+-[a-z]+[0-9]$", var.region))
    error_message = "region must look like asia-south1."
  }
}

variable "apps_cidr" {
  description = "Primary CIDR of the applications subnet. Never overlaps another environment."
  type        = string
  default     = "10.30.0.0/20"

  validation {
    condition     = can(cidrhost(var.apps_cidr, 0))
    error_message = "apps_cidr must be a valid CIDR block."
  }
}

variable "pods_cidr" {
  description = "Secondary range for Kubernetes pods."
  type        = string
  default     = "10.31.0.0/16"

  validation {
    condition     = can(cidrhost(var.pods_cidr, 0))
    error_message = "pods_cidr must be a valid CIDR block."
  }
}

variable "services_cidr" {
  description = "Secondary range for Kubernetes services."
  type        = string
  default     = "10.32.0.0/20"

  validation {
    condition     = can(cidrhost(var.services_cidr, 0))
    error_message = "services_cidr must be a valid CIDR block."
  }
}

variable "zone" {
  description = "Zone of the Catalift VM and its data disk."
  type        = string
  default     = "asia-south1-a"

  validation {
    condition     = can(regex("^[a-z]+-[a-z]+[0-9]-[a-z]$", var.zone))
    error_message = "zone must look like asia-south1-a."
  }
}

variable "machine_type" {
  description = "Machine type of the Catalift VM (ADR-0002: about 10 users a minute)."
  type        = string
  default     = "e2-small"

  validation {
    condition     = can(regex("^[a-z0-9]+-[a-z0-9-]+$", var.machine_type))
    error_message = "machine_type must look like e2-small."
  }
}

variable "data_disk_gb" {
  description = "Size of the data disk (Postgres, product images, web builds). It can grow later, never shrink."
  type        = number
  default     = 50

  validation {
    condition     = var.data_disk_gb >= 10 && var.data_disk_gb <= 1000
    error_message = "data_disk_gb must be between 10 and 1000."
  }
}

variable "dump_retention_days" {
  description = "Days a nightly database dump is kept (decided 2026-10-08: 14)."
  type        = number
  default     = 14

  validation {
    condition     = var.dump_retention_days >= 1 && var.dump_retention_days <= 365
    error_message = "dump_retention_days must be between 1 and 365."
  }
}

variable "domain" {
  description = "Domain Caddy serves; its A record points at vm_ip. Empty serves <ip>.sslip.io."
  type        = string
  default     = ""

  validation {
    condition     = var.domain == "" || can(regex("^([a-z0-9-]+\\.)+[a-z]{2,}$", var.domain))
    error_message = "domain must be empty or a lowercase host name such as catalift.example.com."
  }
}

variable "ci_service_account" {
  description = "Email of the CI service account (bootstrap, ADR-0011) that pushes images and deploys. Empty grants nothing."
  type        = string
  default     = ""

  validation {
    condition     = var.ci_service_account == "" || can(regex("@.+\\.iam\\.gserviceaccount\\.com$", var.ci_service_account))
    error_message = "ci_service_account must be empty or a service account email."
  }
}
