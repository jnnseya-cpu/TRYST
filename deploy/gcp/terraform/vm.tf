resource "google_service_account" "vm" {
  account_id   = "${local.name}-vm"
  display_name = "TRYST ${var.env} VM"
}

resource "google_project_iam_member" "vm" {
  for_each = toset(["roles/logging.logWriter", "roles/monitoring.metricWriter", "roles/artifactregistry.reader"])
  project  = var.project_id
  role     = each.key
  member   = "serviceAccount:${google_service_account.vm.email}"
}

resource "google_compute_instance" "app" {
  name         = local.name
  zone         = local.zone
  machine_type = var.machine_type
  tags         = ["tryst-web"]
  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
      size  = 50
      type  = "pd-ssd"
    }
  }
  network_interface {
    subnetwork = google_compute_subnetwork.main.id # no access_config: no public address
  }
  shielded_instance_config {
    enable_secure_boot          = true
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }
  service_account {
    email  = google_service_account.vm.email
    scopes = ["cloud-platform"]
  }
  metadata = {
    enable-oslogin = "TRUE"
    tryst-env      = var.env
    startup-script = file("${path.module}/../vm-startup.sh")
  }
  allow_stopping_for_update = true
  lifecycle { ignore_changes = [boot_disk[0].initialize_params[0].image] }
  depends_on = [google_project_service.apis]
}

# Daily disk snapshots of the VM (the data itself lives in Cloud SQL).
resource "google_compute_resource_policy" "snapshots" {
  name   = "${local.name}-daily"
  region = var.region
  snapshot_schedule_policy {
    schedule {
      daily_schedule {
        days_in_cycle = 1
        start_time    = "01:00"
      }
    }
    retention_policy { max_retention_days = 7 }
    snapshot_properties { storage_locations = [var.region] }
  }
}

resource "google_compute_disk_resource_policy_attachment" "snapshots" {
  name = google_compute_resource_policy.snapshots.name
  disk = google_compute_instance.app.name
  zone = local.zone
}
