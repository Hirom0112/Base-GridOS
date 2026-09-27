# Terraform mock-provider plan

Command: `/tmp/gridos-terraform-bin/terraform -chdir=infrastructure/aws test -filter=tests/staging.tftest.hcl -verbose -no-color`

Mock providers: `mock_provider "aws" {}` and `mock_provider "google" {}` in `infrastructure/aws/tests/staging.tftest.hcl`. The test supplies placeholder VPC, subnet, certificate, image digest, AWS account, Google project, dataset, and Temporal address values. No provider credentials or `apply` were used.

```text
tests/staging.tftest.hcl... in progress
  run "staging_plan"... pass

Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # aws_cloudwatch_log_group.service["control"] will be created
  + resource "aws_cloudwatch_log_group" "service" {
      + arn                         = (known after apply)
      + deletion_protection_enabled = (known after apply)
      + id                          = (known after apply)
      + log_group_class             = (known after apply)
      + name                        = "/ecs/gridos-staging/control"
      + name_prefix                 = (known after apply)
      + region                      = (known after apply)
      + retention_in_days           = 30
      + tags_all                    = (known after apply)
    }

  # aws_cloudwatch_log_group.service["decision"] will be created
  + resource "aws_cloudwatch_log_group" "service" {
      + arn                         = (known after apply)
      + deletion_protection_enabled = (known after apply)
      + id                          = (known after apply)
      + log_group_class             = (known after apply)
      + name                        = "/ecs/gridos-staging/decision"
      + name_prefix                 = (known after apply)
      + region                      = (known after apply)
      + retention_in_days           = 30
      + tags_all                    = (known after apply)
    }

  # aws_cloudwatch_log_group.service["gateway"] will be created
  + resource "aws_cloudwatch_log_group" "service" {
      + arn                         = (known after apply)
      + deletion_protection_enabled = (known after apply)
      + id                          = (known after apply)
      + log_group_class             = (known after apply)
      + name                        = "/ecs/gridos-staging/gateway"
      + name_prefix                 = (known after apply)
      + region                      = (known after apply)
      + retention_in_days           = 30
      + tags_all                    = (known after apply)
    }

  # aws_cloudwatch_log_group.service["worker"] will be created
  + resource "aws_cloudwatch_log_group" "service" {
      + arn                         = (known after apply)
      + deletion_protection_enabled = (known after apply)
      + id                          = (known after apply)
      + log_group_class             = (known after apply)
      + name                        = "/ecs/gridos-staging/worker"
      + name_prefix                 = (known after apply)
      + region                      = (known after apply)
      + retention_in_days           = 30
      + tags_all                    = (known after apply)
    }

  # aws_db_instance.main will be created
  + resource "aws_db_instance" "main" {
      + address                               = (known after apply)
      + allocated_storage                     = 50
      + arn                                   = (known after apply)
      + availability_zone                     = (known after apply)
      + backup_retention_period               = 7
      + backup_target                         = (known after apply)
      + backup_window                         = (known after apply)
      + ca_cert_identifier                    = (known after apply)
      + character_set_name                    = (known after apply)
      + database_insights_mode                = (known after apply)
      + db_name                               = "gridos"
      + db_subnet_group_name                  = "gridos-staging"
      + deletion_protection                   = true
      + domain_fqdn                           = (known after apply)
      + endpoint                              = (known after apply)
      + engine                                = "postgres"
      + engine_lifecycle_support              = (known after apply)
      + engine_version                        = "16"
      + engine_version_actual                 = (known after apply)
      + final_snapshot_identifier             = "gridos-staging-final"
      + hosted_zone_id                        = (known after apply)
      + id                                    = (known after apply)
      + identifier                            = "gridos-staging"
      + identifier_prefix                     = (known after apply)
      + instance_class                        = "db.t4g.small"
      + iops                                  = (known after apply)
      + kms_key_id                            = (known after apply)
      + latest_restorable_time                = (known after apply)
      + license_model                         = (known after apply)
      + listener_endpoint                     = (known after apply)
      + maintenance_window                    = (known after apply)
      + manage_master_user_password           = true
      + master_user_secret                    = (known after apply)
      + master_user_secret_kms_key_id         = (known after apply)
      + max_allocated_storage                 = 200
      + monitoring_role_arn                   = (known after apply)
      + multi_az                              = true
      + nchar_character_set_name              = (known after apply)
      + network_type                          = (known after apply)
      + option_group_name                     = (known after apply)
      + parameter_group_name                  = (known after apply)
      + password_wo                           = (write-only attribute)
      + performance_insights_kms_key_id       = (known after apply)
      + performance_insights_retention_period = (known after apply)
      + port                                  = (known after apply)
      + publicly_accessible                   = false
      + region                                = (known after apply)
      + replica_mode                          = (known after apply)
      + replicas                              = (known after apply)
      + resource_id                           = (known after apply)
      + skip_final_snapshot                   = false
      + snapshot_identifier                   = (known after apply)
      + status                                = (known after apply)
      + storage_encrypted                     = true
      + storage_throughput                    = (known after apply)
      + storage_type                          = (known after apply)
      + tags_all                              = (known after apply)
      + timezone                              = (known after apply)
      + upgrade_rollout_order                 = (known after apply)
      + username                              = "gridos"
      + vpc_security_group_ids                = [
          + (known after apply),
        ]
    }

  # aws_db_subnet_group.main will be created
  + resource "aws_db_subnet_group" "main" {
      + arn                     = (known after apply)
      + id                      = (known after apply)
      + name                    = "gridos-staging"
      + name_prefix             = (known after apply)
      + region                  = (known after apply)
      + subnet_ids              = [
          + "subnet-11111111111111111",
          + "subnet-22222222222222222",
        ]
      + supported_network_types = (known after apply)
      + tags_all                = (known after apply)
      + vpc_id                  = (known after apply)
    }

  # aws_ecs_cluster.main will be created
  + resource "aws_ecs_cluster" "main" {
      + arn      = (known after apply)
      + id       = (known after apply)
      + name     = "gridos-staging"
      + region   = (known after apply)
      + tags_all = (known after apply)
    }

  # aws_ecs_service.service["control"] will be created
  + resource "aws_ecs_service" "service" {
      + arn                           = (known after apply)
      + availability_zone_rebalancing = (known after apply)
      + cluster                       = (known after apply)
      + desired_count                 = 1
      + iam_role                      = (known after apply)
      + id                            = (known after apply)
      + launch_type                   = "FARGATE"
      + name                          = "control"
      + platform_version              = (known after apply)
      + region                        = (known after apply)
      + tags_all                      = (known after apply)
      + task_definition               = (known after apply)
      + triggers                      = (known after apply)

      + load_balancer {
          + container_name   = "control"
          + container_port   = 8080
          + target_group_arn = (known after apply)
        }

      + network_configuration {
          + assign_public_ip = false
          + security_groups  = [
              + (known after apply),
            ]
          + subnets          = [
              + "subnet-11111111111111111",
              + "subnet-22222222222222222",
            ]
        }

      + service_registries {
          + registry_arn = (known after apply)
        }
    }

  # aws_ecs_service.service["decision"] will be created
  + resource "aws_ecs_service" "service" {
      + arn                           = (known after apply)
      + availability_zone_rebalancing = (known after apply)
      + cluster                       = (known after apply)
      + desired_count                 = 1
      + iam_role                      = (known after apply)
      + id                            = (known after apply)
      + launch_type                   = "FARGATE"
      + name                          = "decision"
      + platform_version              = (known after apply)
      + region                        = (known after apply)
      + tags_all                      = (known after apply)
      + task_definition               = (known after apply)
      + triggers                      = (known after apply)

      + network_configuration {
          + assign_public_ip = false
          + security_groups  = [
              + (known after apply),
            ]
          + subnets          = [
              + "subnet-11111111111111111",
              + "subnet-22222222222222222",
            ]
        }

      + service_registries {
          + registry_arn = (known after apply)
        }
    }

  # aws_ecs_service.service["gateway"] will be created
  + resource "aws_ecs_service" "service" {
      + arn                           = (known after apply)
      + availability_zone_rebalancing = (known after apply)
      + cluster                       = (known after apply)
      + desired_count                 = 1
      + iam_role                      = (known after apply)
      + id                            = (known after apply)
      + launch_type                   = "FARGATE"
      + name                          = "gateway"
      + platform_version              = (known after apply)
      + region                        = (known after apply)
      + tags_all                      = (known after apply)
      + task_definition               = (known after apply)
      + triggers                      = (known after apply)

      + network_configuration {
          + assign_public_ip = false
          + security_groups  = [
              + (known after apply),
            ]
          + subnets          = [
              + "subnet-11111111111111111",
              + "subnet-22222222222222222",
            ]
        }

      + service_registries {
          + registry_arn = (known after apply)
        }
    }

  # aws_ecs_service.service["worker"] will be created
  + resource "aws_ecs_service" "service" {
      + arn                           = (known after apply)
      + availability_zone_rebalancing = (known after apply)
      + cluster                       = (known after apply)
      + desired_count                 = 1
      + iam_role                      = (known after apply)
      + id                            = (known after apply)
      + launch_type                   = "FARGATE"
      + name                          = "worker"
      + platform_version              = (known after apply)
      + region                        = (known after apply)
      + tags_all                      = (known after apply)
      + task_definition               = (known after apply)
      + triggers                      = (known after apply)

      + network_configuration {
          + assign_public_ip = false
          + security_groups  = [
              + (known after apply),
            ]
          + subnets          = [
              + "subnet-11111111111111111",
              + "subnet-22222222222222222",
            ]
        }
    }

  # aws_ecs_task_definition.service["control"] will be created
  + resource "aws_ecs_task_definition" "service" {
      + arn                      = (known after apply)
      + arn_without_revision     = (known after apply)
      + container_definitions    = (known after apply)
      + cpu                      = "512"
      + enable_fault_injection   = (known after apply)
      + execution_role_arn       = (known after apply)
      + family                   = "gridos-staging-control"
      + id                       = (known after apply)
      + memory                   = "1024"
      + network_mode             = "awsvpc"
      + region                   = (known after apply)
      + requires_compatibilities = [
          + "FARGATE",
        ]
      + revision                 = (known after apply)
      + tags_all                 = (known after apply)
      + task_role_arn            = (known after apply)

      + volume {
          + configure_at_launch = (known after apply)
          + name                = "replay"

          + efs_volume_configuration {
              + file_system_id     = (known after apply)
              + transit_encryption = "ENABLED"

              + authorization_config {
                  + access_point_id = (known after apply)
                }
            }
        }
    }

  # aws_ecs_task_definition.service["decision"] will be created
  + resource "aws_ecs_task_definition" "service" {
      + arn                      = (known after apply)
      + arn_without_revision     = (known after apply)
      + container_definitions    = jsonencode(
            [
              + {
                  + command          = null
                  + environment      = [
                      + {
                          + name  = "GRIDOS_DECISION_ADDRESS"
                          + value = ":50061"
                        },
                    ]
                  + essential        = true
                  + image            = "example.invalid/gridos/decision@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
                  + logConfiguration = {
                      + logDriver = "awslogs"
                      + options   = {
                          + awslogs-group         = "/ecs/gridos-staging/decision"
                          + awslogs-region        = "us-east-1"
                          + awslogs-stream-prefix = "decision"
                        }
                    }
                  + mountPoints      = []
                  + name             = "decision"
                  + portMappings     = [
                      + {
                          + containerPort = 50061
                          + protocol      = "tcp"
                        },
                    ]
                  + secrets          = []
                },
            ]
        )
      + cpu                      = "1024"
      + enable_fault_injection   = (known after apply)
      + execution_role_arn       = (known after apply)
      + family                   = "gridos-staging-decision"
      + id                       = (known after apply)
      + memory                   = "2048"
      + network_mode             = "awsvpc"
      + region                   = (known after apply)
      + requires_compatibilities = [
          + "FARGATE",
        ]
      + revision                 = (known after apply)
      + tags_all                 = (known after apply)
      + task_role_arn            = (known after apply)
    }

  # aws_ecs_task_definition.service["gateway"] will be created
  + resource "aws_ecs_task_definition" "service" {
      + arn                      = (known after apply)
      + arn_without_revision     = (known after apply)
      + container_definitions    = (known after apply)
      + cpu                      = "512"
      + enable_fault_injection   = (known after apply)
      + execution_role_arn       = (known after apply)
      + family                   = "gridos-staging-gateway"
      + id                       = (known after apply)
      + memory                   = "1024"
      + network_mode             = "awsvpc"
      + region                   = (known after apply)
      + requires_compatibilities = [
          + "FARGATE",
        ]
      + revision                 = (known after apply)
      + tags_all                 = (known after apply)
      + task_role_arn            = (known after apply)
    }

  # aws_ecs_task_definition.service["worker"] will be created
  + resource "aws_ecs_task_definition" "service" {
      + arn                      = (known after apply)
      + arn_without_revision     = (known after apply)
      + container_definitions    = (known after apply)
      + cpu                      = "512"
      + enable_fault_injection   = (known after apply)
      + execution_role_arn       = (known after apply)
      + family                   = "gridos-staging-worker"
      + id                       = (known after apply)
      + memory                   = "1024"
      + network_mode             = "awsvpc"
      + region                   = (known after apply)
      + requires_compatibilities = [
          + "FARGATE",
        ]
      + revision                 = (known after apply)
      + tags_all                 = (known after apply)
      + task_role_arn            = (known after apply)

      + volume {
          + configure_at_launch = (known after apply)
          + name                = "replay"

          + efs_volume_configuration {
              + file_system_id     = (known after apply)
              + transit_encryption = "ENABLED"

              + authorization_config {
                  + access_point_id = (known after apply)
                }
            }
        }
    }

  # aws_efs_access_point.replay will be created
  + resource "aws_efs_access_point" "replay" {
      + arn             = (known after apply)
      + file_system_arn = (known after apply)
      + file_system_id  = (known after apply)
      + id              = (known after apply)
      + owner_id        = (known after apply)
      + region          = (known after apply)
      + tags_all        = (known after apply)

      + posix_user {
          + gid = 65532
          + uid = 65532
        }

      + root_directory {
          + path = "/gridos/replay"

          + creation_info {
              + owner_gid   = 65532
              + owner_uid   = 65532
              + permissions = "0750"
            }
        }
    }

  # aws_efs_file_system.replay will be created
  + resource "aws_efs_file_system" "replay" {
      + arn                     = (known after apply)
      + availability_zone_id    = (known after apply)
      + availability_zone_name  = (known after apply)
      + creation_token          = "gridos-staging-replay"
      + dns_name                = (known after apply)
      + encrypted               = true
      + id                      = (known after apply)
      + kms_key_id              = (known after apply)
      + name                    = (known after apply)
      + number_of_mount_targets = (known after apply)
      + owner_id                = (known after apply)
      + performance_mode        = (known after apply)
      + region                  = (known after apply)
      + size_in_bytes           = (known after apply)
      + tags_all                = (known after apply)
    }

  # aws_efs_mount_target.replay["subnet-11111111111111111"] will be created
  + resource "aws_efs_mount_target" "replay" {
      + availability_zone_id   = (known after apply)
      + availability_zone_name = (known after apply)
      + dns_name               = (known after apply)
      + file_system_arn        = (known after apply)
      + file_system_id         = (known after apply)
      + id                     = (known after apply)
      + ip_address             = (known after apply)
      + ip_address_type        = (known after apply)
      + ipv6_address           = (known after apply)
      + mount_target_dns_name  = (known after apply)
      + network_interface_id   = (known after apply)
      + owner_id               = (known after apply)
      + region                 = (known after apply)
      + security_groups        = [
          + (known after apply),
        ]
      + subnet_id              = "subnet-11111111111111111"
    }

  # aws_efs_mount_target.replay["subnet-22222222222222222"] will be created
  + resource "aws_efs_mount_target" "replay" {
      + availability_zone_id   = (known after apply)
      + availability_zone_name = (known after apply)
      + dns_name               = (known after apply)
      + file_system_arn        = (known after apply)
      + file_system_id         = (known after apply)
      + id                     = (known after apply)
      + ip_address             = (known after apply)
      + ip_address_type        = (known after apply)
      + ipv6_address           = (known after apply)
      + mount_target_dns_name  = (known after apply)
      + network_interface_id   = (known after apply)
      + owner_id               = (known after apply)
      + region                 = (known after apply)
      + security_groups        = [
          + (known after apply),
        ]
      + subnet_id              = "subnet-22222222222222222"
    }

  # aws_iam_role.execution will be created
  + resource "aws_iam_role" "execution" {
      + arn                 = (known after apply)
      + assume_role_policy  = jsonencode(
            {
              + Statement = [
                  + {
                      + Action    = "sts:AssumeRole"
                      + Effect    = "Allow"
                      + Principal = {
                          + Service = "ecs-tasks.amazonaws.com"
                        }
                    },
                ]
              + Version   = "2012-10-17"
            }
        )
      + create_date         = (known after apply)
      + id                  = (known after apply)
      + managed_policy_arns = (known after apply)
      + name                = "gridos-staging-execution"
      + name_prefix         = (known after apply)
      + tags_all            = (known after apply)
      + unique_id           = (known after apply)
    }

  # aws_iam_role.task["control"] will be created
  + resource "aws_iam_role" "task" {
      + arn                 = (known after apply)
      + assume_role_policy  = jsonencode(
            {
              + Statement = [
                  + {
                      + Action    = "sts:AssumeRole"
                      + Effect    = "Allow"
                      + Principal = {
                          + Service = "ecs-tasks.amazonaws.com"
                        }
                    },
                ]
              + Version   = "2012-10-17"
            }
        )
      + create_date         = (known after apply)
      + id                  = (known after apply)
      + managed_policy_arns = (known after apply)
      + name                = "gridos-staging-control"
      + name_prefix         = (known after apply)
      + tags_all            = (known after apply)
      + unique_id           = (known after apply)
    }

  # aws_iam_role.task["decision"] will be created
  + resource "aws_iam_role" "task" {
      + arn                 = (known after apply)
      + assume_role_policy  = jsonencode(
            {
              + Statement = [
                  + {
                      + Action    = "sts:AssumeRole"
                      + Effect    = "Allow"
                      + Principal = {
                          + Service = "ecs-tasks.amazonaws.com"
                        }
                    },
                ]
              + Version   = "2012-10-17"
            }
        )
      + create_date         = (known after apply)
      + id                  = (known after apply)
      + managed_policy_arns = (known after apply)
      + name                = "gridos-staging-decision"
      + name_prefix         = (known after apply)
      + tags_all            = (known after apply)
      + unique_id           = (known after apply)
    }

  # aws_iam_role.task["gateway"] will be created
  + resource "aws_iam_role" "task" {
      + arn                 = (known after apply)
      + assume_role_policy  = jsonencode(
            {
              + Statement = [
                  + {
                      + Action    = "sts:AssumeRole"
                      + Effect    = "Allow"
                      + Principal = {
                          + Service = "ecs-tasks.amazonaws.com"
                        }
                    },
                ]
              + Version   = "2012-10-17"
            }
        )
      + create_date         = (known after apply)
      + id                  = (known after apply)
      + managed_policy_arns = (known after apply)
      + name                = "gridos-staging-gateway"
      + name_prefix         = (known after apply)
      + tags_all            = (known after apply)
      + unique_id           = (known after apply)
    }

  # aws_iam_role.task["worker"] will be created
  + resource "aws_iam_role" "task" {
      + arn                 = (known after apply)
      + assume_role_policy  = jsonencode(
            {
              + Statement = [
                  + {
                      + Action    = "sts:AssumeRole"
                      + Effect    = "Allow"
                      + Principal = {
                          + Service = "ecs-tasks.amazonaws.com"
                        }
                    },
                ]
              + Version   = "2012-10-17"
            }
        )
      + create_date         = (known after apply)
      + id                  = (known after apply)
      + managed_policy_arns = (known after apply)
      + name                = "gridos-staging-worker"
      + name_prefix         = (known after apply)
      + tags_all            = (known after apply)
      + unique_id           = (known after apply)
    }

  # aws_iam_role_policy.runtime_secrets will be created
  + resource "aws_iam_role_policy" "runtime_secrets" {
      + id          = (known after apply)
      + name        = "runtime-secrets"
      + name_prefix = (known after apply)
      + policy      = (known after apply)
      + role        = (known after apply)
    }

  # aws_iam_role_policy_attachment.execution will be created
  + resource "aws_iam_role_policy_attachment" "execution" {
      + id         = (known after apply)
      + policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
      + role       = "gridos-staging-execution"
    }

  # aws_lb.main will be created
  + resource "aws_lb" "main" {
      + arn                                                          = (known after apply)
      + arn_suffix                                                   = (known after apply)
      + dns_name                                                     = (known after apply)
      + enable_prefix_for_ipv6_source_nat                            = (known after apply)
      + enforce_security_group_inbound_rules_on_private_link_traffic = (known after apply)
      + id                                                           = (known after apply)
      + internal                                                     = false
      + ip_address_type                                              = (known after apply)
      + load_balancer_type                                           = "application"
      + name                                                         = "gridos-staging"
      + name_prefix                                                  = (known after apply)
      + region                                                       = (known after apply)
      + secondary_ips_auto_assigned_per_subnet                       = (known after apply)
      + security_groups                                              = [
          + (known after apply),
        ]
      + subnets                                                      = [
          + "subnet-33333333333333333",
          + "subnet-44444444444444444",
        ]
      + tags_all                                                     = (known after apply)
      + vpc_id                                                       = (known after apply)
      + zone_id                                                      = (known after apply)
    }

  # aws_lb_listener.https will be created
  + resource "aws_lb_listener" "https" {
      + arn                                                                   = (known after apply)
      + certificate_arn                                                       = "arn:aws:acm:us-east-1:123456789012:certificate/00000000-0000-0000-0000-000000000000"
      + id                                                                    = (known after apply)
      + load_balancer_arn                                                     = (known after apply)
      + port                                                                  = 443
      + protocol                                                              = "HTTPS"
      + region                                                                = (known after apply)
      + routing_http_request_x_amzn_mtls_clientcert_header_name               = (known after apply)
      + routing_http_request_x_amzn_mtls_clientcert_issuer_header_name        = (known after apply)
      + routing_http_request_x_amzn_mtls_clientcert_leaf_header_name          = (known after apply)
      + routing_http_request_x_amzn_mtls_clientcert_serial_number_header_name = (known after apply)
      + routing_http_request_x_amzn_mtls_clientcert_subject_header_name       = (known after apply)
      + routing_http_request_x_amzn_mtls_clientcert_validity_header_name      = (known after apply)
      + routing_http_request_x_amzn_tls_cipher_suite_header_name              = (known after apply)
      + routing_http_request_x_amzn_tls_version_header_name                   = (known after apply)
      + routing_http_response_access_control_allow_credentials_header_value   = (known after apply)
      + routing_http_response_access_control_allow_headers_header_value       = (known after apply)
      + routing_http_response_access_control_allow_methods_header_value       = (known after apply)
      + routing_http_response_access_control_allow_origin_header_value        = (known after apply)
      + routing_http_response_access_control_expose_headers_header_value      = (known after apply)
      + routing_http_response_access_control_max_age_header_value             = (known after apply)
      + routing_http_response_content_security_policy_header_value            = (known after apply)
      + routing_http_response_server_enabled                                  = (known after apply)
      + routing_http_response_strict_transport_security_header_value          = (known after apply)
      + routing_http_response_x_content_type_options_header_value             = (known after apply)
      + routing_http_response_x_frame_options_header_value                    = (known after apply)
      + ssl_policy                                                            = (known after apply)
      + tags_all                                                              = (known after apply)
      + tcp_idle_timeout_seconds                                              = (known after apply)

      + default_action {
          + order            = (known after apply)
          + target_group_arn = (known after apply)
          + type             = "forward"
        }
    }

  # aws_lb_target_group.control will be created
  + resource "aws_lb_target_group" "control" {
      + arn                               = (known after apply)
      + arn_suffix                        = (known after apply)
      + connection_termination            = (known after apply)
      + id                                = (known after apply)
      + ip_address_type                   = (known after apply)
      + load_balancer_arns                = (known after apply)
      + load_balancing_algorithm_type     = (known after apply)
      + load_balancing_anomaly_mitigation = (known after apply)
      + load_balancing_cross_zone_enabled = (known after apply)
      + name                              = "gridos-staging-control"
      + name_prefix                       = (known after apply)
      + port                              = 8080
      + preserve_client_ip                = (known after apply)
      + protocol                          = "HTTP"
      + protocol_version                  = (known after apply)
      + region                            = (known after apply)
      + tags_all                          = (known after apply)
      + target_type                       = "ip"
      + vpc_id                            = "vpc-0123456789abcdef0"

      + health_check {
          + matcher = "200-399"
          + path    = "/geo/style.json"
          + timeout = (known after apply)
        }
    }

  # aws_secretsmanager_secret.runtime["database-url"] will be created
  + resource "aws_secretsmanager_secret" "runtime" {
      + arn                     = (known after apply)
      + id                      = (known after apply)
      + name                    = "gridos-staging/database-url"
      + name_prefix             = (known after apply)
      + policy                  = (known after apply)
      + recovery_window_in_days = 7
      + region                  = (known after apply)
      + tags_all                = (known after apply)
    }

  # aws_secretsmanager_secret.runtime["gateway-token"] will be created
  + resource "aws_secretsmanager_secret" "runtime" {
      + arn                     = (known after apply)
      + id                      = (known after apply)
      + name                    = "gridos-staging/gateway-token"
      + name_prefix             = (known after apply)
      + policy                  = (known after apply)
      + recovery_window_in_days = 7
      + region                  = (known after apply)
      + tags_all                = (known after apply)
    }

  # aws_secretsmanager_secret.runtime["step-up-key"] will be created
  + resource "aws_secretsmanager_secret" "runtime" {
      + arn                     = (known after apply)
      + id                      = (known after apply)
      + name                    = "gridos-staging/step-up-key"
      + name_prefix             = (known after apply)
      + policy                  = (known after apply)
      + recovery_window_in_days = 7
      + region                  = (known after apply)
      + tags_all                = (known after apply)
    }

  # aws_security_group.alb will be created
  + resource "aws_security_group" "alb" {
      + arn         = (known after apply)
      + egress      = (known after apply)
      + id          = (known after apply)
      + ingress     = (known after apply)
      + name        = (known after apply)
      + name_prefix = "gridos-staging-alb-"
      + owner_id    = (known after apply)
      + region      = (known after apply)
      + tags_all    = (known after apply)
      + vpc_id      = "vpc-0123456789abcdef0"
    }

  # aws_security_group.database will be created
  + resource "aws_security_group" "database" {
      + arn         = (known after apply)
      + egress      = (known after apply)
      + id          = (known after apply)
      + ingress     = (known after apply)
      + name        = (known after apply)
      + name_prefix = "gridos-staging-database-"
      + owner_id    = (known after apply)
      + region      = (known after apply)
      + tags_all    = (known after apply)
      + vpc_id      = "vpc-0123456789abcdef0"
    }

  # aws_security_group.replay will be created
  + resource "aws_security_group" "replay" {
      + arn         = (known after apply)
      + egress      = (known after apply)
      + id          = (known after apply)
      + ingress     = (known after apply)
      + name        = (known after apply)
      + name_prefix = "gridos-staging-replay-"
      + owner_id    = (known after apply)
      + region      = (known after apply)
      + tags_all    = (known after apply)
      + vpc_id      = "vpc-0123456789abcdef0"
    }

  # aws_security_group.service will be created
  + resource "aws_security_group" "service" {
      + arn         = (known after apply)
      + egress      = (known after apply)
      + id          = (known after apply)
      + ingress     = (known after apply)
      + name        = (known after apply)
      + name_prefix = "gridos-staging-service-"
      + owner_id    = (known after apply)
      + region      = (known after apply)
      + tags_all    = (known after apply)
      + vpc_id      = "vpc-0123456789abcdef0"
    }

  # aws_service_discovery_private_dns_namespace.main will be created
  + resource "aws_service_discovery_private_dns_namespace" "main" {
      + arn         = (known after apply)
      + hosted_zone = (known after apply)
      + id          = (known after apply)
      + name        = "gridos.internal"
      + region      = (known after apply)
      + tags_all    = (known after apply)
      + vpc         = "vpc-0123456789abcdef0"
    }

  # aws_service_discovery_service.service["control"] will be created
  + resource "aws_service_discovery_service" "service" {
      + arn          = (known after apply)
      + id           = (known after apply)
      + name         = "control"
      + namespace_id = (known after apply)
      + region       = (known after apply)
      + tags_all     = (known after apply)
      + type         = (known after apply)

      + dns_config {
          + namespace_id   = (known after apply)
          + routing_policy = "MULTIVALUE"

          + dns_records {
              + ttl  = 10
              + type = "A"
            }
        }

      + health_check_custom_config {}
    }

  # aws_service_discovery_service.service["decision"] will be created
  + resource "aws_service_discovery_service" "service" {
      + arn          = (known after apply)
      + id           = (known after apply)
      + name         = "decision"
      + namespace_id = (known after apply)
      + region       = (known after apply)
      + tags_all     = (known after apply)
      + type         = (known after apply)

      + dns_config {
          + namespace_id   = (known after apply)
          + routing_policy = "MULTIVALUE"

          + dns_records {
              + ttl  = 10
              + type = "A"
            }
        }

      + health_check_custom_config {}
    }

  # aws_service_discovery_service.service["gateway"] will be created
  + resource "aws_service_discovery_service" "service" {
      + arn          = (known after apply)
      + id           = (known after apply)
      + name         = "gateway"
      + namespace_id = (known after apply)
      + region       = (known after apply)
      + tags_all     = (known after apply)
      + type         = (known after apply)

      + dns_config {
          + namespace_id   = (known after apply)
          + routing_policy = "MULTIVALUE"

          + dns_records {
              + ttl  = 10
              + type = "A"
            }
        }

      + health_check_custom_config {}
    }

  # aws_vpc_security_group_egress_rule.alb will be created
  + resource "aws_vpc_security_group_egress_rule" "alb" {
      + arn                          = (known after apply)
      + from_port                    = 8080
      + id                           = (known after apply)
      + ip_protocol                  = "tcp"
      + referenced_security_group_id = (known after apply)
      + region                       = (known after apply)
      + security_group_id            = (known after apply)
      + security_group_rule_id       = (known after apply)
      + tags_all                     = (known after apply)
      + to_port                      = 8080
    }

  # aws_vpc_security_group_egress_rule.service will be created
  + resource "aws_vpc_security_group_egress_rule" "service" {
      + arn                    = (known after apply)
      + cidr_ipv4              = "0.0.0.0/0"
      + id                     = (known after apply)
      + ip_protocol            = "-1"
      + region                 = (known after apply)
      + security_group_id      = (known after apply)
      + security_group_rule_id = (known after apply)
      + tags_all               = (known after apply)
    }

  # aws_vpc_security_group_ingress_rule.control will be created
  + resource "aws_vpc_security_group_ingress_rule" "control" {
      + arn                          = (known after apply)
      + from_port                    = 8080
      + id                           = (known after apply)
      + ip_protocol                  = "tcp"
      + referenced_security_group_id = (known after apply)
      + region                       = (known after apply)
      + security_group_id            = (known after apply)
      + security_group_rule_id       = (known after apply)
      + tags_all                     = (known after apply)
      + to_port                      = 8080
    }

  # aws_vpc_security_group_ingress_rule.database will be created
  + resource "aws_vpc_security_group_ingress_rule" "database" {
      + arn                          = (known after apply)
      + from_port                    = 5432
      + id                           = (known after apply)
      + ip_protocol                  = "tcp"
      + referenced_security_group_id = (known after apply)
      + region                       = (known after apply)
      + security_group_id            = (known after apply)
      + security_group_rule_id       = (known after apply)
      + tags_all                     = (known after apply)
      + to_port                      = 5432
    }

  # aws_vpc_security_group_ingress_rule.https will be created
  + resource "aws_vpc_security_group_ingress_rule" "https" {
      + arn                    = (known after apply)
      + cidr_ipv4              = "0.0.0.0/0"
      + from_port              = 443
      + id                     = (known after apply)
      + ip_protocol            = "tcp"
      + region                 = (known after apply)
      + security_group_id      = (known after apply)
      + security_group_rule_id = (known after apply)
      + tags_all               = (known after apply)
      + to_port                = 443
    }

  # aws_vpc_security_group_ingress_rule.replay will be created
  + resource "aws_vpc_security_group_ingress_rule" "replay" {
      + arn                          = (known after apply)
      + from_port                    = 2049
      + id                           = (known after apply)
      + ip_protocol                  = "tcp"
      + referenced_security_group_id = (known after apply)
      + region                       = (known after apply)
      + security_group_id            = (known after apply)
      + security_group_rule_id       = (known after apply)
      + tags_all                     = (known after apply)
      + to_port                      = 2049
    }

  # aws_vpc_security_group_ingress_rule.service["50061"] will be created
  + resource "aws_vpc_security_group_ingress_rule" "service" {
      + arn                          = (known after apply)
      + from_port                    = 50061
      + id                           = (known after apply)
      + ip_protocol                  = "tcp"
      + referenced_security_group_id = (known after apply)
      + region                       = (known after apply)
      + security_group_id            = (known after apply)
      + security_group_rule_id       = (known after apply)
      + tags_all                     = (known after apply)
      + to_port                      = 50061
    }

  # aws_vpc_security_group_ingress_rule.service["8080"] will be created
  + resource "aws_vpc_security_group_ingress_rule" "service" {
      + arn                          = (known after apply)
      + from_port                    = 8080
      + id                           = (known after apply)
      + ip_protocol                  = "tcp"
      + referenced_security_group_id = (known after apply)
      + region                       = (known after apply)
      + security_group_id            = (known after apply)
      + security_group_rule_id       = (known after apply)
      + tags_all                     = (known after apply)
      + to_port                      = 8080
    }

  # aws_vpc_security_group_ingress_rule.service["8081"] will be created
  + resource "aws_vpc_security_group_ingress_rule" "service" {
      + arn                          = (known after apply)
      + from_port                    = 8081
      + id                           = (known after apply)
      + ip_protocol                  = "tcp"
      + referenced_security_group_id = (known after apply)
      + region                       = (known after apply)
      + security_group_id            = (known after apply)
      + security_group_rule_id       = (known after apply)
      + tags_all                     = (known after apply)
      + to_port                      = 8081
    }

  # google_bigquery_dataset_iam_member.writer will be created
  + resource "google_bigquery_dataset_iam_member" "writer" {
      + dataset_id = "telemetry"
      + etag       = (known after apply)
      + id         = (known after apply)
      + member     = (known after apply)
      + project    = "gridos-plan-example"
      + role       = "roles/bigquery.dataEditor"
    }

  # google_iam_workload_identity_pool.aws will be created
  + resource "google_iam_workload_identity_pool" "aws" {
      + deletion_policy           = (known after apply)
      + display_name              = "GridOS AWS workloads"
      + id                        = (known after apply)
      + mode                      = (known after apply)
      + name                      = (known after apply)
      + project                   = "gridos-plan-example"
      + state                     = (known after apply)
      + workload_identity_pool_id = "gridos-aws"
    }

  # google_iam_workload_identity_pool_provider.aws will be created
  + resource "google_iam_workload_identity_pool_provider" "aws" {
      + attribute_condition                = "assertion.arn.startsWith('arn:aws:sts::123456789012:assumed-role/gridos-staging-worker/')"
      + attribute_mapping                  = {
          + "attribute.aws_role" = "assertion.arn.extract('assumed-role/{role_name}/')"
          + "google.subject"     = "assertion.arn"
        }
      + deletion_policy                    = (known after apply)
      + display_name                       = "GridOS ECS task roles"
      + id                                 = (known after apply)
      + name                               = (known after apply)
      + project                            = "gridos-plan-example"
      + state                              = (known after apply)
      + workload_identity_pool_id          = "gridos-aws"
      + workload_identity_pool_provider_id = "ecs-task"

      + aws {
          + account_id = "123456789012"
        }
    }

  # google_service_account.telemetry_writer will be created
  + resource "google_service_account" "telemetry_writer" {
      + account_id      = "gridos-telemetry-writer"
      + deletion_policy = (known after apply)
      + display_name    = "GridOS telemetry writer"
      + email           = (known after apply)
      + id              = (known after apply)
      + member          = (known after apply)
      + name            = (known after apply)
      + project         = "gridos-plan-example"
      + unique_id       = (known after apply)
    }

  # google_service_account_iam_member.worker will be created
  + resource "google_service_account_iam_member" "worker" {
      + etag               = (known after apply)
      + id                 = (known after apply)
      + member             = "principalSet://iam.googleapis.com/projects/123456789012/locations/global/workloadIdentityPools/gridos-aws/attribute.aws_role/gridos-staging-worker"
      + role               = "roles/iam.workloadIdentityUser"
      + service_account_id = (known after apply)
    }

Plan: 54 to add, 0 to change, 0 to destroy.

Changes to Outputs:
  + alb_dns_name                = (known after apply)
  + database_endpoint           = (known after apply)
  + runtime_secret_arns         = {
      + database-url  = (known after apply)
      + gateway-token = (known after apply)
      + step-up-key   = (known after apply)
    }
  + task_role_arns              = {
      + control  = (known after apply)
      + decision = (known after apply)
      + gateway  = (known after apply)
      + worker   = (known after apply)
    }
  + warehouse_identity_audience = "//iam.googleapis.com/projects/123456789012/locations/global/workloadIdentityPools/gridos-aws/providers/ecs-task"
  + warehouse_service_account   = (known after apply)

tests/staging.tftest.hcl... tearing down
tests/staging.tftest.hcl... pass

Success! 1 passed, 0 failed.
```
