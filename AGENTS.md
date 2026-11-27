# AGENTS.md

ClickHouse Operator providing ClickHouse as a service on Kubernetes and OpenShift.
Monorepo with Helm charts, Go microservices, and integration tests.

See [CLAUDE.md](CLAUDE.md) for architecture, Docker components, and CI/CD details.

## Repository Layout

Monorepo structure. `go.mod` files live under each `services/<name>/` directory.

- `/services/site-manager` — Backup scheduler + HTTP server; uses gocron and K8s controller-runtime.
- `/services/backup-orchestrator` — CLI for backup/restore/delete ops; flag-based actions.
- `/services/dbaas-adapter` — DBaaS API server (Fiber); three adapter modes: `basic/`, `cluster/`, `initial/`.
- `/services/sidecar` — Minimal backup transfer server on port 7172.
- `/services/secret-monitor` — K8s informer watcher; reconciles secrets and credentials.
- `/services/hook` — Post-deployment hook that calls `handler.UnlockSecret()`.
- `/services/pvc-cleanup` — Persistent volume cleanup utility.
- `/helm/clickhouse/` — Main Helm chart (v0.1.0); includes CRDs, operator deployment, cluster configs, hooks.
- `/helm/clickhouse-services/` — Supplementary services chart.
- `/integration-tests/` — Python/Robot Framework test suite.

## Service Code Organization

Each service under `services/<name>/` has:
- `Makefile` — Shared targets: `deps`, `fmt`, `compile`, `docker-build`, `docker-push`, `clean`.
- `go.mod`, `go.sum` — Go modules (Go 1.26.1).
- `cmd/main.go` or `adapter/main.go` — Entry point.

Common dependencies: `go.uber.org/zap` (logging), `k8s.io/client-go`, `sigs.k8s.io/controller-runtime`.

**Note:** `dbaas-adapter` uses `adapter/main.go` (not `cmd/main.go`); compile builds from `./adapter`.

## Helm Chart Organization

Both charts under `helm/` follow standard Helm structure:

- `Chart.yaml` — Chart metadata (v0.1.0 for both; appVersion: 0.17.0).
- `values.yaml` — Default configuration parameters; must have sensible defaults.
- `templates/` — Kubernetes manifests rendered from values.
  - `_helpers.tpl` — Shared template functions for labels, annotations, security contexts.
  - `crds/crd.yaml` — ClickHouseInstallation CRD definition (in `helm/clickhouse/` only).
- `helm/clickhouse/` — Operator + cluster deployment; defines ClickHouseInstallation CR.
- `helm/clickhouse-services/` — Supplementary services (backup daemon, monitoring, etc.).

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

Pass: Exit 0; no gofmt changes; binary in `build/_output/bin/`.

### dbaas-adapter Tests (`services/dbaas-adapter/**`)

```sh
cd services/dbaas-adapter
make test
```

Pass: All tests pass; exit 0.

### Helm Charts (`helm/clickhouse/**`, `helm/clickhouse-services/**`)

```sh
helm lint helm/clickhouse
helm lint helm/clickhouse-services
helm template test helm/clickhouse
helm template test helm/clickhouse-services
```

Pass: Exit 0 all; no lint warnings; templates render without error.

## Compatibility Rules

All changes MUST be backward compatible.

### Helm `helm/clickhouse` Values

- Every new parameter MUST have a default in `values.yaml`; never break
  existing values files.
- Never rename or remove existing keys. Use deprecation aliases instead.
- Document any migration path for users; renaming requires explicit user
  approval and ticket in commit.

### ClickHouseInstallation CRD (`helm/clickhouse/crds/crd.yaml`)

- Do not change the shape or required fields of existing CRD spec.
- New fields must be optional. Removals or type changes are breaking changes.
- Before merging, verify: existing CR instances still deploy without error
  (`helm template` succeeds). If not, stop and ask the user: "Breaking CRD
  change detected. Continue only with explicit approval."

### Docker Image Push

Never run `make docker-push` or push to any registry unless the user
explicitly asks. Confirm destination registry and tag first; then ask
permission before pushing.

## Commit Message Format

```
<type>: [<ticket-number>] <message>

- <change 1>
- <change 2>
```

Types: `fix`, `feat`, `chore`, `chore(deps)`, `refactor`, `docs`, `build`, `config`.
Always ask user for ticket number before committing.
