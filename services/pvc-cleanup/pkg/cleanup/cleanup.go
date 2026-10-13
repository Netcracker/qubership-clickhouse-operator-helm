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

package cleanup

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"
)

const customAnnotationsKey = "deployment.netcracker.com/custom-annotations"

func getKubeClient() (*kubernetes.Clientset, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubec", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubec", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		k8sConfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}
	k8sConfig.Timeout = 60 * time.Second

	client, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	return client, nil
}

func Run(namespace, desiredCustom string) error {
	clientset, err := getKubeClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pvcs, err := clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list PVCs: %w", err)
	}

	desiredKeys := parseKeys(desiredCustom)

	for i := range pvcs.Items {
		pvcName := pvcs.Items[i].Name
		if err := cleanupPVC(ctx, clientset, namespace, pvcName, desiredKeys); err != nil {
			fmt.Printf("WARNING: failed to cleanup PVC %s: %v\n", pvcName, err)
		}
	}

	return nil
}

func cleanupPVC(ctx context.Context, clientset kubernetes.Interface, namespace, pvcName string, desiredKeys map[string]bool) error {
	backoff := retry.DefaultBackoff
	backoff.Steps = 3

	return retry.OnError(backoff, isRetryableError, func() error {
		current, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get PVC: %w", err)
		}

		if current.Annotations == nil {
			return nil
		}

		tracked, exists := current.Annotations[customAnnotationsKey]
		if !exists {
			return nil
		}

		trackedKeys := parseKeys(tracked)

		changed := false
		for key := range trackedKeys {
			if !desiredKeys[key] {
				fmt.Printf("PVC %s: removing stale annotation '%s'\n", current.Name, key)
				delete(current.Annotations, key)
				changed = true
			}
		}

		newTrackedValue := formatKeys(desiredKeys)
		if current.Annotations[customAnnotationsKey] != newTrackedValue {
			current.Annotations[customAnnotationsKey] = newTrackedValue
			changed = true
		}

		if !changed {
			fmt.Printf("PVC %s: annotations up to date, skipping\n", current.Name)
			return nil
		}

		_, err = clientset.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, current, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update PVC: %w", err)
		}

		fmt.Printf("PVC %s: patched successfully\n", current.Name)
		return nil
	})
}

func isRetryableError(err error) bool {
	if apierrors.IsConflict(err) {
		fmt.Printf("Conflict detected, retrying: %v\n", err)
		return true
	}
	if apierrors.IsServiceUnavailable(err) {
		fmt.Printf("Service unavailable, retrying: %v\n", err)
		return true
	}
	return false
}

func parseKeys(s string) map[string]bool {
	keys := make(map[string]bool)
	for _, k := range strings.Split(s, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			keys[k] = true
		}
	}
	return keys
}

func formatKeys(keys map[string]bool) string {
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}
