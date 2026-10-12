# One address, TLS by a Google-managed certificate, Cloud Armor in front of both sites, and
# Identity-Aware Proxy on the staff console only.
resource "google_compute_global_address" "lb" {
  name = local.name
}

resource "google_compute_instance_group" "app" {
  name      = local.name
  zone      = local.zone
  instances = [google_compute_instance.app.self_link]
  named_port {
    name = "web"
    port = 3000
  }
  named_port {
    name = "console"
    port = 3001
  }
}

resource "google_compute_health_check" "hc" {
  for_each = { web = 3000, console = 3001 }
  name     = "${local.name}-${each.key}"
  http_health_check {
    port         = each.value
    request_path = "/"
  }
}

resource "google_compute_security_policy" "armor" {
  name = local.name
  rule {
    priority    = 1000
    action      = "deny(403)"
    description = "OWASP: SQL injection, XSS, local file inclusion, remote code execution"
    match {
      expr {
        expression = "evaluatePreconfiguredWaf('sqli-v33-stable') || evaluatePreconfiguredWaf('xss-v33-stable') || evaluatePreconfiguredWaf('lfi-v33-stable') || evaluatePreconfiguredWaf('rce-v33-stable')"
      }
    }
  }
  rule {
    priority    = 2000
    action      = "throttle"
    description = "Sign-in, recovery and emergency lock: 60 a minute per address"
    match {
      expr { expression = "request.path.startsWith('/v1/auth/') || request.path.startsWith('/v1/emergency-lock')" }
    }
    rate_limit_options {
      conform_action = "allow"
      exceed_action  = "deny(429)"
      enforce_on_key = "IP"
      rate_limit_threshold {
        count        = 60
        interval_sec = 60
      }
    }
  }
  rule {
    priority = 2147483647
    action   = "allow"
    match {
      versioned_expr = "SRC_IPS_V1"
      config { src_ip_ranges = ["*"] }
    }
    description = "default"
  }
}

resource "google_compute_backend_service" "b" {
  for_each              = { web = false, console = true } # IAP on the console only
  name                  = "${local.name}-${each.key}"
  protocol              = "HTTP"
  port_name             = each.key
  load_balancing_scheme = "EXTERNAL_MANAGED"
  timeout_sec           = 60
  health_checks         = [google_compute_health_check.hc[each.key].id]
  security_policy       = google_compute_security_policy.armor.id
  backend {
    group           = google_compute_instance_group.app.self_link
    balancing_mode  = "UTILIZATION"
    capacity_scaler = 1
  }
  log_config { enable = false } # request logs would hold members' addresses (NFR-11)
  dynamic "iap" {
    for_each = each.value ? [1] : []
    content { enabled = true }
  }
}

resource "google_iap_web_backend_service_iam_member" "staff" {
  web_backend_service = google_compute_backend_service.b["console"].name
  role                = "roles/iap.httpsResourceAccessor"
  member              = "group:${var.staff_group}"
}

resource "google_compute_url_map" "https" {
  name            = local.name
  default_service = google_compute_backend_service.b["web"].id
  host_rule {
    hosts        = [var.console_domain]
    path_matcher = "console"
  }
  path_matcher {
    name            = "console"
    default_service = google_compute_backend_service.b["console"].id
  }
}

resource "google_compute_managed_ssl_certificate" "cert" {
  name = local.name
  managed { domains = [var.domain, var.console_domain] }
  lifecycle { create_before_destroy = true }
}

resource "google_compute_ssl_policy" "tls" {
  name            = local.name
  profile         = "MODERN"
  min_tls_version = "TLS_1_2"
}

resource "google_compute_target_https_proxy" "https" {
  name             = local.name
  url_map          = google_compute_url_map.https.id
  ssl_certificates = [google_compute_managed_ssl_certificate.cert.id]
  ssl_policy       = google_compute_ssl_policy.tls.id
}

resource "google_compute_global_forwarding_rule" "https" {
  name                  = "${local.name}-https"
  target                = google_compute_target_https_proxy.https.id
  ip_address            = google_compute_global_address.lb.address
  port_range            = "443"
  load_balancing_scheme = "EXTERNAL_MANAGED"
}

# Port 80 only redirects to HTTPS.
resource "google_compute_url_map" "redirect" {
  name = "${local.name}-redirect"
  default_url_redirect {
    https_redirect         = true
    strip_query            = false
    redirect_response_code = "MOVED_PERMANENTLY_DEFAULT"
  }
}

resource "google_compute_target_http_proxy" "redirect" {
  name    = "${local.name}-redirect"
  url_map = google_compute_url_map.redirect.id
}

resource "google_compute_global_forwarding_rule" "http" {
  name                  = "${local.name}-http"
  target                = google_compute_target_http_proxy.redirect.id
  ip_address            = google_compute_global_address.lb.address
  port_range            = "80"
  load_balancing_scheme = "EXTERNAL_MANAGED"
}
