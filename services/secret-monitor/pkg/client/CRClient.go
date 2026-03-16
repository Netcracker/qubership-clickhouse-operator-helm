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

package client

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/Netcracker/qubership-credential-manager/pkg/utils"
	clickhousev1 "github.com/altinity/clickhouse-operator/pkg/apis/clickhouse.altinity.com/v1"
	chopClientSet "github.com/altinity/clickhouse-operator/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type ClickHouseClient struct {
	chopClient chopClientSet.Interface
}

var (
	logger    = utils.GetLogger()
	namespace = utils.GetNamespace()
)

type UserCreds struct {
	User     string
	Password string
	Secret   *corev1.Secret
}

const (
	ClickHouseInstallationCRDResource = "cluster"
)

func userKey(user string) string {
	haskey := "password_sha256_hex" // Fixed string
	// Format the string as "clickhouse/password_sha256_hex"
	passwordKey := fmt.Sprintf("%s/%s", user, haskey)
	return passwordKey
}

// NewClickHouseClient creates a new ClickHouseClient instance
func NewClickHouseClient() (*ClickHouseClient, error) {

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubec", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubec", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		// use the current context in kubeconfig
		k8sConfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}

	}

	chopClient, err := chopClientSet.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ClickHouse client: %v", err)
	}

	return &ClickHouseClient{
		chopClient: chopClient,
	}, nil
}

func (c *ClickHouseClient) getClickHouseInstallation(ctx context.Context) (*clickhousev1.ClickHouseInstallation, error) {
	return c.chopClient.ClickhouseV1().ClickHouseInstallations(namespace).Get(ctx, ClickHouseInstallationCRDResource, metav1.GetOptions{})

}

func (c *ClickHouseClient) UpdateClickhouseUser(ctx context.Context, userCreds []UserCreds) error {
	// Fetch ClickHouse installation resource
	clickhouseResource, err := c.getClickHouseInstallation(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch ClickHouse installation: %w", err)
	}

	for _, userCred := range userCreds {
		passwordKey := userKey(userCred.User)
		updatedUser := clickhousev1.NewSettingScalar(userCred.Password).SetAttribute(passwordKey, userCred.Password)

		// Ensure the password key is updated in the ClickHouse configuration
		clickhouseResource.Spec.Configuration.Users.Ensure().Set(passwordKey, updatedUser)

		logger.Debug("Updating password for user", zap.String("user", userCred.User))
	}

	_, err = c.chopClient.ClickhouseV1().ClickHouseInstallations(namespace).Update(ctx, clickhouseResource, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ClickHouse resource: %v", err)
	}

	logger.Info("Password(s) updated successfully")
	return nil
}
