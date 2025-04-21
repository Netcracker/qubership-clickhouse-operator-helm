package client

import "github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/util"

const (
	chBackupHost      = "clickhouse-replicator"
	chBackupPort      = "8080"
	chBackupTlsPort   = "8443"
	IncrementalBackup = "incremental"
	FullBackup        = "full"
)

var logger = util.GetLogger()

type HttpBackupClient struct {
	Protocol string
	Host     string
	Port     string
}

type BackupIdsList []string

type BackupInfo struct {
	Id         string            `json:"id"`
	Failed     bool              `json:"failed"`
	Valid      bool              `json:"valid"`
	IsGranular bool              `json:"is_granular"`
	CustomVars map[string]string `json:"custom_vars"`
	Locked     bool              `json:"locked"`
}

type RestoreInfo struct {
	Status string `json:"status"`
	Vault  string `json:"vault"`
	Error  string `json:"err"`
}
