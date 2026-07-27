# Input variables — secrets supplied via TF_VAR_ env or terraform.tfvars (gitignored)
# Reference: docs/delivery/04-infrastructure-as-code.md §2

variable "cloudflare_api_token" {
  description = "Cloudflare API token (DNS edit scope for the zone)"
  type        = string
  sensitive   = true
}

variable "cloudflare_zone_id" {
  description = "Cloudflare zone ID for the domain"
  type        = string
}

variable "vps_ip" {
  description = "Elastic IP of the ForecastIQ EC2 instance (provisioned by the external instances Terraform project)"
  type        = string
}

variable "domain" {
  description = "Base domain (e.g. forecastiq.example)"
  type        = string
}

variable "pages_project" {
  description = "Cloudflare Pages project name (target of the app CNAME: <name>.pages.dev)"
  type        = string
  default     = "forecastiq"
}
