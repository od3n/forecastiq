# Outputs — connection strings and DNS names for deploy scripts
# Sensitive values are masked in CLI output.

output "database_url" {
  description = "PostgreSQL connection string for the app role"
  value       = "postgres://${neon_role.app.name}:${neon_role.app.password}@${neon_project.forecastiq.database_host}/${neon_database.forecastiq.name}?sslmode=require"
  sensitive   = true
}

output "database_migrate_url" {
  description = "PostgreSQL connection string for the migration role"
  value       = "postgres://${neon_role.migrate.name}:${neon_role.migrate.password}@${neon_project.forecastiq.database_host}/${neon_database.forecastiq.name}?sslmode=require"
  sensitive   = true
}

output "dns_api_fqdn" {
  description = "Fully qualified domain name for the API"
  value       = cloudflare_record.api.hostname
}

output "dns_app_fqdn" {
  description = "Fully qualified domain name for the dashboard"
  value       = cloudflare_record.app.hostname
}
