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
