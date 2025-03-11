package helper

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

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
