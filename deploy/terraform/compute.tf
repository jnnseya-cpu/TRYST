resource "aws_ecr_repository" "r" {
  for_each             = toset(["api", "web", "ops"])
  name                 = "${local.name}-${each.key}"
  image_tag_mutability = "IMMUTABLE"
  image_scanning_configuration { scan_on_push = true }
  encryption_configuration {
    encryption_type = "KMS"
    kms_key         = aws_kms_key.data.arn
  }
}

resource "aws_cloudwatch_log_group" "l" {
  for_each          = toset(["api", "web", "safety", "ops"])
  name              = "/${local.name}/${each.key}"
  retention_in_days = 90 # logs: 90 days (02 §7)
  kms_key_id        = aws_kms_key.data.arn
}

resource "aws_ecs_cluster" "c" {
  name = local.name
  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "exec" {
  name               = "${local.name}-exec"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

resource "aws_iam_role_policy_attachment" "exec" {
  role       = aws_iam_role.exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "secrets" {
  statement {
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [for s in aws_secretsmanager_secret.s : s.arn]
  }
  statement {
    actions   = ["kms:Decrypt"]
    resources = [aws_kms_key.data.arn]
  }
}

resource "aws_iam_role_policy" "secrets" {
  role   = aws_iam_role.exec.id
  policy = data.aws_iam_policy_document.secrets.json
}

resource "aws_ecs_task_definition" "api" {
  family                   = "${local.name}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 512
  memory                   = 1024
  execution_role_arn       = aws_iam_role.exec.arn
  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "X86_64"
  }
  container_definitions = jsonencode([{
    name         = "api"
    image        = var.api_image
    essential    = true
    portMappings = [{ containerPort = 8081 }, { containerPort = 9090 }]
    environment = concat([
      { name = "TRYST_ENV", value = "prod" }, # staging runs with every production guard too
      { name = "METRICS_ADDR", value = ":9090" },
      { name = "TRUSTED_PROXIES", value = aws_vpc.main.cidr_block },
      { name = "INTERNAL_ADDR", value = ":8091" },
      { name = "SAFETY_INTERNAL_URL", value = "http://safety.tryst.internal:8092" },
    ], [for k, v in var.api_config : { name = k, value = v }])
    secrets                = [for k, s in aws_secretsmanager_secret.s : { name = k, valueFrom = s.arn }]
    readonlyRootFilesystem = true
    stopTimeout            = 30
    healthCheck            = { command = ["/identity-svc", "healthcheck"], interval = 10, retries = 3, timeout = 5 }
    logConfiguration       = { logDriver = "awslogs", options = { "awslogs-group" = aws_cloudwatch_log_group.l["api"].name, "awslogs-region" = var.region, "awslogs-stream-prefix" = "api" } }
  }])
}

resource "aws_ecs_task_definition" "web" {
  family                   = "${local.name}-web"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 512
  memory                   = 1024
  execution_role_arn       = aws_iam_role.exec.arn
  container_definitions = jsonencode([{
    name             = "web"
    image            = var.web_image
    essential        = true
    portMappings     = [{ containerPort = 3000 }]
    logConfiguration = { logDriver = "awslogs", options = { "awslogs-group" = aws_cloudwatch_log_group.l["web"].name, "awslogs-region" = var.region, "awslogs-stream-prefix" = "web" } }
  }])
}

resource "aws_service_discovery_private_dns_namespace" "ns" {
  name = "tryst.internal"
  vpc  = aws_vpc.main.id
}

resource "aws_service_discovery_service" "api" {
  name = "api"
  dns_config {
    namespace_id = aws_service_discovery_private_dns_namespace.ns.id
    dns_records {
      ttl  = 10
      type = "A"
    }
  }
}

resource "aws_ecs_service" "api" {
  name            = "api"
  cluster         = aws_ecs_cluster.c.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = var.api_count
  launch_type     = "FARGATE"
  network_configuration {
    subnets         = aws_subnet.private[*].id
    security_groups = [aws_security_group.api.id]
  }
  service_registries { registry_arn = aws_service_discovery_service.api.arn }
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
}

resource "aws_lb" "public" {
  name                       = local.name
  load_balancer_type         = "application"
  subnets                    = aws_subnet.public[*].id
  security_groups            = [aws_security_group.alb.id]
  drop_invalid_header_fields = true
}

resource "aws_lb_target_group" "web" {
  name        = "${local.name}-web"
  port        = 3000
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"
  health_check { path = "/robots.txt" }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.public.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = var.certificate_arn
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.web.arn
  }
}

resource "aws_ecs_service" "web" {
  name            = "web"
  cluster         = aws_ecs_cluster.c.id
  task_definition = aws_ecs_task_definition.web.arn
  desired_count   = 2
  launch_type     = "FARGATE"
  network_configuration {
    subnets         = aws_subnet.private[*].id
    security_groups = [aws_security_group.web.id]
  }
  load_balancer {
    target_group_arn = aws_lb_target_group.web.arn
    container_name   = "web"
    container_port   = 3000
  }
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
}

# Standalone safety service (D-59): same image, its own task, secrets and database.
resource "aws_ecs_task_definition" "safety" {
  family                   = "${local.name}-safety"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.exec.arn
  container_definitions = jsonencode([{
    name         = "safety"
    image        = var.api_image
    entryPoint   = ["/safety-svc"]
    essential    = true
    portMappings = [{ containerPort = 8082 }, { containerPort = 8092 }, { containerPort = 9092 }]
    environment = [
      { name = "TRYST_ENV", value = "prod" },
      { name = "INTERNAL_ADDR", value = ":8092" },
      { name = "METRICS_ADDR", value = ":9092" },
      { name = "PLATFORM_INTERNAL_URL", value = "http://api.tryst.internal:8091" },
      { name = "STAFF_ACCESS_ISSUER", value = var.staff_access.issuer },
      { name = "STAFF_ACCESS_AUDIENCE", value = var.staff_access.audience },
      { name = "STAFF_ACCESS_JWKS_URL", value = var.staff_access.jwks_url },
      { name = "STAFF_ACCESS_REQUIRE_AMR", value = var.staff_access.require_amr },
    ]
    secrets                = [for k in local.safety_secrets : { name = k, valueFrom = aws_secretsmanager_secret.s[k].arn }]
    readonlyRootFilesystem = true
    stopTimeout            = 30
    healthCheck            = { command = ["/safety-svc", "healthcheck"], interval = 10, retries = 3, timeout = 5 }
    logConfiguration       = { logDriver = "awslogs", options = { "awslogs-group" = aws_cloudwatch_log_group.l["safety"].name, "awslogs-region" = var.region, "awslogs-stream-prefix" = "safety" } }
  }])
}

resource "aws_service_discovery_service" "safety" {
  name = "safety"
  dns_config {
    namespace_id = aws_service_discovery_private_dns_namespace.ns.id
    dns_records {
      ttl  = 10
      type = "A"
    }
  }
}

resource "aws_ecs_service" "safety" {
  name            = "safety"
  cluster         = aws_ecs_cluster.c.id
  task_definition = aws_ecs_task_definition.safety.arn
  desired_count   = 2
  launch_type     = "FARGATE"
  network_configuration {
    subnets         = aws_subnet.private[*].id
    security_groups = [aws_security_group.safety.id]
  }
  service_registries { registry_arn = aws_service_discovery_service.safety.arn }
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
}

# Moderator console: the web app built with OPS_CONSOLE=1 on a staff-only host. The host's
# DNS points at the company sign-in proxy (Cloudflare Access), which forwards here with its
# signed token; the safety service verifies that token (STAFF_ACCESS_*).
resource "aws_ecs_task_definition" "ops" {
  family                   = "${local.name}-ops"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.exec.arn
  container_definitions = jsonencode([{
    name             = "ops"
    image            = var.ops_image
    essential        = true
    portMappings     = [{ containerPort = 3000 }]
    logConfiguration = { logDriver = "awslogs", options = { "awslogs-group" = aws_cloudwatch_log_group.l["ops"].name, "awslogs-region" = var.region, "awslogs-stream-prefix" = "ops" } }
  }])
}

resource "aws_lb_target_group" "ops" {
  name        = "${local.name}-ops"
  port        = 3000
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"
  health_check { path = "/robots.txt" }
}

resource "aws_lb_listener_rule" "ops" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 10
  condition {
    host_header { values = [var.ops_host] }
  }
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.ops.arn
  }
}

resource "aws_ecs_service" "ops" {
  name            = "ops"
  cluster         = aws_ecs_cluster.c.id
  task_definition = aws_ecs_task_definition.ops.arn
  desired_count   = 1
  launch_type     = "FARGATE"
  network_configuration {
    subnets         = aws_subnet.private[*].id
    security_groups = [aws_security_group.web.id]
  }
  load_balancer {
    target_group_arn = aws_lb_target_group.ops.arn
    container_name   = "ops"
    container_port   = 3000
  }
}

output "alb_dns_name" { value = aws_lb.public.dns_name }
output "backup_bucket" { value = aws_s3_bucket.backups.bucket }
output "key_backup_bucket" { value = aws_s3_bucket.key_backups.bucket }
