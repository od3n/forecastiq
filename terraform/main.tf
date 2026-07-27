# ForecastIQ — Terraform Configuration
# Manages Cloudflare DNS records only (ADR-033).
#
# The EC2 instance (t3.small) is provisioned by a SEPARATE Terraform project;
# this module consumes its Elastic IP via var.vps_ip. If the instances project
# later exposes outputs, wire them here with terraform_remote_state instead of
# the manual variable:
#
#   data "terraform_remote_state" "infra" { backend = "..." config = { ... } }
#   → data.terraform_remote_state.infra.outputs.forecastiq_eip
#
# PostgreSQL runs as a container on the instance (ADR-033) — the previous Neon
# provider/resources were removed with that decision.
#
# State: remote (Cloudflare R2 or Terraform Cloud free tier).
#
# Usage:
#   cp terraform.tfvars.example terraform.tfvars  # fill in values
#   terraform init
#   terraform plan
#   terraform apply  # manual approval required
#
# Reference: docs/delivery/04-infrastructure-as-code.md
# Reference: docs/adr/ADR-033-personal-use-ec2-docker-deployment.md

terraform {
  required_version = ">= 1.6"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.0"
    }
  }

  # Remote state — uncomment one backend after initial bootstrap:
  #
  # Option A: Cloudflare R2
  # backend "s3" {
  #   bucket                      = "forecastiq-tfstate"
  #   key                         = "prod/terraform.tfstate"
  #   region                      = "auto"
  #   skip_credentials_validation = true
  #   skip_metadata_api_check     = true
  #   skip_region_validation      = true
  #   endpoints = {
  #     s3 = "https://<account_id>.r2.cloudflarestorage.com"
  #   }
  # }
  #
  # Option B: Terraform Cloud (free tier)
  # cloud {
  #   organization = "forecastiq"
  #   workspaces { name = "forecastiq-prod" }
  # }
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
