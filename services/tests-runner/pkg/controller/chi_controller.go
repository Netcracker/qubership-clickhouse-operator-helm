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

package controller

import (
	"context"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Netcracker/qubership-clickhouse-operator-helm/tests-runner/pkg/pod"
)

var chiGVK = schema.GroupVersionKind{
	Group:   "clickhouse.altinity.com",
	Version: "v1",
	Kind:    "ClickHouseInstallation",
}

// CHIReconciler watches ClickHouseInstallation objects and manages the lifecycle
// of the integration test pod according to the following state machine:
//
//  1. CHI not Completed                                     → requeue, wait
//  2. backup-orchestrator Deployment exists
//     but not Available                                     → requeue, wait
//  3. No test pod exists                                    → create pod, annotate with CHI taskID
//  4. Pod exists, annotation taskID == CHI taskID           → do nothing (tests ran for this version)
//  5. Pod exists, annotation taskID != CHI taskID           → delete stale pod, create new one
//  6. Pod Pending or Unknown                                → requeue, wait
//
// The CHI status.taskID is set by the Altinity operator on every reconciliation cycle
// (install or upgrade). The controller stamps this value on the test pod as annotation
// "clickhouse.qubership.org/chi-task-id". This ensures tests run exactly once per
// ClickHouse reconciliation and are never restarted after Succeeded/Failed unless
// the cluster is actually updated.
//
// The test pod always has a fixed name, so Kubernetes uniqueness guarantees
// that at most one pod exists at any time.
type CHIReconciler struct {
	client.Client
	Log     *zap.Logger
	Config  *pod.Config
	CHIName string
}

func New(c client.Client, log *zap.Logger, cfg *pod.Config, chiName string) *CHIReconciler {
	return &CHIReconciler{
		Client:  c,
		Log:     log,
		Config:  cfg,
		CHIName: chiName,
	}
}

func (r *CHIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Only handle the CHI we are configured to watch.
	if req.Name != r.CHIName {
		return ctrl.Result{}, nil
	}

	log := r.Log.With(zap.String("chi", req.String()))

	// --- Step 1: fetch CHI and check its status ---
	chi := &unstructured.Unstructured{}
	chi.SetGroupVersionKind(chiGVK)
	if err := r.Get(ctx, req.NamespacedName, chi); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("CHI not found, skipping")
			return ctrl.Result{}, nil
		}
		log.Error("Failed to fetch CHI", zap.Error(err))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	chiStatus, _, _ := unstructured.NestedString(chi.Object, "status", "status")
	hosts, _, _ := unstructured.NestedInt64(chi.Object, "status", "hosts")
	taskID, _, _ := unstructured.NestedString(chi.Object, "status", "taskID")

	log.Info("CHI status", zap.String("status", chiStatus), zap.Int64("hosts", hosts), zap.String("taskID", taskID))

	// Possible values from the Altinity operator source (type_status.go):
	//   "InProgress"  — operator is actively reconciling
	//   "Completed"   — reconcile finished successfully
	//   "Aborted"     — reconcile was interrupted
	//   "Terminating" — cluster is being deleted
	// "Completed" means the operator finished reconciling all hosts successfully.
	switch chiStatus {
	case "Aborted", "Terminating":
		log.Info("CHI is in terminal non-ready state, skipping tests", zap.String("status", chiStatus))
		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	case "Completed":
		// fall through — cluster is ready
	default:
		// "InProgress" or any unknown future status — wait
		log.Info("CHI not ready yet, requeuing", zap.String("status", chiStatus))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// --- Step 2: check backup-orchestrator readiness (optional component) ---
	if r.Config.BackupOrchestratorDeployment != "" {
		ready, err := r.isBackupOrchestratorReady(ctx, req.Namespace, log)
		if err != nil {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
		if !ready {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// --- Step 3: check existing test pod against current taskID ---
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList,
		client.InNamespace(req.Namespace),
		client.MatchingLabels(pod.Labels()),
	); err != nil {
		log.Error("Failed to list test pods", zap.Error(err))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	return r.reconcileTestPods(ctx, podList.Items, taskID, log)
}

func (r *CHIReconciler) reconcileTestPods(ctx context.Context, pods []corev1.Pod, taskID string, log *zap.Logger) (ctrl.Result, error) {
	if len(pods) == 0 {
		log.Info("No test pod found, creating", zap.String("taskID", taskID))
		return r.createTestPod(ctx, taskID, log)
	}

	p := pods[0]
	podTaskID := p.Annotations[pod.TaskIDAnnotation]
	log.Info("Found test pod", zap.String("name", p.Name), zap.String("phase", string(p.Status.Phase)),
		zap.String("podTaskID", podTaskID), zap.String("chiTaskID", taskID))

	// If the pod was created for the current CHI reconciliation cycle, leave it alone
	// regardless of its phase — tests already ran (or are running) for this version.
	if podTaskID == taskID {
		log.Info("Test pod is up-to-date with current CHI taskID, nothing to do",
			zap.String("phase", string(p.Status.Phase)))
		return ctrl.Result{}, nil
	}

	// taskID changed — CHI was upgraded. Delete the stale pod and run tests for the new version.
	log.Info("CHI taskID changed, replacing test pod", zap.String("oldTaskID", podTaskID), zap.String("newTaskID", taskID))
	if err := r.deletePod(ctx, &p, log); err != nil {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}
	return r.createTestPod(ctx, taskID, log)
}

func (r *CHIReconciler) createTestPod(ctx context.Context, taskID string, log *zap.Logger) (ctrl.Result, error) {
	testPod := pod.New(r.Config, taskID)
	log.Info("Creating test pod", zap.String("name", testPod.Name), zap.String("taskID", taskID))
	if err := r.Create(ctx, testPod); err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Info("Test pod already exists, requeuing")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		log.Error("Failed to create test pod", zap.Error(err))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	log.Info("Test pod created successfully")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *CHIReconciler) deletePod(ctx context.Context, p *corev1.Pod, log *zap.Logger) error {
	log.Info("Deleting pod", zap.String("name", p.Name))
	if err := r.Delete(ctx, p); err != nil && !apierrors.IsNotFound(err) {
		log.Error("Failed to delete pod", zap.String("name", p.Name), zap.Error(err))
		return err
	}
	return nil
}

// isBackupOrchestratorReady checks whether the clickhouse-backup-orchestrator Deployment
// is Available. Returns true if the Deployment does not exist (the component is optional).
func (r *CHIReconciler) isBackupOrchestratorReady(ctx context.Context, namespace string, log *zap.Logger) (bool, error) {
	dep := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: r.Config.BackupOrchestratorDeployment}, dep)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Backup orchestrator deployment not found, skipping check",
				zap.String("deployment", r.Config.BackupOrchestratorDeployment))
			return true, nil
		}
		log.Error("Failed to fetch backup orchestrator deployment", zap.Error(err))
		return false, err
	}

	for _, c := range dep.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
			log.Info("Backup orchestrator is Available",
				zap.String("deployment", r.Config.BackupOrchestratorDeployment))
			return true, nil
		}
	}
	log.Info("Backup orchestrator not yet Available, requeuing",
		zap.String("deployment", r.Config.BackupOrchestratorDeployment))
	return false, nil
}

// SetupWithManager registers the controller to watch ClickHouseInstallation objects.
// No scheme registration is required for the CHI type because we use unstructured access.
func (r *CHIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	chi := &unstructured.Unstructured{}
	chi.SetGroupVersionKind(chiGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(chi).
		Complete(r)
}
