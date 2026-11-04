# BillionMail SaaS on AWS

This Terraform root deploys the managed platform runtime across three availability zones:

- public ALB with TLS, WAF managed rules, and IP rate limiting;
- private ECS/Fargate API and worker services with deployment rollback and autoscaling;
- Multi-AZ RDS PostgreSQL with encrypted storage, PITR backups, forced TLS, and enhanced monitoring;
- encrypted Multi-AZ ElastiCache Redis with authentication and automatic failover;
- KMS-encrypted, versioned S3 media storage behind CloudFront OAC;
- Secrets Manager runtime credentials, encrypted CloudWatch logs, alarms, and SNS notifications.

The image must be immutable and contain both `/opt/billionmail/core/billionmail` and `/opt/billionmail/core/saas-worker`. Legacy mail-control APIs are disabled by `SAAS_MANAGED_MODE=true`; the existing self-hosted Compose deployment remains independent.

## State bootstrap

Create a versioned, encrypted S3 state bucket outside this root, then initialize with native S3 locking:

```sh
terraform init \
  -backend-config="bucket=YOUR_TERRAFORM_STATE_BUCKET" \
  -backend-config="key=billionmail/production.tfstate" \
  -backend-config="region=us-east-1" \
  -backend-config="encrypt=true" \
  -backend-config="use_lockfile=true"
```

Use a dedicated deployment role with least-privilege permissions. Do not store `terraform.tfvars`, private keys, provider credentials, or generated plans in Git.

## Deployment

```sh
cp terraform.tfvars.example terraform.tfvars
terraform fmt -check -recursive
terraform validate
terraform plan -out=production.tfplan
terraform apply production.tfplan
```

Before traffic is enabled, point the application DNS record to the `api_endpoint` ALB, confirm `/health/ready`, run migrations from one task, verify RDS/Redis failover alarms, and execute the documented restore rehearsal.

The generated database and Redis passwords exist in Terraform state and Secrets Manager. The remote state bucket therefore requires restricted IAM access, versioning, KMS encryption, access logging, and recovery controls.

CloudFront rejects unsigned media requests through its trusted key group. Store the corresponding private key outside Terraform (for example in a separately controlled Secrets Manager secret) and grant it only to the media-signing service; never place it in `tfvars` or task environment variables.
