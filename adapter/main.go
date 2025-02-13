package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/Netcracker/qubership-clickhouse-dbaas-adapter/adapter/basic"
	"github.com/Netcracker/qubership-clickhouse-dbaas-adapter/adapter/cluster"
	"github.com/Netcracker/qubership-clickhouse-dbaas-adapter/adapter/initial"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/dao"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/dbaas"
	fiber2 "github.com/Netcracker/qubership-dbaas-adapter-core/pkg/impl/fiber"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/service"
	coreUtils "github.com/Netcracker/qubership-dbaas-adapter-core/pkg/utils"
	"go.uber.org/zap"
)

const (
	appName = "clickhouse"
	appPath = "/" + appName
)

var (
	logger = coreUtils.GetLogger(false)

	apiVersion = "v1"

	chHost              = flag.String("ch_host", coreUtils.GetEnv("CLICKHOUSE_HOST", "chi-cluster-replicated-0-0.click-aliv.svc"), "Host of clickhouse cluster, env: CLICKHOUSE_HOST")
	chPort              = flag.Int("ch_port", coreUtils.GetEnvAsInt("CLICKHOUSE_PORT", 9000), "Port of clickhouse cluster, env: CLICKHOUSE_PORT")
	chUser              = flag.String("ch_user", coreUtils.GetEnv("CLICKHOUSE_USERNAME", "clickhouse_operator"), "Username of dbaas user in clickhouse, env: CLICKHOUSE_USERNAME")
	chPass              = flag.String("ch_pass", coreUtils.GetEnv("CLICKHOUSE_PASSWORD", "clickhouse_operator_password"), "Password of dbaas user in clickhouse, env: CLICKHOUSE_PASSWORD")
	chSsl               = flag.Bool("ch_ssl", coreUtils.GetEnvAsBool("CLICKHOUSE_SSL", false), "Enable ssl connection to clickhouse, env: CLICKHOUSE_SSL")
	isMultiUsersEnabled = flag.Bool("multi_users_enabled", coreUtils.GetEnvAsBool("MULTI_USERS_ENABLED", false), "Is multi Users functionality enabled, env: MULTI_USERS_ENABLED")
	chDatabase          = flag.String("ch_database", coreUtils.GetEnv("CLICKHOUSE_DATABASE", "system"), "Clickhouse database, env: CLICKHOUSE_DATABASE")

	backupAddress       = flag.String("backup_address", coreUtils.GetEnv("BACKUP_DAEMON_ADDRESS", "http://clickhouse-backup-orchestrator:8080"), "Address of clickhouse backup orchestrator, env: BACKUP_DAEMON_ADDRESS")
	backupDaemonApiUser = flag.String("backup_daemon_api_user", coreUtils.GetEnv("BACKUP_DAEMON_API_CREDENTIALS_USERNAME", ""), "Username of api clickhouse backup orchestrator, env: BACKUP_DAEMON_API_CREDENTIALS_USERNAME")
	backupDaemonApiPass = flag.String("backup_daemon_api_pass", coreUtils.GetEnv("BACKUP_DAEMON_API_CREDENTIALS_PASSWORD", ""), "Password of api clickhouse backup orchestrator, env: BACKUP_DAEMON_API_CREDENTIALS_PASSWORD")

	servePort = flag.Int("serve_port", 8080, "Port to serve requests incoming to adapter")
	serveUser = flag.String(
		"serve_user",
		coreUtils.GetEnv("DBAAS_ADAPTER_API_USER", "dbaas-aggregator"),
		"Username to authorize incoming requests, env: DBAAS_ADAPTER_API_USER",
	)
	servePass = flag.String(
		"serve_pass",
		coreUtils.GetEnv("DBAAS_ADAPTER_API_PASSWORD", "dbaas-aggregator"),
		"Password to authorize incoming requests, env: DBAAS_ADAPTER_API_PASSWORD",
	)

	phydbid = flag.String(
		"phydbid",
		coreUtils.GetEnv("DBAAS_AGGREGATOR_PHYSICAL_DATABASE_IDENTIFIER", "unknown_clickhouse"),
		"Identifier to register physical database in dbaas aggregator, env DBAAS_AGGREGATOR_PHYSICAL_DATABASE_IDENTIFIER",
	)

	selfAddress = flag.String(
		"self_address",
		coreUtils.GetEnv("DBAAS_ADAPTER_ADDRESS", ""),
		"Address in the form <scheme>://<host>:<port> how adapter could be reached from aggregator, env DBAAS_ADAPTER_ADDRESS",
	)

	dbaasAggregatorRegistrationAddress = flag.String(
		"registration_address",
		coreUtils.GetEnv("DBAAS_AGGREGATOR_REGISTRATION_ADDRESS", "http://dbaas-aggregator.dbaas:8080"),
		"Address in the form <scheme>://<host>:<port> to reach aggregator for registration, env DBAAS_AGGREGATOR_REGISTRATION_ADDRESS",
	)

	dbaasAggregatorRegistrationUsername = flag.String(
		"registration_username",
		coreUtils.GetEnv("DBAAS_AGGREGATOR_REGISTRATION_USERNAME", "cluster-dba"),
		"Username of basic auth to reach aggregator for registration, env DBAAS_AGGREGATOR_REGISTRATION_USERNAME ",
	)

	dbaasAggregatorRegistrationPassword = flag.String(
		"registration_password",
		coreUtils.GetEnv("DBAAS_AGGREGATOR_REGISTRATION_PASSWORD", ""),
		"Username of basic auth to reach aggregator for registration, env DBAAS_AGGREGATOR_REGISTRATION_PASSWORD ",
	)

	labelsFileName = flag.String(
		"labels_file_location_name",
		"dbaas.physical_databases.registration.labels.json",
		"File name where labels are located in json key-value format, env LABELS_FILE_LOCATION_NAME",
	)

	labelsLocationDir = flag.String(
		"labels_file_location_dir",
		"/app/config/",
		"Directory with file where labels are located in json key-value format, env LABELS_FILE_LOCATION_DIR",
	)

	registrationFixedDelay = flag.Int(
		"registration_fixed_delay",
		coreUtils.GetEnvAsInt("DBAAS_AGGREGATOR_REGISTRATION_FIXED_DELAY_MS", 150000), // default 2.5 min
		"Scheduled physical database registration fixed delay in milliseconds")

	registrationRetryTime = flag.Int(
		"registration_retry_time",
		coreUtils.GetEnvAsInt("DBAAS_AGGREGATOR_REGISTRATION_RETRY_TIME_MS", 60000), // default 1 min
		"Force physical database registration retry time in milliseconds")

	registrationRetryDelay = flag.Int(
		"registration_retry_delay",
		coreUtils.GetEnvAsInt("DBAAS_AGGREGATOR_REGISTRATION_RETRY_DELAY_MS", 5000), // default 5 sec
		"Force physical database registration retry delay between attempts in milliseconds")

	namespace = flag.String("namespace", coreUtils.GetEnv("CLOUD_NAMESPACE", ""), "Namespace name, env: CLOUD_NAMESPACE")

	supports = dao.SupportsBase{
		Users:             true,
		Settings:          false,
		DescribeDatabases: false,
	}
)

func main() {
	clusterAdapter := cluster.NewAdapter(*chHost, *chPort, *chUser, *chPass, *chDatabase, *chSsl)

	basicRegistrationAuth := dao.BasicAuth{
		Username: *dbaasAggregatorRegistrationUsername,
		Password: *dbaasAggregatorRegistrationPassword,
	}

	dbaasClient, err := dbaas.NewDbaasClient(*dbaasAggregatorRegistrationAddress, &basicRegistrationAuth, nil)

	if dbaasClient == nil {
		panic(fmt.Errorf("failed to establish connection to DBaaS aggregator, err: %v", err))
	}

	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get DBaaS aggregator version, err %v. Setting default API version", err))
	}
	// Getting dbaas-adpater api version
	version, _ := dbaasClient.GetVersion()
	if version == "v3" {
		apiVersion = "v2"
	} else {
		apiVersion = "v1"
	}
	logger.Info(fmt.Sprintf("API version obtained: %s", apiVersion))

	var dbAdminImpl = basic.NewServiceAdapter(clusterAdapter, dao.ApiVersion(apiVersion), getRoles(), getFeatures())
	// Backup Daemon Administration
	сlient := &http.Client{}
	if *chSsl {
		сlient.Transport = &http.Transport{
			TLSClientConfig: setTLSConfig(),
		}
	}

	backupAdminServiceImpl := service.DefaultBackupAdministrationService(
		logger,
		*backupAddress,
		*backupDaemonApiUser,
		*backupDaemonApiPass,
		false,
		сlient,
		basic.DbNameMaxLength,
		basic.GetSpecialSymbols(),
	)

	if checkInitMode() {
		initial.MigrateRoles(dbAdminImpl)
		return
	}

	administrationService := service.NewCoreAdministrationService(
		*namespace,
		*servePort,
		dbAdminImpl,
		logger,
		false,
		&coreUtils.VaultClient{},
		"",
	)
	updateTLSStatusForAdapter()
	log.Fatal(fiber2.RunFiberServer(*servePort, func(app *fiber.App, ctx context.Context) error {
		fiber2.BuildFiberDBaaSAdapterHandlers(
			app,
			*serveUser,
			*servePass,
			appPath,
			administrationService,
			service.NewPhysicalRegistrationService(
				appName,
				logger,
				*phydbid,
				*selfAddress,
				dao.BasicAuth{
					Username: *serveUser,
					Password: *servePass,
				},
				ReadLabelsFile(), //labels
				dbaasClient,
				*registrationFixedDelay,
				*registrationRetryTime,
				*registrationRetryDelay,
				administrationService,
				ctx,
			),
			backupAdminServiceImpl,
			supports.ToMap(),
			logger,
			false,
			"")
		return nil
	}))
}

func ReadLabelsFile() map[string]string {
	file, err := os.ReadFile(*labelsLocationDir + *labelsFileName)
	if err != nil {
		logger.Info(fmt.Sprintf("Skipping labels file, cannot read it: %s", *labelsLocationDir+*labelsFileName))
		return make(map[string]string)
	}
	var labels map[string]string
	err = json.Unmarshal(file, &labels)
	if err != nil {
		logger.Info(fmt.Sprintf("Failed to parse labels file %s", *labelsLocationDir+*labelsFileName), zap.Error(err))
		labels = make(map[string]string)
	}
	logger.Info(fmt.Sprintf("Labels: %v", labels))
	return labels
}

func getRoles() []string {
	return []string{basic.AdminRole, basic.RWRole, basic.RORole}
}

func getFeatures() map[string]bool {
	return map[string]bool{
		basic.FeatureMultiUsers: *isMultiUsersEnabled && apiVersion != "v1",
		basic.FeatureTls:        *chSsl,
	}
}

func updateTLSStatusForAdapter() {
	if strings.Contains(*dbaasAggregatorRegistrationAddress, "https") {
		logger.Info("tls is enabled, will check if https presented in adapter url")
		if !strings.Contains(*selfAddress, "https") {
			*selfAddress = strings.ReplaceAll(*selfAddress, "http", "https")
		}
		*selfAddress = strings.ReplaceAll(*selfAddress, "8080", "8443")
		logger.Info(fmt.Sprintf("replacing self address with https, %s", *selfAddress))
	}
}

func checkInitMode() bool {
	for _, arg := range os.Args {
		if arg == "init" {
			return true
		}
	}
	return false
}

func setTLSConfig() *tls.Config {
	cert, err := tls.LoadX509KeyPair("/certs/tls.crt", "/certs/tls.key")
	if err != nil {
		panic(fmt.Sprintf("Error during load a key pair %v", zap.Error(err)))
	}

	// Load CA cert
	caCert, err := os.ReadFile("/certs/ca.crt")
	if err != nil {
		panic(fmt.Sprintf("Error during load a key pair %v", zap.Error(err)))
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Setup HTTPS client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}
	return tlsConfig
}
