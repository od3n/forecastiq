# ForecastIQ — offsite backup storage (S3 + scoped IAM user).
#
# Provisions the offsite target for deploy/scripts/backup.sh (weekly Sunday
# rclone copy, 90 d offsite retention — docs/operations/04-backup-and-restore.md
# §1, B2-equivalent per operator decision 2026-07-30). Like ../instances this
# is a standalone project with local state (personal-use scale).
#
# The bucket lives in the od3n.com account (077101397287) — enforced by the
# expected_account_id precondition so an apply under the wrong AWS_PROFILE
# fails at plan time instead of provisioning into a foreign account.
#
# Usage:
#   cd terraform/backup
#   AWS_PROFILE=od3n.com terraform init && terraform plan
#   AWS_PROFILE=od3n.com terraform apply          # creates billable infra
#   terraform output -raw rclone_access_key_id
#   terraform output -raw rclone_secret_access_key   # sensitive — feed straight
#                                                    # into the host rclone.conf
#
# Host wiring (after apply): write ~/.config/rclone/rclone.conf ([offsite],
# type=s3, provider=AWS, region + keys from outputs; chmod 600) and set
# FIQ_RCLONE_REMOTE=offsite:<bucket> in /etc/cron.d/forecastiq.
#
# NOTE: the access key secret is stored in the local terraform.tfstate —
# acceptable at personal-use scale with local state kept out of git.

terraform {
  required_version = ">= 1.6"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 5.0" }
  }
}

provider "aws" {
  region = var.region
}

data "aws_caller_identity" "current" {}

locals {
  bucket_name = "forecastiq-backups-${data.aws_caller_identity.current.account_id}"
}

# Fail fast under the wrong profile/account (the exact mistake this guard
# exists to prevent — backups must live in the od3n.com account).
resource "terraform_data" "account_guard" {
  lifecycle {
    precondition {
      condition     = data.aws_caller_identity.current.account_id == var.expected_account_id
      error_message = "Refusing to provision: authenticated account ${data.aws_caller_identity.current.account_id} != expected ${var.expected_account_id} (use AWS_PROFILE=od3n.com)."
    }
  }
}

resource "aws_s3_bucket" "backups" {
  bucket     = local.bucket_name
  depends_on = [terraform_data.account_guard]

  tags = { Name = local.bucket_name, Project = "forecastiq" }
}

resource "aws_s3_bucket_public_access_block" "backups" {
  bucket                  = aws_s3_bucket.backups.id
  block_public_acls       = true
  ignore_public_acls      = true
  block_public_policy     = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# Safety net only: backup.sh prunes offsite at 90 d (rclone delete --min-age).
# This rule caps runaway cost if the script's prune ever stops running.
resource "aws_s3_bucket_lifecycle_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id
  rule {
    id     = "expire-safety-net"
    status = "Enabled"
    filter {
      prefix = ""
    }
    expiration {
      days = var.safety_net_expiry_days
    }
    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}

# Dedicated user for the production host's rclone remote — scoped to exactly
# this bucket (list + object RW), nothing else in the account.
resource "aws_iam_user" "backup" {
  name = "forecastiq-backup"
  tags = { purpose = "forecastiq-offsite-backup", Project = "forecastiq" }
}

resource "aws_iam_user_policy" "backup_bucket_rw" {
  name = "forecastiq-backup-bucket-rw"
  user = aws_iam_user.backup.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "ListBucket"
        Effect   = "Allow"
        Action   = ["s3:ListBucket"]
        Resource = aws_s3_bucket.backups.arn
      },
      {
        Sid      = "ObjectRW"
        Effect   = "Allow"
        Action   = ["s3:PutObject", "s3:GetObject", "s3:DeleteObject"]
        Resource = "${aws_s3_bucket.backups.arn}/*"
      }
    ]
  })
}

resource "aws_iam_access_key" "backup" {
  user = aws_iam_user.backup.name
}
