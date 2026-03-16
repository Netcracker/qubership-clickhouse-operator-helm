package main

import (
	"github.com/Netcracker/qubership-clickhouse-backup-sidecar/pkg/server"
	"github.com/Netcracker/qubership-clickhouse-backup-sidecar/pkg/util"
)

func main() {

	var log = util.GetLogger()
	log.Info(":7172")
	server.InitBackupSenderServer()
}
