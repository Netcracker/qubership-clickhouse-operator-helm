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
	"context"
	"log"
	"os"
	"strings"

	"github.com/Netcracker/qubership-clickhouse-operator-helm/pvc-cleanup/pkg/cleanup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	ctx := context.Background()

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		log.Fatal("NAMESPACE environment variable is required")
	}

	chiName := envOrDefault("CHI_NAME", cleanup.DefaultCHIName)
	pvcPrefix := envOrDefault(
		"PVC_NAME_PREFIX",
		cleanup.DefaultPVCNamePrefix,
	)
	desiredKeys := parseDesiredKeys(
		os.Getenv("DESIRED_CUSTOM_ANNOTATIONS"),
	)

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("create Kubernetes config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("create Kubernetes client: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("create dynamic Kubernetes client: %v", err)
	}

	log.Printf(
		"Starting PVC annotation cleanup: namespace=%s CHI=%s prefix=%s desiredKeys=%v",
		namespace, chiName, pvcPrefix, desiredKeys,
	)

	if err := cleanup.WaitForCHICompleted(
		ctx,
		dynamicClient,
		namespace,
		chiName,
		cleanup.WaitTimeout,
	); err != nil {
		log.Fatalf("wait for CHI reconciliation: %v", err)
	}

	log.Printf("CHI reconciliation reported completed; listing PVCs")

	pvcList, err := kubeClient.CoreV1().
		PersistentVolumeClaims(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("list PVCs: %v", err)
	}

	matched := 0

	for _, pvc := range pvcList.Items {
		if !strings.HasPrefix(pvc.Name, pvcPrefix) {
			continue
		}

		matched++
		log.Printf("Processing PVC %s", pvc.Name)

		if err := cleanup.CleanupPVC(
			ctx,
			kubeClient,
			namespace,
			pvc.Name,
			desiredKeys,
		); err != nil {
			log.Printf("ERROR: cleanup PVC %s failed: %v", pvc.Name, err)
			log.Fatal("PVC annotation cleanup failed")
		}
	}

	if matched == 0 {
		log.Fatalf(
			"no PVCs found with prefix %q in namespace %q",
			pvcPrefix,
			namespace,
		)
	}

	log.Printf("PVC annotation cleanup completed successfully; processed %d PVC(s)", matched)

}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseDesiredKeys(value string) []string {
	var keys []string
	for _, key := range strings.Split(value, ",") {
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}
