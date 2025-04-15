package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/client"
	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/scheduler"
	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/util"
	"go.uber.org/zap"
	k8sAuth "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	log             = util.GetLogger()
	fullBackupTries = 5
)

type siteManagerHandler struct {
	K8sWrapper util.K8sWrapper
}

func (smHandler *siteManagerHandler) processSiteManagerRequest(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case "GET":
		smStatus := smHandler.K8sWrapper.GetSmStatus()
		sendResponse(response, http.StatusOK, smStatus)
	case "POST":
		payload := smHandler.parseRequest(request)
		if smHandler.targetModeReachedForSm(payload) {
			log.Info("skipping change of SM mode, because we already in target mode")
			sendResponse(response, http.StatusOK, smHandler.K8sWrapper.GetSmStatus())
			return
		}
		smHandler.K8sWrapper.UpdateSMStatus(payload.Mode, "running")
		sendResponse(response, http.StatusOK, smHandler.K8sWrapper.GetSmStatus())
		go smHandler.changeSmMode(&payload)
	}
}

func (smHandler *siteManagerHandler) processPreConfigureRequest(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case "GET":
		preConfigureStatus := smHandler.K8sWrapper.GetPreConfigureStatus()
		sendResponse(response, http.StatusOK, preConfigureStatus)
	case "POST":
		payload := smHandler.parseRequest(request)
		if smHandler.targetModeReachedForPreConfigure(payload) {
			log.Info("skipping change of pre configure SM mode, because we already in target mode")
			sendResponse(response, http.StatusOK, smHandler.K8sWrapper.GetPreConfigureStatus())
			return
		}
		smHandler.K8sWrapper.UpdatePreConfigureStatus(payload.Mode, "running")
		sendResponse(response, http.StatusOK, smHandler.K8sWrapper.GetPreConfigureStatus())
		go smHandler.changePreConfigureMode(&payload)
	}

}

func (smHandler *siteManagerHandler) processHealthRequest(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case "GET":
		chStatus := smHandler.K8sWrapper.GetChStatus()
		sendResponse(response, http.StatusOK, chStatus)
	}
}

func (smHandler *siteManagerHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if !util.IsHttpAuthEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := strings.Split(r.Header.Get("Authorization"), "Bearer ")

		if len(authHeader) != 2 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		tokenReview := &k8sAuth.TokenReview{
			Spec: k8sAuth.TokenReviewSpec{
				Token: authHeader[1],
			},
		}

		smCustomAudience := util.GetEnv("NC_SM_CUSTOM_AUDIENCE", "")

		if smCustomAudience != "" {
			tokenReview.Spec.Audiences = []string{smCustomAudience}
		}

		reviewRes, err := smHandler.K8sWrapper.K8sClientSet.AuthenticationV1().TokenReviews().
			Create(context.TODO(), tokenReview, metav1.CreateOptions{})

		if err != nil {
			log.Error("There is an error during TokenReview Request", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
		}

		if authenticated(reviewRes) {
			next.ServeHTTP(w, r)
			return
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})
}

func authenticated(tokenReview *k8sAuth.TokenReview) bool {
	if !tokenReview.Status.Authenticated {
		return false
	}
	userName := tokenReview.Status.User.Username
	if util.GetSmAuthUserName() != userName {
		return false
	}
	return true
}

func (smHandler *siteManagerHandler) parseRequest(request *http.Request) SmPayload {
	var payload SmPayload
	err := json.NewDecoder(request.Body).Decode(&payload)
	if err != nil {
		return SmPayload{}
	}
	return payload
}

func (smHandler *siteManagerHandler) changePreConfigureMode(payload *SmPayload) {
	if payload.Active() {
		log.Info("pre-configure changed mode to active, no work for pre-configure")
		smHandler.K8sWrapper.UpdatePreConfigureStatus(payload.Mode, "done")
	} else if payload.Standby() {
		log.Info("pre-configure changed mode to standby")
		backupClient := client.GetDefaultBackupClient()
		hosts, err := smHandler.K8sWrapper.GetChHosts()
		if err != nil {
			return
		}

		// check if there is no action in progress
		for _, host := range hosts {
			if err := util.WaitTillActionCompletedForHost(host); err != nil {
				log.Error("there is an error during wait of completed actions in ch-backup", zap.Error(err))
			}
		}

		// reconfigure replicator
		log.Info("start to patch replicator")
		err = smHandler.K8sWrapper.PatchReplicatorDeploymentForMode(payload.Mode)
		if err != nil {
			log.Error("there is an error during pathing replicator", zap.Error(err))
			return
		}

		defer func() {
			if err != nil {
				log.Info("Returning replicator in the active state")
				err = smHandler.K8sWrapper.PatchReplicatorDeploymentForMode(util.ModeActive)
				if err != nil {
					log.Error("there is an error during pathing replicator", zap.Error(err))
					return
				}
			}
		}()

		if !payload.NoWait {
			log.Info("Requesting incremental backup")
			if err = backupClient.RequestIncrementalBackup(); err == nil {
				log.Info("pre-configure change mode successful")
			} else {
				log.Error("there is an error during requesting incremental backup", zap.Error(err))
				smHandler.K8sWrapper.UpdatePreConfigureStatus(payload.Mode, "failed")
				return
			}
		}

		smHandler.K8sWrapper.UpdatePreConfigureStatus(payload.Mode, "done")
	}
}

func (smHandler *siteManagerHandler) targetModeReachedForPreConfigure(target SmPayload) bool {
	return smHandler.targetModeReached(smHandler.K8sWrapper.GetPreConfigureStatus(), target)
}

func (smHandler *siteManagerHandler) targetModeReachedForSm(target SmPayload) bool {
	return smHandler.targetModeReached(smHandler.K8sWrapper.GetSmStatus(), target)
}

func (smHandler *siteManagerHandler) targetModeReached(current map[string]string, target SmPayload) bool {
	mode, status := current["mode"], current["status"]
	if status == "done" && mode == target.Mode {
		return true
	}
	return false
}

func (smHandler *siteManagerHandler) changeSmMode(payload *SmPayload) {
	if payload.Active() {
		lastRestore.start()
		// get latest incremental
		backupClient := client.GetDefaultBackupClient()
		downloader := scheduler.BackupsDownloader{BackupClient: backupClient, K8sWrapper: smHandler.K8sWrapper, DownloadAll: true}
		if err := downloader.Download(true); err != nil {
			errMsg := "there is an error during download of latest backup"
			log.Error(errMsg, zap.Error(err))
			lastRestore.setFailed(errMsg)
			smHandler.K8sWrapper.UpdateSMStatus(payload.Mode, "failed")
			return
		}
		// do the restore of latest
		if !payload.NoWait {
			if ok := backupClient.CheckLatestIncrementalBackup(); ok {
				log.Info("latest incremental backup found")
				if err := backupClient.RequestRestoreOfLatestIncremental(); err != nil {
					errMsg := "there is an error during restore of latest backup"
					log.Error(errMsg, zap.Error(err))
					lastRestore.setFailed(errMsg)
					smHandler.K8sWrapper.UpdateSMStatus(payload.Mode, "failed")
					return
				}
			}
		} else {
			log.Info("proceeding with incremental backups check")
			if ok := backupClient.CheckLatestIncrementalBackup(); ok {
				log.Info("latest incremental backup found")
				if err := backupClient.RequestRestoreOfLatestIncremental(); err != nil {
					errMsg := "there is an error during restore of latest incremental backup"
					log.Error(errMsg, zap.Error(err))
					lastRestore.setFailed(errMsg)
					smHandler.K8sWrapper.UpdateSMStatus(payload.Mode, "failed")
					return
				}
			} else {

				log.Info("proceeding with full backups check")
				if ok := backupClient.CheckLatestFullBackup(); ok {
					log.Info("latest Full backup found")
					if err := backupClient.RequestRestoreOfLatestFullBackup(); err != nil {
						errMsg := "there is an error during restore of latest Full backup"
						log.Error(errMsg, zap.Error(err))
						lastRestore.setFailed(errMsg)
						smHandler.K8sWrapper.UpdateSMStatus(payload.Mode, "failed")
						return
					}
				}
			}
		}

		// change service
		if err := smHandler.UpdateExternalServiceForActive(); err != nil {
			errMsg := "there is an error during update of external service"
			log.Error(errMsg, zap.Error(err))
			lastRestore.setFailed(errMsg)
			smHandler.K8sWrapper.UpdateSMStatus(payload.Mode, "failed")
			return
		}

		go func() {
			defer lastRestore.stop()
			log.Info("start async part of reconfiguration")

			hosts, _ := smHandler.K8sWrapper.GetChHosts()
			for _, host := range hosts {
				if err := util.WaitTillActionCompletedForHost(host); err != nil {
					log.Error("there is an error during wait of completed actions in ch-backup", zap.Error(err))
				}
			}
			// reconfigure replicator
			log.Info("start to patch replicator with scheduling for backups")
			if err := smHandler.K8sWrapper.PatchReplicatorDeploymentForMode(payload.Mode); err != nil {
				log.Error("there is an error during patching replicator", zap.Error(err))
			}

			// trigger full backup
			_, err := smHandler.doFullBackupWithRetries(backupClient)
			if err != nil {
				lastRestore.setFailed("error during performing full backup")
				smHandler.K8sWrapper.UpdateSMStatus(payload.Mode, "failed")
				return
			}
			log.Info("async part of reconfiguration has been completed successfully")
		}()
		log.Info("activation successful")
		smHandler.K8sWrapper.UpdateSMStatus(payload.Mode, "done")
	} else if payload.Standby() {
		// change service
		if err := smHandler.UpdateExternalServiceForStandby(); err != nil {
			log.Error("there is an error during update of external service", zap.Error(err))
			smHandler.K8sWrapper.UpdatePreConfigureStatus(payload.Mode, "failed")
		}
		smHandler.K8sWrapper.UpdateSMStatus(payload.Mode, "done")
	}
}

func (smHandler *siteManagerHandler) doFullBackupWithRetries(backupClient *client.HttpBackupClient) (string, error) {
	// trigger full backup
	hosts, _ := smHandler.K8sWrapper.GetChHosts()
	try := 0
	for {
		try++
		if backupId, err := backupClient.RequestFullBackup(); err != nil {
			log.Error("there is an error during requesting full backup", zap.Error(err))
			for _, host := range hosts {
				lastAction, err := util.GetLastActionStatusForHost(host)
				if err != nil {
					log.Error("there is an error during getting last action", zap.Error(err))
				}
				log.Info(fmt.Sprintf("last action info: %s for host: %s", host, lastAction))
			}
		} else {
			return backupId, nil
		}

		if fullBackupTries == try {
			return "", errors.New("not possible to request full backup with retries")
		}
		time.Sleep(10 * time.Second)
	}
}

func (smHandler *siteManagerHandler) UpdateExternalServiceForActive() error {
	extService := smHandler.K8sWrapper.GetExternalService()
	extService.Spec.ExternalName = fmt.Sprintf("%s.%s", "clickhouse-cluster", util.GetNameSpace())
	if err := smHandler.K8sWrapper.UpdateService(extService); err != nil {
		return err
	}
	return nil
}

func (smHandler *siteManagerHandler) UpdateExternalServiceForStandby() error {
	extService := smHandler.K8sWrapper.GetExternalService()
	extService.Spec.ExternalName = smHandler.K8sWrapper.GetOppositeChHost()
	if err := smHandler.K8sWrapper.UpdateService(extService); err != nil {
		return err
	}
	return nil
}

func Serve() error {
	k8sC, err := util.GetK8sClient()
	if err != nil {
		return err
	}
	k8sClientSet := util.GetKubeClientSet()

	k8sWrapper := util.K8sWrapper{K8sClient: k8sC, K8sClientSet: k8sClientSet}
	handler := siteManagerHandler{k8sWrapper}

	http.Handle("/sitemanager", handler.Middleware(http.HandlerFunc(handler.processSiteManagerRequest)))
	http.Handle("/health", handler.Middleware(http.HandlerFunc(handler.processHealthRequest)))
	http.Handle("/pre-configure", handler.Middleware(http.HandlerFunc(handler.processPreConfigureRequest)))
	http.Handle("/metrics", handler.Middleware(http.HandlerFunc(handler.Metrics)))

	if util.IsTlsEnabled() {
		err = http.ListenAndServeTLS(":8443", "/tls/tls.crt", "/tls/tls.key", nil)
	} else {
		err = http.ListenAndServe(":8080", nil)
	}
	if err != nil {
		return err
	}
	return nil
}

func sendResponse(w http.ResponseWriter, statusCode int, response interface{}) {
	w.WriteHeader(statusCode)
	responseBody, _ := json.Marshal(response)
	_, _ = w.Write(responseBody)
	w.Header().Set("Content-Type", "application/json")
}

func sendResponseStr(w http.ResponseWriter, value string) {
	_, err := w.Write([]byte(value))
	if err != nil {
		log.Error("error during writing response", zap.Error(err))
	}
}

type SmPayload struct {
	Mode   string `json:"mode"`
	Status string `json:"status"`
	NoWait bool   `json:"no-wait,omitempty"`
}

func (sm *SmPayload) Active() bool {
	return sm.Mode == util.ModeActive
}

func (sm *SmPayload) Standby() bool {
	return sm.Mode == util.ModeStandby
}
