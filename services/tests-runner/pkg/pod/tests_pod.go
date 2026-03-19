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

package pod

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TestPodName        = "clickhouse-integration-tests"
	TestPodLabelKey    = "app"
	TestPodLabelVal    = "clickhouse-integration-tests"
	CredentialsSecret  = "clickhouse-integration-tests-secret"
	TaskIDAnnotation   = "clickhouse.qubership.org/chi-task-id"
)

// Config holds all configuration needed to build the test pod spec.
// Values come from environment variables set by the Helm chart on the controller Deployment.
type Config struct {
	Namespace            string
	TestImage            string
	Tags                 string
	ClickhouseHost       string
	ClickhousePort       string
	ClickhouseBackupHost string
	ClickhouseBackupPort string
	// BackupOrchestratorDeployment is the name of the clickhouse-backup-orchestrator
	// Deployment. When non-empty, the controller waits for it to become Available
	// before creating the test pod. Set to empty string to skip this check.
	BackupOrchestratorDeployment string
	ServiceAccountName           string
	TLSEnabled                   string
	ChiName                      string
}

// Labels returns the label set used to identify test pods.
func Labels() map[string]string {
	return map[string]string{TestPodLabelKey: TestPodLabelVal}
}

// New builds the test Pod spec. The pod uses RestartPolicy=Never so it runs
// exactly once. The taskID is stamped as an annotation so the controller can
// detect when a CHI upgrade has produced a new reconciliation cycle.
func New(cfg *Config, taskID string) *corev1.Pod {
	allowPrivilegeEscalation := false
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestPodName,
			Namespace:   cfg.Namespace,
			Labels:      Labels(),
			Annotations: map[string]string{TaskIDAnnotation: taskID},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: cfg.ServiceAccountName,
			Volumes: []corev1.Volume{
				{
					Name: "robot-output",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "clickhouse-integration-tests",
					Image:           cfg.TestImage,
					ImagePullPolicy: corev1.PullAlways,
					Args:            []string{"run-robot-without-ttyd"},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: &allowPrivilegeEscalation,
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "robot-output",
							MountPath: "/opt/robot/output",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name: "NAMESPACE",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.namespace",
								},
							},
						},
						{Name: "STATUS_CUSTOM_RESOURCE_NAMESPACE", Value: "$(NAMESPACE)"},
					{Name: "STATUS_CUSTOM_RESOURCE_NAME", Value: cfg.ChiName},
					{Name: "CLICKHOUSE_HOST", Value: cfg.ClickhouseHost},
						{Name: "CLICKHOUSE_PORT", Value: cfg.ClickhousePort},
						{Name: "CLICKHOUSE_BACKUP_HOST", Value: cfg.ClickhouseBackupHost},
						{Name: "CLICKHOUSE_BACKUP_PORT", Value: cfg.ClickhouseBackupPort},
						{Name: "TAGS", Value: cfg.Tags},
						{Name: "TLS_ENABLED", Value: cfg.TLSEnabled},
						// Cluster readiness is guaranteed by the controller before pod creation,
						// so skip the pre-test wait.
						{Name: "TIMEOUT_BEFORE_START", Value: "0"},
						{
							Name: "CLICKHOUSE_USER",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: CredentialsSecret},
									Key:                  "clickhouse_user",
								},
							},
						},
						{
							Name: "CLICKHOUSE_PASSWORD",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: CredentialsSecret},
									Key:                  "clickhouse_password",
								},
							},
						},
					},
				},
			},
		},
	}
}
