# Inputs for the ForecastIQ production compute module (ADR-033).
variable "region" {
  description = "AWS region for the production instance"
  type        = string
  default     = "ap-southeast-1"
}

variable "instance_type" {
  description = "EC2 instance type (ADR-033: t3.small)"
  type        = string
  default     = "t3.small"
}

variable "root_volume_gb" {
  description = "Root EBS volume size (GB) — app image + Postgres data + backups"
  type        = number
  default     = 30
}

variable "deploy_public_key" {
  description = "SSH public key installed on the instance for the deploy user (ed25519)"
  type        = string
}
