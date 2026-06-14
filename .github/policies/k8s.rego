package kubernetes

# ── Policy: No privileged containers ─────────────────────────────────────────
deny[msg] {
  input.kind == "Deployment"
  container := input.spec.template.spec.containers[_]
  container.securityContext.privileged == true
  msg := sprintf("Container '%s' must not run as privileged", [container.name])
}

# ── Policy: No 'latest' image tag ────────────────────────────────────────────
deny[msg] {
  input.kind == "Deployment"
  container := input.spec.template.spec.containers[_]
  endswith(container.image, ":latest")
  msg := sprintf("Container '%s' must not use ':latest' tag — use immutable SHA tags", [container.name])
}

# ── Policy: Resource limits required ─────────────────────────────────────────
deny[msg] {
  input.kind == "Deployment"
  container := input.spec.template.spec.containers[_]
  not container.resources.limits.cpu
  msg := sprintf("Container '%s' must have CPU limits set", [container.name])
}

deny[msg] {
  input.kind == "Deployment"
  container := input.spec.template.spec.containers[_]
  not container.resources.limits.memory
  msg := sprintf("Container '%s' must have memory limits set", [container.name])
}

# ── Policy: Must run as non-root ──────────────────────────────────────────────
deny[msg] {
  input.kind == "Deployment"
  container := input.spec.template.spec.containers[_]
  not container.securityContext.runAsNonRoot
  msg := sprintf("Container '%s' must set runAsNonRoot: true", [container.name])
}

# ── Policy: Must drop ALL capabilities ───────────────────────────────────────
deny[msg] {
  input.kind == "Deployment"
  container := input.spec.template.spec.containers[_]
  not container.securityContext.capabilities.drop
  msg := sprintf("Container '%s' must drop ALL capabilities", [container.name])
}

# ── Policy: Readiness probe required ─────────────────────────────────────────
deny[msg] {
  input.kind == "Deployment"
  container := input.spec.template.spec.containers[_]
  not container.readinessProbe
  msg := sprintf("Container '%s' must have a readinessProbe", [container.name])
}

# ── Policy: Liveness probe required ──────────────────────────────────────────
deny[msg] {
  input.kind == "Deployment"
  container := input.spec.template.spec.containers[_]
  not container.livenessProbe
  msg := sprintf("Container '%s' must have a livenessProbe", [container.name])
}

# ── Policy: No secrets in env vars ───────────────────────────────────────────
deny[msg] {
  input.kind == "Deployment"
  container := input.spec.template.spec.containers[_]
  env := container.env[_]
  # Detect hardcoded secrets (value present, not from secretKeyRef)
  env.value
  contains(lower(env.name), "password")
  msg := sprintf("Container '%s' env var '%s' must use secretKeyRef, not hardcoded value", [container.name, env.name])
}

deny[msg] {
  input.kind == "Deployment"
  container := input.spec.template.spec.containers[_]
  env := container.env[_]
  env.value
  contains(lower(env.name), "secret")
  msg := sprintf("Container '%s' env var '%s' must use secretKeyRef, not hardcoded value", [container.name, env.name])
}
