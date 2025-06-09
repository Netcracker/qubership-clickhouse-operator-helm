package main

import (
	"log"
	"os"

	"github.com/Netcracker/qubership-clickhouse-operator-helm/secret-monitor/pkg/credmanager"
	"github.com/Netcracker/qubership-credential-manager/pkg/hook"
	"github.com/Netcracker/qubership-credential-manager/pkg/informer"
	"github.com/Netcracker/qubership-credential-manager/pkg/utils"
	"go.uber.org/zap"
)

func main() {

	// Clear pre-deployment job
	err := os.Setenv("HOOK_NAME", "credentials-saver")
	if err != nil {
		log.Fatal("Error setting environment variable for pre-deployment job:", err)
	}

	_ = hook.ClearHooks()

	// Clear post-deployment job
	err = os.Setenv("HOOK_NAME", "post-deployment-job")
	if err != nil {
		log.Fatal("Error setting environment variable for post-deployment job:", err)
	}

	_ = hook.ClearHooks()

	secretNames := utils.GetSecretNames()
	credmanager.Reconcile()
	err = informer.Watch(secretNames, credmanager.Reconcile)
	if err != nil {
		utils.GetLogger().Error("Failed to watch secret", zap.Error(err))
	}
	select {}
}
