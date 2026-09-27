mock_provider "aws" {}
mock_provider "google" {}

run "staging_plan" {
  command = plan
  variables {
    region                = "us-east-1"
    vpc_id                = "vpc-0123456789abcdef0"
    private_subnet_ids    = ["subnet-11111111111111111", "subnet-22222222222222222"]
    public_subnet_ids     = ["subnet-33333333333333333", "subnet-44444444444444444"]
    certificate_arn       = "arn:aws:acm:us-east-1:123456789012:certificate/00000000-0000-0000-0000-000000000000"
    aws_account_id        = "123456789012"
    google_project_id     = "gridos-plan-example"
    google_project_number = "123456789012"
    bigquery_dataset_id   = "telemetry"
    bigquery_table_id     = "observations"
    temporal_address      = "temporal.example.internal:7233"
    build_revision        = "0000000000000000000000000000000000000000"
    images = {
      control  = "example.invalid/gridos/control@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      worker   = "example.invalid/gridos/worker@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      decision = "example.invalid/gridos/decision@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
      gateway  = "example.invalid/gridos/gateway@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    }
  }
  assert {
    condition     = aws_ecs_service.service["control"].desired_count == 1 && aws_ecs_service.service["worker"].desired_count == 1
    error_message = "Control and worker must each run a staging task."
  }
  assert {
    condition     = aws_db_instance.main.storage_encrypted && aws_efs_file_system.replay.encrypted
    error_message = "Operational stores must be encrypted."
  }
}
