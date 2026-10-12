resource "google_project_service" "apis" {
  for_each = toset(["compute.googleapis.com", "sqladmin.googleapis.com", "secretmanager.googleapis.com",
    "artifactregistry.googleapis.com", "servicenetworking.googleapis.com", "iap.googleapis.com",
    "iam.googleapis.com", "iamcredentials.googleapis.com", "sts.googleapis.com", "monitoring.googleapis.com",
  "logging.googleapis.com", "cloudresourcemanager.googleapis.com", "oslogin.googleapis.com"])
  service            = each.key
  disable_on_destroy = false
}

resource "google_compute_network" "vpc" {
  name                    = local.name
  auto_create_subnetworks = false
  depends_on              = [google_project_service.apis]
}

resource "google_compute_subnetwork" "main" {
  name                     = local.name
  network                  = google_compute_network.vpc.id
  region                   = var.region
  ip_cidr_range            = "10.10.0.0/24"
  private_ip_google_access = true
}

# Private services access: Cloud SQL gets addresses inside this network only.
resource "google_compute_global_address" "sql" {
  name          = "${local.name}-sql"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 20
  network       = google_compute_network.vpc.id
}

resource "google_service_networking_connection" "sql" {
  network                 = google_compute_network.vpc.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.sql.name]
}

# Outbound only (vendors, image pulls); the VM has no public address.
resource "google_compute_router" "nat" {
  name    = "${local.name}-nat"
  network = google_compute_network.vpc.id
  region  = var.region
}

resource "google_compute_router_nat" "nat" {
  name                               = "${local.name}-nat"
  router                             = google_compute_router.nat.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

resource "google_compute_firewall" "lb" {
  name          = "${local.name}-lb"
  network       = google_compute_network.vpc.id
  source_ranges = ["35.191.0.0/16", "130.211.0.0/22"] # Google's load balancers and health checks
  target_tags   = ["tryst-web"]
  allow {
    protocol = "tcp"
    ports    = ["3000", "3001"]
  }
}

resource "google_compute_firewall" "iap_ssh" {
  name          = "${local.name}-iap-ssh"
  network       = google_compute_network.vpc.id
  source_ranges = ["35.235.240.0/20"] # IAP TCP forwarding only
  target_tags   = ["tryst-web"]
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

resource "google_artifact_registry_repository" "images" {
  repository_id = "tryst"
  location      = var.region
  format        = "DOCKER"
  cleanup_policies {
    id     = "keep-recent"
    action = "KEEP"
    most_recent_versions { keep_count = 30 }
  }
  cleanup_policies {
    id     = "drop-old"
    action = "DELETE"
    condition { older_than = "7776000s" } # 90 days
  }
  depends_on = [google_project_service.apis]
}
