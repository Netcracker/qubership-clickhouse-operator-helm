# qubership-clickhouse-operator-helm

ClickHouse Operator provides ClickHouse as a service on Kubernetes and OpenShift. It is built on top of the [Altinity ClickHouse Operator](https://github.com/Altinity/clickhouse-operator) and packages the full lifecycle of a production ClickHouse cluster — including credential management, TLS, backup, disaster recovery, monitoring, and DBaaS integration — as a pair of Helm charts plus several purpose-built Go microservices.

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Repository Structure](#repository-structure)
- [Helm Charts](#helm-charts)
  - [clickhouse](#clickhouse-chart)
  - [clickhouse-services](#clickhouse-services-chart)
- [Go Microservices](#go-microservices)
  - [hook (Credential Saver)](#hook-credential-saver)
  - [secret-monitor (Secret Watcher)](#secret-monitor-secret-watcher)
  - [site-manager (Disaster Recovery)](#site-manager-disaster-recovery)
  - [tests-runner (Integration Test Controller)](#tests-runner-integration-test-controller)
- [Container Images](#container-images)
- [Feature Reference](#feature-reference)
  - [ClickHouse Operator](#clickhouse-operator)
  - [ClickHouse Cluster Configuration](#clickhouse-cluster-configuration)
  - [TLS / Security](#tls--security)
  - [Backup and Restore](#backup-and-restore)
  - [Disaster Recovery](#disaster-recovery)
  - [DBaaS Integration](#dbaas-integration)
  - [Monitoring and Alerting](#monitoring-and-alerting)
  - [Integration Tests](#integration-tests)
- [Values Reference](#values-reference)
  - [clickhouse chart values](#clickhouse-chart-values)
  - [clickhouse-services chart values](#clickhouse-services-chart-values)
- [Deployment Hooks](#deployment-hooks)
- [CI/CD](#cicd)

---

## Overview

This repository ships everything needed to run a hardened ClickHouse cluster on Kubernetes:

| Layer | What is delivered |
|---|---|
| **Helm charts** | `clickhouse` (operator + cluster) and `clickhouse-services` (supplementary services) |
| **Custom ClickHouse image** | Extends the official 24.3 image with a `/tmp/ping.sh` health-check script |
| **Hook** | Helm pre-/post-upgrade job that saves and unlocks credentials |
| **Secret monitor** | Always-running watcher that propagates Kubernetes Secret changes into ClickHouse users in real time |
| **Site manager** | HTTP server that orchestrates active ↔ standby DR transitions |
| **Integration tests** | Robot Framework test suite covering smoke, backup, DBaaS, and HA scenarios |

---

## Architecture

```
Kubernetes Namespace
│
├── Altinity ClickHouse Operator (Deployment)
│   └── metrics-exporter sidecar
│
├── ClickHouseInstallation CR  ──► operator reconciles ──► StatefulSets
│   ├── Shard 1 / Replica 1  (Pod: clickhouse-pod + clickhouse-backup sidecar)
│   └── Shard N / Replica M  (Pod: clickhouse-pod + clickhouse-backup sidecar)
│
├── ZooKeeper (external, required for replication and DDL)
│
├── Secret Monitor (Deployment)           ← watches Secrets, patches CHI CR
├── Hook Job (pre/post Helm hook)         ← locks/unlocks secrets around upgrades
│
├── Backup Orchestrator (Deployment)      ← manages full + incremental backups
│
├── [optional] DBaaS Adapter (Deployment) ← registers cluster with DBaaS Aggregator
│
├── [optional] Site Manager (Deployment)  ← DR HTTP API + backup scheduler
│   └── [optional] Replicator (Deployment) ← syncs backups active → standby via S3
│
└── [optional] Tests Runner (Deployment)  ← watches CHI, creates integration test pod on upgrade
    └── clickhouse-integration-tests (Pod) ← Robot Framework tests, created per CHI taskID
```

---

## Repository Structure

```
.
├── 243/                          Custom ClickHouse 24.3 Docker image
│   ├── Dockerfile
│   └── ping.sh                   Health check script injected into CH image
│
├── helm/
│   ├── clickhouse/               Main Helm chart (operator + cluster)
│   │   ├── Chart.yaml
│   │   ├── values.yaml
│   │   ├── values.schema.json
│   │   ├── crds/crd.yaml         ClickHouseInstallation CRD
│   │   ├── monitoring/           Grafana dashboard JSON sources
│   │   └── templates/
│   │       ├── clickhouse-operator/   Operator Deployment, RBAC, config maps, metrics
│   │       ├── clickhouse-cluster/    ClickHouseInstallation CR (single + DR), Ingress, Route
│   │       ├── clickhouse-backup-daemon/  Backup config map + ServiceMonitor
│   │       ├── hook/              Pre/post Helm hook jobs + RBAC
│   │       ├── secret-watchers/   Secret monitor Deployment
│   │       ├── secrets/           ClickHouse credential Secret
│   │       ├── tls/               TLS certificate resources
│   │       └── clickhouse-tests/  Integration test runner manifests
│   │
│   └── clickhouse-services/      Supplementary services chart
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
│           ├── clickhouse-backup-daemon/   Backup orchestrator CronJob/Deployment
│           ├── clickhouse-tests/           Test runner
│           ├── dbaas/                      DBaaS adapter Deployment + Secrets
│           ├── disaster-recovery/          Replicator + Site Manager + external Service
│           └── tls/                        TLS certificates (services side)
│
├── hook/                         Go source: credential saver/unlocker hook
│   ├── cmd/main.go
│   ├── pkg/handler/secretHandler.go
│   ├── build/Dockerfile
│   └── Makefile
│
├── secret-monitor/               Go source: continuous secret watcher
│   ├── cmd/main.go
│   ├── pkg/credmanager/secretWatcher.go
│   ├── pkg/client/CRClient.go
│   ├── build/Dockerfile
│   └── Makefile
│
├── site-manager/                 Go source: DR site manager HTTP server
│   ├── cmd/main.go
│   ├── pkg/server/server.go      HTTP handlers + DR orchestration logic
│   ├── pkg/server/metrics.go     Prometheus metrics endpoint
│   ├── pkg/scheduler/scheduler.go  Cron-based backup download scheduler
│   ├── pkg/client/backup.go      Backup orchestrator HTTP client
│   ├── pkg/util/kub-wrapper.go   Kubernetes API helpers
│   ├── build/Dockerfile
│   └── Makefile
│
├── integration-tests/            Robot Framework test suite image
│   ├── robot/tests/
│   │   ├── smoke/                Pod status + basic CRUD
│   │   ├── backup/               Backup and restore flows
│   │   ├── dbaas/                DBaaS adapter scenarios
│   │   ├── ha/                   High availability scenarios
│   │   └── image_tests/          Image-level checks
│   └── Dockerfile
│
├── services/
│   └── tests-runner/             Go controller that manages integration test pod lifecycle
│       ├── cmd/main.go           Entrypoint: sets up controller-runtime manager
│       ├── pkg/controller/       CHI watcher — triggers tests on taskID change
│       ├── pkg/pod/              Test pod spec builder
│       └── build/Dockerfile
│
├── docker-transfer/              Docker image used to ship/transfer images
│   └── Dockerfile
│
└── .github/workflows/
    ├── build.yaml                Multiplatform (amd64 + arm64) Docker image builds
    ├── clean.yaml                Cleanup old package versions
    └── license.yaml              License header check
```

---

## Helm Charts

### `clickhouse` chart

**Location:** `helm/clickhouse/`
**Chart version:** 0.1.0 — **App version:** 0.17.0

This chart installs the full ClickHouse stack into a single namespace:

| Template group | Resources created |
|---|---|
| `clickhouse-operator/` | Deployment (operator + metrics-exporter containers), ServiceAccount, ClusterRole/Binding, ConfigMaps for operator config, ServiceMonitor, Prometheus alert rules, Grafana dashboards |
| `clickhouse-cluster/` | `ClickHouseInstallation` CR, HTTP Ingress (Kubernetes), HTTP Route (OpenShift) |
| `clickhouse-backup-daemon/` | ConfigMap with backup config, ServiceMonitor for backup daemon |
| `hook/` | Pre-install/pre-upgrade Job (`credentials-saver`), Post-install/post-upgrade Job, associated RBAC |
| `secret-watchers/` | Deployment for the secret monitor service |
| `secrets/` | Kubernetes Secret holding ClickHouse admin credentials |
| `tls/` | Certificate, Issuer, and Secret for TLS (cert-manager or manual) |
| `clickhouse-tests/` | Deployment + Service for integration test runner |

**Required dependencies:**
- A ZooKeeper cluster reachable at the address specified by `clickhouseCluster.zookeeperHost`
- `altinity/clickhouse-operator` CRD (`ClickHouseInstallation`) — included in `crds/crd.yaml`

---

### `clickhouse-services` chart

**Location:** `helm/clickhouse-services/`
**Chart version:** 0.1.0 — **App version:** 0.17.0

This chart manages optional supplementary services that run alongside the core ClickHouse cluster. It is typically installed **after** the `clickhouse` chart.

| Template group | Resources created |
|---|---|
| `clickhouse-backup-daemon/` | Backup orchestrator Deployment (PVC, environment, eviction policy) |
| `clickhouse-tests/` | Integration test runner |
| `dbaas/` | DBaaS adapter Deployment, Service, Secrets, physical database labels ConfigMap, aggregator registration Secret |
| `disaster-recovery/` | Site Manager Deployment + Service, Replicator Deployment + Service + S3 Secret, external Service (DNS switchover), ConfigMap, Site Manager CR |
| `tls/` | TLS certificates for service-side communication |

---

## Go Microservices

### hook (Credential Saver)

**Source:** `hook/`
**Image:** `ghcr.io/netcracker/qubership-clickhouse-hook`

The hook runs as a Kubernetes **Job** triggered by two Helm lifecycle events:

1. **Pre-install / pre-upgrade** (`credentials-saver` job): reads the current ClickHouse credentials from named Kubernetes Secrets, marks each secret with annotation `locked-for-watcher: true` so the secret monitor knows not to act on changes during the upgrade, then saves them for later restoration.

2. **Post-install / post-upgrade** (post-deployment job): calls `handler.UnlockSecret()` which removes the `locked-for-watcher: true` annotation (sets it to `false`), signalling the secret monitor that the upgrade is complete.

**Key code path:**

```
hook/cmd/main.go
  └── handler.UnlockSecret(utils.GetSecretNames())
        └── hook/pkg/handler/secretHandler.go
              ├── getSecret()          ← Kubernetes API Get
              └── secret.Annotations["locked-for-watcher"] = "false"
                  manager.GetK8SClient().Update()
```

---

### secret-monitor (Secret Watcher)

**Source:** `secret-monitor/`
**Image:** `ghcr.io/netcracker/qubership-clickhouse-secret-monitor`

A long-running pod that continuously watches for changes to ClickHouse credential Secrets and automatically propagates them into the running ClickHouse cluster **without a full restart**.

**Startup sequence (`cmd/main.go`):**

1. Clears stale hook Jobs (`credentials-saver`, `post-deployment-job`) left over from previous deployments.
2. Calls `credmanager.Reconcile()` immediately to sync any pending credential changes.
3. Starts `informer.Watch()` — a Kubernetes informer loop — that calls `credmanager.Reconcile()` on every Secret change event.
4. Blocks forever with `select{}`.

**Reconciliation loop (`credmanager/secretWatcher.go`):**

1. Acquires a mutex to serialize concurrent reconciliations.
2. Waits 20 seconds (debounce period) to let multiple rapid changes settle.
3. For each watched Secret, calls `manager.ActualizeCreds()` to compare old vs new credentials; collects `ChangedUsers` entries.
4. Creates a `ClickHouseClient` on first run (or reuses the existing one).
5. Calls `updateClickhouseCreds()`:
   - Marks all changed secrets `locked-for-watcher: true`.
   - Calls `clickhouseClient.UpdateClickhouseUser()` which:
     - Fetches the `ClickHouseInstallation` CR named `cluster`.
     - For each user, computes `<username>/password_sha256_hex` and sets it in `CHI.Spec.Configuration.Users`.
     - PATCHes the CR via the Altinity operator client — the operator then reconciles the ClickHouse config.
   - Sets `locked-for-watcher: false` on success.

**Password hashing:** SHA-256 hex-encoded (ClickHouse `password_sha256_hex` format).

---

### tests-runner (Integration Test Controller)

**Source:** `services/tests-runner/`
**Image:** `ghcr.io/netcracker/qubership-clickhouse-tests-runner`

A lightweight Go controller (Deployment) that watches `ClickHouseInstallation` CRs and manages the lifecycle of the integration test pod. It replaces the old `clickhouse_pods_checker.py` pre-test script approach.

**State machine:**

| Condition | Action |
|---|---|
| CHI status is not `Completed` | Requeue and wait |
| Backup orchestrator not yet `Available` | Requeue and wait |
| No test pod exists | Create test pod, annotate with current CHI `taskID` |
| Test pod exists, `taskID` annotation matches CHI | Do nothing — tests already ran for this version |
| Test pod exists, `taskID` annotation differs | Delete stale pod, create new one for the new version |

**Key design decisions:**

- **`taskID`-based trigger:** The Altinity operator writes a new `status.taskID` on every reconciliation (install or upgrade). The controller stamps this value on the test pod as annotation `clickhouse.qubership.org/chi-task-id`. Tests run exactly once per CHI reconciliation cycle and are not restarted after `Succeeded`.
- **`run-robot-without-ttyd` entrypoint:** The test pod uses this argument so it exits naturally with the Robot Framework exit code instead of starting a `ttyd` terminal server (which would keep the pod `Running` forever).
- **Output volume:** An `emptyDir` is mounted at `/opt/robot/output` to give the test container write access to the output directory (the image directory is root-owned).

**Key code path:**

```
services/tests-runner/cmd/main.go
  └── ctrl.SetLogger(zapr.NewLogger(log))   ← wires controller-runtime to zap logger
  └── controller.New(...).SetupWithManager(mgr)
        └── pkg/controller/chi_controller.go
              ├── Reconcile()               ← fetches CHI, checks status + taskID
              └── reconcileTestPods()       ← compares pod annotation vs CHI taskID
                    └── pkg/pod/tests_pod.go
                          └── New(cfg, taskID) ← builds Pod spec with annotation + emptyDir
```

---

### site-manager (Disaster Recovery)

**Source:** `site-manager/`
**Image:** `ghcr.io/netcracker/qubership-clickhouse-site-manager`

An HTTP server that orchestrates **active ↔ standby** mode transitions for disaster recovery. It is deployed only when `disasterRecovery` is configured in the `clickhouse-services` chart.

#### HTTP API

| Endpoint | Methods | Description |
|---|---|---|
| `/sitemanager` | GET | Returns current mode and status from the `site-manager` ConfigMap |
| `/sitemanager` | POST `{"mode":"active"\|"standby"}` | Triggers an async mode transition |
| `/pre-configure` | GET | Returns pre-configure status |
| `/pre-configure` | POST | Triggers pre-configure (e.g. incremental backup before standby) |
| `/health` | GET | Returns ClickHouse cluster health: `up`, `degraded`, or `down` |
| `/metrics` | GET | Prometheus text metrics: restore in-progress + restore failure |

#### TLS

When `tls.enabled` is set, the server listens on `:8443` using `/tls/tls.crt` and `/tls/tls.key`. In plain mode it listens on `:8080`. The CA certificate is loaded at startup from `/tls/ca.crt` and set as the default root CA for all outgoing HTTP requests (backup orchestrator calls).

#### Authentication middleware

If HTTP auth is enabled, every request must carry a Kubernetes `ServiceAccount` bearer token. The middleware performs a `TokenReview` API call and validates the username against `NC_SM_CUSTOM_AUDIENCE` and the expected service account name.

#### Mode transitions

**Switching to `standby`:**
1. Updates the external Service `clickhouse-cluster-external` to point to the opposite site's ClickHouse host.
2. Patches the replicator Deployment environment (`INC_BACKUP_SCHEDULE=None`, `BACKUP_SCHEDULE=None`, `MODE=standby`) to stop backup scheduling.
3. (Pre-configure phase) Requests an **incremental backup** of current data so the standby site can start fresh.

**Switching to `active`:**
1. Downloads all missing backups from the backup orchestrator into local ClickHouse pods via the backup daemon HTTP API (port 7171).
2. Restores the latest incremental backup (falls back to latest full backup if no incremental is found), polling `/incremental/jobstatus/<id>` every 5 seconds for up to 15 minutes.
3. Updates the external Service to point to the local `clickhouse-cluster` service.
4. Asynchronously: waits for any in-progress backup daemon actions, re-enables replicator backup schedules, and triggers a **full backup** (with up to 5 retries).

#### Backup scheduler

A `gocron` cron job runs on the schedule defined by the `NC_SM_DOWNLOAD_CRON` environment variable. On each tick it:
1. Lists local backups for each ClickHouse host via `GET http://<host>:7171/backup/list/local`.
2. Lists remote backups (full + incremental) from the backup orchestrator.
3. Downloads any missing backups to each host.
4. Deletes local backups older than the last successful full backup.

#### Prometheus metrics

| Metric | Description |
|---|---|
| `clickhouse_dr_restore_in_progress{}` | `1` while a restore is actively running |
| `clickhouse_dr_restore_failed{failureMessage, failureTime}` | `1` if the last restore failed |

---

## Container Images

All images are built for `linux/amd64` and `linux/arm64` and published to `ghcr.io/netcracker/`.

| Image name | Source | Purpose |
|---|---|---|
| `qubership-clickhouse-243` | `243/Dockerfile` | Custom ClickHouse 24.3 with `ping.sh` health check |
| `qubership-clickhouse-hook` | `hook/build/Dockerfile` | Pre/post Helm hook (credential saver) |
| `qubership-clickhouse-secret-monitor` | `secret-monitor/build/Dockerfile` | Continuous secret watcher |
| `qubership-clickhouse-site-manager` | `site-manager/build/Dockerfile` | DR site manager HTTP server |
| `qubership-clickhouse-integration-tests` | `integration-tests/Dockerfile` | Robot Framework test runner (test pod) |
| `qubership-clickhouse-tests-runner` | `services/tests-runner/build/Dockerfile` | Integration test controller (watches CHI, creates test pods) |
| `qubership-clickhouse-transfer` | `docker-transfer/Dockerfile` | Image transfer utility |

External images used (configured in `values.yaml`):

| Image | Default tag | Role |
|---|---|---|
| `altinity/clickhouse-operator` | `0.25.6` | ClickHouse operator |
| `altinity/metrics-exporter` | `0.25.6` | Prometheus metrics exporter sidecar |
| `altinity/clickhouse-backup` | `2.6.39` | Per-pod backup daemon sidecar |
| `ghcr.io/netcracker/qubership-clickhouse-backup-orchestrator` | `main` | Central backup orchestrator |
| `ghcr.io/netcracker/qubership-clickhouse-dbaas-adapter` | `main` | DBaaS adapter |
| `ghcr.io/netcracker/qubership-credential-manager` | `main` | Credential lifecycle library (hook) |
| `ghcr.io/netcracker/qubership-clickhouse-backup-sidecar` | `main` | NFS backup sidecar (optional) |

---

## Feature Reference

### ClickHouse Operator

The `clickhouse` chart deploys the Altinity ClickHouse Operator as a Deployment with two containers:

- **`clickhouse-operator`** — the main operator container. It watches `ClickHouseInstallation` CRs and reconciles StatefulSets, Services, and ConfigMaps accordingly.
- **`metrics-exporter`** — a sidecar that exposes ClickHouse cluster metrics at port `8888` for Prometheus scraping.

Operator configuration is injected via five ConfigMaps mounted into the pod:

| Config path | ConfigMap | Purpose |
|---|---|---|
| `/etc/clickhouse-operator/` | `etc-clickhouse-operator-files` | Operator main config |
| `/etc/clickhouse-operator/config.d/` | `etc-clickhouse-operator-confd-files` | Common ClickHouse config fragments |
| `/etc/clickhouse-operator/conf.d/` | `etc-clickhouse-operator-configd-files` | Per-host ClickHouse config fragments |
| `/etc/clickhouse-operator/templates.d/` | `etc-clickhouse-operator-templatesd-files` | CHI templates |
| `/etc/clickhouse-operator/users.d/` | `etc-clickhouse-operator-usersd-files` | User config fragments |

Key operator settings (via `clickhouseOperator.*` values):

| Value | Default | Description |
|---|---|---|
| `statefulSetUpdateTimeout` | `300` | Seconds to wait for a StatefulSet to become Ready |
| `statefulSetUpdatePollPeriod` | `10` | Seconds between readiness checks |
| `onStatefulSetCreateFailureAction` | `ignore` | Action on create failure: `abort`, `delete`, or `ignore` |
| `onStatefulSetUpdateFailureAction` | `rollback` | Action on update failure: `abort`, `rollback`, or `ignore` |
| `reconcileThreadsNumber` | `10` | Concurrent reconciliation goroutines |

---

### ClickHouse Cluster Configuration

The `ClickHouseInstallation` CR is rendered from `templates/clickhouse-cluster/clickhouse_cluster.yaml` (single mode) or `clickhouse_cluster_dr.yaml` (DR mode).

**Cluster topology** is controlled by:

| Value | Default | Description |
|---|---|---|
| `clickhouseCluster.scheme` | `single` | Deployment mode: `single` or `dr` |
| `clickhouseCluster.shardsCount` | `1` | Number of shards |
| `clickhouseCluster.replicasCount` | `2` | Number of replicas per shard |
| `clickhouseCluster.zookeeperHost` | `zookeeper.zookeeper.svc` | ZooKeeper address for replication |

**Storage:**

| Value | Default | Description |
|---|---|---|
| `clickhouseCluster.pvSize` | `5Gi` | PVC size per ClickHouse pod |
| `clickhouseCluster.storageClassName` | _(unset)_ | StorageClass; omit to use cluster default |
| `clickhouseCluster.localStorage` | _(unset)_ | Enable local storage PVC label selectors |

**Pod distribution:** By default, `ClickHouseAntiAffinity` is applied at `ClickHouseInstallation` scope. Custom distribution rules can be set via `clickhouseCluster.podDistribution`:

```yaml
clickhouseCluster:
  podDistribution:
    - type: ShardAntiAffinity
      scope: ClickHouseInstallation
```

**Health checks:**

When `clickhouseCluster.strictHealthCheck: true` (default), both liveness and readiness probes use the `/tmp/ping.sh` script from the custom image. This script also checks ZooKeeper connectivity, providing a stricter health signal than an HTTP ping. Setting it to `false` falls back to a plain HTTP `GET /ping`.

**Built-in Prometheus settings** (always enabled):

```xml
prometheus/asynchronous_metrics: true
prometheus/endpoint: /metrics
prometheus/events: true
prometheus/metrics: true
prometheus/port: 8888
```

**Multi-shard user directory:** When `shardsCount > 1`, a replicated user directory backed by ZooKeeper is automatically configured:

```xml
<user_directories replace="replace">
  <users_xml><path>users.xml</path></users_xml>
  <local_directory><path>/var/lib/clickhouse/access/</path></local_directory>
  <replicated>
    <zookeeper_path>/clickhouse/<namespace>/access</zookeeper_path>
  </replicated>
</user_directories>
```

**PodDisruptionBudget:** Controlled by `clickhouseCluster.pdb.pdbManaged` (default `true`) and `clickhouseCluster.pdb.maxUnavailable`.

---

### TLS / Security

TLS can be enabled for:

1. **ClickHouse HTTP/TCP ports** — sets `https_port: 8443`, `tcp_port_secure: 9440`, and configures `openSSL/server/*` settings.
2. **ZooKeeper connection** — enables `openSSL/client/*` settings for encrypted ZooKeeper communication.
3. **Site Manager / Backup Orchestrator** — served on `:8443` with the same certificate.

**Certificate provisioning:**

Two modes are supported:

- **Manual:** Provide base64-encoded `ca_crt`, `tls_key`, and `tls_crt` under `tls.certificates`.
- **cert-manager:** Set `tls.generateCerts.enabled: true` and provide a `clusterIssuerName`. A `Certificate` and `Issuer` resource are created automatically.

| Value | Default | Description |
|---|---|---|
| `tls.enabled` | `false` | Enable TLS for ClickHouse |
| `tls.zookeeper` | `false` | Enable TLS for ZooKeeper connection |
| `tls.certificateSecretName` | `ch-cert` | Name of the Kubernetes Secret with TLS material |
| `tls.generateCerts.enabled` | `false` | Use cert-manager to auto-generate certs |
| `tls.generateCerts.duration` | `365` | Certificate validity in days |

**Credential security:**

- ClickHouse users are stored with `password_sha256_hex` (never plaintext).
- Operator credentials are kept in a dedicated Secret (`clickhouse-operator-credentials`).
- The `locked-for-watcher` annotation protocol prevents credential race conditions during rolling upgrades.

---

### Backup and Restore

The backup system uses two components:

**Backup daemon sidecar** (`altinity/clickhouse-backup`) runs as a container in every ClickHouse pod and exposes a local HTTP API on port `7171`. Enable it with `backupDaemon.install: true`.

**Backup orchestrator** (`qubership-clickhouse-backup-orchestrator`) is a central deployment that coordinates backup operations across all shards/replicas. Enabled via `backupDaemon.install: yes` in the `clickhouse-services` chart.

**Storage backends:**

| Mode | Configuration |
|---|---|
| `none` | Local disk only (no remote storage) |
| `nfs` | NFS PVC sidecar (`backupDaemon.storage.nfs.*`) |
| `s3` | S3-compatible object storage (`backupDaemon.storage.s3.*`) |

**Backup schedule (orchestrator):**

| Env variable | Default | Description |
|---|---|---|
| `BACKUP_SCHEDULE` | `0 * * * *` | Full backup cron expression |
| `INC_BACKUP_SCHEDULE` | `None` | Incremental backup cron expression |
| `EVICTION_POLICY` | `7d/7d,1y/delete` | Retention policy |
| `ENABLE_INCREMENTAL` | `true` | Enable incremental backups |

**Backup daemon API** (port `7171` per pod):

- `GET /backup/list/local` — list locally stored backups
- `POST /backup/download/<id>` — download a backup from remote storage
- `POST /backup/delete/local/<id>` — delete a local backup

**Tracing:** OpenTelemetry tracing can be enabled via `tracing.enabled: true` with a configurable OTLP endpoint (defaults to `jaeger-collector.tracing.svc:4317`).

**pprof profiling:** Enable Go pprof for the backup daemon with `backupDaemon.enablePprof: true`.

---

### Disaster Recovery

DR requires the `clickhouse-services` chart with `disasterRecovery` configuration. The setup involves two Kubernetes namespaces (or clusters), one designated **active** and one **standby**.

**Components deployed when DR is configured:**

| Component | Description |
|---|---|
| `clickhouse-replicator` | Runs the backup orchestrator image in replicator mode; pushes backups to shared S3 storage |
| `site-manager` | Orchestrates mode transitions via HTTP API |
| `clickhouse-cluster-external` | Kubernetes `ExternalName` Service pointing to the active site |
| `opposite-ch-host` ConfigMap | Stores the DNS name of the opposite site's ClickHouse cluster |

**Replicator environment:**

| Variable | Standby value | Active value |
|---|---|---|
| `MODE` | `standby` | `active` |
| `BACKUP_SCHEDULE` | `None` | Configured full backup schedule |
| `INC_BACKUP_SCHEDULE` | `None` | Configured incremental schedule |
| `S3_ENABLED` | `True` | `True` |

The site manager dynamically patches these environment variables on the replicator Deployment when a mode switch is requested.

**DR failover workflow (`POST /sitemanager {"mode":"active"}`):**

```
1. Download all missing backups from orchestrator to local CH pods
2. Restore latest incremental backup (or full if no incremental exists)
   └── Poll restore status every 5s, timeout 15min
3. Switch clickhouse-cluster-external → local clickhouse-cluster service
4. (async) Wait for in-progress backup actions to finish
5. (async) Re-enable backup schedules on replicator
6. (async) Trigger full backup (up to 5 retries)
```

**DR pre-configure (`POST /pre-configure {"mode":"standby"}`):**

```
1. Wait for all in-progress backup actions
2. Patch replicator to disable backup schedules
3. Request incremental backup (sync) so data is captured before cutover
```

**Site manager configuration:**

```yaml
disasterRecovery:
  mode: "active"                                # or "standby"
  oppositeClickhouseHost: "clickhouse-cluster.other-namespace"
  replicator:
    backupSchedule: "*/30 * * * *"             # incremental
    backupFullSchedule: "0 1 * * *"            # full
    s3:
      access_key: "..."
      secret_key: "..."
      bucket: "clickhouse"
      endpoint: "https://minio-service"
  siteManager:
    image: ghcr.io/netcracker/qubership-clickhouse-site-manager:main
    downloadSchedule: "*/10 * * * *"
    httpAuth:
      enabled: true
      smNamespace: "site-manager"
      smServiceAccountName: "sm-auth-sa"
```

---

### DBaaS Integration

The DBaaS adapter (`nc-dbaas-clickhouse-adapter`) integrates the ClickHouse cluster with a DBaaS Aggregator, enabling database-as-a-service lifecycle management (create/delete databases, manage users).

Enable with `dbaas.install: true` in either chart.

**Registration:** On startup, the adapter registers with the DBaaS Aggregator at `dbaas.aggregator.registrationAddress` using the credentials from `nc-dbaas-aggregator-registration-credentials` Secret.

**Multi-shard mode:** When `clickhouseCluster.shardsCount > 1`, `REPLICATED_USER_STORAGE=true` is set, and the adapter uses ZooKeeper-backed replicated user directories.

**Init container:** When `dbaas.migrateRoles: true` or multi-shard mode is active, an init container runs with `args: [init]` to perform schema migrations and role grants before the main adapter starts.

**Key environment variables:**

| Variable | Source |
|---|---|
| `CLICKHOUSE_HOST` | `dbaas.chHost` or `clickhouse-cluster.<namespace>` |
| `CLICKHOUSE_PORT` | `dbaas.chPort` (default `9000`) |
| `CLICKHOUSE_USERNAME` | `nc-dbaas-user-credentials` Secret |
| `DBAAS_AGGREGATOR_REGISTRATION_ADDRESS` | `dbaas.aggregator.registrationAddress` |
| `DBAAS_AGGREGATOR_PHYSICAL_DATABASE_IDENTIFIER` | `<namespace>:clickhouse` |

---

### Monitoring and Alerting

**Grafana dashboards** (deployed as ConfigMap resources):

- `clickhouse_grafana_dashboard.json` — General cluster dashboard
- `clickhouse_performance_grafana_dashboard.json` — Performance metrics dashboard

**ServiceMonitors** (Prometheus Operator):

- Operator metrics (`clickhouseOperator.metricsExporterPort: 8888`)
- Cluster metrics (per-shard/replica, port `8888`)
- Backup daemon metrics
- Site manager / replicator metrics

**Prometheus alert rules** (configurable thresholds in `clickhouseCluster.prometheusRules`):

| Alert | Threshold value key | Default |
|---|---|---|
| ClickHouse uptime too low | `clickhouseUptime` | 180s |
| Too many distributed files to insert | `distributedFilesToInsertThreshold` | 50 |
| Max part count for partition too high | `maxPartCountForPartitionThreshold` | 100 |
| Low inserted rows per query | `lowInsertedRowsPerQueryThreshold` | 1000 |
| Long running queries | `longestRunningQueryTimeThreshold` | 600s |
| Replica max absolute delay too high | `replicasMaxAbsoluteDelayThreshold` | 300s |
| Too many connections | `clickHouseConnectionsThreshold` | 100 |
| Too many running queries | `clickHouseRunningQueriesThreshold` | 80 |
| Too many mutations | `clickHouseMutationsThreshold` | 100 |

---

### Integration Tests

Integration tests use a two-component architecture deployed by the `clickhouse-services` chart:

1. **Tests Runner** (`integrationTests.testsRunnerImage`) — an always-running Deployment that watches the `ClickHouseInstallation` CR. When it detects a new reconciliation cycle (identified by a changed `status.taskID`), it creates a test pod for that version and leaves it alone once done.

2. **Test Pod** (`integrationTests.image`) — a one-shot Pod running the Robot Framework suite. Created by the tests-runner controller; exits with the Robot Framework exit code so it reaches `Succeeded` or `Failed` status cleanly.

Enable with `integrationTests.install: true` in the `clickhouse-services` chart.

**Test suites:**

| Tag/Suite | Description |
|---|---|
| `smoke` | Verifies all pods are `Running`, performs basic CRUD (create database/table, insert/update/delete) |
| `backup` | Tests full and incremental backup and restore flows |
| `dbaas` | Tests DBaaS adapter database and user lifecycle |
| `ha` | High-availability and replica failover scenarios |
| `image_tests` | Validates container image properties |

Configure which suites to run with `integrationTests.tags` (default: `backupORdbaas` for `clickhouse-services` chart).

**Test trigger logic:** Tests run exactly once per ClickHouse reconciliation cycle. The controller compares the CHI `status.taskID` with the annotation `clickhouse.qubership.org/chi-task-id` on the existing test pod. A new pod is created only when the `taskID` changes (i.e., after an upgrade).

---

## Values Reference

### `clickhouse` chart values

#### Global

| Key | Type | Default | Description |
|---|---|---|---|
| `affinity` | object | `{}` | Pod affinity rules applied to all pods |
| `tolerations` | list | `[]` | Pod tolerations applied to all pods |
| `podLabels` | object | `{}` | Labels applied to all pods |
| `global.cloudIntegrationEnabled` | bool | `true` | Enables cloud-specific integration |

#### `clickhouseOperator`

| Key | Default | Description |
|---|---|---|
| `name` | `clickhouse` | Deployment name for the operator |
| `image` | `altinity/clickhouse-operator:0.25.6` | Operator image |
| `metricsExporterImage` | `altinity/metrics-exporter:0.25.6` | Metrics exporter image |
| `metricsExporterPort` | `8888` | Port exposed by metrics exporter |
| `serviceAccountName` | `clickhouse-operator` | ServiceAccount name |
| `statefulSetUpdateTimeout` | `300` | Seconds to wait for StatefulSet readiness |
| `onStatefulSetCreateFailureAction` | `ignore` | `abort`, `delete`, or `ignore` |
| `onStatefulSetUpdateFailureAction` | `rollback` | `abort`, `rollback`, or `ignore` |
| `reconcileThreadsNumber` | `10` | Parallel reconcile threads |
| `chCommonConfigsPath` | `config.d` | Path for common ClickHouse config |
| `chHostConfigsPath` | `conf.d` | Path for per-host ClickHouse config |
| `chUsersConfigsPath` | `users.d` | Path for user config |
| `chiTemplatesPath` | `templates.d` | Path for CHI templates |
| `credentialsToInstances.chUsername` | `clickhouse_operator` | Operator's ClickHouse username |
| `credentialsToInstances.chPassword` | `clickhouse_operator_password` | Operator's ClickHouse password |
| `credentialsToInstances.chPort` | `8123` | ClickHouse HTTP port |
| `credentialsToInstances.chCredentialsSecretName` | `clickhouse-operator-credentials` | Secret name for operator credentials |
| `logParams.loglevel` | `warning` | Operator log level |
| `role.install` | `yes` | Whether to create Role/RoleBinding |
| `events` | `false` | Enable Kubernetes event publishing |

#### `clickhouseCluster`

| Key | Default | Description |
|---|---|---|
| `install` | `yes` | Whether to deploy the cluster |
| `scheme` | `single` | `single` or `dr` |
| `name` | `cluster` | Name of the ClickHouseInstallation CR |
| `image` | `ghcr.io/netcracker/qubership-clickhouse-243:main` | ClickHouse server image |
| `zookeeperHost` | `zookeeper.zookeeper.svc` | ZooKeeper address |
| `shardsCount` | `1` | Number of shards |
| `replicasCount` | `2` | Number of replicas |
| `pvSize` | `5Gi` | PVC storage size per pod |
| `storageClassName` | _(none)_ | StorageClass for PVCs |
| `strictHealthCheck` | `true` | Use `ping.sh` for health probes |
| `serviceMonitor` | `false` | Create ServiceMonitor CR |
| `pdb.pdbManaged` | `""` (→ `true`) | Enable PodDisruptionBudget |
| `pdb.maxUnavailable` | `{}` (→ `1`) | Max unavailable pods in PDB |
| `podDistribution` | `[]` | Pod distribution rules (defaults to ClickHouseAntiAffinity) |
| `configuration.profiles` | `{}` | Custom ClickHouse profiles |
| `configuration.settings` | _(none)_ | Custom ClickHouse server settings |
| `configuration.custom` | `{}` | TTL settings for system logs |
| `users.clickhouse.password` | `clickhouse` | Default admin password |
| `users.clickhouse.networks/ip` | `::/0` | Allowed networks for admin user |
| `prometheusRules.*` | various | Alert thresholds (see Monitoring section) |

#### `backupDaemon`

| Key | Default | Description |
|---|---|---|
| `install` | `no` | Deploy backup sidecar in ClickHouse pods |
| `image` | `altinity/clickhouse-backup:2.6.39` | Backup daemon image |
| `timeout` | `"10"` | Backup operation timeout (minutes) |
| `storage.remote` | `none` | Remote storage: `none`, `nfs`, or `s3` |
| `storage.s3.*` | _(see values.yaml)_ | S3 connection settings |
| `storage.nfs.*` | _(see values.yaml)_ | NFS PVC settings |
| `enablePprof` | `false` | Enable Go pprof endpoint |
| `resources.limits.cpu` | `100m` | CPU limit |
| `resources.limits.memory` | `256Mi` | Memory limit |

#### `dbaas`

| Key | Default | Description |
|---|---|---|
| `install` | `false` | Deploy DBaaS adapter |
| `clickhouse.username` | `nc_dbaas_user` | DBaaS ClickHouse username |
| `clickhouse.password` | `pAssW0RD1` | DBaaS ClickHouse password |

#### `tls`

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable TLS for ClickHouse |
| `zookeeper` | `false` | Enable TLS for ZooKeeper |
| `certificateSecretName` | `ch-cert` | TLS Secret name |
| `generateCerts.enabled` | `false` | Use cert-manager |
| `generateCerts.clusterIssuerName` | `""` | cert-manager ClusterIssuer name |
| `generateCerts.duration` | `365` | Certificate validity (days) |

#### `tracing`

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable OpenTelemetry tracing |
| `host` | `jaeger-collector.tracing.svc:4317` | OTLP collector endpoint |
| `service` | `nc-clickhouse-backup-daemon` | Service name in traces |

#### `clickhouseSecretMonitor`

| Key | Default | Description |
|---|---|---|
| `install` | `true` | Deploy the secret monitor |
| `dockerImage` | `ghcr.io/netcracker/qubership-clickhouse-secret-monitor:main` | Image |

#### `clickhouseHook`

| Key | Default | Description |
|---|---|---|
| `install` | `true` | Deploy credential saver hook |
| `dockerImage` | `ghcr.io/netcracker/qubership-credential-manager:main` | Pre-hook image |

#### `postHook`

| Key | Default | Description |
|---|---|---|
| `install` | `true` | Deploy post-upgrade unlock hook |
| `dockerImage` | `ghcr.io/netcracker/qubership-clickhouse-hook:main` | Post-hook image |

---

### `clickhouse-services` chart values

The `clickhouse-services` chart shares many top-level keys with the `clickhouse` chart. Key additions:

#### `backupDaemon.orchestrator`

| Key | Default | Description |
|---|---|---|
| `image` | `ghcr.io/netcracker/qubership-clickhouse-backup-orchestrator:main` | Orchestrator image |
| `pvSize` | `2Gi` | Orchestrator PVC size |
| `envs.BACKUP_SCHEDULE` | `0 * * * *` | Full backup cron |
| `envs.INC_BACKUP_SCHEDULE` | `None` | Incremental backup cron |
| `envs.EVICTION_POLICY` | `7d/7d,1y/delete` | Retention policy |
| `envs.ENABLE_INCREMENTAL` | `true` | Enable incremental backups |

#### `integrationTests` (clickhouse-services)

| Key | Default | Description |
|---|---|---|
| `install` | `false` | Deploy the tests-runner controller and enable test execution |
| `image` | `ghcr.io/netcracker/qubership-clickhouse-integration-tests:main` | Robot Framework test pod image |
| `testsRunnerImage` | `ghcr.io/netcracker/qubership-clickhouse-tests-runner:main` | Tests-runner controller image |
| `chiName` | `cluster` | Name of the `ClickHouseInstallation` CR to watch |
| `tags` | `backupORdbaas` | Robot Framework tag filter for test selection |
| `clickhouseHost` | `clickhouse-cluster` | ClickHouse host passed to tests |
| `clickhousePort` | `8123` | ClickHouse HTTP port |
| `clickhouseBackupHost` | `clickhouse-backup-orchestrator` | Backup orchestrator host |
| `clickhouseBackupPort` | `8080` | Backup orchestrator port |
| `timeoutBeforeStart` | `30` | Seconds the test pod waits before starting (0 = skip, controller guarantees readiness) |
| `resources` | `256Mi / 200m` | Resource requests/limits for the test pod |
| `runnerResources` | `64Mi / 50m` | Resource requests/limits for the tests-runner controller |

---

#### `disasterRecovery`

| Key | Description |
|---|---|
| `mode` | Current mode: `active` or `standby` |
| `oppositeClickhouseHost` | FQDN of the other site's ClickHouse cluster |
| `replicator.backupSchedule` | Incremental backup cron for active site |
| `replicator.backupFullSchedule` | Full backup cron for active site |
| `replicator.s3.*` | S3 credentials for inter-site backup storage |
| `siteManager.image` | Site manager image |
| `siteManager.downloadSchedule` | Cron for backup download scheduler |
| `siteManager.httpAuth.enabled` | Enable Kubernetes TokenReview auth |

#### `dbaas` (extended)

| Key | Default | Description |
|---|---|---|
| `dockerImage` | `ghcr.io/netcracker/qubership-clickhouse-dbaas-adapter:main` | Adapter image |
| `aggregator.registrationAddress` | `http://dbaas-aggregator.dbaas:8080` | Aggregator address |
| `aggregator.registrationUsername` | `cluster-dba` | Aggregator auth username |
| `labels.clusterName` | `clickhouse` | Physical database label |
| `multiUsers` | `false` | Enable multi-user support |

---

## Deployment Hooks

Two Helm hooks are part of the `clickhouse` chart and execute around every install/upgrade:

### Pre-install / Pre-upgrade: `credentials-saver`

```yaml
annotations:
  "helm.sh/hook": pre-install, pre-upgrade
  "helm.sh/hook-weight": "2"
  "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
```

Runs the `qubership-credential-manager` image. It reads the Secret names listed in the `SECRET_NAMES` env var, stores the current credential values, and sets `locked-for-watcher: true` on each Secret's annotations. This prevents the secret monitor from acting on any changes while the upgrade is in flight.

### Post-install / Post-upgrade: `post-deployment-job`

```yaml
annotations:
  "helm.sh/hook": post-install, post-upgrade
```

Runs the `qubership-clickhouse-hook` image. It calls `handler.UnlockSecret()` for each Secret, setting `locked-for-watcher: false`, which signals to the secret monitor that it is safe to resume credential reconciliation.

---

## CI/CD

### Build workflow (`.github/workflows/build.yaml`)

Triggered on:
- Every push to any branch
- Creation of a GitHub Release
- Manual dispatch (`workflow_dispatch`)

Builds all images in a matrix strategy:

```
qubership-clickhouse-243
qubership-clickhouse-258
qubership-clickhouse-integration-tests
qubership-clickhouse-tests-runner
qubership-clickhouse-hook
qubership-clickhouse-secret-monitor
qubership-clickhouse-transfer
qubership-clickhouse-site-manager
```

Each image is built for `linux/amd64` and `linux/arm64` using Docker Buildx and pushed to `ghcr.io/netcracker/<name>:<branch-or-tag>`.

Old versions of each image package are automatically deleted after a new build using the `Netcracker/get-package-ids` action.

### License check (`.github/workflows/license.yaml`)

Validates that all source files carry the required Apache 2.0 license header.

### Cleanup workflow (`.github/workflows/clean.yaml`)

Periodically removes stale container image versions from the GitHub Container Registry.
