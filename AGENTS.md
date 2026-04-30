# AGENTS.md

This file provides guidance to AI coding assistants when working with code in this repository.

## Project Overview

ClickHouse Operator providing ClickHouse as a service on Kubernetes and OpenShift. The repo contains Helm charts for deployment and six Go microservices that handle backup, monitoring, secrets, and database administration.

## Build Commands

Each service lives under `services/<name>/` with its own Makefile and Go module (Go 1.26.1). All services share the same Make targets:

```bash
cd services/<service-name>

make deps        # go mod tidy
make fmt         # gofmt -l -s -w .
make compile     # CGO_ENABLED=0 go build (output: build/_output/bin/...)
make docker-build  # build Docker image (TAG_ENV=local by default)
make docker-push   # push Docker image
make clean       # rm -rf build/_output
```

Only `dbaas-adapter` has a `make test` target:
```bash
cd services/dbaas-adapter
make test   # runs go test -v ./...
```

Unit tests exist only in `services/dbaas-adapter/adapter/basic/` (testify/assert). Integration tests are Python/Robot Framework based in `integration-tests/`.

## Architecture

### Services (`services/`)

| Service | Entry Point | Purpose |
|---|---|---|
| **site-manager** | `cmd/main.go` | Backup scheduler + HTTP server; uses gocron and K8s controller-runtime |
| **backup-orchestrator** | `cmd/main.go` | CLI for backup/restore/delete operations against ClickHouse; flag-based actions (`backup`, `restore`, `delete`, `incrementalDelete`, `dblist`, `incremental-backup`, `cancel`) |
| **dbaas-adapter** | `adapter/main.go` | DBaaS API server using Fiber framework; three adapter modes: `basic/`, `cluster/`, `initial/` |
| **sidecar** | `cmd/main.go` | Minimal backup transfer server on port 7172 |
| **secret-monitor** | `cmd/main.go` | K8s informer-based watcher that reconciles secrets and manages credentials |
| **hook** | `cmd/main.go` | One-shot post-deployment hook that calls `handler.UnlockSecret()` |

Common dependencies across services: `go.uber.org/zap` (logging), `k8s.io/client-go` (K8s API), `sigs.k8s.io/controller-runtime`.

Note: `dbaas-adapter` uses `adapter/main.go` as entry point (not `cmd/main.go`), and its compile target builds from `./adapter`.

### Helm Charts (`helm/`)

- **`helm/clickhouse/`** — Main chart (v0.1.0). Includes CRDs (`crds/crd.yaml`), operator deployment, cluster definitions, backup daemon config, hooks, secret watchers, TLS issuers, and test templates. Template helpers in `_helpers.tpl`.
- **`helm/clickhouse-services/`** — Supplementary services chart.

### Docker Components

10 Docker images defined in `.github/build-config.cfg`. Six are the Go services above; four are standalone:
- `qubership-clickhouse-258` (context: `258/`)
- `qubership-clickhouse-243` (context: `243/`)
- `qubership-clickhouse-integration-tests` (context: `integration-tests/`)
- `qubership-clickhouse-transfer` (context: `docker-transfer/`)

### CI/CD (`.github/workflows/`)

- **build.yaml** — Triggers on release, push to main, and PRs. Multi-platform builds (amd64/arm64) with changeset-based matrix strategy. Registry: `ghcr.io/netcracker`.
- **helm-charts-release.yaml** — Manual trigger for Helm chart releases.
- **license.yaml** — Apache 2.0 license header compliance checks on Go, Shell, and Python files.

## Key Patterns

- All Go services use `GOPRIVATE=https://github.com/Netcracker` for private module access.
- Docker builds are multi-stage (Alpine 3.23 + Go builder → minimal Alpine runtime).
- Services read TLS certs from `/tls/ca.crt` at runtime in Kubernetes.
- The backup-orchestrator uses a config file at `build/backup-daemon.conf` for runtime settings.

## Commit Message Pattern

All commits in this project MUST follow this format:

```
<type>: [<ticket-number>] <meaningful message> <list of details>
```

Where:
- `<type>` = fix, chore, feat, chore(deps), etc.
- `<ticket-number>` = REQUIRED - Always ask the user for the ticket number before creating any commit
- `<meaningful message>` = Clear, concise description of the change
- `<list of details>` - list of commit changes

**Example:**
```
feat: [CPCAP-1234] add backup retention policy configuration

- <change 1>
- <change 2>
- ...


fix: [CPCAP-5678] resolve memory leak in monitoring agent

- <change 1>
- <change 2>
- ...


chore(deps): [CPCAP-9012] bump kubernetes dependencies to v0.35.1

- <change 1>
- <change 2>
- ...
```

**Important:** Never create a commit without first asking the user for the ticket number.
