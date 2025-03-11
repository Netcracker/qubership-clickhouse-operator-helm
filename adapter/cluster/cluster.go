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

package cluster

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/utils"
	"go.uber.org/zap"
)

const (
	healthUp  = "UP"
	healthOOS = "OUT_OF_SERVICE"

	healthQuery      = "SELECT * FROM system.tables"
	trustCertsFolder = "/certs"
)

var log = utils.GetLogger(false)

type ClusterAdapterImpl struct {
	Host      string
	Port      int
	Database  string
	Health    string
	user      string
	password  string
	tlsConfig *tls.Config
}

type ConnImpl struct {
	conn *sql.DB
}

type Conn interface {
	Query(query string, args ...any) (Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
	Close() error
	Ping() error
}

type Rows interface {
	Next() bool
	Close() error
	Scan(dest ...any) error
}

type ClusterAdapter interface {
	GetConnection() (Conn, error)
	GetConnectionToDb(database string) (Conn, error)
	GetConnectionToDbWithUser(database string, username string, password string) (Conn, error)
	GetHost() string
	GetPort() int
	GetDatabase() string
	GetHealth() string
}

func (ci ConnImpl) Exec(query string, args ...any) (sql.Result, error) {
	return ci.conn.Exec(query, args...)
}

func (ci ConnImpl) Close() error {
	return ci.conn.Close()
}

func (ci ConnImpl) Ping() error {
	return ci.conn.Ping()
}

func (ci ConnImpl) Query(query string, args ...any) (Rows, error) {
	rows, err := ci.conn.Query(query, args...)
	return rows, err
}

func (c *ClusterAdapterImpl) GetHost() string {
	return c.Host
}

func (c *ClusterAdapterImpl) GetPort() int {
	return c.Port
}

func (c *ClusterAdapterImpl) GetDatabase() string {
	return c.Database
}

func (c *ClusterAdapterImpl) GetHealth() string {
	return c.Health
}

func NewAdapter(host string, port int, username, password string, database string, ssl bool) *ClusterAdapterImpl {
	var tlsClientConfig *tls.Config
	// Use port for TLS
	if ssl {
		port = 9440
		tlsClientConfig = getTLSConfig()
	}

	c := &ClusterAdapterImpl{
		Database:  database,
		Host:      host,
		Port:      port,
		user:      username,
		password:  password,
		Health:    healthUp,
		tlsConfig: tlsClientConfig,
	}

	if c.RequestHealth() == healthOOS {
		log.Fatal("cannot connect to clickhouse")
	}

	return c
}

func (ca ClusterAdapterImpl) RequestHealth() string {
	ca.Health = ca.getHealth()
	return ca.Health
}

func (ca ClusterAdapterImpl) GetConnection() (Conn, error) {
	return ca.GetConnectionToDb(ca.Database)
}

func (ca ClusterAdapterImpl) GetConnectionToDb(database string) (Conn, error) {
	return ca.GetConnectionToDbWithUser(database, ca.user, ca.password)
}

func (ca ClusterAdapterImpl) GetConnectionToDbWithUser(database string, username string, password string) (Conn, error) {
	conn := clickhouse.OpenDB(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", ca.Host, ca.Port)},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
		TLS: ca.tlsConfig,
	})

	if err := conn.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			log.Error(fmt.Sprintf("Exception [%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace))
		}
		return nil, err
	}
	return ConnImpl{conn: conn}, nil
}

func (ca ClusterAdapterImpl) getHealth() string {
	err := ca.executeHealthQuery()
	if err != nil {
		log.Error("Clickhouse is unavailable", zap.Error(err))
		return healthOOS
	} else {
		return healthUp
	}
}

func (ca ClusterAdapterImpl) executeHealthQuery() error {
	conn, err := ca.GetConnection()
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Exec(healthQuery)
	return err
}

func getTLSConfig() (tlsClientConfig *tls.Config) {
	certsDir, err := os.ReadDir(trustCertsFolder)
	if err != nil || len(certsDir) == 0 {
		log.Info(fmt.Sprintf("Cannot load trusted TLS certificates from path '%s'. InsecureSkipVerify is used.", trustCertsFolder))
		tlsClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		certs := x509.NewCertPool()
		for _, cert := range certsDir {
			if isNotDir(cert) {
				pemData, err := os.ReadFile(fmt.Sprintf("%s/%s", trustCertsFolder, cert.Name()))
				if err != nil {
					log.Fatal(fmt.Sprintf("Failed to read certificate '%s'", cert.Name()), zap.Error(err))
				}
				certs.AppendCertsFromPEM(pemData)
				log.Info(fmt.Sprintf("Trusted certificate '%s' was added to client", cert.Name()))
			}
		}
		tlsClientConfig = &tls.Config{RootCAs: certs}
	}
	return tlsClientConfig
}

func isNotDir(info os.DirEntry) bool {
	return !info.IsDir() && !strings.HasPrefix(info.Name(), "..")
}
