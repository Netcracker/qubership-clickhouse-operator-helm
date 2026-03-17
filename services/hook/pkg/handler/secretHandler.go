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

package handler

import (
	"context"
	"fmt"

	"github.com/Netcracker/qubership-credential-manager/pkg/manager"
	"github.com/Netcracker/qubership-credential-manager/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	logger    = utils.GetLogger()
	namespace = utils.GetNamespace()
)

const (
	lockLabel = "locked-for-watcher"
)

func getSecret(secretName string) (*corev1.Secret, error) {
	foundSecret := &corev1.Secret{}
	err := manager.GetK8SClient().Get(context.TODO(), types.NamespacedName{
		Name: secretName, Namespace: namespace,
	}, foundSecret)
	if err != nil {
		logger.Error(fmt.Sprintf("can't find the secret %s", secretName), zap.Error(err))
		return foundSecret, err
	}
	return foundSecret, nil
}

func UnlockSecret(secrets []string) error {
	logger.Info("Secret will be locked")
	for _, secretName := range secrets {
		secret, err := getSecret(secretName)
		logger.Info(fmt.Sprintf("cannot create %v secret in namespace ", secret))
		if err != nil {
			return err
		}
		secret.Annotations[lockLabel] = "false"

		if err := manager.GetK8SClient().Update(context.Background(), secret); err != nil {
			// Log and return the error if the update fails
			logger.Error(fmt.Sprintf("Failed to update secret %v: %v", secretName, err))
			return err
		}
	}
	return nil

}
