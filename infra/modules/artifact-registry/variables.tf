variable "project" {
  description = "Google Cloud project id."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.project))
    error_message = "project must be a valid Google Cloud project id (6 to 30 lowercase characters)."
  }
}

variable "region" {
  description = "Region of the repository; the VM pulls from the same region."
  type        = string

  validation {
    condition     = can(regex("^[a-z]+-[a-z]+[0-9]$", var.region))
    error_message = "region must look like asia-south1."
  }
}

variable "name" {
  description = "Repository id, lowercase and hyphenated."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{0,61}[a-z0-9]$", var.name))
    error_message = "name must be lowercase letters, digits and hyphens."
  }
}

variable "readers" {
  description = "Members that pull images, such as serviceAccount:vm@project.iam.gserviceaccount.com."
  type        = list(string)
  default     = []

  validation {
    condition     = alltrue([for m in var.readers : can(regex("^(serviceAccount|user|group):", m))])
    error_message = "each reader must be a member string such as serviceAccount:email."
  }
}

variable "writers" {
  description = "Members that push images (the CI service account)."
  type        = list(string)
  default     = []

  validation {
    condition     = alltrue([for m in var.writers : can(regex("^(serviceAccount|user|group):", m))])
    error_message = "each writer must be a member string such as serviceAccount:email."
  }
}
