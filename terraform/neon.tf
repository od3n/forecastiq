# Neon PostgreSQL project and roles
# Reference: docs/architecture/06-deployment-architecture.md §1
# Reference: docs/security/04-secrets-management.md §5 (least privilege)

resource "neon_project" "forecastiq" {
  name      = "forecastiq-prod"
  region_id = "aws-eu-central-1"

  default_endpoint_settings {
    autoscaling_limit_min_cu = 0.25
    autoscaling_limit_max_cu = 2
  }
}

resource "neon_database" "forecastiq" {
  project_id = neon_project.forecastiq.id
  branch_id  = neon_project.forecastiq.default_branch_id
  name       = "forecastiq"
  owner_name = neon_role.app.name
}

# Application role (DML only — no DDL)
resource "neon_role" "app" {
  project_id = neon_project.forecastiq.id
  branch_id  = neon_project.forecastiq.default_branch_id
  name       = "forecastiq_app"
}

# Migration role (DDL — used by deploy pipeline only)
resource "neon_role" "migrate" {
  project_id = neon_project.forecastiq.id
  branch_id  = neon_project.forecastiq.default_branch_id
  name       = "forecastiq_migrate"
}
