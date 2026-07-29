# ForecastIQ — production compute (ADR-033: single EC2 t3.small + Docker).
#
# This is the "external instances project" referenced by ../main.tf. It stands
# up the persistent origin host; its Elastic IP feeds the DNS module's
# var.vps_ip. State is local by default (personal-use scale) — wire a remote
# backend below if this grows.
#
# Usage:
#   cd terraform/instances
#   cp instances.tfvars.example instances.tfvars   # set deploy_public_key
#   terraform init && terraform plan
#   terraform apply                                  # creates billable infra
#   terraform output -raw public_ip                  # → DNS module var.vps_ip
#
# Reference: docs/adr/ADR-033-personal-use-ec2-docker-deployment.md
# Reference: deploy/bootstrap.sh (OS prep, run once after apply)

terraform {
  required_version = ">= 1.6"
  required_providers {
    aws  = { source = "hashicorp/aws", version = "~> 5.0" }
    http = { source = "hashicorp/http", version = "~> 3.4" }
  }
}

provider "aws" {
  region = var.region
}

# Ubuntu 22.04 LTS (Jammy), amd64 — Canonical. bootstrap.sh is apt-based, so
# the AMI family must stay Ubuntu (not Amazon Linux).
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }
  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# Default VPC + a subnet (personal-use scale; no custom networking per ADR-033).
data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# Live Cloudflare origin ranges — :80 is only reachable from Cloudflare's
# proxy (TLS terminates there; the origin serves plain HTTP). Fetched at plan
# time so the allowlist tracks Cloudflare's published set.
data "http" "cf_v4" {
  url = "https://www.cloudflare.com/ips-v4"
}

data "http" "cf_v6" {
  url = "https://www.cloudflare.com/ips-v6"
}

locals {
  cf_v4 = [for c in split("\n", trimspace(data.http.cf_v4.response_body)) : c if c != ""]
  cf_v6 = [for c in split("\n", trimspace(data.http.cf_v6.response_body)) : c if c != ""]
}

resource "aws_key_pair" "deploy" {
  key_name   = "forecastiq-deploy"
  public_key = var.deploy_public_key
}

resource "aws_security_group" "forecastiq" {
  name        = "forecastiq-prod"
  description = "ForecastIQ origin - SSH (key-only) + HTTP from Cloudflare only"
  vpc_id      = data.aws_vpc.default.id

  # SSH: key-only auth + fail2ban (bootstrap.sh) is the access control. Open
  # to the internet so both the operator and GitHub-hosted CI runners (dynamic
  # egress IPs) can deploy — matches the committed ufw posture (ADR-033).
  ingress {
    description = "SSH (key-only auth; operator + CI)"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # HTTP origin: Cloudflare proxy IPs only. The CF→origin hop is plain HTTP
  # (ssl=flexible), so this allowlist is what keeps the origin off the open
  # internet (cloudflare.tf comment / DRB-WP25 posture).
  ingress {
    description      = "HTTP from Cloudflare proxy"
    from_port        = 80
    to_port          = 80
    protocol         = "tcp"
    cidr_blocks      = local.cf_v4
    ipv6_cidr_blocks = local.cf_v6
  }

  egress {
    description      = "all outbound (provider APIs, ghcr, apt, backups)"
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }

  tags = { Name = "forecastiq-prod", Project = "forecastiq" }
}

resource "aws_instance" "forecastiq" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  key_name               = aws_key_pair.deploy.key_name
  subnet_id              = data.aws_subnets.default.ids[0]
  vpc_security_group_ids = [aws_security_group.forecastiq.id]

  # t3 unlimited: sustained bursts (collection + analysis batches) never
  # throttle at the personal-use scale; the surcharge is negligible.
  credit_specification {
    cpu_credits = "unlimited"
  }

  root_block_device {
    volume_size           = var.root_volume_gb
    volume_type           = "gp3"
    encrypted             = true
    delete_on_termination = true
  }

  tags = { Name = "forecastiq-prod", Project = "forecastiq" }
}

# Stable public address for DNS (survives stop/start, unlike the auto IP).
resource "aws_eip" "forecastiq" {
  instance = aws_instance.forecastiq.id
  domain   = "vpc"
  tags     = { Name = "forecastiq-prod", Project = "forecastiq" }
}
