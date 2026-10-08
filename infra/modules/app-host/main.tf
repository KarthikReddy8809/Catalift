# The one VM Catalift runs on (ADR-0002): a static address, the instance with
# the data disk attached, the firewall rules for web traffic and IAP SSH, and
# the daily snapshot schedule of the data disk. The data disk itself is
# declared in the environment root, where prevent_destroy can be literal.

resource "google_compute_address" "this" {
  name         = "${var.name}-ip"
  project      = var.project
  region       = var.region
  address_type = "EXTERNAL"
  description  = "Managed by Terraform (Catalift)"
}

locals {
  # Without a domain, <ip-with-dashes>.sslip.io resolves to the static IP,
  # so Caddy gets a real certificate with no DNS setup (ADR-0013).
  site_address = var.domain != "" ? var.domain : "${replace(google_compute_address.this.address, ".", "-")}.sslip.io"
}

resource "google_compute_instance" "this" {
  name         = var.name
  project      = var.project
  zone         = var.zone
  machine_type = var.machine_type
  tags         = [var.name]

  # A restart for a machine type change is acceptable (ADR-0002: short outages).
  allow_stopping_for_update = true
  deletion_protection       = var.deletion_protection

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
      size  = var.boot_disk_gb
      type  = "pd-balanced"
    }
  }

  attached_disk {
    source      = var.data_disk_self_link
    device_name = "data"
  }

  network_interface {
    subnetwork = var.subnetwork
    access_config {
      nat_ip = google_compute_address.this.address
    }
  }

  service_account {
    email  = var.service_account_email
    scopes = ["cloud-platform"]
  }

  shielded_instance_config {
    enable_secure_boot          = true
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }

  metadata = merge(var.metadata, {
    catalift-site-address  = local.site_address
    enable-oslogin         = "TRUE"
    block-project-ssh-keys = "TRUE"
    startup-script = templatefile("${path.module}/startup.sh.tftpl", {
      data_device   = "data"
      registry_host = "${var.region}-docker.pkg.dev"
    })
  })
}

# Web traffic: the documented public entry point. Caddy answers 80 (redirect
# and certificate challenges) and 443; nothing else on the VM is published.
resource "google_compute_firewall" "web" {
  name        = "${var.name}-allow-web"
  project     = var.project
  network     = var.network
  description = "HTTP and HTTPS to Caddy on the Catalift VM (public site)"
  direction   = "INGRESS"
  #trivy:ignore:AVD-GCP-0027 public website: Caddy is the only listener, by design (ADR-0013)
  source_ranges = ["0.0.0.0/0"] # checkov:skip=CKV_GCP_106:public website on 80 and 443 only
  target_tags   = [var.name]

  allow {
    protocol = "tcp"
    ports    = ["80", "443"]
  }

  allow {
    protocol = "udp"
    ports    = ["443"]
  }
}

# SSH only through Identity-Aware Proxy: deploys and the operator reach the VM
# with OS Login, and no SSH port is open to the internet.
resource "google_compute_firewall" "iap_ssh" {
  name          = "${var.name}-allow-iap-ssh"
  project       = var.project
  network       = var.network
  description   = "SSH from Google's IAP range only"
  direction     = "INGRESS"
  source_ranges = ["35.235.240.0/20"]
  target_tags   = [var.name]

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

resource "google_compute_resource_policy" "snapshots" {
  name    = "${var.name}-daily-snapshots"
  project = var.project
  region  = var.region

  snapshot_schedule_policy {
    schedule {
      daily_schedule {
        days_in_cycle = 1
        start_time    = "21:00" # UTC, 02:30 IST
      }
    }
    retention_policy {
      max_retention_days    = var.snapshot_retention_days
      on_source_disk_delete = "KEEP_AUTO_SNAPSHOTS"
    }
    snapshot_properties {
      storage_locations = [var.region]
      labels = {
        managed-by = "terraform"
      }
    }
  }
}

resource "google_compute_disk_resource_policy_attachment" "data" {
  name    = google_compute_resource_policy.snapshots.name
  project = var.project
  zone    = var.zone
  disk    = var.data_disk_name
}
