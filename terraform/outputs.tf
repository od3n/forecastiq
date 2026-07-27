# Outputs — DNS names for deploy scripts and docs.

output "dns_api_fqdn" {
  description = "Fully qualified domain name for the API"
  value       = cloudflare_record.api.hostname
}

output "dns_app_fqdn" {
  description = "Fully qualified domain name for the dashboard"
  value       = cloudflare_record.app.hostname
}
