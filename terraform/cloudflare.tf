# Cloudflare DNS records
# Reference: docs/architecture/06-deployment-architecture.md §8

resource "cloudflare_record" "api" {
  zone_id = var.cloudflare_zone_id
  name    = "api"
  content = var.vps_ip
  type    = "A"
  ttl     = 60 # Fast failover (deployment architecture §8)
  proxied = false # Preserve client IPs for rate limiting

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
