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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/client"
	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/util"
	"github.com/go-co-op/gocron"
	"go.uber.org/zap"
)

var s = gocron.NewScheduler(time.UTC)

func ScheduleBackupsDownload() error {
	logger.Info("starting scheduler")

	k8sC, err := util.GetK8sClient()
	if err != nil {
		return err
	}

	downloader := BackupsDownloader{
		BackupClient: client.GetDefaultBackupClient(), //TODO
		K8sWrapper:   util.K8sWrapper{K8sClient: k8sC},
		DownloadAll:  false,
	}

	cronExpr := downloader.getCronExpr()
	logger.Info(fmt.Sprintf("cronExpr %s", cronExpr))

	job, err := s.Cron(cronExpr).Do(downloader.Download, false)

	if err != nil {
		logger.Error("error during scheduling cron job", zap.Error(err))
		return err
	}
	s.StartAsync()
	logger.Info(fmt.Sprintf("next run at %s", job.NextRun().String()))

	return nil
}

func (downloader *BackupsDownloader) Download(ignoreActive bool) error {
	logger.Info("downloadBackups invoked")
	if downloader.K8sWrapper.IsActive() && !ignoreActive {
		logger.Info("mode is set to active, skipping")
		return errors.New("mode is set to active, skipping")
	}
	// get local backups from hosts
	mapOfLocalBackups, err := downloader.GetListOfLocalChBackups()
	logger.Info(fmt.Sprintf("list of local backups: %s", mapOfLocalBackups))
	if err != nil {
		logger.Error("there is an error during list of local ch backups", zap.Error(err))
		return err
	}

	remoteBackups, err := downloader.BackupClient.GetRemoteBackups()
	if err != nil {
		logger.Error("there is an error during list of backups in S3", zap.Error(err))
		return err
	}
	logger.Info(fmt.Sprintf("list of remoteBackups %s", remoteBackups))

	missingBackups := downloader.getMissingLocalBackups(mapOfLocalBackups, remoteBackups)

	logger.Info(fmt.Sprintf("map of missing local backups %s", missingBackups))

	if err = downloader.downloadMissingBackups(missingBackups); err != nil {
		logger.Error("there is an error during backup download: ", zap.Error(err))
		return err
	}

	go func() {
		// clean up old backups
		logger.Info("start to delete outdated backups")
		backupId, errBack := downloader.BackupClient.GetLatestFullBackupId()
		if errBack != nil {
			logger.Error("Unable to obtain last full backup Id")
			return
		}
		logger.Info(fmt.Sprintf("latest full backup Id %s", backupId))

		if err := downloader.DeleteBackupsOlder(backupId, mapOfLocalBackups); err != nil {
			logger.Error("there is an error during deleting old backups", zap.Error(err))
		}
	}()

	return nil
}

func (downloader *BackupsDownloader) DeleteBackupsOlder(lastFullBackupId string, backups map[string][]string) error {
	timesTemp, err := strconv.ParseInt(strings.Replace(lastFullBackupId, "T", "", -1), 10, 64)
	logger.Info(fmt.Sprintf("list of backups to delete %s, but no later than %s", backups, lastFullBackupId))
	if err != nil {
		return err
	}

	for host, backupIds := range backups {
		for _, backupId := range backupIds {
			// curl -XPOST localhost:8080/evict/<backupid>
			backupTT, errConv := strconv.ParseInt(strings.Replace(backupId, "T", "", -1), 10, 64)
			if errConv != nil {
				logger.Error(fmt.Sprintf("Unable to convert backup ID to timestamp. %s", backupId), zap.Error(err))
				continue
			} else if backupTT >= timesTemp {
				logger.Debug(fmt.Sprintf("Backup %s will not be deleted", backupId))
				continue
			}
			err := downloader.deleteLocalBackup(host, backupId)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (downloader *BackupsDownloader) deleteLocalBackup(host, backupId string) error {
	backupName := backupId + "_" + host
	logger.Info(fmt.Sprintf("Start to delete local backup: %s", backupName))
	backupAction := fmt.Sprintf("backup/delete/local/%s", backupName)
	url := fmt.Sprintf("%s://%s:%s/%s", util.GetProtocol(), host, "7171", backupAction)
	res, err := http.Post(url, "text/plain", nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			logger.Error("there is an error during body read", zap.Error(err))
		}
		logger.Warn(fmt.Sprintf("failed to do the actions: %s", string(bodyBytes)))
	}

	if err = util.WaitTillActionCompletedForHost(host); err != nil {
		return err
	}

	return nil
}

func (downloader *BackupsDownloader) GetListOfLocalChBackups() (map[string][]string, error) {
	hosts, err := downloader.K8sWrapper.GetChHosts()
	if err != nil {
		return nil, err
	}
	backupsMap := make(map[string][]string, 0)
	for _, host := range hosts {
		backupsForHost, err := getLocalBackupsForHost(host)
		if err != nil {
			return nil, err
		}
		backupsMap[host] = backupsForHost
	}

	return backupsMap, nil
}

func getLocalBackupsForHost(host string) ([]string, error) {
	var backupsList []string
	res, err := http.Get(fmt.Sprintf("%s://%s:7171/backup/list/local", util.GetProtocol(), host))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var chBackup ChBackup
	dec := json.NewDecoder(bytes.NewReader(body))
	for {
		err := dec.Decode(&chBackup)
		if err == io.EOF {
			//all done
			break
		}
		if err != nil {
			//log.Fatal("There is an error, during Actions Read", zap.Error(err))
			return nil, err
		}
		splittedBackupName := strings.Split(chBackup.Name, "_")
		backupsList = append(backupsList, splittedBackupName[0])
	}
	return backupsList, nil
}

func (downloader *BackupsDownloader) getCronExpr() string {
	return util.GetEnv(cronEnv, defaultCronExpression)
}

func (downloader *BackupsDownloader) getMissingLocalBackups(localBackupsMap map[string][]string, remoteBackups []string) map[string][]string {
	missingBackups := make(map[string][]string)
	for host, localBackups := range localBackupsMap {
		mb := make(map[string]struct{}, len(localBackups))
		for _, x := range localBackups {
			mb[x] = struct{}{}
		}
		for _, x := range remoteBackups {
			if _, found := mb[x]; !found {
				missingBackups[host] = append(missingBackups[host], x)
			}
		}
	}

	return missingBackups
}

func (downloader *BackupsDownloader) downloadMissingBackups(missingBackupsMap map[string][]string) error {
	hosts, err := downloader.K8sWrapper.GetChHosts()
	logger.Info(fmt.Sprintf("list of hosts: %s", hosts))
	logger.Info(fmt.Sprintf("map of missing backups: %s", missingBackupsMap))
	if err != nil {
		return err
	}
	for host, missingBackups := range missingBackupsMap {
		if len(missingBackups) == 0 {
			continue
		}
		if !downloader.DownloadAll {
			missingBackups = []string{missingBackups[0]}
		}
		for _, backupId := range missingBackups {
			logger.Info(fmt.Sprintf("starting to download %s backups", backupId))
			downloadBackupAction := fmt.Sprintf("backup/download/%s", backupId+"_"+host)
			url := fmt.Sprintf("%s://%s:7171/%s", util.GetProtocol(), host, downloadBackupAction)
			logger.Info("will post action: " + url)
			res, err := http.Post(url, "text/plain", nil)
			if err != nil {
				return err
			}
			defer res.Body.Close()
			if res.StatusCode != http.StatusOK {
				bodyBytes, err := io.ReadAll(res.Body)
				if err != nil {
					logger.Error("there is an error during body read", zap.Error(err))
				}
				return errors.New(fmt.Sprintf("failed to do the actions: %s", string(bodyBytes)))
			}

			if err = res.Body.Close(); err != nil {
				return err
			}

			if err = util.WaitTillActionCompletedForHost(host); err != nil {
				return err
			}
		}
	}
	return nil
}
