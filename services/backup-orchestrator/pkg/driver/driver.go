package driver

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/constants"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/utils"
	"go.uber.org/zap"
)

var (
	log = utils.GetLogger()
)

func GetDatabaseList(chHosts []string) ([]string, error) {
	var (
		database string
		dbC      *sql.DB
		err      error
	)

	databases := make([]string, 0)

	for _, chHost := range chHosts {
		dbC, err = getChDb(chHost, constants.DefaultDb)
		if err != nil {
			return nil, err
		}
		break
	}
	defer dbC.Close()

	rows, err := dbC.Query(`SELECT name FROM system.databases where name not like 'default' and name not like 'system';`)
	if err != nil {
		log.Error("Can't perform query: SHOW DATABASES NOT LIKE 'system'", zap.Error(err))
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		if err = rows.Scan(&database); err != nil {
			log.Error("Can't get response from rows", zap.Error(err))
			return nil, err
		}
		if strings.ToLower(database) != "information_schema" {
			databases = append(databases, database)
		}
	}

	return databases, nil
}

func DropDatabases(chHost string, databases []string) error {
	var (
		rows *sql.Rows
		dbC  *sql.DB
		err  error
	)

	dbC, err = getChDb(chHost, constants.DefaultDb)
	if err != nil {
		return err
	}
	defer dbC.Close()

	for _, database := range databases {
		//we don't want to drop this one, coz we need to connect to any db
		if database != "default" {
			rows, err = dbC.Query(fmt.Sprintf("DROP DATABASE IF EXISTS %s ON CLUSTER '{cluster}' SYNC", database))
		}
		if err != nil {
			log.Error("Can't perform query: DROP DATABASE", zap.Error(err))
			return err
		}
	}

	defer rows.Close()
	return nil
}

func GetQueueSizeForHostAndDb(chHost, db string) (string, error) {
	dbC, err := getChDb(chHost, db)
	if err != nil {
		return "", err
	}

	defer dbC.Close()
	var qSize string
	rows, err := dbC.Query("SELECT sum(queue_size) as q FROM system.replicas;")
	for rows.Next() {
		if err = rows.Scan(&qSize); err != nil {
			log.Error("Can't get response from rows", zap.Error(err))
			return qSize, err
		}
	}
	return qSize, nil
}

func getChDb(chHost, database string) (*sql.DB, error) {
	log.Info(fmt.Sprintf("will connect to ch host: %s,port: %s ch database: %s", chHost, utils.GetDbPort(), database))
	dbC := clickhouse.OpenDB(&clickhouse.Options{
		Addr: []string{chHost + ":" + utils.GetDbPort()},
		Auth: clickhouse.Auth{
			Database: database,
			Username: utils.GetClickhouseUserName(),
			Password: utils.GetClusterPassword(),
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 10 * time.Second,
		TLS:         utils.GetTlsConfig(),
	})

	if err := dbC.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			log.Error(fmt.Sprintf("[%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace))
		} else {
			log.Error("No connection to database", zap.Error(err))
			return nil, err
		}
	}
	return dbC, nil
}

func DropMarkCache(chHost string) error {
	dbC, err := getChDb(chHost, constants.DefaultDb)
	if err != nil {
		return err
	}

	_, err = dbC.Exec("SYSTEM DROP MARK CACHE")
	if err != nil {
		log.Error(fmt.Sprintf("[Host %s] Can't perform query: SYSTEM DROP MARK CACHE", chHost), zap.Error(err))
		return err
	}

	return nil
}
