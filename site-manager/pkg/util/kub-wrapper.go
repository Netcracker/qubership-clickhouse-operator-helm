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

package util

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sWrapper struct {
	K8sClient    client.Client
	K8sClientSet kubernetes.Clientset
}

func (wrapper *K8sWrapper) GetCurrentMode() string {
	smCm := wrapper.getCm(SiteManagerCmName)
	return smCm.Data["mode"]
}

func (wrapper *K8sWrapper) getCm(cmName string) *corev1.ConfigMap {
	foundCm := &corev1.ConfigMap{}
	err := wrapper.K8sClient.Get(context.TODO(), types.NamespacedName{
		Name: cmName, Namespace: GetNameSpace(),
	}, foundCm)

	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		log.Error(fmt.Sprintf("Failed to get configMap %s", cmName), zap.Error(err))
		return nil
	}
	return foundCm
}

func (wrapper *K8sWrapper) IsActive() bool {
	return wrapper.GetCurrentMode() == ModeActive
}

func (wrapper *K8sWrapper) GetChHosts() ([]string, error) {
	var resultServices []string
	serviceList := &corev1.ServiceList{}
	listOps := &client.ListOptions{
		Namespace: GetNameSpace(),
	}

	if err := wrapper.K8sClient.List(context.TODO(), serviceList, listOps); err == nil {
		for idx := 0; idx < len(serviceList.Items); idx++ {
			service := &serviceList.Items[idx]
			if r, e := regexp.MatchString(fmt.Sprintf("chi-cluster-%s.*", getClusterName()), service.Name); e == nil && r {
				resultServices = append(resultServices, service.Name)
			}
		}
	} else {
		//log.Info(fmt.Sprintf("Can not get k8s services"))
		return nil, err
	}
	return resultServices, nil
}

func (wrapper *K8sWrapper) GetPreConfigureStatus() map[string]string {
	return wrapper.getCm(PreConfigureCmName).Data
}

func (wrapper *K8sWrapper) GetSmStatus() map[string]string {
	return wrapper.getCm(SiteManagerCmName).Data
}

func (wrapper *K8sWrapper) GetChStatus() interface{} {
	statefulSetList := &v1.StatefulSetList{}
	labelSelector, _ := labels.Parse(ChLabels)
	statefulSetListOpts := &client.ListOptions{Namespace: GetNameSpace(), LabelSelector: labelSelector}
	if err := wrapper.K8sClient.List(context.TODO(), statefulSetList, statefulSetListOpts); err == nil {
		numberOfStateful := len(statefulSetList.Items)
		for _, item := range statefulSetList.Items {
			if (item.Status.ReadyReplicas == 1) &&
				(item.Status.CurrentReplicas == 1) {
				numberOfStateful = numberOfStateful - 1
			}
		}
		if numberOfStateful == 0 {
			return Health{Status: "up"}
		} else if numberOfStateful < len(statefulSetList.Items) {
			return Health{Status: "degraded"}
		} else if numberOfStateful == len(statefulSetList.Items) {
			return Health{Status: "down"}
		}
	}
	return Health{Status: "n/a"}
}

func (wrapper *K8sWrapper) UpdatePreConfigureStatus(mode string, status string) {
	wrapper.updateConfigMapWithData(PreConfigureCmName, map[string]string{"mode": mode, "status": status})
}

func (wrapper *K8sWrapper) UpdateSMStatus(mode string, status string) {
	wrapper.updateConfigMapWithData(SiteManagerCmName, map[string]string{"mode": mode, "status": status})
}

func (wrapper *K8sWrapper) updateConfigMapWithData(cmName string, data map[string]string) {
	cm := &corev1.ConfigMap{}
	_ = wrapper.K8sClient.Get(context.TODO(), types.NamespacedName{
		Name: cmName, Namespace: GetNameSpace(),
	}, cm)
	cm.Data = data
	_ = wrapper.K8sClient.Update(context.TODO(), cm)
}

func (wrapper *K8sWrapper) GetExternalService() *corev1.Service {
	foundService := &corev1.Service{}
	err := wrapper.K8sClient.Get(context.TODO(), types.NamespacedName{Name: "clickhouse-cluster-external", Namespace: GetNameSpace()}, foundService)
	if err != nil && errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return nil
	} else {
		return foundService
	}
}

func (wrapper *K8sWrapper) UpdateService(service *corev1.Service) error {
	return wrapper.K8sClient.Update(context.TODO(), service)
}

func (wrapper *K8sWrapper) GetOppositeChHost() string {
	cm := wrapper.getCm("opposite-ch-host")
	return cm.Data["host"]
}

func (wrapper *K8sWrapper) PatchReplicatorDeploymentForMode(mode string) error {
	// swap envs, real scheduling for incr backups always placed in FAKE_INCREMENTAL_BACKUP_SCHEDULE env
	// in case of enabling need take this value and set it to INC_BACKUP_SCHEDULE env
	// for disabling set it to None
	deps := []*v1.Deployment{
		wrapper.getDeployment(ReplicatorName),
		wrapper.getDeployment(OrchestratorName),
	}

	for _, dep := range deps {
		var newIncBackupSchedule corev1.EnvVar
		var newBackupSchedule corev1.EnvVar
		if mode == ModeStandby {
			newIncBackupSchedule = corev1.EnvVar{Name: "INC_BACKUP_SCHEDULE", Value: "None"}
			newBackupSchedule = corev1.EnvVar{Name: "BACKUP_SCHEDULE", Value: "None"}
		} else {
			envs := dep.Spec.Template.Spec.Containers[0].Env
			for _, env := range envs {
				if env.Name == "FAKE_INCREMENTAL_BACKUP_SCHEDULE" {
					newIncBackupSchedule = corev1.EnvVar{Name: "INC_BACKUP_SCHEDULE", Value: env.Value}
				}
				if env.Name == "FAKE_BACKUP_SCHEDULE" {
					newBackupSchedule = corev1.EnvVar{Name: "BACKUP_SCHEDULE", Value: env.Value}
				}
			}
		}
		envs := dep.Spec.Template.Spec.Containers[0].Env
		for idx, env := range envs {
			switch env.Name {
			case "INC_BACKUP_SCHEDULE":
				envs[idx] = corev1.EnvVar{
					Name:  "INC_BACKUP_SCHEDULE",
					Value: newIncBackupSchedule.Value,
				}
			case "BACKUP_SCHEDULE":
				envs[idx] = corev1.EnvVar{
					Name:  "BACKUP_SCHEDULE",
					Value: newBackupSchedule.Value,
				}
			case "MODE":
				envs[idx] = corev1.EnvVar{
					Name:  "MODE",
					Value: mode,
				}
			}
		}

		dep.Spec.Template.Spec.Containers[0].Env = envs
		if err := wrapper.K8sClient.Update(context.TODO(), dep); err != nil {
			return err
		}
	}
	time.Sleep(2 * time.Second)
	err := wrapper.waitForReplicator()
	if err != nil {
		return err
	}
	return nil
}

func (wrapper *K8sWrapper) waitForReplicator() error {
	siteManagerLabels := map[string]string{"app": ReplicatorName}
	return wait.PollImmediate(time.Second, 5*time.Minute, func() (done bool, err error) {
		return wrapper.checkPodsByLabel(siteManagerLabels, 1)
	})
}

func (wrapper *K8sWrapper) checkPodsByLabel(labelSelectors map[string]string, numberOfPods int) (done bool, err error) {
	log.Info(fmt.Sprintf("Will try to find only %d Pod(s) with labels %q", numberOfPods, labelSelectors))
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(GetNameSpace()),
		client.MatchingLabels(labelSelectors),
	}
	if err = wrapper.K8sClient.List(context.Background(), podList, listOpts...); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Pods doesn't exist yet.")
			return false, nil
		}
		return false, err
	}
	if len(podList.Items) == numberOfPods {
		log.Info("Pods are exists, exiting.")
		return true, nil
	}
	return false, nil
}

func (wrapper *K8sWrapper) getDeployment(name string) *v1.Deployment {
	foundDepl := &v1.Deployment{}
	err := wrapper.K8sClient.Get(context.TODO(), types.NamespacedName{
		Name: name, Namespace: GetNameSpace(),
	}, foundDepl)
	if err != nil {
		return nil
	}
	return foundDepl
}

type Health struct {
	Status string `json:"status"`
}
