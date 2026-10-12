# Outages and resource pressure reach a person by email without anyone setting it up again.
resource "google_monitoring_notification_channel" "email" {
  display_name = "TRYST ${var.env} on-call"
  type         = "email"
  labels       = { email_address = var.alert_email }
  depends_on   = [google_project_service.apis]
}

resource "google_monitoring_uptime_check_config" "site" {
  display_name = "${local.name} site"
  timeout      = "10s"
  period       = "60s"
  http_check {
    path         = "/"
    port         = 443
    use_ssl      = true
    validate_ssl = true
  }
  monitored_resource {
    type   = "uptime_url"
    labels = { project_id = var.project_id, host = var.domain }
  }
  selected_regions = ["EUROPE", "USA_VIRGINIA", "ASIA_PACIFIC"]
}

locals {
  alerts = {
    site_down = {
      name   = "Member site down"
      filter = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" AND resource.type=\"uptime_url\" AND metric.label.check_id=\"${google_monitoring_uptime_check_config.site.uptime_check_id}\""
      cmp    = "COMPARISON_GT", threshold = 1, duration = "120s", align = "ALIGN_NEXT_OLDER", reducer = "REDUCE_COUNT_FALSE" # more than one region failing
    }
    vm_cpu = {
      name   = "App server CPU above 80%"
      filter = "metric.type=\"compute.googleapis.com/instance/cpu/utilization\" AND resource.type=\"gce_instance\""
      cmp    = "COMPARISON_GT", threshold = 0.8, duration = "900s", align = "ALIGN_MEAN", reducer = "REDUCE_NONE"
    }
    db_disk = {
      name   = "Database disk above 80%"
      filter = "metric.type=\"cloudsql.googleapis.com/database/disk/utilization\" AND resource.type=\"cloudsql_database\""
      cmp    = "COMPARISON_GT", threshold = 0.8, duration = "600s", align = "ALIGN_MEAN", reducer = "REDUCE_NONE"
    }
  }
}

resource "google_monitoring_alert_policy" "a" {
  for_each              = local.alerts
  display_name          = "${local.name}: ${each.value.name}"
  combiner              = "OR"
  notification_channels = [google_monitoring_notification_channel.email.id]
  conditions {
    display_name = each.value.name
    condition_threshold {
      filter          = each.value.filter
      comparison      = each.value.cmp
      threshold_value = each.value.threshold
      duration        = each.value.duration
      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = each.value.align
        cross_series_reducer = each.value.reducer
      }
    }
  }
}
