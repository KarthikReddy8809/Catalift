variable "project" {
  description = "Google Cloud project id."
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.project))
    error_message = "project must be a valid Google Cloud project id (6 to 30 lowercase characters)."
  }
}

variable "region" {
  description = "The one region the secret's replica lives in."
  type        = string

  validation {
    condition     = can(regex("^[a-z]+-[a-z]+[0-9]$", var.region))
    error_message = "region must look like asia-south1."
  }
}

variable "secret_id" {
  description = "Secret name, such as catalift-openrouter-key."
  type        = string

  validation {
    condition     = can(regex("^[A-Za-z0-9_-]{1,255}$", var.secret_id))
    error_message = "secret_id may use letters, digits, hyphens and underscores."
  }
}

variable "accessors" {
  description = "Members that may read the secret's value."
  type        = list(string)
  default     = []

  validation {
    condition     = alltrue([for m in var.accessors : can(regex("^(serviceAccount|user|group):", m))])
    error_message = "each accessor must be a member string such as serviceAccount:email."
  }
}
