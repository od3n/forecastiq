# ForecastIQ — Offsite Backup Provisioning (terraform/backup)

Provisions the offsite backup target consumed by `deploy/scripts/backup.sh`
(weekly Sunday rclone copy, 90 d offsite retention) and
`deploy/scripts/restore-test.sh` (monthly, prefers the offsite dump).
Reference: `docs/operations/04-backup-and-restore.md` §1; S3 substituted for
B2 per operator decision 2026-07-30.

All resources live in the **od3n.com AWS account (`077101397287`)**, region
`ap-southeast-5` (same region as the production instance — free intra-region
transfer). An account-guard precondition in `main.tf` makes an apply under any
other `AWS_PROFILE` fail at plan time.

## Usage

```sh
cd terraform/backup
AWS_PROFILE=od3n.com terraform init && terraform plan
AWS_PROFILE=od3n.com terraform apply          # creates billable infra
terraform output -raw rclone_access_key_id
terraform output -raw rclone_secret_access_key   # sensitive — host rclone.conf only
```

## IAM policies required by this project

Two policies exist, with different owners and lifecycles:

### 1. Provisioner policy (manual, operator-attached) — `provisioner-policy.json`

Attached **by the operator** as an inline policy (suggested name
`forecastiq-backup-provisioning`) to the Terraform CLI user
`codex-usage-terraform`, which by default has no S3/IAM rights. It grants the
minimum needed to `apply`/`destroy` this project and nothing else — both
statements are pinned to the exact bucket and user names:

| Statement | Resource | Why each action group |
|-----------|----------|-----------------------|
| `BackupBucketProvisioning` | `arn:aws:s3:::forecastiq-backups-077101397287` | `CreateBucket`/`DeleteBucket`/`ListBucket` (lifecycle of the bucket itself); `GetBucket*`/`PutBucket*` (public-access block, tagging, versioning/ACL/CORS/website/logging reads during provider refresh); `Get/PutEncryptionConfiguration`, `Get/PutLifecycleConfiguration`, `Get/PutAccelerateConfiguration`, `GetReplicationConfiguration` — configuration actions whose IAM names do **not** start with `GetBucket`/`PutBucket`, so the wildcards above do not cover them (the provider reads all of these when refreshing `aws_s3_bucket`; `GetReplicationConfiguration` was the one discovered missing in practice) |
| `BackupUserProvisioning` | `arn:aws:iam::077101397287:user/forecastiq-backup` | `CreateUser`/`DeleteUser`/`GetUser`/`TagUser`/`ListUserTags` (user lifecycle); `Put/Delete/GetUserPolicy` (the inline bucket-RW policy below); `Create/Delete/ListAccessKeys` (the rclone credential) |

Not granted (deliberately): nothing account-wide, no `iam:*` on any other
user, no `s3:*Object*` (the provisioner never touches dump contents), no
replication/versioning writes.

### 2. Runtime policy (Terraform-managed) — `aws_iam_user_policy.backup_bucket_rw` in `main.tf`

Attached by Terraform to the dedicated `forecastiq-backup` user whose access
key lives in the production host's `~/.config/rclone/rclone.conf`. This is
the credential rclone runs with every night:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ListBucket",
      "Effect": "Allow",
      "Action": ["s3:ListBucket"],
      "Resource": "arn:aws:s3:::forecastiq-backups-077101397287"
    },
    {
      "Sid": "ObjectRW",
      "Effect": "Allow",
      "Action": ["s3:PutObject", "s3:GetObject", "s3:DeleteObject"],
      "Resource": "arn:aws:s3:::forecastiq-backups-077101397287/*"
    }
  ]
}
```

Scope rationale: `ListBucket` (rclone directory listing + restore-test latest-dump
discovery), `PutObject` (weekly offsite copy), `GetObject` (restore-test
download), `DeleteObject` (90 d offsite prune). No bucket-level writes, no
other buckets — a leaked host credential exposes only the backup objects.

## Host wiring (after apply)

On the production host, from the Terraform outputs:

1. `~/.config/rclone/rclone.conf` (mode 600, owner `ubuntu`):

   ```ini
   [offsite]
   type = s3
   provider = AWS
   region = ap-southeast-5
   access_key_id = <rclone_access_key_id>
   secret_access_key = <rclone_secret_access_key>
   ```

2. `/etc/cron.d/forecastiq`: add
   `FIQ_RCLONE_REMOTE=offsite:forecastiq-backups-077101397287` to the env block.

3. Verify end-to-end: `rclone lsf offsite:forecastiq-backups-077101397287`,
   run one manual `backup.sh` + `rclone copy`, then a `restore-test.sh` run that
   sources from offsite.

## Secrets

The access-key secret is stored in the local `terraform.tfstate` (gitignored)
and in the host `rclone.conf` (mode 600). It is never committed, echoed to
logs, or placed in CI. Rotation: `terraform apply -replace=aws_iam_access_key.backup`,
then rewrite the host `rclone.conf`.
