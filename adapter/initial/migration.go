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

package initial

import (
	// "context"
	"context"
	"errors"
	"fmt"

	"github.com/Netcracker/qubership-clickhouse-dbaas-adapter/adapter/basic"
	"github.com/Netcracker/qubership-clickhouse-dbaas-adapter/adapter/cluster"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/service"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/utils"
	"go.uber.org/zap"
)

var (
	log = utils.GetLogger(false)
)

func MigrateRoles(adAdministration service.DbAdministration) {
	log.Info("Migration started")
	a := adAdministration.(basic.ClickhouseServiceAdapter)
	databases := a.GetDatabases(context.TODO())

	for _, d := range databases {
		if d == "information_schema" || d == "default" || d == "system" || d == "INFORMATION_SCHEMA" {
			continue
		}
		grantUsersForDatabase(a, d)
	}
	log.Info("Migration finished")
}

func grantUsersForDatabase(a basic.ClickhouseServiceAdapter, database string) {
	log.Info(fmt.Sprintf("Processing database %s", database))
	users, err := getUsersForDatabase(a, database)
	if err != nil {
		log.Error(fmt.Sprintf("processing of database %s aborted", database))
		return
	}

	log.Info(fmt.Sprintf("Users for migration %v", users))
	for role, username := range users {
		if role == basic.AdminRole {
			grantAdminRemote(a, username)
			grantDictionaryReload(a, username)
		}
	}
}

func grantDictionaryReload(a basic.ClickhouseServiceAdapter, username string) {
	log.Debug(fmt.Sprintf("Setting SYSTEM RELOAD DICTIONARY grants to %s", username))
	conn, err := a.GetConnection()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	_, err = conn.Exec(basic.GrantDictionaryReloadQuery(username))
	if err != nil {
		log.Error(fmt.Sprintf("cannot grant SYSTEM RELOAD DICTIONARY to %s", username), zap.Error(err))
	}
}

func grantAdminRemote(a basic.ClickhouseServiceAdapter, username string) {
	log.Debug(fmt.Sprintf("Granting ON CLUSTER REMOTE permissions to %s ", username))
	conn, err := a.GetConnection()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	_, err = conn.Exec(basic.GrantAdminRemoteQuery(username))
	if err != nil {
		log.Error(fmt.Sprintf("Cannot grant ON CLUSTER REMOTE ON to %s ", username), zap.Error(err))
	}
}

func getUsersForDatabase(a basic.ClickhouseServiceAdapter, database string) (map[string]string, error) {
	conn, err := a.GetConnection()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	adminRole, err := getUserByRole(conn, database, basic.AdminRole)
	if err != nil {
		return nil, err
	}
	users := map[string]string{
		basic.AdminRole: adminRole, // Only admin users currently required for migration
	}
	return users, nil
}

func getUserByRole(conn cluster.Conn, database, role string) (string, error) {
	query := basic.GetAdminUserQuery(database)
	switch role {
	case basic.RWRole:
		query = basic.GetRWUserQuery(database)
	case basic.RORole:
		query = basic.GetROUserQuery(database)
	}

	rows, err := conn.Query(query)
	if err != nil {
		errMsg := fmt.Sprintf("cannot get user with role %s for database %s", role, database)
		log.Error(errMsg, zap.Error(err))
		return "", err
	}
	defer rows.Close()

	var username string
	for rows.Next() {
		err = rows.Scan(&username)
		if err != nil {
			errMsg := fmt.Sprintf("cannot scan user with role %s for database %s", role, database)
			log.Error(errMsg, zap.Error(err))
			return "", err
		}
	}

	if len(username) == 0 {
		errMsg := fmt.Sprintf("cannot find user with role %s for database %s", role, database)
		log.Error(errMsg)
		return "", errors.New(errMsg)
	}

	return username, nil
}
