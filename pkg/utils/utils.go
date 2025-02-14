package utils

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/constants"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	k8sClient  client.Client
	backupPath = "/backup-storage"
	activeMode = "active"
	log        = GetLogger()
)

func GetClient() (client.Client, error) {
	if k8sClient == nil {
		crclient, err := createClient()
		if err != nil {
			return nil, err
		}
		k8sClient = crclient
	}
	return k8sClient, nil
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

func GetNameSpace() string {
	return os.Getenv("WATCH_NAMESPACE")
}

func GetClusterName() string {
	return os.Getenv("CLUSTER")
}

func GetClickhouseUserName() string {
	if os.Getenv("CLICKHOUSE_USERNAME") == "" {
		return "clickhouse"
	}
	return os.Getenv("CLICKHOUSE_USERNAME")
}

func GetClusterPassword() string {
	if os.Getenv("CLICKHOUSE_PASSWORD") == "" {
		return "clickhouse"
	}
	return os.Getenv("CLICKHOUSE_PASSWORD")
}

func getTimeOut() int {
	if os.Getenv("TIMEOUT") == "" {
		return 10
	}
	ret, _ := strconv.Atoi(os.Getenv("TIMEOUT"))
	return ret
}

func WriteActionInfo(data interface{}, fileName string, backupId string) error {
	var file *os.File
	var err error

	fullFileName := fmt.Sprintf("%s/%s.json", backupId, fileName)

	if !FileExist(fmt.Sprintf(fullFileName)) {
		file, err = create(fullFileName)
		if err != nil {
			return err
		}
	} else {
		file, err = os.OpenFile(fullFileName, os.O_APPEND|os.O_WRONLY, 770)
	}
	defer file.Close()
	result, err := json.Marshal(data)
	if _, err := file.Write(result); err != nil {
		return err
	}
	return nil

}

func GetDbsFromFile(backupName string) ([]string, error) {
	dbFilePath := fmt.Sprintf("%s/databases.json", backupName)
	b, err := ioutil.ReadFile(dbFilePath)
	var j []string
	err = json.Unmarshal(b, &j)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func create(p string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(p), 0770); err != nil {
		return nil, err
	}
	return os.Create(p)
}

func ResponseBodyToJson(response io.ReadCloser) (interface{}, error) {
	var j interface{}
	err := json.NewDecoder(response).Decode(&j)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func ActionsRead(input []byte) (types.ActionsRow, error) {
	var action types.ActionRow
	var actions types.ActionsRow
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

func FileExist(filename string) bool {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	}
	return true
}

func CovertDbsToTables(dbs []string) string {
	result := make([]string, 0)
	for _, db := range dbs {
		db = fmt.Sprintf("%s.*", db)
		result = append(result, db)
	}
	return strings.Join(result, ",")

}

func GetLastActionStatusForHost(client http.Client, service string) (types.ActionRow, error) {
	var result types.ActionRow
	url := fmt.Sprintf("http://%s:7171/backup/actions?last=1", service)
	if IsTlsEnabled() {
		url = fmt.Sprintf("https://%s:7171/backup/actions?last=1", service)
	}
	timeout := time.Duration(getTimeOut())
	err := wait.PollImmediate(5*time.Second, timeout*time.Minute, func() (bool, error) {
		res, err := client.Get(url)
		if err != nil {
			log.Warn("there is an error in GET URL", zap.Error(err))
			return false, nil // Retry
		}
		defer res.Body.Close()

		body, readErr := io.ReadAll(res.Body)
		if err != nil {
			log.Warn("there is an error during body read", zap.Error(readErr))
			return false, nil // Retry
		}

		actions, err := ActionsRead(body)
		if err != nil {
			log.Warn("there is an error, during Actions Read", zap.Error(err))
			return false, nil // Retry
		}

		if len(actions) != 0 {
			result = actions[len(actions)-1]
			log.Info(fmt.Sprintf("action found: %s", result))
			return true, nil // Stop retrying
		}

		log.Warn("failing to get action status, retrying...")
		return false, nil // Retry
	})

	if err != nil {
		return types.ActionRow{}, errors.New(fmt.Sprintf("failed to get last action status, error found: %s.", err))
	}

	return result, nil
}

func IsBackupFailedFileExist(backupId string) bool {
	return FileExist(fmt.Sprintf("%s/.failed", backupId))
}

func IsS3Remote() bool {
	remoteStorage, ok := os.LookupEnv("REMOTE_STORAGE")
	if !ok {
		return false
	} else {
		return remoteStorage == "s3"
	}
}

func KeepLocalBackups() bool {
	keepLocalBackups, ok := os.LookupEnv("KEEP_LOCAL_BACKUPS")
	if !ok {
		return false
	} else {
		return keepLocalBackups == "true"
	}
}

func IsExternal(path string) bool {
	storageExternal := os.Getenv("STORAGE_EXTERNAL")
	if storageExternal != "" && strings.HasPrefix(path, storageExternal) {
		return true
	}
	return false
}

func RemoteStorage() (bool, string) {
	remoteStorage, ok := os.LookupEnv("REMOTE_STORAGE")
	remoteStorage = strings.ToLower(remoteStorage)
	if ok && (remoteStorage == "s3" || remoteStorage == "nfs") {
		return true, remoteStorage
	} else {
		return false, remoteStorage
	}
}

func GetEnvBool(key string, def bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		bvalue, err := strconv.ParseBool(value)
		if err != nil {
			log.Error(fmt.Sprintf("Can't parse %s boolean variable", key), zap.Error(err))
			panic(err)
		}
		return bvalue
	}
	return def
}

func PostActionAndWait(client http.Client, httpMethod string, hostname string, port string, action string) error {
	var (
		res *http.Response
		err error
	)
	url := fmt.Sprintf("http://%s:%s/%s", hostname, port, action)
	if IsTlsEnabled() {
		url = fmt.Sprintf("https://%s:%s/%s", hostname, port, action)
	}

	timeout := time.Duration(getTimeOut())
	pollErr := wait.PollImmediate(5*time.Second, timeout*time.Minute, func() (done bool, err error) {

		if strings.ToLower(httpMethod) == "get" {
			log.Info("Method: GET URL: " + url)
			res, err = client.Get(url)
		} else {
			log.Info("Method: POST: " + url)
			res, err = client.Post(url, "text/plain", nil)
		}
		if err != nil {
			return false, err
		}
		status := res.StatusCode
		if status == http.StatusLocked {
			log.Info(fmt.Sprintf("status is Locked, status code is: %d, retrying....", status))
			return false, nil
		} else if status == http.StatusOK {
			log.Info(fmt.Sprintf("status is OK, status code is: %d ", status))
			return true, nil
		} else if status == http.StatusCreated {
			log.Info(fmt.Sprintf("status is Created, status code is: %d ", status))
			return true, nil
		}

		// If none of the previous conditions match, it means there was an error.
		// Read the response body and log the error details.
		bodyBytes, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			log.Error("there is an error during body read", zap.Error(readErr))
		}
		return false, errors.New(fmt.Sprintf("failed to do the actions: %s. status code was %d", string(bodyBytes), status))
	})
	if pollErr != nil {
		return pollErr
	}
	if err := res.Body.Close(); err != nil {
		return err
	}
	if port == constants.CHBackupPort {
		if err = waitTillActionCompletedForHost(client, hostname); err != nil {
			return err
		}
	}
	return nil
}

func waitTillActionCompletedForHost(client http.Client, host string) error {
	timeout := time.Duration(getTimeOut())
	err := wait.PollImmediate(5*time.Second, timeout*time.Minute, func() (done bool, err error) {
		var actionRow types.ActionRow
		log.Info("waiting until action will success")
		if actionRow, err = GetLastActionStatusForHost(client, host); err != nil {
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
		case "cancel":
			log.Info("action was canceled")
			return true, nil
		}
		return false, nil
	})
	return err
}

func IsTlsEnabled() bool {
	tlsEnabled := os.Getenv("TLS_ENABLED")
	if tlsEnabled != "" && strings.ToLower(tlsEnabled) == "true" {
		return true
	}
	return false
}

func GetTlsConfig() (tlsConfig *tls.Config) {
	if IsTlsEnabled() {
		certPath := os.Getenv("CERTS_PATH")
		if certPath == "" {
			certPath = "/tls/"
		}
		clientCertificates, err := os.ReadFile(certPath + "ca.crt")
		if err != nil {
			log.Error("certificate cannot be initialized", zap.Error(err))
		}
		rootCAs := x509.NewCertPool()
		if ok := rootCAs.AppendCertsFromPEM(clientCertificates); !ok {
			log.Error("certificate cannot be initialized")
		}
		tlsConfig = &tls.Config{
			RootCAs:    rootCAs,
			MinVersion: tls.VersionTLS12,
			ClientAuth: tls.VerifyClientCertIfGiven,
			ClientCAs:  rootCAs,
		}
	}
	return tlsConfig
}

func GetDbPort() string {
	if IsTlsEnabled() {
		return "9440"
	}
	return "9000"
}

func GetPort() string {
	if IsTlsEnabled() {
		return "8443"
	}
	return "8080"
}
