terraform {
  required_version = ">= 1.12.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.64"
    }
    google = {
      source  = "hashicorp/google"
      version = "~> 7.0"
    }
  }
}

provider "aws" {
  region = var.region
}

locals {
  ports = {
    control  = 8080
    worker   = 0
    decision = 50061
    gateway  = 8081
  }
  service_names = toset(keys(local.ports))
  common_environment = [
    { name = "GRIDOS_FLEET", value = var.fleet_path },
    { name = "GRIDOS_REPLAY_DIR", value = "/app/replay" },
    { name = "TEMPORAL_ADDRESS", value = var.temporal_address },
    { name = "GRIDOS_DECISION_ADDR", value = "http://decision.gridos.internal:50061" },
    { name = "GRIDOS_GATEWAY_ADDR", value = "http://gateway.gridos.internal:8081" },
  ]
  service_environment = {
    control = concat(local.common_environment, [{ name = "GRIDOS_CONTROL_ADDRESS", value = ":8080" }])
    worker = concat(local.common_environment, [
      { name = "GRIDOS_CODE_VERSION", value = var.build_revision },
      { name = "GRIDOS_ANALYTICS", value = "bigquery" },
      { name = "GRIDOS_BIGQUERY_PROJECT", value = var.google_project_id },
      { name = "GRIDOS_BIGQUERY_DATASET", value = var.bigquery_dataset_id },
      { name = "GRIDOS_BIGQUERY_TABLE", value = var.bigquery_table_id },
      { name = "GRIDOS_WIF_AUDIENCE", value = "//iam.googleapis.com/projects/${var.google_project_number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.aws.workload_identity_pool_id}/providers/${google_iam_workload_identity_pool_provider.aws.workload_identity_pool_provider_id}" },
      { name = "GRIDOS_WIF_SERVICE_ACCOUNT", value = google_service_account.telemetry_writer.email },
      { name = "AWS_REGION", value = var.region },
    ])
    decision = [{ name = "GRIDOS_DECISION_ADDRESS", value = ":50061" }]
    gateway = [
      { name = "GRIDOS_CONTROL_ADDR", value = "http://control.gridos.internal:8080" },
      { name = "GRIDOS_GATEWAY_ADDRESS", value = ":8081" },
    ]
  }
}

resource "aws_security_group" "service" {
  name_prefix = "${var.name}-service-"
  vpc_id      = var.vpc_id
}

resource "aws_vpc_security_group_egress_rule" "service" {
  security_group_id = aws_security_group.service.id
  ip_protocol       = "-1"
  cidr_ipv4         = "0.0.0.0/0"
}

resource "aws_vpc_security_group_ingress_rule" "service" {
  for_each                     = toset(["8080", "8081", "50061"])
  security_group_id            = aws_security_group.service.id
  referenced_security_group_id = aws_security_group.service.id
  ip_protocol                  = "tcp"
  from_port                    = tonumber(each.key)
  to_port                      = tonumber(each.key)
}

resource "aws_security_group" "alb" {
  name_prefix = "${var.name}-alb-"
  vpc_id      = var.vpc_id
}

resource "aws_vpc_security_group_ingress_rule" "https" {
  security_group_id = aws_security_group.alb.id
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  cidr_ipv4         = "0.0.0.0/0"
}

resource "aws_vpc_security_group_egress_rule" "alb" {
  security_group_id            = aws_security_group.alb.id
  referenced_security_group_id = aws_security_group.service.id
  ip_protocol                  = "tcp"
  from_port                    = 8080
  to_port                      = 8080
}

resource "aws_vpc_security_group_ingress_rule" "control" {
  security_group_id            = aws_security_group.service.id
  referenced_security_group_id = aws_security_group.alb.id
  ip_protocol                  = "tcp"
  from_port                    = 8080
  to_port                      = 8080
}

resource "aws_security_group" "database" {
  name_prefix = "${var.name}-database-"
  vpc_id      = var.vpc_id
}

resource "aws_vpc_security_group_ingress_rule" "database" {
  security_group_id            = aws_security_group.database.id
  referenced_security_group_id = aws_security_group.service.id
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
}

resource "aws_db_subnet_group" "main" {
  name       = var.name
  subnet_ids = var.private_subnet_ids
}

resource "aws_db_instance" "main" {
  identifier                  = var.name
  engine                      = "postgres"
  engine_version              = "16"
  instance_class              = var.db_instance_class
  allocated_storage           = 50
  max_allocated_storage       = 200
  storage_encrypted           = true
  multi_az                    = true
  db_name                     = "gridos"
  username                    = "gridos"
  manage_master_user_password = true
  db_subnet_group_name        = aws_db_subnet_group.main.name
  vpc_security_group_ids      = [aws_security_group.database.id]
  publicly_accessible         = false
  backup_retention_period     = 7
  deletion_protection         = true
  skip_final_snapshot         = false
  final_snapshot_identifier   = "${var.name}-final"
}

resource "aws_secretsmanager_secret" "runtime" {
  for_each                = toset(["database-url", "gateway-token", "step-up-key"])
  name                    = "${var.name}/${each.key}"
  recovery_window_in_days = 7
}

resource "aws_iam_role" "execution" {
  name = "${var.name}-execution"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Principal = { Service = "ecs-tasks.amazonaws.com" }, Action = "sts:AssumeRole" }]
  })
}

resource "aws_iam_role_policy_attachment" "execution" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "runtime_secrets" {
  name = "runtime-secrets"
  role = aws_iam_role.execution.id
  policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = ["secretsmanager:GetSecretValue"], Resource = [for secret in aws_secretsmanager_secret.runtime : secret.arn] }]
  })
}

resource "aws_iam_role" "task" {
  for_each = local.service_names
  name     = "${var.name}-${each.key}"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Principal = { Service = "ecs-tasks.amazonaws.com" }, Action = "sts:AssumeRole" }]
  })
}

resource "aws_cloudwatch_log_group" "service" {
  for_each          = local.service_names
  name              = "/ecs/${var.name}/${each.key}"
  retention_in_days = 30
}

resource "aws_ecs_cluster" "main" {
  name = var.name
}

resource "aws_service_discovery_private_dns_namespace" "main" {
  name = "gridos.internal"
  vpc  = var.vpc_id
}

resource "aws_service_discovery_service" "service" {
  for_each = toset(["control", "decision", "gateway"])
  name     = each.key
  dns_config {
    namespace_id = aws_service_discovery_private_dns_namespace.main.id
    dns_records {
      ttl  = 10
      type = "A"
    }
    routing_policy = "MULTIVALUE"
  }
  health_check_custom_config {}
}

resource "aws_ecs_task_definition" "service" {
  for_each                 = local.service_names
  family                   = "${var.name}-${each.key}"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = each.key == "decision" ? 1024 : 512
  memory                   = each.key == "decision" ? 2048 : 1024
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task[each.key].arn
  container_definitions = jsonencode([{
    name         = each.key
    image        = var.images[each.key]
    essential    = true
    command      = each.key == "gateway" ? ["--scenario", var.scenario_path, "--live", "--gateway-id", "${var.name}-gateway", "--address", ":8081", "--database", "/tmp/gateway.db", "--control-address", "http://control.gridos.internal:8080"] : null
    portMappings = local.ports[each.key] == 0 ? [] : [{ containerPort = local.ports[each.key], protocol = "tcp" }]
    environment  = local.service_environment[each.key]
    mountPoints  = contains(["control", "worker"], each.key) ? [{ sourceVolume = "replay", containerPath = "/app/replay", readOnly = false }] : []
    secrets = concat(
      contains(["control", "worker"], each.key) ? [{ name = "GRIDOS_DATABASE_URL", valueFrom = aws_secretsmanager_secret.runtime["database-url"].arn }] : [],
      contains(["control", "worker", "gateway"], each.key) ? [{ name = "GRIDOS_GATEWAY_TOKEN", valueFrom = aws_secretsmanager_secret.runtime["gateway-token"].arn }] : [],
      each.key == "control" ? [{ name = "GRIDOS_STEP_UP_KEY", valueFrom = aws_secretsmanager_secret.runtime["step-up-key"].arn }] : []
    )
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.service[each.key].name
        awslogs-region        = var.region
        awslogs-stream-prefix = each.key
      }
    }
  }])
  dynamic "volume" {
    for_each = contains(["control", "worker"], each.key) ? [1] : []
    content {
      name = "replay"
      efs_volume_configuration {
        file_system_id     = aws_efs_file_system.replay.id
        transit_encryption = "ENABLED"
        authorization_config {
          access_point_id = aws_efs_access_point.replay.id
        }
      }
    }
  }
}

resource "aws_lb" "main" {
  name               = var.name
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = var.public_subnet_ids
}

resource "aws_lb_target_group" "control" {
  name        = "${var.name}-control"
  port        = 8080
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = var.vpc_id
  health_check {
    path    = "/geo/style.json"
    matcher = "200-399"
  }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.main.arn
  port              = 443
  protocol          = "HTTPS"
  certificate_arn   = var.certificate_arn
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.control.arn
  }
}

resource "aws_ecs_service" "service" {
  for_each        = local.service_names
  name            = each.key
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.service[each.key].arn
  desired_count   = 1
  launch_type     = "FARGATE"
  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.service.id]
    assign_public_ip = false
  }
  dynamic "service_registries" {
    for_each = contains(["control", "decision", "gateway"], each.key) ? [1] : []
    content {
      registry_arn = aws_service_discovery_service.service[each.key].arn
    }
  }
  dynamic "load_balancer" {
    for_each = each.key == "control" ? [1] : []
    content {
      target_group_arn = aws_lb_target_group.control.arn
      container_name   = "control"
      container_port   = 8080
    }
  }
  depends_on = [aws_lb_listener.https, aws_iam_role_policy.runtime_secrets, aws_efs_mount_target.replay]
}
