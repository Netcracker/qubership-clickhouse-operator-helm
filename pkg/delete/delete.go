package delete

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/client"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/constants"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/helper"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/types"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/utils"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

type Delete struct {
	Helper     *helper.Helper
	Log        *zap.Logger
	RawDbs     string
	BackupPath string
	FullEvict  bool
}

func (delete *Delete) BackupId(hostname string) string {
	splitBackupPath := strings.Split(delete.BackupPath, "/")
	backupId := splitBackupPath[len(splitBackupPath)-1]
	remote, remoteStorageType := utils.RemoteStorage()
	if remote {
		if remoteStorageType == "s3" || utils.IsExternal(delete.BackupPath) {
			backupId = backupId + "_" + hostname
		}
	}
	return backupId
}

func (delete *Delete) GetExternalBackupPath() string {
	storageExternal := os.Getenv("STORAGE_EXTERNAL")
	if storageExternal != "" && strings.HasPrefix(delete.BackupPath, storageExternal) {
		resstr := strings.TrimPrefix(delete.BackupPath, storageExternal)
		splitBackupPath := strings.Split(resstr, "/")
		backupId := splitBackupPath[len(splitBackupPath)-1]
		return strings.TrimSuffix(resstr, backupId)
	}
	return ""
}

func (delete *Delete) PerformDelete() error {
	hosts, err := delete.Helper.GetClickhouseServices()
	if err != nil {
		return err
	}

	delete.Log.Info(fmt.Sprintf("list of services for delete request %s", hosts))

	for _, host := range hosts {

		if err := delete.deleteLocalBackupForHost(host); err != nil {
			delete.Log.Error("there is an error during delete of local backup: ", zap.Error(err))
			//return err
		}

		if err := delete.deleteRemoteBackupForHost(host); err != nil {
			delete.Log.Error("there is an error during delete of remote backup: ", zap.Error(err))
			//return err
		}
	}
	if err := delete.deleteLocalFiles(); err != nil {
		return err
	}

	if delete.FullEvict {
		delete.evictionLogic()
	}

	return nil
}

func (delete *Delete) evictionLogic() {
	protocol := "http"
	if utils.IsTlsEnabled() {
		protocol = "https"
	}
	backupClient := &client.HttpBackupClient{Host: "localhost", Port: utils.GetPort(), HttpClient: delete.Helper.HttpClient, Protocol: protocol}
	lastFullBackupId := ""
	isFound := false
	_ = wait.PollImmediate(5*time.Second, 5*time.Minute, func() (done bool, err error) { //todo
		lastFullBackupId, isFound = backupClient.GetLatestFullBackup()
		if !isFound {
			delete.Log.Info("last full backup is not found. Trying again...")
		}
		return isFound, nil
	})
	if !isFound {
		delete.Log.Info("last full backup is not found")
		return
	}
	backupTypes := []string{types.FullBackup, types.IncrementalBackup}
	for _, backupType := range backupTypes {
		backups, err := backupClient.ListBackupsByType(backupType)
		if err != nil {
			delete.Log.Error("error during listing full backups ", zap.Error(err))
		}
		lastFullTime, err := strconv.ParseInt(strings.Replace(lastFullBackupId, "T", "", -1), 10, 64)
		if err != nil {
			return
		}
		for _, backupName := range backups {
			backupTime, err := strconv.ParseInt(strings.Replace(backupName, "T", "", -1), 10, 64)
			if err != nil {
				delete.Log.Error(fmt.Sprintf("error during try to get timestamp for %s", backupName))
			}
			if backupTime < lastFullTime {
				delete.Log.Info(fmt.Sprintf("%s backup %s will be deleted", backupType, backupName))
				message, ok := backupClient.EvictBackups(backupName, backupType)
				if !ok {
					delete.Log.Error(fmt.Sprintf("cannot evict %s, message: %s", backupName, message), zap.Error(fmt.Errorf("cannot evict %s", backupName)))
				}
			}
		}
	}

}

func (delete *Delete) deleteLocalBackupForHost(hostname string) error {
	if !utils.IsS3Remote() && !utils.IsExternal(delete.BackupPath) {
		return nil
	}
	delete.Log.Info(fmt.Sprintf("Start to delete local backup: %s", delete.BackupId(hostname)))
	backupAction := fmt.Sprintf("backup/delete/local/%s", delete.BackupId(hostname))
	if err := utils.PostActionAndWait(delete.Helper.HttpClient, "post", hostname, constants.CHBackupPort, backupAction); err != nil {
		return err
	}
	return nil
}

func (delete *Delete) deleteRemoteBackupForHost(hostname string) error {

	if !utils.IsS3Remote() && !utils.IsnfsRemote() {
		return nil
	}
	delete.Log.Info(fmt.Sprintf("Start to delete remote backup: %s", delete.BackupId(hostname)))
	backupAction := fmt.Sprintf("backup/delete/remote/%s", delete.BackupId(hostname))
	port := constants.CHBackupPort
	if !utils.IsS3Remote() {
		port = constants.CHUploaderPort
		deletePath := delete.GetExternalBackupPath() + delete.BackupId(hostname)
		backupAction = fmt.Sprintf("delete?path=%s", deletePath)
	}
	if err := utils.PostActionAndWait(delete.Helper.HttpClient, "post", hostname, port, backupAction); err != nil {
		return err
	}
	return nil
}

func (delete *Delete) deleteLocalFiles() error {
	if err := os.RemoveAll(delete.BackupPath); err != nil {
		return err
	}
	return nil
}
