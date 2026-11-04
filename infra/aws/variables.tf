variable "aws_region" {
  type        = string
  description = "AWS region for the platform."
  default     = "us-east-1"
}

variable "environment" {
  type        = string
  description = "Short environment identifier."
  default     = "production"
  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,18}$", var.environment))
    error_message = "environment must be a lowercase slug between 2 and 19 characters."
  }
}

variable "vpc_cidr" {
  type    = string
  default = "10.42.0.0/16"
}

variable "application_image" {
  type        = string
  description = "Immutable API/worker image URI, preferably addressed by ECR digest."
  validation {
    condition     = strcontains(var.application_image, "@sha256:")
    error_message = "application_image must use an immutable sha256 digest."
  }
}

variable "certificate_arn" {
  type        = string
  description = "ACM certificate ARN for the public ALB HTTPS listener."
  validation {
    condition     = can(regex("^arn:aws[a-z-]*:acm:", var.certificate_arn))
    error_message = "certificate_arn must be an ACM certificate ARN."
  }
}

variable "public_url" {
  type        = string
  description = "Canonical HTTPS URL used in verification and invitation links."
  validation {
    condition     = can(regex("^https://", var.public_url))
    error_message = "public_url must use HTTPS."
  }
}

variable "from_email" {
  type        = string
  description = "Verified SES sender address used for platform identity emails."
  validation {
    condition     = can(regex("^[^@[:space:]]+@[^@[:space:]]+\\.[^@[:space:]]+$", var.from_email))
    error_message = "from_email must be a valid email address."
  }
}

variable "ses_identity_arn" {
  type        = string
  description = "Verified SES email or domain identity ARN allowed for the task role."
  validation {
    condition     = can(regex("^arn:aws[a-z-]*:ses:", var.ses_identity_arn))
    error_message = "ses_identity_arn must be an SES identity ARN."
  }
}

variable "cloudfront_public_key_pem" {
  type        = string
  description = "PEM-encoded public key used by CloudFront to verify application-signed media URLs."
  sensitive   = true
  validation {
    condition     = strcontains(var.cloudfront_public_key_pem, "BEGIN PUBLIC KEY")
    error_message = "cloudfront_public_key_pem must contain a PEM public key."
  }
}

variable "meta_secret_arn" {
  type        = string
  description = "Secrets Manager ARN for a JSON secret containing META_APP_SECRET and META_VERIFY_TOKEN."
  validation {
    condition     = can(regex("^arn:aws[a-z-]*:secretsmanager:", var.meta_secret_arn))
    error_message = "meta_secret_arn must be a Secrets Manager ARN."
  }
}

variable "database_instance_class" {
  type    = string
  default = "db.r7g.large"
}

variable "database_allocated_storage" {
  type    = number
  default = 100
}

variable "postgres_version" {
  type    = string
  default = "17.4"
}

variable "redis_node_type" {
  type    = string
  default = "cache.r7g.large"
}

variable "api_cpu" {
  type    = number
  default = 1024
}

variable "api_memory" {
  type    = number
  default = 2048
}

variable "worker_cpu" {
  type    = number
  default = 1024
}

variable "worker_memory" {
  type    = number
  default = 2048
}

variable "api_desired_count" {
  type    = number
  default = 2
}

variable "worker_desired_count" {
  type    = number
  default = 2
}

variable "deletion_protection" {
  type        = bool
  description = "Protect stateful production resources from accidental deletion."
  default     = true
}

variable "alert_email" {
  type        = string
  description = "Optional email subscribed to the operational alarm topic."
  default     = ""
}
