variable "region" {
  type = string
}

variable "name" {
  type    = string
  default = "gridos-staging"
}

variable "vpc_id" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
  validation {
    condition     = length(var.private_subnet_ids) >= 2
    error_message = "At least two private subnets are required."
  }
}

variable "public_subnet_ids" {
  type = list(string)
  validation {
    condition     = length(var.public_subnet_ids) >= 2
    error_message = "At least two public subnets are required."
  }
}

variable "certificate_arn" {
  type = string
}

variable "images" {
  type = object({
    control  = string
    worker   = string
    decision = string
    gateway  = string
  })
}

variable "build_revision" {
  type = string
  validation {
    condition     = length(trimspace(var.build_revision)) > 0
    error_message = "A code revision is required for replay manifests."
  }
}

variable "temporal_address" {
  type = string
  validation {
    condition     = can(regex("^[^:]+:[0-9]+$", var.temporal_address))
    error_message = "Use a managed or self-hosted Temporal host:port address."
  }
}

variable "fleet_path" {
  type    = string
  default = "/app/testdata/fleets/austin-5000.jsonl"
}

variable "db_instance_class" {
  type    = string
  default = "db.t4g.small"
}
