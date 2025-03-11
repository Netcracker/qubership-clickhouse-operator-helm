package credmanager

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"

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
)

const (
	lockLabel = "locked-for-watcher"
)

func Reconcile() {

	mutex.Lock()
	defer mutex.Unlock()

	SecretNames := utils.GetSecretNames()

	for _, secretName := range SecretNames {
		err := manager.ActualizeCreds(secretName, changeCredsFunc)
		if err != nil {
			logger.Error("cannot update clickhouse creds", zap.Error(err))
		}
	}
}

func changeCredsFunc(newSecret, oldSecret *corev1.Secret) error {

	logger.Info("Secret will be locked")
	newSecret.Annotations[lockLabel] = "true"

	clickhouse_user := string(newSecret.Data["clickhouse_user"])
	newPassword := string(newSecret.Data["password"])

	// Hash the new password
	hashedPassword := HashPassword(newPassword)

	if clickhouseClient == nil {
		var err error
		clickhouseClient, err = client.NewClickHouseClient()
		if err != nil {
			logger.Error("Failed to create ClickHouse client", zap.Error(err))
			return err
		}
	}

	// Update the ClickHouse user's password
	_, err := clickhouseClient.UpdateClickhouseUser(context.Background(), hashedPassword, clickhouse_user)
	if err != nil {
		logger.Error("Failed to update ClickHouseInstallation CR", zap.Error(err))
		return err
	}

	return nil
}

func HashPassword(password string) string {
	hash := sha256.New()
	hash.Write([]byte(password))
	return fmt.Sprintf("%x", hash.Sum(nil))
}
