output "lb_ip" {
  value       = google_compute_global_address.lb.address
  description = "Point the domain and the console domain at this address (A records)"
}
output "console_iap_audience" {
  value = "/projects/${data.google_project.this.number}/global/backendServices/${google_compute_backend_service.b["console"].generated_id}"
}
output "vm" { value = { name = google_compute_instance.app.name, zone = local.zone } }
output "sql_private_ip" { value = { for k, v in google_sql_database_instance.db : k => v.private_ip_address } }
output "sql_instances" { value = { for k, v in google_sql_database_instance.db : k => v.name } }
output "registry" { value = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.images.repository_id}" }
output "github_wif_provider" { value = google_iam_workload_identity_pool_provider.github.name }
output "github_deploy_sa" { value = google_service_account.deploy.email }
