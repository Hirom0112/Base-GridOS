output "alb_dns_name" {
  value = aws_lb.main.dns_name
}

output "database_endpoint" {
  value = aws_db_instance.main.endpoint
}

output "runtime_secret_arns" {
  value = { for name, secret in aws_secretsmanager_secret.runtime : name => secret.arn }
}

output "task_role_arns" {
  value = { for name, role in aws_iam_role.task : name => role.arn }
}
