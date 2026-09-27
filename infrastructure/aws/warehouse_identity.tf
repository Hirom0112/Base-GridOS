provider "google" {
  project = var.google_project_id
}

resource "google_iam_workload_identity_pool" "aws" {
  project                   = var.google_project_id
  workload_identity_pool_id = "gridos-aws"
  display_name              = "GridOS AWS workloads"
}

resource "google_iam_workload_identity_pool_provider" "aws" {
  project                            = var.google_project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.aws.workload_identity_pool_id
  workload_identity_pool_provider_id = "ecs-task"
  display_name                       = "GridOS ECS task roles"
  attribute_mapping = {
    "google.subject"     = "assertion.arn"
    "attribute.aws_role" = "assertion.arn.extract('assumed-role/{role_name}/')"
  }
  attribute_condition = "assertion.arn.startsWith('arn:aws:sts::${var.aws_account_id}:assumed-role/${aws_iam_role.task["worker"].name}/')"
  aws {
    account_id = var.aws_account_id
  }
}

resource "google_service_account" "telemetry_writer" {
  project      = var.google_project_id
  account_id   = "gridos-telemetry-writer"
  display_name = "GridOS telemetry writer"
}

resource "google_service_account_iam_member" "worker" {
  service_account_id = google_service_account.telemetry_writer.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/projects/${var.google_project_number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.aws.workload_identity_pool_id}/attribute.aws_role/${aws_iam_role.task["worker"].name}"
}

resource "google_bigquery_dataset_iam_member" "writer" {
  project    = var.google_project_id
  dataset_id = var.bigquery_dataset_id
  role       = "roles/bigquery.dataEditor"
  member     = "serviceAccount:${google_service_account.telemetry_writer.email}"
}

output "warehouse_identity_audience" {
  value = "//iam.googleapis.com/projects/${var.google_project_number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.aws.workload_identity_pool_id}/providers/${google_iam_workload_identity_pool_provider.aws.workload_identity_pool_provider_id}"
}

output "warehouse_service_account" {
  value = google_service_account.telemetry_writer.email
}
