# Secret names only. setup.sh puts the values (generated keys, database passwords, vendor
# credentials); Terraform never sees them. The VM reads its own secrets and nothing else.
locals {
  secret_names = ["DATA_KEK", "DSR_SIGNING_KEY", "CONTACT_PEPPER", "ANCHOR_PEPPER", "VAPID_PRIVATE_KEY",
    "SMTP_PASSWORD", "POSTMARK_SERVER_TOKEN", "TWILIO_AUTH_TOKEN", "STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET",
    "LIVENESS_SECRET", "LIVENESS_API_KEY", "ASSURANCE_SECRET", "ASSURANCE_API_KEY", "ASSURANCE_CALLBACK_SECRET",
    "SIGHTENGINE_SECRET", "PHOTODNA_KEY", "ARC_SECRET", "ARC_API_KEY",
    "DATABASE_URL_IDENTITY", "DATABASE_URL_PROFILE", "DATABASE_URL_KEYS", "DATABASE_URL_COMMERCE", "DATABASE_URL_LEDGER",
    "DATABASE_URL_SAFETY", "INTERNAL_SECRET_SAFETY", "INTERNAL_SECRET_PLATFORM", "INTERNAL_SECRET_SAFETY_PREVIOUS",
  "INTERNAL_SECRET_PLATFORM_PREVIOUS", "STAFF_ROSTER", "API_CONFIG", "SAFETY_CONFIG", "BACKUP_KEY", "OFFSITE_SSH_KEY", "OFFSITE_TARGET"]
}

resource "google_secret_manager_secret" "s" {
  for_each  = toset(local.secret_names)
  secret_id = "${local.name}-${each.key}"
  replication {
    user_managed {
      replicas { location = var.region }
    }
  }
  depends_on = [google_project_service.apis]
}

resource "google_secret_manager_secret_iam_member" "vm" {
  for_each  = google_secret_manager_secret.s
  secret_id = each.value.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.vm.email}"
}
