# AGENTS.md

ClickHouse Operator providing ClickHouse as a service on Kubernetes and OpenShift.
Monorepo with Helm charts, Go microservices, and integration tests.


## Repository Layout

Monorepo with Go services using per-service `go.mod` files.

- `services/site-manager` — Backup scheduler + HTTP server; gocron and K8s controller-runtime.
- `services/backup-orchestrator` — CLI for backup/restore/delete ops; flag-based actions.
- `services/dbaas-adapter` — DBaaS API server (Fiber); four adapter modes: `basic/`, `cluster/`, `declarative/`, `initial/`.
- `services/sidecar` — Minimal backup transfer server on port 7172.
- `services/secret-monitor` — K8s informer watcher; reconciles secrets and credentials.
- `services/hook` — Post-deployment hook that calls `handler.UnlockSecret()`.
- `services/pvc-cleanup` — Persistent volume cleanup utility.
- `helm/clickhouse/` — Main Helm chart; includes CRDs, operator deployment, cluster configs, hooks.
- `helm/clickhouse-services/` — Supplementary services chart.
- `integration-tests/` — Python/Robot Framework test suite.
- `docker-transfer/` — Docker image context.
- `243/`, `258/`, `269/` — Docker image contexts.
- `docs/` — User-facing documentation.
- `agent-packages/` — Agent automation packages.

## Service Code Organization

Each service under `services/<name>/` has:
- `Makefile` — Targets: `deps`, `fmt`, `compile`, `docker-build`, `docker-push`, `clean`. See Docker Image Push Rules before using image-related targets.
- `go.mod`, `go.sum` — Check `go.mod` for the Go version and dependencies required by each service
- `cmd/main.go` or `adapter/main.go` — Entry point.

Common dependencies: `go.uber.org/zap` (logging), `k8s.io/client-go`, `sigs.k8s.io/controller-runtime`.

**Note:** `dbaas-adapter` uses `adapter/main.go`; compile builds from `./adapter`.

## Helm Chart Organization

Both charts follow standard Helm structure:

- `Chart.yaml` — Chart metadata and versions.
- `values.yaml` — Default parameters; must have sensible defaults for all new keys.
- `templates/` — Kubernetes manifests rendered from values.
  - `_helpers.tpl` — Shared template functions for labels, annotations, security contexts.
- `crds/` — Contains the ClickHouseInstallation CRD.
  - `crd.yaml` — CRD vendored from upstream Altinity and updated alongside operator version upgrades.
- `monitoring/` — Pre-built Grafana dashboards included in the ClickHouse chart.
  - `clickhouse_grafana_dashboard.json` — General ClickHouse metrics dashboard.
  - `clickhouse_performance_grafana_dashboard.json` — Performance metrics dashboard.

## Build Triggers and Pass Conditions

### Service Dependencies (`services/*/go.mod`, `services/*/go.sum`)

```sh
cd services/<service-name>
make deps
```

Pass: Exit 0; `go.mod` and `go.sum` consistent.

### Service Code (`services/**/*.go`)

```sh
cd services/<service-name>
make fmt && make compile
```

Pass: Exit 0; binary in `build/_output/bin/`. `make fmt` runs `gofmt -l -s -w .` which rewrites files; review the diff for formatting changes.

### Services with Tests

For services that have `*_test.go` files:

```sh
cd services/<service-name>
go test ./...
```

Pass: All tests pass; exit 0.

**Note:** Site-manager, backup-orchestrator, and other services have test files but no test target defined. Run `go test ./...` for each service with tests.

### Helm Charts (`helm/clickhouse/**`, `helm/clickhouse-services/**`)

```sh
helm lint helm/clickhouse
helm lint helm/clickhouse-services
helm template test helm/clickhouse
helm template test helm/clickhouse-services
```

Pass: All commands exit 0; no Helm lint errors; review any warnings; templates render successfully.

## Compatibility Rules

All changes MUST preserve backward compatibility unless an explicit breaking change is approved.

### Helm Charts

Applies to:
- `helm/clickhouse/`
- `helm/clickhouse-services/`

#### Helm Values

- Every new parameter MUST have a default; never break existing values files.
- Never rename or remove keys; breaking changes require explicit user approval and ticket.
- Document any migration path for users.
- Use deprecation aliases when deprecating old keys; test both old and new keys work during transition.

### ClickHouseInstallation CRD (`helm/clickhouse/crds/crd.yaml`)

- CRD at `helm/clickhouse/crds/crd.yaml` is sourced from the upstream Altinity ClickHouse Operator and should remain aligned with the corresponding operator release. Update it only as part of an operator version upgrade. Do not modify it independently without an explicit requirement.
- When updating the operator version, verify compatibility with existing ClickHouseInstallation (CHI) resources using current production values. Run `helm template test helm/clickhouse --include-crds` to include the CRD in the rendered output for review. Note that Helm template rendering alone does not validate existing CR instances against the CRD schema.

### Docker Image Push Rules

Never run `make all`, `make docker-push`, or push to any registry unless the user explicitly asks. Confirm destination registry and tag first; then ask permission before pushing.

## Key Architecture and Development Patterns

### Docker and CI/CD

- `make docker-build` — Build images locally.
- `make docker-push` — Push images to the configured registry (see Docker Image Push Rules).

Helm charts maintain independent chart versions. Docker image tags are managed by the CI/CD build and release workflows.

Automated workflows under `.github/workflows/`:
- **build.yaml** — Builds Docker images; triggered on releases, pushes to `main`, pull requests, and manual dispatch. Supports `amd64` and `arm64`. Registry: `ghcr.io/netcracker`.
- **run_tests.yaml** — Runs integration tests after build.yaml completes; triggered on PR and push to `main`.
- **helm-charts-release.yaml** — Manually triggered; releases Helm charts with version input.
- **license.yaml** — Auto-adds license headers to Go, Shell, Python files on push to `main`; creates PR.
- **run_nightly_tests.yaml** — Scheduled nightly integration test runs.
- **clean.yaml** — Cleanup workflow for image registry.
- **renovate-approve.yaml** — Auto-approves Renovate dependency updates.
- **apm-package-update.yml** — APM package synchronization.

### Go Dependencies
- **Private Go modules:** Go services use `GOPRIVATE` for private modules hosted under `github.com/Netcracker`. Preserve the existing configuration when modifying dependencies.

### Dockerfile Conventions
- Images use multi-stage builds with an Alpine-based Go builder stage and a minimal Alpine runtime stage. Follow the existing Dockerfile pattern.

### TLS Configuration
- Services running in Kubernetes read CA certificates from `/tls/ca.crt`. Preserve this path and the existing TLS configuration when modifying certificate handling.

### Backup Orchestrator Configuration
- The backup-orchestrator uses `build/backup-daemon.conf` for runtime configuration. Preserve its expected format and configuration handling.

## Commit Message Format

```
<type>: <ticket-number> <message>

- <change 1>
- <change 2>
```

Types: `fix`, `feat`, `chore`, `chore(deps)`, `refactor`, `docs`, `build`, `config`.
Always include ticket number and ask user to confirm before committing.
