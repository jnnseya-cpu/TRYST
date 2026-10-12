# Three instances (D-50, D-57): main (identity, profile, safety, commerce); keys (member data
# keys, 7-day backups so a deletion reaches backups within a week); ledger (erasure ledger,
# kept apart so restores can re-apply deletions). Users and passwords are made by setup.sh
# straight into Secret Manager, never in Terraform state.
locals {
  sql = {
    main   = { tier = var.db_tier_main, ha = var.db_high_availability, backups = 30, logs = 7, dbs = ["tryst_identity", "tryst_profile", "tryst_safety", "tryst_commerce"] }
    keys   = { tier = var.db_tier_small, ha = false, backups = 7, logs = 7, dbs = ["tryst_keys"] }
    ledger = { tier = var.db_tier_small, ha = false, backups = 30, logs = 7, dbs = ["tryst_ledger"] }
  }
  sql_dbs = merge([for k, v in local.sql : { for d in v.dbs : d => k }]...)
}

resource "google_sql_database_instance" "db" {
  for_each            = local.sql
  name                = "${local.name}-${each.key}"
  database_version    = "POSTGRES_16"
  region              = var.region
  deletion_protection = true
  settings {
    tier                        = each.value.tier
    edition                     = "ENTERPRISE"
    availability_type           = each.value.ha ? "REGIONAL" : "ZONAL"
    disk_autoresize             = true
    deletion_protection_enabled = true
    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.vpc.id
      ssl_mode        = "ENCRYPTED_ONLY"
    }
    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = true
      start_time                     = "02:00"
      location                       = var.region
      transaction_log_retention_days = each.value.logs
      backup_retention_settings { retained_backups = each.value.backups }
    }
    maintenance_window {
      day          = 2 # Tuesday
      hour         = 3
      update_track = "stable"
    }
    insights_config { query_insights_enabled = false } # no query text leaves the database
  }
  depends_on = [google_service_networking_connection.sql]
}

resource "google_sql_database" "db" {
  for_each = local.sql_dbs
  name     = each.key
  instance = google_sql_database_instance.db[each.value].name
}
