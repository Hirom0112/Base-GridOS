resource "aws_efs_file_system" "replay" {
  creation_token = "${var.name}-replay"
  encrypted      = true
}

resource "aws_efs_access_point" "replay" {
  file_system_id = aws_efs_file_system.replay.id
  posix_user {
    uid = 65532
    gid = 65532
  }
  root_directory {
    path = "/gridos/replay"
    creation_info {
      owner_uid   = 65532
      owner_gid   = 65532
      permissions = "0750"
    }
  }
}

resource "aws_security_group" "replay" {
  name_prefix = "${var.name}-replay-"
  vpc_id      = var.vpc_id
}

resource "aws_vpc_security_group_ingress_rule" "replay" {
  security_group_id            = aws_security_group.replay.id
  referenced_security_group_id = aws_security_group.service.id
  ip_protocol                  = "tcp"
  from_port                    = 2049
  to_port                      = 2049
}

resource "aws_efs_mount_target" "replay" {
  for_each        = toset(var.private_subnet_ids)
  file_system_id  = aws_efs_file_system.replay.id
  subnet_id       = each.key
  security_groups = [aws_security_group.replay.id]
}
