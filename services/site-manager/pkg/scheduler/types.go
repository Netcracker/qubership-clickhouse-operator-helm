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

package scheduler

import (
	"fmt"

	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/client"
	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/util"
)

const (
	cronEnv               = "NC_CRON_EXPR"
	defaultCronExpression = "0 0 */1 *"
	active                = "active"
)

var logger = util.GetLogger()

type BackupsDownloader struct {
	BackupClient *client.HttpBackupClient
	K8sWrapper   util.K8sWrapper
	DownloadAll  bool
}

type ChBackup struct {
	Name string `json:"name"`
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

func (info BackupInfo) isActiveRegion() bool {
	if val, ok := info.CustomVars["mode"]; ok {
		return val == active
	}
	return false
}

func (info BackupInfo) String() string {
	return fmt.Sprintf("Id=%s, Failed=%s, Valid=%s, CustomVars=%s", info.Id, info.Failed, info.Valid, info.CustomVars)
}

type RestoreInfo struct {
	Status string `json:"status"`
	Vault  string `json:"vault"`
	Error  string `json:"err"`
}
