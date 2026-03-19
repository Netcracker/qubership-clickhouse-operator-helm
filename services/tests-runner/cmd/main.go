// Copyright 2024-2025 NetCracker Technology Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/Netcracker/qubership-clickhouse-operator-helm/tests-runner/pkg/controller"
	"github.com/Netcracker/qubership-clickhouse-operator-helm/tests-runner/pkg/pod"
	"github.com/Netcracker/qubership-clickhouse-operator-helm/tests-runner/pkg/util"
)

var scheme = runtime.NewScheme()

func init() {
	// Register core Kubernetes types (Pod, etc.) — CHI is accessed as unstructured.
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	log := util.GetLogger()
	ctrl.SetLogger(zapr.NewLogger(log))
	namespace := util.GetNamespace()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: ":8081",
		// Restrict the cache to the operator namespace to limit memory usage.
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{namespace: {}},
		},
	})
	if err != nil {
		log.Error("Unable to start manager", zap.Error(err))
		os.Exit(1)
	}

	cfg := &pod.Config{
		Namespace:                    namespace,
		TestImage:                    util.GetEnv("TEST_IMAGE", ""),
		Tags:                         util.GetEnv("TAGS", "smoke"),
		ClickhouseHost:               util.GetEnv("CLICKHOUSE_HOST", "clickhouse-cluster"),
		ClickhousePort:               util.GetEnv("CLICKHOUSE_PORT", "8123"),
		ClickhouseBackupHost:         util.GetEnv("CLICKHOUSE_BACKUP_HOST", "clickhouse-backup-orchestrator"),
		ClickhouseBackupPort:         util.GetEnv("CLICKHOUSE_BACKUP_PORT", "8080"),
		BackupOrchestratorDeployment: util.GetEnv("BACKUP_ORCHESTRATOR_DEPLOYMENT", "clickhouse-backup-orchestrator"),
		ServiceAccountName:           util.GetEnv("SERVICE_ACCOUNT_NAME", "clickhouse-integration-tests"),
		TLSEnabled:                   util.GetEnv("TLS_ENABLED", "false"),
	}
	chiName := util.GetEnv("CHI_NAME", "cluster")
	cfg.ChiName = chiName

	if err = controller.New(mgr.GetClient(), log, cfg, chiName).SetupWithManager(mgr); err != nil {
		log.Error("Unable to create CHI controller", zap.Error(err))
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error("Unable to set up health check", zap.Error(err))
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error("Unable to set up ready check", zap.Error(err))
		os.Exit(1)
	}

	log.Info("Starting tests-runner controller", zap.String("namespace", namespace), zap.String("chiName", chiName))
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error("Problem running manager", zap.Error(err))
		os.Exit(1)
	}
}
