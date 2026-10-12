# TRYST on Google Cloud, London (D-66). One environment per project: prod or staging.
# Run through deploy/gcp/setup.sh, which also creates the state bucket and every secret
# value (values are never in Terraform or its state).
terraform {
  required_version = ">= 1.6"
  required_providers {
    google = { source = "hashicorp/google", version = "~> 8.4" }
  }
  backend "gcs" {} # bucket and prefix passed by setup.sh
}

provider "google" {
  project = var.project_id
  region  = var.region
}

data "google_project" "this" {}

locals {
  name = "tryst-${var.env}"
  zone = "${var.region}-a"
}
