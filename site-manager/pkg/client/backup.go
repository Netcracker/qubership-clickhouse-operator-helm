package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/util"
	"go.uber.org/zap"
	"io"
	"k8s.io/apimachinery/pkg/util/wait"
	"net/http"
	"time"
)

func GetDefaultBackupClient() *HttpBackupClient {
	return &HttpBackupClient{Protocol: util.GetProtocol(), Host: chBackupHost, Port: GetReplicatorPort()}
}

func GetReplicatorPort() string {
	if util.IsTlsEnabled() {
		return chBackupTlsPort
	}
	return chBackupPort
}

func (client *HttpBackupClient) GetRemoteBackups() ([]string, error) {
	fullBackups, err := client.listBackupsByType(FullBackup)
	if err != nil {
		return nil, err
	}
	logger.Debug(fmt.Sprintf("list of full backups: %s", fullBackups))
	successfulFullBackups := client.filterSuccessfulBackups(fullBackups, FullBackup)
	logger.Info(fmt.Sprintf("list of successfulFullBackups: %s", successfulFullBackups))

	incrementalBackups, err := client.listBackupsByType(IncrementalBackup)
	if err != nil {
		return nil, err
	}
	logger.Debug(fmt.Sprintf("list of full backups: %s", incrementalBackups))
	successfulIncrementalBackups := client.filterSuccessfulBackups(incrementalBackups, IncrementalBackup)
	logger.Info(fmt.Sprintf("list of successfulIncrementalBackups: %s", successfulIncrementalBackups))

	return append(successfulFullBackups, successfulIncrementalBackups...), nil
}

func (client *HttpBackupClient) listBackupsByType(backupType string) (BackupIdsList, error) {
	listPath := "listbackups"
	if IncrementalBackup == backupType {
		listPath = "incremental/" + listPath
	}

	res, err := http.Get(fmt.Sprintf("%s://%s:%s/%s", client.Protocol, client.Host, client.Port, listPath))

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return nil, err
	}

	var backupIds BackupIdsList
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.Decode(&backupIds)

	return backupIds, nil
}

func (client *HttpBackupClient) getInfoForBackupByType(backupId string, backupType string) (BackupInfo, error) {
	listPath := "listbackups"
	if IncrementalBackup == backupType {
		listPath = "incremental/" + listPath
	}

	res, err := http.Get(fmt.Sprintf("%s://%s:%s/%s/%s", client.Protocol, client.Host, client.Port, listPath, backupId))
	if err != nil {
		return BackupInfo{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return BackupInfo{}, err
	}
	var backupInfo BackupInfo
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.Decode(&backupInfo)
	return backupInfo, nil
}

func (client *HttpBackupClient) filterSuccessfulBackups(backups BackupIdsList, backupType string) (ret []string) {
	for _, backupId := range backups {
		backupInfo, err := client.getInfoForBackupByType(backupId, backupType)
		if err != nil {
			logger.Error("error during backup info request", zap.Error(err))
			continue
		}
		if backupInfo.Valid {
			ret = append(ret, backupId)
		}
	}
	return ret
}

func (client *HttpBackupClient) GetLatestFullBackupId() (backupId string, err error) {
	incrementalBackups, err := client.listBackupsByType(FullBackup)
	if err != nil {
		return "", err
	}
	successfulFullBackups := client.filterSuccessfulBackups(incrementalBackups, FullBackup)
	if len(successfulFullBackups) == 0 {
		return "", fmt.Errorf("there is no sucessfull full backup")
	}
	return successfulFullBackups[len(successfulFullBackups)-1], err
}

func (client *HttpBackupClient) RequestIncrementalBackup() error {
	backupId, err := client.requestBackup(IncrementalBackup)
	if err != nil {
		return nil
	}
	logger.Info(fmt.Sprintf("incremental backupId: %s", backupId))
	if err = client.waitForIncrementalBackupCompleted(backupId); err != nil {
		return err
	}
	return nil
}

func (client *HttpBackupClient) requestBackup(backupType string) (string, error) {
	backupPath := "backup"
	if IncrementalBackup == backupType {
		backupPath = "incremental/" + backupPath
	}
	res, err := http.Post(fmt.Sprintf("%s://%s:%s/%s", client.Protocol, client.Host, client.Port, backupPath), "text/plain", nil)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (client *HttpBackupClient) waitForIncrementalBackupCompleted(backupId string) error {
	return client.waitForBackupCompleted(backupId, IncrementalBackup)
}

func (client *HttpBackupClient) waitForFullBackupCompleted(backupId string) error {
	return client.waitForBackupCompleted(backupId, FullBackup)
}

func (client *HttpBackupClient) waitForBackupCompleted(backupId string, backupType string) error {
	logger.Info(fmt.Sprintf("wait for %s backup to complete: %s", backupType, backupId))

	err := wait.PollImmediate(5*time.Second, 15*time.Minute, func() (done bool, err error) {
		backupInfo, err := client.getInfoForBackupByType(backupId, backupType)
		if err != nil {
			return false, err
		}
		if backupInfo.Locked {
			logger.Info(fmt.Sprintf("%s backup is in progress, retrying ...", backupType))
			return false, nil
		} else if backupInfo.Failed {
			logger.Error(fmt.Sprintf("%s backup failed", backupType))
			return false, errors.New("backup is failed")
		} else if backupInfo.Valid {
			return true, nil
		}
		return false, nil
	})
	return err
}

func (client *HttpBackupClient) getLatestIncrementalBackupId() (backupId string, err error) {
	incrementalBackups, err := client.listBackupsByType(IncrementalBackup)
	if err != nil {
		return "", err
	}
	successfulIncrementalBackups := client.filterSuccessfulBackups(incrementalBackups, IncrementalBackup)
	if len(successfulIncrementalBackups) == 0 {
		return "", fmt.Errorf("there is no sucessfull incremental backups")
	}
	return successfulIncrementalBackups[len(successfulIncrementalBackups)-1], err
}

func (client *HttpBackupClient) RequestFullBackup() (string, error) {
	backupId, err := client.requestBackup(FullBackup)
	logger.Info(fmt.Sprintf("full backupId during activation of standby %s", backupId))
	if err != nil {
		return "", err
	}
	if err = client.waitForFullBackupCompleted(backupId); err != nil {
		return "", err
	} else {
		info, err := client.getInfoForBackupByType(backupId, FullBackup)
		if err != nil {
			return "", err
		}
		logger.Warn(fmt.Sprintf("info about full backup: %v", info))

	}
	return backupId, nil
}

// curl -XGET localhost:8080/incremental/jobstatus/<task_id>
func (client *HttpBackupClient) waitTillSuccessfulRestore(trackingId string, backupType string) error {
	StatusPath := "jobstatus"
	if IncrementalBackup == backupType {
		StatusPath = "incremental/" + StatusPath
	}
	url := fmt.Sprintf("%s://%s:%s/%s/%s", client.Protocol, client.Host, client.Port, StatusPath, trackingId)
	err := wait.PollImmediate(5*time.Second, 15*time.Minute, func() (done bool, err error) {
		res, err := http.Get(url)
		if err != nil {
			logger.Error(fmt.Sprintf("cannot connect to %s", url))
			return false, nil
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		var restoreInfo RestoreInfo
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.Decode(&restoreInfo)
		logger.Info(fmt.Sprintf("Restore is in progress, current status: %s", restoreInfo.Status))
		switch restoreInfo.Status {
		case "Successful":
			return true, nil
		case "Processing":
			return false, nil
		case "Failed":
			return false, errors.New(restoreInfo.Error)
		case "Queued":
			return false, nil
		}

		return false, nil
	})
	return err
}

// curl -XPOST -H "Content-Type: application/json" -d  '{"vault":"20170913T1114"}' localhost:8080/incremental/restore
func (client *HttpBackupClient) requestIncrementalRestore(backupId string) (trackingId string, err error) {
	url := fmt.Sprintf("%s://%s:%s/%s", client.Protocol, client.Host, client.Port, "incremental/restore")
	jsonBody := map[string]string{"vault": backupId}
	jsonValue, _ := json.Marshal(jsonBody)
	res, err := http.Post(url, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return "", err
		}
		return string(bodyBytes), nil
	}

	return "", nil
}

func (client *HttpBackupClient) RequestRestoreOfLatestIncremental() error {
	latestBackupId, err := client.getLatestIncrementalBackupId()
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("latest backupId to restore from: %s", latestBackupId))
	trackingId, err := client.requestIncrementalRestore(latestBackupId)
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("incremental restore tracking id: %s", trackingId))
	if err = client.waitTillSuccessfulRestore(trackingId, IncrementalBackup); err != nil {
		return err
	}
	return nil
}

func (client *HttpBackupClient) RequestRestoreOfLatestFullBackup() error {
	latestBackupId, err := client.GetLatestFullBackupId()
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("latest backupId to restore from: %s", latestBackupId))
	trackingId, err := client.requestFullRestore(latestBackupId)
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf("Full restore tracking id: %s", trackingId))
	if err = client.waitTillSuccessfulRestore(trackingId, FullBackup); err != nil {
		return err
	}
	return nil
}

func (client *HttpBackupClient) requestFullRestore(backupId string) (trackingId string, err error) {
	url := fmt.Sprintf("%s://%s:%s/%s", client.Protocol, client.Host, client.Port, "/restore")
	jsonBody := map[string]string{"vault": backupId}
	jsonValue, _ := json.Marshal(jsonBody)
	res, err := http.Post(url, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return "", err
		}
		return string(bodyBytes), nil
	}

	return "", nil
}

func (client *HttpBackupClient) CheckLatestIncrementalBackup() bool {
	return client.checkLatestBackupByType(IncrementalBackup)
}

func (client *HttpBackupClient) CheckLatestFullBackup() bool {
	return client.checkLatestBackupByType(FullBackup)
}

func (client *HttpBackupClient) checkLatestBackupByType(backupType string) bool {
	listOfBackups, err := client.listBackupsByType(backupType)
	logger.Info(fmt.Sprintf("listOfBackups : %s", listOfBackups))
	if err != nil {
		return false
	}
	successfulIncrementalBackups := client.filterSuccessfulBackups(listOfBackups, backupType)
	if len(successfulIncrementalBackups) == 0 {
		logger.Info(fmt.Sprintf("there is no sucessfull %s backups", backupType))
		return false
	}
	return true
}
