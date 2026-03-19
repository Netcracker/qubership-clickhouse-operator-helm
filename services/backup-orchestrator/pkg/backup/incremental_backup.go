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

package backup

import (
	"errors"
	"fmt"

	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/client"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/constants"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/utils"
)

func (backup *Backup) PerformIncrementalBackup() error {
	// if incremental backups enabled, check for latest full
	// or incremental backup and trigger incremental as diff from latest
	protocol := "http"
	if utils.IsTlsEnabled() {
		protocol = "https"
	}
	backupClient := &client.HttpBackupClient{Host: "localhost", Port: "8080", HttpClient: backup.Helper.HttpClient, Protocol: protocol}

	backup.Log.Info("incremental command invoked, processing")

	if !utils.IsS3Remote() {
		return errors.New("incremental backups are supported only for S3 remote storage")
	}

	if backupId, ok := backupClient.GetLatestBackup(); ok {
		backup.Log.Info(fmt.Sprintf("latest full backup with Id: %s, found, requesting incremental", backupId))
		if err := backup.requestIncrementalForBackup(backupId); err != nil {
			return err
		}
	} else {
		return errors.New("full backup not found, not possible to do the incremental, failing")
	}
	return nil
}

func (backup *Backup) requestIncrementalForBackup(backupIdFrom string) error {
	hosts, err := backup.Helper.GetClickhouseServices()
	if err != nil {
		return err
	}
	tables, err := backup.decideWhatToBackup(hosts)
	if err != nil {
		return err
	}
	for _, hostname := range hosts {
		backup.Log.Info(fmt.Sprintf("start to backup: %s", backup.BackupId(hostname)))
		backupAction := fmt.Sprintf("backup/create?name=%s&%s", backup.BackupId(hostname), tables)

		if err := utils.PostActionAndWait(backup.Helper.HttpClient, "post", hostname, constants.CHBackupPort, backupAction); err != nil {
			return err
		}

		if err := backup.uploadIncrementalBackupForHost(hostname, backupIdFrom); err != nil {
			return err
		}

		if err := backup.deleteLocalBackupForHost(hostname); err != nil {
			return err
		}
	}
	return nil
}
