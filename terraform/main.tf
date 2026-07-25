# ForecastIQ — Terraform Configuration
# Manages Cloudflare DNS records and Neon PostgreSQL project.
# State: remote (Cloudflare R2 or Terraform Cloud free tier).
#
# Usage:
#   cp terraform.tfvars.example terraform.tfvars  # fill in values
#   terraform init
#   terraform plan
#   terraform apply  # manual approval required
#
# Reference: docs/delivery/04-infrastructure-as-code.md

terraform {
  required_version = ">= 1.6"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.0"
    }
    neon = {
      source  = "kislerdm/neon"
      version = "~> 0.6"
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

provider "neon" {
  api_key = var.neon_api_key
}
