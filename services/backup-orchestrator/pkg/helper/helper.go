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

package helper

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log         = utils.GetLogger()
	namespace   = utils.GetNameSpace()
	clusterName = utils.GetClusterName()
)

func Init(client client.Client, httpClient http.Client) *Helper {
	return &Helper{client: client, HttpClient: httpClient}
}

type Helper struct {
	client     client.Client
	HttpClient http.Client
}

func (h *Helper) GetClickhouseServices() ([]string, error) {
	var resultServices []string
	serviceList := &corev1.ServiceList{}
	listOps := &client.ListOptions{
		Namespace: namespace,
	}

	if err := h.client.List(context.TODO(), serviceList, listOps); err == nil {
		for idx := 0; idx < len(serviceList.Items); idx++ {
			service := &serviceList.Items[idx]
			if r, e := regexp.MatchString(fmt.Sprintf("chi-cluster-%s.*", clusterName), service.Name); e == nil && r {
				resultServices = append(resultServices, service.Name)
			}
		}
	} else {
		log.Info(fmt.Sprintf("Can not get k8s services"))
		return nil, err
	}
	return resultServices, nil
}

func (h *Helper) UpdateClickhouseClusterService(service *corev1.Service) error {
	err := h.client.Update(context.TODO(), service)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to update ClickhouseClusterService %v", service.ObjectMeta.Name), zap.Error(err))
		return err
	}
	return nil
}

func (h *Helper) GetClickhouseClusterService() (*corev1.Service, error) {
	service := &corev1.Service{}
	if err := h.client.Get(context.TODO(), types.NamespacedName{
		Name: "clickhouse-cluster", Namespace: namespace,
	}, service); err != nil {
		if errors.IsNotFound(err) {
			return service, nil
		}
		log.Error("Cannot fetch ClickhouseClusterService", zap.Error(err))
		return service, err
	}
	return service, nil
}

func (h *Helper) GetClickhouseClusterServiceSelectors() map[string]string {
	selectors := map[string]string{
		"clickhouse.altinity.com/app":       "chop",
		"clickhouse.altinity.com/chi":       "cluster",
		"clickhouse.altinity.com/namespace": namespace,
		"clickhouse.altinity.com/ready":     "yes",
	}
	return selectors
}

func GetForHostForEachShard(hosts []string) []string {
	shards := make([]string, 0)
	activeShard := 0
	for _, host := range hosts {
		if strings.Contains(host, fmt.Sprintf("chi-cluster-replicated-%d", activeShard)) {
			shards = append(shards, host)
			activeShard++
		}
	}
	return shards
}
