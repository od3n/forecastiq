# ForecastIQ — offsite backup project outputs.

output "bucket_name" {
  description = "Offsite backup bucket — use as FIQ_RCLONE_REMOTE=offsite:<bucket_name> in /etc/cron.d/forecastiq."
  value       = aws_s3_bucket.backups.bucket
}

output "region" {
  description = "Bucket region for the host rclone.conf."
  value       = var.region
}

output "rclone_access_key_id" {
  description = "Access key id for the host rclone.conf [offsite] remote."
  value       = aws_iam_access_key.backup.id
}

output "rclone_secret_access_key" {
  description = "Secret for the host rclone.conf [offsite] remote. Read with terraform output -raw; never commit or echo."
  value       = aws_iam_access_key.backup.secret
  sensitive   = true
}
