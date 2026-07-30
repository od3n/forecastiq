# ForecastIQ — offsite backup project variables.

variable "region" {
  description = "AWS region for the backup bucket (same as the instance region: free intra-region transfer)."
  type        = string
  default     = "ap-southeast-5"
}

variable "expected_account_id" {
  description = "AWS account that must own the backup bucket (od3n.com profile). Guards against applying under the wrong AWS_PROFILE."
  type        = string
  default     = "077101397287"
}

variable "safety_net_expiry_days" {
  description = "S3 lifecycle hard-expiry (safety net only; backup.sh owns the 90 d offsite prune)."
  type        = number
  default     = 180
}
