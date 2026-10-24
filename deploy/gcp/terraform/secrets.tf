# Secret names only. setup.sh puts the values (generated keys, database passwords, vendor
# credentials); Terraform never sees them. The VM reads its own secrets and nothing else.
locals {
  # Vendor settings a safety lead enters in the staff console (backend/internal/vendorkeys;
  # a test keeps this list in step with its catalogue). The VM may add versions to these and
  # to SETTINGS_REVISION, and nothing else.
  vendor_settings = ["SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD", "OTP_EMAIL_FROM",
    "ASSURANCE_URL", "ASSURANCE_PROVIDER", "ASSURANCE_CERTS", "ASSURANCE_API_KEY", "ASSURANCE_SECRET", "ASSURANCE_CALLBACK_SECRET",
    "LIVENESS_URL", "LIVENESS_API_KEY", "LIVENESS_SECRET", "ARC_URL", "ARC_API_KEY", "ARC_SECRET",
    "STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "STRIPE_PRICE_TRYST_PLUS", "STRIPE_PRICE_TRYST_PLUS_ANNUAL",
  "SIGHTENGINE_USER", "SIGHTENGINE_SECRET", "PHOTODNA_KEY", "SETTINGS_REVISION"]
  secret_names = ["DATA_KEK", "DSR_SIGNING_KEY", "CONTACT_PEPPER", "ANCHOR_PEPPER", "VAPID_PRIVATE_KEY",
    "SMTP_PASSWORD", "POSTMARK_SERVER_TOKEN", "TWILIO_AUTH_TOKEN", "STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET",
    "LIVENESS_SECRET", "LIVENESS_API_KEY", "ASSURANCE_SECRET", "ASSURANCE_API_KEY", "ASSURANCE_CALLBACK_SECRET",
    "SIGHTENGINE_SECRET", "PHOTODNA_KEY", "ARC_SECRET", "ARC_API_KEY",
    "DATABASE_URL_IDENTITY", "DATABASE_URL_PROFILE", "DATABASE_URL_KEYS", "DATABASE_URL_COMMERCE", "DATABASE_URL_LEDGER",
    "DATABASE_URL_SAFETY", "INTERNAL_SECRET_SAFETY", "INTERNAL_SECRET_PLATFORM", "INTERNAL_SECRET_SAFETY_PREVIOUS",
  "INTERNAL_SECRET_PLATFORM_PREVIOUS", "STAFF_ROSTER", "API_CONFIG", "SAFETY_CONFIG", "BACKUP_KEY", "OFFSITE_SSH_KEY", "OFFSITE_TARGET"]
  all_secrets = setunion(local.secret_names, local.vendor_settings)
}

resource "google_secret_manager_secret" "s" {
  for_each  = local.all_secrets
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

# The staff console stores vendor keys: add versions (never read others' history) and list
# versions' metadata to show what is set, on the vendor settings only.
resource "google_secret_manager_secret_iam_member" "vm_vendor_write" {
  for_each  = toset(local.vendor_settings)
  secret_id = google_secret_manager_secret.s[each.key].id
  role      = "roles/secretmanager.secretVersionAdder"
  member    = "serviceAccount:${google_service_account.vm.email}"
}

resource "google_secret_manager_secret_iam_member" "vm_vendor_view" {
  for_each  = toset(local.vendor_settings)
  secret_id = google_secret_manager_secret.s[each.key].id
  role      = "roles/secretmanager.viewer"
  member    = "serviceAccount:${google_service_account.vm.email}"
}
