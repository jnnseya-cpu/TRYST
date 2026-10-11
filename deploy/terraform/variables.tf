variable "region" {
  type    = string
  default = "eu-west-2" # London
}
variable "env" {
  type    = string
  default = "prod"
}
variable "api_image" {
  type        = string
  description = "ECR image URI for the API, pinned by digest"
}
variable "web_image" {
  type        = string
  description = "ECR image URI for the web app, pinned by digest"
}
variable "certificate_arn" {
  type        = string
  description = "ACM certificate for the member site"
}
variable "db_instance_class" {
  type    = string
  default = "db.t4g.medium"
}
variable "api_count" {
  type    = number
  default = 2
}
variable "ops_image" {
  type        = string
  description = "Web image built with OPS_CONSOLE=1 (moderator console), pinned by digest"
}
variable "ops_host" {
  type        = string
  description = "Staff-only console host name, fronted by the company sign-in proxy (Cloudflare Access)"
}
variable "staff_access" {
  type        = object({ issuer = string, audience = string, jwks_url = string, require_amr = optional(string, "") })
  description = "Sign-in proxy token settings for the safety service (STAFF_ACCESS_*)"
}
variable "api_config" {
  type        = map(string)
  default     = {}
  description = <<-EOT
    Non-secret API settings passed as environment variables, for example RP_ID, RP_ORIGINS,
    PUBLIC_URL, SALE_ITEMS, ASSURANCE_URL, ASSURANCE_PROVIDER, ASSURANCE_CERTS, LIVENESS_URL,
    ARC_URL, SIGHTENGINE_USER, TWILIO_ACCOUNT_SID, TWILIO_MESSAGING_SERVICE, OTP_EMAIL_FROM,
    VAPID_SUBJECT and STRIPE_PRICE_<KEY>. Secrets never go here: they live in Secrets Manager.
  EOT
  validation {
    condition     = alltrue([for k in keys(var.api_config) : !can(regex("(SECRET|TOKEN|PEPPER|KEK|_KEY$|DATABASE_URL|ROSTER)", k))])
    error_message = "api_config must not carry secrets; put them in Secrets Manager."
  }
}
