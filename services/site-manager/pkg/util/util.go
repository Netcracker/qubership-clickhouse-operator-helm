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

package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"strconv"
	"strings"
	"time"
)

const tlsEnv = "TLS_ENABLED"

var (
	log          = GetLogger()
	isTlsEnabled = GetEnvBool(tlsEnv, true)
)

const (
	SiteManagerCmName  = "site-manager-status"
	PreConfigureCmName = "pre-configure-site-manager-status"
	ChLabels           = "clickhouse.altinity.com/cluster=replicated"
	ModeStandby        = "standby"
	ModeActive         = "active"
	ReplicatorName     = "clickhouse-replicator"
	OrchestratorName   = "clickhouse-backup-orchestrator"
)

func GetEnv(key, def string) string {
	v := os.Getenv(key)
	if len(v) == 0 {
		return def
	}
	return v
}

func GetEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if len(v) == 0 {
		return def
	}
	result, err := strconv.ParseBool(v)
	if err != nil {
		log.Error(fmt.Sprintf("cannot parse bool value for %s", key))
		panic(err)
	}
	return result
}

func GetLogger() *zap.Logger {
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	))
	defer logger.Sync()
	return logger
}

func GetK8sClient() (client.Client, error) {
	crclient, err := createClient()
	if err != nil {
		return nil, err
	}
	return crclient, nil
}

func createClient() (client.Client, error) {
	clientConfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	crclient, err := client.New(clientConfig, client.Options{})
	if err != nil {
		return nil, err
	}
	return crclient, nil
}

func GetNameSpace() string {
	return os.Getenv("WATCH_NAMESPACE")
}

func getClusterName() string {
	return os.Getenv("CLUSTER")
}

func IsHttpAuthEnabled() bool {
	return strings.ToLower(GetEnv("NC_SM_HTTP_AUTH", "false")) == "true"
}

func GetSmAuthUserName() string {
	smNs := os.Getenv("NC_SM_NAMESPACE")
	smSaName := os.Getenv("NC_SM_AUTH_SA")
	return fmt.Sprintf("system:serviceaccount:%s:%s", smNs, smSaName)
}

func GetKubeClientSet() kubernetes.Clientset {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		//panic(err)
	}
	K8sClientSet, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		//panic(err)
	}
	return *K8sClientSet
}

func WaitTillActionCompletedForHost(host string) error {
	timeout := time.Duration(getTimeOut())
	err := wait.PollImmediate(5*time.Second, timeout*time.Minute, func() (done bool, err error) {
		var actionRow ActionRow
		log.Info("waiting until action will success")
		if actionRow, err = GetLastActionStatusForHost(host); err != nil {
			log.Error("Error occurred", zap.Error(err))
			return false, err
		}
		log.Info(fmt.Sprintf("action found: %s", actionRow))
		switch actionRow.Status {
		case "success":
			log.Info("action status is success")
			return true, nil
		case "in progress":
			log.Info("action in progress, waiting...")
			return false, nil
		case "error":
			if actionRow.Error == "backup is already exists" {
				return true, nil
			}
			return false, errors.New(fmt.Sprintf("desired action ends with error: %s. exiting", actionRow.Error))
		}
		return false, nil
	})
	return err
}

func getTimeOut() int {
	if os.Getenv("TIMEOUT") == "" {
		return 10
	}
	ret, _ := strconv.Atoi(os.Getenv("TIMEOUT"))
	return ret
}

func GetLastActionStatusForHost(service string) (ActionRow, error) {
	res, err := http.Get(fmt.Sprintf("%s://%s:7171/backup/actions?last=1", GetProtocol(), service))
	if err != nil {
		return ActionRow{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return ActionRow{}, err
	}
	actions, err := actionsRead(body)
	if err != nil {
		return ActionRow{}, err
	}
	if len(actions) != 0 {
		return actions[len(actions)-1], nil
	} else {
		// return fake success action
		return ActionRow{Status: "success"}, nil
	}
}

func actionsRead(input []byte) (ActionsRow, error) {
	var action ActionRow
	var actions ActionsRow
	dec := json.NewDecoder(bytes.NewReader(input))
	for {
		err := dec.Decode(&action)
		if err == io.EOF {
			//all done
			break
		}
		if err != nil {
			log.Fatal("There is an error, during Actions Read", zap.Error(err))
			return nil, err
		}

		actions = append(actions, action)
	}
	return actions, nil
}

type ActionRow struct {
	Command string `json:"command"`
	Status  string `json:"status"`
	Start   string `json:"start,omitempty"`
	Finish  string `json:"finish,omitempty"`
	Error   string `json:"error,omitempty"`
}

type ActionsRow []ActionRow

func (row ActionRow) String() string {
	return fmt.Sprintf("Command=%s, Status=%s, Error=%s", row.Command, row.Status, row.Error)
}

func BoolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func IsTlsEnabled() bool {
	return isTlsEnabled
}

func GetProtocol() string {
	if IsTlsEnabled() {
		return "https"
	}
	return "http"
}
