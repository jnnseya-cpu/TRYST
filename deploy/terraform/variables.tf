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
  type        = object({ issuer = string, audience = string, jwks_url = string })
  description = "Sign-in proxy token settings for the safety service (STAFF_ACCESS_*)"
}
