# Keys: one for data at rest, one for backups. Rotation on.
resource "aws_kms_key" "data" {
  description         = "${local.name} data at rest"
  enable_key_rotation = true
}

resource "aws_kms_key" "backups" {
  description         = "${local.name} backups"
  enable_key_rotation = true
}

resource "aws_db_subnet_group" "db" {
  name       = local.name
  subnet_ids = aws_subnet.private[*].id
}

# Three instances (D-50, D-57):
#   main   - identity, profile, safety, commerce databases; 35-day point-in-time recovery
#   keys   - member data keys only; 7-day recovery so erasure reaches backups within a week
#   ledger - erasure ledger; restored alone, always the latest copy
locals {
  dbs = {
    main   = { retention = 35 }
    keys   = { retention = 7 }
    ledger = { retention = 35 }
  }
}

resource "aws_db_instance" "pg" {
  for_each                            = local.dbs
  identifier                          = "${local.name}-${each.key}"
  engine                              = "postgres"
  engine_version                      = "16"
  instance_class                      = var.db_instance_class
  allocated_storage                   = 50
  max_allocated_storage               = 500
  storage_encrypted                   = true
  kms_key_id                          = aws_kms_key.data.arn
  multi_az                            = true
  db_subnet_group_name                = aws_db_subnet_group.db.name
  vpc_security_group_ids              = [aws_security_group.db.id]
  username                            = "tryst_admin"
  manage_master_user_password         = true # in Secrets Manager, never in state
  backup_retention_period             = each.value.retention
  deletion_protection                 = true
  copy_tags_to_snapshot               = true
  iam_database_authentication_enabled = true
  performance_insights_enabled        = true
  performance_insights_kms_key_id     = aws_kms_key.data.arn
  auto_minor_version_upgrade          = true
  skip_final_snapshot                 = false
  final_snapshot_identifier           = "${local.name}-${each.key}-final"
  publicly_accessible                 = false
}

# Logical backups (scripts/backup.sh): object-locked so a compromised account can't delete
# them; the key database's copies expire after 7 days.
resource "aws_s3_bucket" "backups" {
  bucket_prefix       = "${local.name}-backups-"
  object_lock_enabled = true
}

resource "aws_s3_bucket" "key_backups" {
  bucket_prefix       = "${local.name}-key-backups-"
  object_lock_enabled = true
}

resource "aws_s3_bucket_public_access_block" "b" {
  for_each                = { backups = aws_s3_bucket.backups.id, keys = aws_s3_bucket.key_backups.id }
  bucket                  = each.value
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "b" {
  for_each = { backups = aws_s3_bucket.backups.id, keys = aws_s3_bucket.key_backups.id }
  bucket   = each.value
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = aws_kms_key.backups.arn
    }
  }
}

resource "aws_s3_bucket_object_lock_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id
  rule {
    default_retention {
      mode = "COMPLIANCE"
      days = 35
    }
  }
}

resource "aws_s3_bucket_object_lock_configuration" "key_backups" {
  bucket = aws_s3_bucket.key_backups.id
  rule {
    default_retention {
      mode = "COMPLIANCE"
      days = 7
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id
  rule {
    id     = "expire"
    status = "Enabled"
    filter {}
    expiration { days = 36 }
    noncurrent_version_expiration { noncurrent_days = 1 }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "key_backups" {
  bucket = aws_s3_bucket.key_backups.id
  rule {
    id     = "expire"
    status = "Enabled"
    filter {}
    expiration { days = 8 }
    noncurrent_version_expiration { noncurrent_days = 1 }
  }
}

# Secrets: created empty; values are put by the operator (never in state or the repo).
locals {
  # Least privilege: each service reads only its own secrets.
  api_secrets = ["DATA_KEK", "DSR_SIGNING_KEY", "CONTACT_PEPPER", "VAPID_PRIVATE_KEY", "POSTMARK_SERVER_TOKEN",
    "TWILIO_AUTH_TOKEN", "STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "LIVENESS_SECRET", "SIGHTENGINE_SECRET",
    "PHOTODNA_KEY", "ARC_SECRET", "ARC_API_KEY", "LIVENESS_API_KEY", "ANCHOR_PEPPER", "ASSURANCE_SECRET", "ASSURANCE_API_KEY",
    "ASSURANCE_CALLBACK_SECRET", "DATABASE_URL_IDENTITY", "DATABASE_URL_PROFILE", "DATABASE_URL_KEYS",
    "DATABASE_URL_COMMERCE", "DATABASE_URL_LEDGER", "INTERNAL_SECRET_SAFETY", "INTERNAL_SECRET_PLATFORM", "INTERNAL_SECRET_SAFETY_PREVIOUS",
  "INTERNAL_SECRET_PLATFORM_PREVIOUS", "STAFF_ROSTER"]
  # Values are set by scripts/provision-safety-secrets.sh, never by Terraform.
  safety_secrets = ["DATABASE_URL_SAFETY", "DATABASE_URL_LEDGER", "INTERNAL_SECRET_SAFETY", "INTERNAL_SECRET_PLATFORM",
  "INTERNAL_SECRET_SAFETY_PREVIOUS", "INTERNAL_SECRET_PLATFORM_PREVIOUS", "STAFF_ROSTER"]
  secret_names = distinct(concat(local.api_secrets, local.safety_secrets))
}

resource "aws_secretsmanager_secret" "s" {
  for_each   = toset(local.secret_names)
  name       = "${local.name}/${each.key}"
  kms_key_id = aws_kms_key.data.arn
}
