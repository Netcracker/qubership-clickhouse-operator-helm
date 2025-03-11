package main

import (
	"github.com/Netcracker/qubership-clickhouse-operator-helm/hook/pkg/handler"
	"github.com/Netcracker/qubership-credential-manager/pkg/utils"
)

var (
	logger = utils.GetLogger()
)

func main() {

	logger.Info("post hook executing started")
	handler.UnlockSecret(utils.GetSecretNames())

}
