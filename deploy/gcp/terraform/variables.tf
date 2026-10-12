variable "project_id" { type = string }
variable "env" {
  type = string
  validation {
    condition     = contains(["prod", "staging"], var.env)
    error_message = "env is prod or staging."
  }
}
variable "region" {
  type    = string
  default = "europe-west2" # London: members' data stays in the UK
}
variable "domain" {
  type        = string
  description = "Member site host, e.g. tryst.example or staging.tryst.example"
}
variable "console_domain" {
  type        = string
  description = "Staff console host, e.g. console.tryst.example"
}
variable "staff_group" {
  type        = string
  description = "Google group of the safety team (IAP access to the console), e.g. safety-team@company.example"
}
variable "alert_email" {
  type        = string
  description = "Where outages and alerts go (an on-call mailbox or group)"
}
variable "github_repo" {
  type        = string
  description = "owner/repo allowed to deploy (Workload Identity Federation)"
}
variable "machine_type" {
  type    = string
  default = "e2-standard-4"
}
variable "db_tier_main" {
  type    = string
  default = "db-custom-2-7680"
}
variable "db_tier_small" {
  type    = string
  default = "db-custom-1-3840"
}
variable "db_high_availability" {
  type    = bool
  default = true
}
