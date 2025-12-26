package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/constants"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/driver"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/helper"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/utils"
	"go.uber.org/zap"
)

type Backup struct {
	Helper     *helper.Helper
	Log        *zap.Logger
	RawDbs     string
	BackupPath string
}

type DbList struct {
	Log        *zap.Logger
	BackupPath string
}

func (backup *Backup) BackupId(hostname string) string {
	splitBackupPath := strings.Split(backup.BackupPath, "/")
	backupId := splitBackupPath[len(splitBackupPath)-1]
	remote, remoteStorageType := utils.RemoteStorage()
	if remote {
		if remoteStorageType == "s3" || utils.IsExternal(backup.BackupPath) {
			backupId = backupId + "_" + hostname
		}
	}
	return backupId
}

func (backup *Backup) GetExternalBackupPath() string {
	storageExternal := os.Getenv("STORAGE_EXTERNAL")
	if storageExternal != "" && strings.HasPrefix(backup.BackupPath, storageExternal) {
		resstr := strings.TrimPrefix(backup.BackupPath, storageExternal)
		splitBackupPath := strings.Split(resstr, "/")
		backupId := splitBackupPath[len(splitBackupPath)-1]
		return strings.TrimSuffix(resstr, backupId)
	}
	return ""
}

func (backup *Backup) PerformBackup() error {
	hosts, err := backup.Helper.GetClickhouseServices()
	if err != nil {
		return err
	}

	backup.Log.Info(fmt.Sprintf("list of services for backup request %s", hosts))

	tables, err := backup.decideWhatToBackup(hosts)
	if err != nil {
		return err
	}

	for _, host := range hosts {

		if err := backup.createBackupForHost(host, tables); err != nil {
			return err
		}
		if err = backup.uploadBackupToRemoteStorage(host); err != nil {
			return err
		}
	}
	return nil
}

func (backup *Backup) decideWhatToBackup(chServices []string) (string, error) {
	var databaseList []string
	var err error
	backup.Log.Info(fmt.Sprintf("input list of dbs: %s", backup.RawDbs))

	if len(backup.RawDbs) == 0 {
		backup.Log.Info("input list of dbs is empty, collecting it from db")
		if databaseList, err = driver.GetDatabaseList(chServices); err != nil {
			return "", err
		}
	} else {
		backup.Log.Info("input list of dbs is not empty")
		if err := json.Unmarshal([]byte(backup.RawDbs), &databaseList); err != nil {
			return "", err
		}
	}

	databasesAsString := utils.CovertDbsToTables(databaseList)

	if err := utils.WriteActionInfo(databaseList, "databases", backup.BackupPath); err != nil {
		return "", err
	}
	backup.Log.Info(fmt.Sprintf("list of dbs set as: %s", databasesAsString))
	return fmt.Sprintf("table=%s", databasesAsString), nil
}

func (backup *Backup) createBackupForHost(hostname string, tables string) error {
	backup.Log.Info(fmt.Sprintf("Start to backup: %s scheme on %s", backup.BackupId(hostname), tables))
	backupAction := fmt.Sprintf("backup/create?name=%s&%s", backup.BackupId(hostname), tables)
	port := constants.CHBackupPort
	if err := utils.PostActionAndWait(backup.Helper.HttpClient, "post", hostname, port, backupAction); err != nil {
		return err
	}
	return nil
}

func (backup *Backup) uploadBackupToRemoteStorage(hostname string) error {

	if !utils.IsS3Remote() && !utils.IsnfsRemote() {
		return nil
	}

	if err := backup.uploadBackupForHost(hostname); err != nil {
		return err
	}

	if utils.KeepLocalBackups() {
		return nil
	}

	if err := backup.deleteLocalBackupForHost(hostname); err != nil {
		return err
	}

	return nil
}

func (backup *Backup) uploadBackupForHost(hostname string) error {
	//_, remoteType := utils.RemoteStorage()
	backup.Log.Info(fmt.Sprintf("Start to upload backup: %s to remote host", backup.BackupId(hostname)))
	backupAction := fmt.Sprintf("backup/upload/%s", backup.BackupId(hostname))
	port := constants.CHBackupPort
	if utils.IsExternal(backup.BackupPath) {
		remoteBackup := backup.GetExternalBackupPath() + backup.BackupId(hostname)
		backupAction = fmt.Sprintf("nfssync?backupid=%s&remoteBackup=%s", backup.BackupId(hostname), remoteBackup)
		port = constants.CHUploaderPort
	}
	if err := utils.PostActionAndWait(backup.Helper.HttpClient, "post", hostname, port, backupAction); err != nil {
		return err
	}
	return nil
}

func (backup *Backup) deleteLocalBackupForHost(hostname string) error {
	backup.Log.Info(fmt.Sprintf("Start to delete local backup: %s", backup.BackupId(hostname)))
	backupAction := fmt.Sprintf("backup/delete/local/%s", backup.BackupId(hostname))
	if err := utils.PostActionAndWait(backup.Helper.HttpClient, "post", hostname, constants.CHBackupPort, backupAction); err != nil {
		return err
	}
	return nil
}

func (backup *Backup) uploadIncrementalBackupForHost(hostname string, fromBackupId string) error {
	backup.Log.Info(fmt.Sprintf("Start to upload backup: %s to remote host with base backup as: %s", backup.BackupId(hostname), fromBackupId+"_"+hostname))

	backupAction := fmt.Sprintf("backup/upload/%s?diff-from-remote=%s", backup.BackupId(hostname), fromBackupId+"_"+hostname)
	if err := utils.PostActionAndWait(backup.Helper.HttpClient, "post", hostname, constants.CHBackupPort, backupAction); err != nil {
		return err
	}
	return nil
}

func (dbList *DbList) GetDbList() error {
	var dbsFromFile []string
	var err error
	if dbsFromFile, err = utils.GetDbsFromFile(dbList.BackupPath); err != nil {
		return err
	}
	// this is coz we mimic ls -la output for backup-daemon logic
	justString := strings.Join(dbsFromFile, "\n")
	fmt.Println(justString + "\n")
	return err
}
