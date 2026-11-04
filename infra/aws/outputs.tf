output "api_endpoint" {
  value       = "https://${aws_lb.api.dns_name}"
  description = "ALB endpoint. Point the public DNS name at this target."
}

output "media_bucket" {
  value = aws_s3_bucket.media.id
}

output "media_distribution_domain" {
  value = aws_cloudfront_distribution.media.domain_name
}

output "ecs_cluster" {
  value = aws_ecs_cluster.main.name
}

output "runtime_secret_arn" {
  value     = aws_secretsmanager_secret.runtime.arn
  sensitive = true
}
