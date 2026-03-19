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

package terminate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/constants"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/helper"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/types"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/utils"
	"go.uber.org/zap"
)

type Terminate struct {
	Helper     *helper.Helper
	Log        *zap.Logger
	BackupPath string
}

func (terminate *Terminate) BackupId(hostname string) string {
	splitBackupPath := strings.Split(terminate.BackupPath, "/")
	backupId := splitBackupPath[len(splitBackupPath)-1]
	if utils.IsS3Remote() {
		backupId = backupId + "_" + hostname
	}
	return backupId
}

func (terminate *Terminate) KillAction() error {

	hosts, err := terminate.Helper.GetClickhouseServices()
	if err != nil {
		return err
	}

	for _, host := range hosts {
		terminate.Log.Info(fmt.Sprintf("Going to terminate on host %s", host))
		if err := terminate.killRequest(host); err != nil {
			return err
		}
	}
	return nil
}

func (terminate *Terminate) killRequest(hostname string) error {
	var actionRow types.ActionRow
	var err error
	backupId := terminate.BackupId(hostname)
	if actionRow, err = utils.GetLastActionStatusForHost(terminate.Helper.HttpClient, hostname); err != nil {
		terminate.Log.Error("Error occurred", zap.Error(err))
		return err
	}
	if !strings.Contains(actionRow.Command, backupId) {
		terminate.Log.Warn(fmt.Sprintf("No actions matches with %s backup id", backupId))
		return nil
	}
	command := strings.Replace(actionRow.Command, " ", "%20", -1)

	switch actionRow.Status {
	case "in progress":
		terminate.Log.Info(fmt.Sprintf("Start to terminate action: %s on %s", command, hostname))
		cancelAction := fmt.Sprintf("backup/kill?command=%s", command)
		if err = utils.PostActionAndWait(terminate.Helper.HttpClient, "post", hostname, constants.CHBackupPort, cancelAction); err != nil {
			return err
		}
		terminate.Log.Info(fmt.Sprintf("Action %s was terminated", command))
		return nil
	case "success":
		terminate.Log.Info(fmt.Sprintf("Action %s was ended with success. Nothing to terminate", command))
		return nil

	case "error":
		err = errors.New(actionRow.Error)
		terminate.Log.Error(fmt.Sprintf("Action %s was ended with an error", command), zap.Error(err))
		return err

	case "canceled":
		terminate.Log.Info(fmt.Sprintf("Action %s was canceled earlier", command))
		return nil
	}
	return nil
}
