# Cloudflare DNS records
# Reference: docs/architecture/06-deployment-architecture.md §8

resource "cloudflare_record" "api" {
  zone_id = var.cloudflare_zone_id
  name    = "api"
  content = var.vps_ip
  type    = "A"
  ttl     = 1 # Auto (required when proxied)
  # Proxied: TLS terminates at Cloudflare; the EC2 origin serves HTTP :80
  # only (ADR-033 — no Caddy on the instance). Client IPs arrive via
  # CF-Connecting-IP; revisit IP-keyed rate limiting if traffic grows.
  proxied = true

  comment = "ForecastIQ API — managed by Terraform"
}

resource "cloudflare_record" "app" {
  zone_id = var.cloudflare_zone_id
  name    = "app"
  # Pages hostnames are <project-name>.pages.dev — project names cannot
  # contain dots, so the base domain is never a valid target (DRB-WP23-012).
  content = "${var.pages_project}.pages.dev"
  type    = "CNAME"
  ttl     = 1 # Auto (Cloudflare proxied)
  proxied = true

  comment = "ForecastIQ Dashboard (Cloudflare Pages) — managed by Terraform"
}

# Zone-level TLS + security headers. Under ADR-033 Cloudflare is the TLS
# terminator, so HSTS and HTTPS enforcement belong here, not at the origin
# (which serves plain HTTP :80) and not in the app middleware (DRB-WP25-001).
resource "cloudflare_zone_settings_override" "forecastiq" {
  zone_id = var.cloudflare_zone_id

  settings {
    always_use_https = "on"
    # flexible: browser↔Cloudflare is HTTPS; Cloudflare↔origin is plain HTTP
    # (ADR-033 origin serves :80, no cert). This leaves the CF→origin hop
    # unencrypted over the internet, so the EC2 security group MUST restrict
    # :80 to Cloudflare IP ranges (tracked with the CF-Connecting-IP work in
    # ADR-033). Move to "strict" if the origin ever terminates TLS.
    ssl = "flexible"

    security_header {
      enabled            = true
      include_subdomains = true
      max_age            = 31536000 # 1 year (security architecture §4)
      preload            = true
      nosniff            = true
    }
  }
}
