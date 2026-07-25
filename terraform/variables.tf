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

variable "neon_api_key" {
  description = "Neon API key for project management"
  type        = string
  sensitive   = true
}

variable "vps_ip" {
  description = "Public IPv4 address of the Hetzner VPS"
  type        = string
}

variable "domain" {
  description = "Base domain (e.g. forecastiq.example)"
  type        = string
}
