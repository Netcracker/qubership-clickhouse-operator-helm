package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/types"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/utils"
	"go.uber.org/zap"
)

var (
	Log = utils.GetLogger()
)

type HttpBackupClient struct {
	Host       string
	Port       string
	HttpClient http.Client
	Protocol   string
}

// GetLatestBackup find the latest backup, incremental or full
func (client *HttpBackupClient) GetLatestBackup() (string, bool) {
	// incremental goes first
	if backupId, ok := client.getLatestIncrementalBackup(); ok {
		Log.Info("latest incremental found.")
		return backupId, ok
	} else {
		Log.Info("Latest incremental not found, proceeding with full backups check")
		if backupId, ok := client.GetLatestFullBackup(); ok {
			Log.Info("Latest full found.")
			return backupId, ok
		}
	}
	return "", false
}

func (client *HttpBackupClient) getLatestIncrementalBackup() (string, bool) {
	return client.getLatestBackupByType(types.IncrementalBackup)
}

func (client *HttpBackupClient) GetLatestFullBackup() (string, bool) {
	return client.getLatestBackupByType(types.FullBackup)
}

func (client *HttpBackupClient) getLatestBackupByType(backupType string) (string, bool) {
	listOfBackups, err := client.ListBackupsByType(backupType)
	if err != nil {
		Log.Error("there is a error during last backup searching", zap.Error(err))
		return "", false
	}
	Log.Info(fmt.Sprintf("list of %s backups: %v", backupType, listOfBackups))

	if len(listOfBackups) == 0 ||
		(types.IncrementalBackup == backupType && len(listOfBackups) == 1) {
		return "", false
	}

	latestBackup := listOfBackups[len(listOfBackups)-1]

	Log.Info(fmt.Sprintf("latest backup: : %v", latestBackup))

	latestBackupInfo, err := client.getInfoForBackupByType(latestBackup, backupType)
	if err != nil {
		Log.Error("there is a error during request of info about backup", zap.Error(err))
		return "", false
	}
	Log.Info(fmt.Sprintf("info about latest backup: %s", latestBackupInfo))
	if latestBackupInfo.Valid {
		return latestBackup, true
	}
	return "", false
}

func (client *HttpBackupClient) EvictBackups(backup, backupType string) (string, bool) {
	listPath := "evict"
	if types.IncrementalBackup == backupType {
		listPath = "incremental/" + listPath
	}

	res, err := client.HttpClient.Post(fmt.Sprintf("%s://%s:%s/%s/%s", client.Protocol, client.Host, client.Port, listPath, backup), "application/x-www-form-urlencoded", nil)
	if err != nil {
		return "", false
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return "", false
	}
	var evictInfo string
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.Decode(&evictInfo)
	return evictInfo, true
}

func (client *HttpBackupClient) getInfoForBackupByType(lastBackupId string, backupType string) (types.BackupInfo, error) {
	listPath := "listbackups"
	if types.IncrementalBackup == backupType {
		listPath = "incremental/" + listPath
	}

	res, err := client.HttpClient.Get(fmt.Sprintf("%s://%s:%s/%s/%s", client.Protocol, client.Host, client.Port, listPath, lastBackupId))
	if err != nil {
		return types.BackupInfo{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return types.BackupInfo{}, err
	}
	var backupInfo types.BackupInfo
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.Decode(&backupInfo)
	return backupInfo, nil
}

func (client *HttpBackupClient) ListBackupsByType(backupType string) (types.BackupIdsList, error) {
	listPath := "listbackups"
	if types.IncrementalBackup == backupType {
		listPath = "incremental/" + listPath
	}

	res, err := client.HttpClient.Get(fmt.Sprintf("%s://%s:%s/%s", client.Protocol, client.Host, client.Port, listPath))

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return nil, err
	}

	var backupIds types.BackupIdsList
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.Decode(&backupIds)

	return backupIds, nil
}
