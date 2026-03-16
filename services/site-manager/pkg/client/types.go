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
