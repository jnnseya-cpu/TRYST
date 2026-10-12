# GitHub Actions deploys with short-lived Google credentials (Workload Identity Federation):
# no keys stored anywhere. The deploy account can push images and run the deploy script on
# this environment's VM over IAP, and nothing else.
resource "google_service_account" "deploy" {
  account_id   = "${local.name}-deploy"
  display_name = "TRYST ${var.env} deploys (GitHub Actions)"
}

resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "${local.name}-github"
  display_name              = "GitHub (${var.env})"
  depends_on                = [google_project_service.apis]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
    "attribute.ref"        = "assertion.ref"
  }
  # Only this repository's main branch can deploy.
  attribute_condition = "assertion.repository == '${var.github_repo}' && assertion.ref == 'refs/heads/main'"
  oidc { issuer_uri = "https://token.actions.githubusercontent.com" }
}

resource "google_service_account_iam_member" "deploy_wif" {
  service_account_id = google_service_account.deploy.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repo}"
}

resource "google_artifact_registry_repository_iam_member" "deploy_push" {
  repository = google_artifact_registry_repository.images.name
  location   = var.region
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.deploy.email}"
}

resource "google_project_iam_member" "deploy" {
  for_each = toset(["roles/iap.tunnelResourceAccessor", "roles/compute.viewer"])
  project  = var.project_id
  role     = each.key
  member   = "serviceAccount:${google_service_account.deploy.email}"
}

resource "google_compute_instance_iam_member" "deploy_login" {
  instance_name = google_compute_instance.app.name
  zone          = local.zone
  role          = "roles/compute.osAdminLogin"
  member        = "serviceAccount:${google_service_account.deploy.email}"
}

resource "google_service_account_iam_member" "deploy_act_as_vm" {
  service_account_id = google_service_account.vm.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.deploy.email}"
}
