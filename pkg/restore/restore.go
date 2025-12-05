package restore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/constants"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/driver"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/helper"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/utils"
	"go.uber.org/zap"
)

type Restore struct {
	Helper     *helper.Helper
	Log        *zap.Logger
	RawDbs     string
	BackupPath string
	DbMap      string
	DropSrcDb  string
	ResetCache bool
}

func (restore *Restore) BackupId(hostname string) string {
	splitBackupPath := strings.Split(restore.BackupPath, "/")
	backupId := splitBackupPath[len(splitBackupPath)-1]
	remote, remoteStorageType := utils.RemoteStorage()
	if remote {
		if remoteStorageType == "s3" || utils.IsExternal(restore.BackupPath) {
			backupId = backupId + "_" + hostname
		}
	}
	return backupId
}

func (restore *Restore) GetExternalBackupPath() string {
	storageExternal := os.Getenv("STORAGE_EXTERNAL")
	if storageExternal != "" && strings.HasPrefix(restore.BackupPath, storageExternal) {
		resstr := strings.TrimPrefix(restore.BackupPath, storageExternal)
		splitBackupPath := strings.Split(resstr, "/")
		backupId := splitBackupPath[len(splitBackupPath)-1]
		return strings.TrimSuffix(resstr, backupId)
	}
	return ""
}

func (restore *Restore) PerformRestore() error {
	if restore.isBackupFailed() {
		return errors.New("not able to restore failed backup, exiting")
	}

	hosts, err := restore.Helper.GetClickhouseServices()
	if err != nil {
		return err
	}

	dbs, dbMapStr, dbList, willBeRestored, err := restore.decideWhatToRestore()
	if err != nil {
		return err
	}

	// Turn off external service before run restore procedure
	service, err := restore.Helper.GetClickhouseClusterService()
	if err != nil {
		return err
	}
	emptyMap := make(map[string]string)
	service.Spec.Selector = emptyMap
	if err := restore.Helper.UpdateClickhouseClusterService(service); err != nil {
		return err
	}

	host := hosts[0]
	if len(dbMapStr) > 0 && strings.ToLower(restore.DropSrcDb) == "true" {
		restore.Log.Info(fmt.Sprintf("Databases %s will be deleted!", dbList))
		driver.DropDatabases(hosts, dbList)
	}
	if err := restore.downloadBackupFromRemoteStorage(host); err != nil {
		return err
	}

	if err := restore.restoreSchema(host, dbs, dbMapStr); err != nil {
		return err
	}
	restore.Log.Info("data is not restored yet, trying to restore")
	if err := restore.restoreData(host, dbs, dbMapStr); err != nil {
		return err
	}
	for _, chHost := range hosts {
		restore.Log.Info(fmt.Sprintf("data is restored, waiting till there is no q on: %s", chHost))
		if err := restore.waitForZeroQueue(chHost, willBeRestored); err != nil {
			return err
		}
		if restore.ResetCache {
			_ = driver.DropMarkCache(chHost)
		}
	}
	if err := restore.deleteLocalBackup(host); err != nil {
		return err
	}
	service, err = restore.Helper.GetClickhouseClusterService()
	if err != nil {
		return err
	}
	service.Spec.Selector = restore.Helper.GetClickhouseClusterServiceSelectors()
	if err := restore.Helper.UpdateClickhouseClusterService(service); err != nil {
		return err
	}
	return nil
}

func (restore *Restore) decideWhatToRestore() (string, string, []string, []string, error) {
	var databaseList []string
	var willBeRestored []string
	var dbMap map[string]string
	var dbMapStr string
	var err error
	//restore.Log.Info(fmt.Sprintf("input list of dbs: %s", restore.RawDbs))
	if len(restore.RawDbs) == 0 && len(restore.DbMap) == 0 {
		restore.Log.Info("input list of dbs is empty, restoring all that included to backup")
		databaseList, err = utils.GetDbsFromFile(restore.BackupPath)
		willBeRestored = databaseList
		if err != nil {
			return "", "", nil, nil, err
		}
	} else if len(restore.DbMap) > 0 {
		if err := json.Unmarshal([]byte(restore.DbMap), &dbMap); err != nil {
			return "", "", nil, nil, err
		}
		itr := 0
		for key, value := range dbMap {
			databaseList = append(databaseList, key)
			willBeRestored = append(willBeRestored, value)
			if itr == 0 {
				dbMapStr = dbMapStr + fmt.Sprintf("%s:%s", key, value)
			} else {
				dbMapStr = dbMapStr + fmt.Sprintf(",%s:%s", key, value)
			}
			itr++
		}
		dbMapStr = "restore_database_mapping=" + dbMapStr
	} else {
		restore.Log.Info("input list of dbs is not empty")
		if err := json.Unmarshal([]byte(restore.RawDbs), &databaseList); err != nil {
			return "", "", nil, nil, err
		}
		willBeRestored = databaseList
	}
	dbs := utils.CovertDbsToTables(databaseList)
	dbs = fmt.Sprintf("table=%s", dbs)
	return dbs, dbMapStr, databaseList, willBeRestored, nil
}

func (restore *Restore) isBackupFailed() bool {
	return utils.IsBackupFailedFileExist(restore.BackupPath)
}

func (restore *Restore) downloadBackupFromRemoteStorage(hostname string) error {
	if !utils.IsS3Remote() && !utils.IsnfsRemote() {
		return nil
	}
	if utils.IsS3Remote() {
		downloadBackupAction := fmt.Sprintf("backup/download/%s", restore.BackupId(hostname))
		port := constants.CHBackupPort
		if err := utils.PostActionAndWait(restore.Helper.HttpClient, "post", hostname, port, downloadBackupAction); err != nil {
			return err
		}
	} else if utils.IsExternal(restore.BackupPath) {
		remoteBackup := restore.GetExternalBackupPath() + restore.BackupId(hostname)
		downloadBackupAction := fmt.Sprintf("nfssync?backupid=%s&remoteBackup=%s", restore.BackupId(hostname), remoteBackup)
		port := constants.CHUploaderPort
		if err := utils.PostActionAndWait(restore.Helper.HttpClient, "get", hostname, port, downloadBackupAction); err != nil {
			return err
		}
	}
	return nil
}

func (restore *Restore) restoreSchema(hostname string, dbs string, dbMapStr string) error {
	restore.Log.Info(fmt.Sprintf("Start to restore backup: %s scheme on %s", restore.BackupId(hostname), hostname))
	if len(dbMapStr) > 0 {
		restore.Log.Info(fmt.Sprintf("Restore database mapping: %s", dbMapStr))
	}
	restoreSchemaAction := fmt.Sprintf("backup/restore/%s?schema=true&rm=true&%s&%s", restore.BackupId(hostname), dbs, dbMapStr)
	if err := utils.PostActionAndWait(restore.Helper.HttpClient, "post", hostname, constants.CHBackupPort, restoreSchemaAction); err != nil {
		return err
	}
	return nil
}

func (restore *Restore) restoreData(hostname string, dbs string, dbMapStr string) error {
	restore.Log.Info(fmt.Sprintf("Start to restore backup: %s data on %s", restore.BackupId(hostname), hostname))
	if len(dbMapStr) > 0 {
		restore.Log.Info(fmt.Sprintf("Restore database mapping: %s", dbMapStr))
	}
	restoreDataAction := fmt.Sprintf("backup/restore/%s?data=true&%s&%s", restore.BackupId(hostname), dbs, dbMapStr)
	if err := utils.PostActionAndWait(restore.Helper.HttpClient, "post", hostname, constants.CHBackupPort, restoreDataAction); err != nil {
		return err
	}
	return nil
}

func (restore *Restore) deleteLocalBackup(hostname string) error {

	if !utils.IsS3Remote() && !utils.IsnfsRemote() {
		return nil
	}
	port := constants.CHBackupPort
	deleteBackupAction := fmt.Sprintf("backup/delete/local/%s", restore.BackupId(hostname))
	if err := utils.PostActionAndWait(restore.Helper.HttpClient, "post", hostname, port, deleteBackupAction); err != nil {
		return err
	}
	return nil
}

func (restore *Restore) waitForZeroQueue(host string, dbList []string) error {
	for _, db := range dbList {
		zeroQ := false
		for !zeroQ {
			qSize, err := driver.GetQueueSizeForHostAndDb(host, db)
			if err != nil {
				return err
			}
			if qSize != "0" {
				restore.Log.Info(fmt.Sprintf("There is a queue: %s for db: %s and host: %s, waiting ..", qSize, db, host))
				time.Sleep(10 * time.Second)
			} else {
				zeroQ = true
			}
		}
	}
	return nil
}
