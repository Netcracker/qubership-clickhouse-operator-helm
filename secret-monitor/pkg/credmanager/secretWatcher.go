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

var (
	logger           = utils.GetLogger()
	mutex            = sync.Mutex{}
	clickhouseClient *client.ClickHouseClient
	changedUser      = []client.UserCreds{}
)

const (
	lockLabel = "locked-for-watcher"
)

func Reconcile() {
	mutex.Lock()
	defer mutex.Unlock()
	time.Sleep(20 * time.Second)

	SecretNames := utils.GetSecretNames()

	for _, secretName := range SecretNames {
		err := manager.ActualizeCreds(secretName, changeCredsFunc)
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

	if len(changedUser) > 0 {
		if err := updateClickhouseCreds(&changedUser); err != nil {
			logger.Error("Failed to update ClickHouse credentials", zap.Error(err))
		}
	}
}

func changeCredsFunc(newSecret, oldSecret *corev1.Secret) error {
	clickhouseUser := string(newSecret.Data["clickhouse_user"])
	newPassword := string(newSecret.Data["password"])

	hashedPassword := hashPassword(newPassword)

	changedUser = append(changedUser, client.UserCreds{

		User:     clickhouseUser,
		Password: hashedPassword,
		Secret:   newSecret,
	},
	)

	return nil
}

func hashPassword(password string) string {
	hash := sha256.New()
	hash.Write([]byte(password))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func updateClickhouseCreds(users *[]client.UserCreds) error {
	defer func() {
		for _, userCred := range *users {
			userCred.Secret.Annotations[lockLabel] = "false"
		}

		// Clear the *users slice after the operation is complete
		*users = (*users)[:0]
	}()

	logger.Info("updateClickhouseCreds called ....")

	if len(*users) == 0 {
		logger.Info("No user credentials to update")
		return nil
	}

	for _, userCred := range *users {
		userCred.Secret.Annotations[lockLabel] = "true"
	}

	err := clickhouseClient.UpdateClickhouseUser(context.Background(), *users)
	if err != nil {
		return fmt.Errorf("failed to update ClickHouse credentials: %v", err)
	}

	return nil
}
