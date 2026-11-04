terraform {
  # Supply bucket, key, region, encryption, and locking through
  # `terraform init -backend-config=...`; state storage is intentionally not
  # created by the same state it protects.
  backend "s3" {}
}
