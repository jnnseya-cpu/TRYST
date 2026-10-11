# TRYST production infrastructure skeleton (D-57; docs/runbooks/deploy.md). UK region for
# data residency. Secrets are created empty here and filled out-of-band; nothing secret is
# ever in Terraform state.
terraform {
  required_version = ">= 1.6"
  required_providers {
    aws = { source = "hashicorp/aws", version = ">= 5.60, < 7.0" }
  }
  # backend "s3" { bucket = "...", key = "tryst/prod.tfstate", region = "eu-west-2", use_lockfile = true, encrypt = true }
}

provider "aws" {
  region = var.region
  default_tags { tags = { app = "tryst", env = var.env } }
}
