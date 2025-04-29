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

package credmanager

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/Netcracker/qubership-clickhouse-operator-helm/secret-monitor/pkg/client"
	"github.com/Netcracker/qubership-credential-manager/pkg/manager"
	"github.com/Netcracker/qubership-credential-manager/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

const lockLabel = "locked-for-watcher"

var (
	logger           = utils.GetLogger()
	mutex            = sync.Mutex{}
	clickhouseClient *client.ClickHouseClient
)

// CredentialManager holds the state of changed credentials
type CredentialManager struct {
	ChangedUsers []client.UserCreds
}

func Reconcile() {
	mutex.Lock()
	defer mutex.Unlock()
	time.Sleep(20 * time.Second)

	credManager := &CredentialManager{}
	secretNames := utils.GetSecretNames()

	for _, secretName := range secretNames {
		err := manager.ActualizeCreds(secretName, credManager.changeCredsFunc)
		if err != nil {
			logger.Error("cannot update clickhouse creds", zap.Error(err))
		}
	}

	if clickhouseClient == nil {
		var err error
		clickhouseClient, err = client.NewClickHouseClient()
		if err != nil {
			logger.Error("Failed to create ClickHouse client", zap.Error(err))
			return
		}
	}

	if len(credManager.ChangedUsers) > 0 {
		updatedUsers, err := credManager.updateClickhouseCreds()
		if err != nil {
			logger.Error("Failed to update ClickHouse credentials", zap.Error(err))
		}
		credManager.ChangedUsers = updatedUsers
	}
}

func (cm *CredentialManager) changeCredsFunc(newSecret, oldSecret *corev1.Secret) error {
	clickhouseUser := string(newSecret.Data["clickhouse_user"])
	newPassword := string(newSecret.Data["password"])
	hashedPassword := hashPassword(newPassword)

	cm.ChangedUsers = append(cm.ChangedUsers, client.UserCreds{
		User:     clickhouseUser,
		Password: hashedPassword,
		Secret:   newSecret,
	})

	return nil
}

func (cm CredentialManager) updateClickhouseCreds() ([]client.UserCreds, error) {

	defer func() {
		for i := range cm.ChangedUsers {
			cm.ChangedUsers[i].Secret.Annotations[lockLabel] = "false"
		}
	}()

	logger.Info("updateClickhouseCreds called ....")

	if len(cm.ChangedUsers) == 0 {
		logger.Info("No user credentials to update")
		return cm.ChangedUsers, nil
	}

	for i := range cm.ChangedUsers {
		cm.ChangedUsers[i].Secret.Annotations[lockLabel] = "true"
	}

	err := clickhouseClient.UpdateClickhouseUser(context.Background(), cm.ChangedUsers)
	if err != nil {
		return cm.ChangedUsers, fmt.Errorf("failed to update ClickHouse credentials: %v", err)
	}

	return []client.UserCreds{}, nil
}

func hashPassword(password string) string {
	hash := sha256.New()
	hash.Write([]byte(password))
	return fmt.Sprintf("%x", hash.Sum(nil))
}
