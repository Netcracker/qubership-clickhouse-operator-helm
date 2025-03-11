package main

import (
	"github.com/Netcracker/qubership-clickhouse-operator-helm/secret-monitor/pkg/credmanager"
	"github.com/Netcracker/qubership-credential-manager/pkg/informer"
	"github.com/Netcracker/qubership-credential-manager/pkg/utils"
	"go.uber.org/zap"
)

func main() {
	secretNames := utils.GetSecretNames()
	credmanager.Reconcile()
	err := informer.Watch(secretNames, credmanager.Reconcile)
	if err != nil {
		utils.GetLogger().Error("Failed to watch secret", zap.Error(err))
	}
	select {}
}
