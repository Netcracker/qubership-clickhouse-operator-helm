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
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	ManagedAnnotationsKey = "deployment.netcracker.com/custom-annotations"
	DefaultCHIName        = "cluster"
	DefaultPVCNamePrefix  = "clickhouse-pvc-chi-cluster"

	WaitTimeout  = 15 * time.Minute
	PollInterval = 5 * time.Second
)

var chiGVR = schema.GroupVersionResource{
	Group:    "clickhouse.altinity.com",
	Version:  "v1",
	Resource: "clickhouseinstallations",
}

// WaitForCHICompleted waits until the CHI reports Completed.
// Verify the status fields against your installed operator version.
func WaitForCHICompleted(
	ctx context.Context,
	client dynamic.Interface,
	namespace string,
	chiName string,
	timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		chi, err := client.Resource(chiGVR).
			Namespace(namespace).
			Get(ctx, chiName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get CHI %s: %w", chiName, err)
		}

		status, _, _ := unstructured.NestedString(
			chi.Object, "status", "status",
		)
		taskID, _, _ := unstructured.NestedString(
			chi.Object, "status", "taskID",
		)
		completedTaskIDs, _, _ := unstructured.NestedStringSlice(
			chi.Object, "status", "taskIDsCompleted",
		)

		taskCompleted := false
		for _, completedID := range completedTaskIDs {
			if taskID != "" && completedID == taskID {
				taskCompleted = true
				break
			}
		}

		log.Printf(
			"CHI=%s status=%q taskID=%q taskCompleted=%t generation=%d",
			chiName,
			status,
			taskID,
			taskCompleted,
			chi.GetGeneration(),
		)

		if status == "Completed" && taskCompleted {
			return nil
		}

		if status == "Error" || status == "Aborted" {
			return fmt.Errorf(
				"CHI %s reconciliation failed: status=%q taskID=%q",
				chiName, status, taskID,
			)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"timed out waiting for CHI %s reconciliation: %w",
				chiName, ctx.Err(),
			)
		case <-ticker.C:
		}
	}
}

// CleanupPVC removes obsolete annotations previously tracked as deployment-managed.
func CleanupPVC(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	pvcName string,
	desiredKeys []string,
) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).
			Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get PVC %s: %w", pvcName, err)
		}

		changed := removeObsoleteAnnotations(pvc, desiredKeys)
		if !changed {
			log.Printf("PVC %s: no annotation changes required", pvcName)
			return nil
		}

		_, err = client.CoreV1().PersistentVolumeClaims(namespace).
			Update(ctx, pvc, metav1.UpdateOptions{})
		if err != nil {
			if apierrors.IsConflict(err) {
				log.Printf("PVC %s: resource-version conflict; retrying", pvcName)
			}
			return err
		}

		log.Printf("PVC %s: obsolete annotations cleaned up", pvcName)
		return nil
	})
}

func removeObsoleteAnnotations(
	pvc *corev1.PersistentVolumeClaim,
	desiredKeys []string,
) bool {
	annotations := pvc.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Without tracking metadata, we cannot safely identify which
	// existing annotations were managed by this deployment.
	previouslyManaged, exists := annotations[ManagedAnnotationsKey]
	if !exists {
		annotations[ManagedAnnotationsKey] = joinSortedKeys(desiredKeys)
		pvc.SetAnnotations(annotations)
		log.Printf(
			"PVC %s: tracking annotation missing; initialized tracking without deleting annotations",
			pvc.Name,
		)
		return true
	}

	desired := make(map[string]struct{}, len(desiredKeys))
	for _, key := range desiredKeys {
		if key != "" {
			desired[key] = struct{}{}
		}
	}

	changed := false

	for _, key := range parseKeys(previouslyManaged) {
		if _, stillDesired := desired[key]; stillDesired {
			continue
		}

		if _, found := annotations[key]; found {
			delete(annotations, key)
			changed = true
			log.Printf("PVC %s: removed obsolete annotation %q", pvc.Name, key)
		}
	}

	newTracking := joinSortedKeys(desiredKeys)
	if annotations[ManagedAnnotationsKey] != newTracking {
		annotations[ManagedAnnotationsKey] = newTracking
		changed = true
	}

	if changed {
		pvc.SetAnnotations(annotations)
	}

	return changed
}

func parseKeys(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	var keys []string
	for _, key := range strings.Split(value, ",") {
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func joinSortedKeys(keys []string) string {
	unique := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" {
			unique[key] = struct{}{}
		}
	}

	result := make([]string, 0, len(unique))
	for key := range unique {
		result = append(result, key)
	}
	sort.Strings(result)

	return strings.Join(result, ",")
}
