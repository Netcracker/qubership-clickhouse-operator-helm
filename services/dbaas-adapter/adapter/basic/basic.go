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

package basic

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/Netcracker/qubership-clickhouse-dbaas-adapter/adapter/cluster"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/dao"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/utils"
	uuid "github.com/satori/go.uuid"
	"go.uber.org/zap"
)

const (
	DBaaSMetadata = "_DBAAS_METADATA"
	Metadata      = "metadata"

	DbKind             = "database"
	UserKind           = "user"
	DeletedStatus      = "DELETED"
	DeleteFailedStatus = "DELETE_FAILED"

	usernamePrefix = "dbaas"

	dbNameParam   = "dbName"
	prefixParam   = "namePrefix"
	userNameParam = "username"

	DbNameMaxLength = 63
)

func GetSpecialSymbols() []string {
	var specialSymbols []string
	return specialSymbols
}

type Generator interface {
	Generate() string
}

type UUIDGenerator struct{}

func NewServiceAdapter(adapter cluster.ClusterAdapter, version dao.ApiVersion, roles []string, features map[string]bool, isReplicatedUserStorage bool) *ClickhouseServiceAdapter {
	return &ClickhouseServiceAdapter{
		Ctx:                     context.Background(),
		ClusterAdapter:          adapter,
		Mutex:                   &sync.Mutex{},
		ApiVersion:              version,
		roles:                   roles,
		features:                features,
		Generator:               UUIDGenerator{},
		IsReplicatedUserStorage: isReplicatedUserStorage,
	}
}

func (c ClickhouseServiceAdapter) CreateDatabase(ctx context.Context, requestOnCreateDb dao.DbCreateRequest) (string, *dao.LogicalDatabaseDescribed, error) {
	log := utils.GetLogger(false)
	utils.AddLoggerContext(log, ctx)

	if !validateDbIdentifierParam(dbNameParam, requestOnCreateDb.DbName, DbIdentifiersPattern) {
		return "", nil, fmt.Errorf("%s must comply to the pattern %s", dbNameParam, DbIdentifiersPattern)
	}
	var dbName string
	if requestOnCreateDb.NamePrefix != nil {
		if !validateDbIdentifierParam(prefixParam, *requestOnCreateDb.NamePrefix, DbPrefixPattern) || *requestOnCreateDb.NamePrefix == "" {
			return "", nil, fmt.Errorf("%s must comply to the pattern %s", prefixParam, DbPrefixPattern)
		}
		dbName = utils.RegenerateDbName(*requestOnCreateDb.NamePrefix, DbNameMaxLength)
	} else {
		namespace, msName, err := utils.GetNsAndMsName(requestOnCreateDb.Metadata)
		if err != nil {
			return "", nil, err
		}
		dbGeneratedName, err := utils.PrepareDatabaseName(namespace, msName, DbNameMaxLength)
		if err != nil {
			log.Error("error during database name preparation", zap.Error(err))
			panic(err)
		}
		dbName = dbGeneratedName
	}

	if !validateDbIdentifierParam(userNameParam, requestOnCreateDb.Username, DbIdentifiersPattern) {
		return "", nil, fmt.Errorf("%s must comply to the pattern %s", userNameParam, DbIdentifiersPattern)
	}

	log.Info(fmt.Sprintf("dbName is set to: %s", dbName))

	conn, err := c.ClusterAdapter.GetConnection()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	_, err = conn.Exec(createDatabaseQuery(dbName))
	if err != nil {
		log.Error(fmt.Sprintf("cannot create database %s", dbName), zap.Error(err))
		panic(err)
	}
	resources := []dao.DbResource{{Kind: DbKind, Name: dbName}}

	users := make(map[string]string)
	connectionProps := make([]dao.ConnectionProperties, 0)
	for _, role := range c.GetSupportedRoles() {
		username := fmt.Sprintf("%s_%s", usernamePrefix, c.Generate())
		password := c.Generate()
		if _, err = conn.Exec(createUserQuery(username, password, c.IsReplicatedUserStorage)); err != nil {
			log.Error(fmt.Sprintf("cannot create user %s with role %s", username, role), zap.Error(err))
			_ = c.dropDatabase(ctx, conn, dbName)
			c.dropUsers(ctx, conn, users)
			panic(err)
		}
		users[username] = role
		connectionProps = append(connectionProps, c.getConnectionProperties(dbName, username, role, password))
		resources = append(resources, dao.DbResource{Kind: UserKind, Name: username})
	}
	err = c.grantUsersAccordingRoles(conn, dbName, users)
	if err != nil {
		_ = c.dropDatabase(ctx, conn, dbName)
		c.dropUsers(ctx, conn, users)
		panic(err)
	}
	metadata := requestOnCreateDb.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	err = c.createMetadata(ctx, dbName, metadata)
	if err != nil {
		_ = c.dropDatabase(ctx, conn, dbName)
		c.dropUsers(ctx, conn, users)
		panic(err)
	}
	log.Info(fmt.Sprintf("Created db resources: %+v", resources))
	response := &dao.LogicalDatabaseDescribed{ConnectionProperties: connectionProps, Resources: resources}
	return dbName, response, nil
}

func (c ClickhouseServiceAdapter) DescribeDatabases(ctx context.Context, logicalDatabases []string, showResources bool, showConnections bool) map[string]dao.LogicalDatabaseDescribed {
	log := utils.GetLogger(false)
	utils.AddLoggerContext(log, ctx)

	log.Info("Describe databases is executed")
	result := make(map[string]dao.LogicalDatabaseDescribed)
	return result
}

func (c ClickhouseServiceAdapter) GetDatabases(ctx context.Context) []string {
	log := utils.GetLogger(false)
	utils.AddLoggerContext(log, ctx)

	conn, err := c.GetConnection()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	rows, err := conn.Query(getDatabasesQuery())
	if err != nil {
		log.Error("cannot get databases list", zap.Error(err))
		panic(err)
	}
	defer rows.Close()

	var databasesNames []string
	for rows.Next() {
		var databaseName string
		err = rows.Scan(&databaseName)
		if err != nil {
			log.Error("Error occurred during scan databases row", zap.Error(err))
			panic(err)
		}
		databasesNames = append(databasesNames, databaseName)
	}
	return databasesNames
}

func (c ClickhouseServiceAdapter) DropResources(ctx context.Context, resources []dao.DbResource) []dao.DbResource {
	var droppedResources []dao.DbResource

	conn, err := c.ClusterAdapter.GetConnection()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	users := c.dropResourceByKind(ctx, conn, resources, UserKind)
	droppedResources = append(droppedResources, users...)

	dataBases := c.dropResourceByKind(ctx, conn, resources, DbKind)
	droppedResources = append(droppedResources, dataBases...)

	return droppedResources
}

func (c ClickhouseServiceAdapter) dropResourceByKind(ctx context.Context, conn cluster.Conn, resources []dao.DbResource, kind string) []dao.DbResource {
	var result []dao.DbResource
	for _, resource := range resources {
		if resource.Kind == kind {
			err := c.dropResource(ctx, conn, resource.Name, resource.Kind)
			if err != nil {
				resource.Status = DeleteFailedStatus
				resource.ErrorMessage = err.Error()
			} else {
				resource.Status = DeletedStatus
			}
			result = append(result, resource)
		}
	}
	return result
}

func (c ClickhouseServiceAdapter) GetMetadata(ctx context.Context, logicalDatabase string) map[string]interface{} {
	metadata, err := c.GetMetadataInternal(ctx, logicalDatabase)
	if err != nil {
		panic(err)
	}
	return metadata
}

func (c ClickhouseServiceAdapter) GetMetadataInternal(ctx context.Context, logicalDatabase string) (map[string]interface{}, error) {
	log := utils.GetLogger(false)
	utils.AddLoggerContext(log, ctx)

	var metadata map[string]interface{}
	var metadataStr string
	log.Info(fmt.Sprintf("Get metadata from %s", logicalDatabase))

	conn, err := c.GetConnectionToDb(logicalDatabase)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	rows, err := conn.Query(getMetadataQuery())
	if err != nil {
		log.Error(fmt.Sprintf("Couldn't obtain data from metadata table from %s", logicalDatabase), zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&metadataStr)
		if err != nil {
			log.Error(fmt.Sprintf("Couldn't scan data from metadata table from %s", logicalDatabase), zap.Error(err))
			return nil, err
		}
	}

	err = json.Unmarshal([]byte(metadataStr), &metadata)
	if err != nil {
		log.Error(fmt.Sprintf("cannot convert to JSON metadata from %s", logicalDatabase), zap.Error(err))
		return nil, err
	}

	return metadata, nil
}

func (c ClickhouseServiceAdapter) UpdateMetadata(ctx context.Context, newMetadata map[string]interface{}, logicalDatabase string) {
	log := utils.GetLogger(false)
	utils.AddLoggerContext(log, ctx)

	if !validateDbIdentifierParam(dbNameParam, logicalDatabase, DbIdentifiersPattern) {
		panic(fmt.Errorf("%s must comply to the pattern %s", dbNameParam, DbIdentifiersPattern))
	}

	conn, err := c.GetConnectionToDb(logicalDatabase)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	metadataJson, err := json.Marshal(newMetadata)
	if err != nil {
		log.Error("Error during marshaling metadata", zap.Error(err))
		panic(err)
	}

	_, err = conn.Exec(createMetadataQuery())
	if err != nil {
		panic(err)
	}

	_, err = conn.Exec(deleteMetadataQuery())
	if err != nil {
		panic(err)
	}

	_, err = conn.Exec(insertMetadataQuery(string(metadataJson)))
	if err != nil {
		panic(err)
	}
}

func (c ClickhouseServiceAdapter) GetDefaultCreateRequest() dao.DbCreateRequest {
	return dao.DbCreateRequest{}
}

func (c ClickhouseServiceAdapter) GetDefaultUserCreateRequest() dao.UserCreateRequest {
	return dao.UserCreateRequest{}
}

func (c ClickhouseServiceAdapter) PreStart() {
	// No actions before start
}

func (c ClickhouseServiceAdapter) CreateUserWithError(ctx context.Context, userName string, requestOnCreateUser dao.UserCreateRequest) (user *dao.CreatedUser, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("error during roles creation")
		}
	}()
	return c.CreateUser(ctx, userName, requestOnCreateUser)
}

func (c ClickhouseServiceAdapter) CreateUser(ctx context.Context, userName string, requestOnCreateUser dao.UserCreateRequest) (*dao.CreatedUser, error) {

	log := utils.GetLogger(false)
	utils.AddLoggerContext(log, ctx)

	if userName == "" {
		userName = fmt.Sprintf("%s_%s", usernamePrefix, c.Generate())
	} else {
		if !validateDbIdentifierParam(userNameParam, userName, DbIdentifiersPattern) {
			return nil, fmt.Errorf("%s must comply to the pattern %s", userNameParam, DbIdentifiersPattern)
		}
	}

	if requestOnCreateUser.DbName != "" && !validateDbIdentifierParam(dbNameParam, requestOnCreateUser.DbName, DbIdentifiersPattern) {
		return nil, fmt.Errorf("%s must comply to the pattern %s", dbNameParam, DbIdentifiersPattern)
	}

	if !Contains(c.GetSupportedRoles(), requestOnCreateUser.Role) {
		return nil, fmt.Errorf("unsupported role. Role should be one of the list %s", c.GetSupportedRoles())
	}

	var resources []dao.DbResource
	userCreated := false
	password := requestOnCreateUser.Password
	if password == "" {
		password = c.Generate()
	}

	conn, err := c.ClusterAdapter.GetConnection()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	rows, err := conn.Query(getUserQuery(userName))
	if err != nil {
		log.Error(fmt.Sprintf("Couldn't get user %s", userName), zap.Error(err))
		panic(err)
	}
	defer rows.Close()

	isUserExist := rows.Next()

	if !isUserExist {
		_, err = conn.Exec(createUserQuery(userName, password, c.IsReplicatedUserStorage))
		if err != nil {
			log.Error(fmt.Sprintf("Couldn't create user %s", userName), zap.Error(err))
			panic(err)
		}
		userCreated = true
	} else {
		log.Info(fmt.Sprintf("Change password for existing user %s", userName))
		_, err = conn.Exec(changeUserPassword(userName, password))
		if err != nil {
			log.Error(fmt.Sprintf("Couldn't update user %s", userName), zap.Error(err))
			panic(err)
		}
	}

	dbName := requestOnCreateUser.DbName
	if dbName != "" {
		rolesMap := map[string]string{userName: requestOnCreateUser.Role}
		err = c.grantUsersAccordingRoles(conn, dbName, rolesMap)
		if err != nil {
			if userCreated {
				_ = c.dropUser(ctx, conn, userName)
			}

			log.Error(fmt.Sprintf("Couldn't update grants for user %s on %s", userName, dbName), zap.Error(err))
			panic(err)
		}

		resources = append(resources, dao.DbResource{
			Kind: DbKind,
			Name: dbName,
		})
	}

	connectionProperties := c.getConnectionProperties(dbName, userName, requestOnCreateUser.Role, password)
	resources = append(resources, dao.DbResource{
		Kind: UserKind,
		Name: userName,
	})

	response := &dao.CreatedUser{
		ConnectionProperties: connectionProperties,
		Name:                 dbName,
		Resources:            resources,
	}
	return response, nil
}

func (c ClickhouseServiceAdapter) GetDBPrefix() string {
	return "dbaas"
}

func (c ClickhouseServiceAdapter) GetDBPrefixDelimiter() string {
	return "_"
}

func (c ClickhouseServiceAdapter) MigrateToVault(context.Context, string, string) error {
	return nil
}

func (c ClickhouseServiceAdapter) GetVersion() dao.ApiVersion {
	return c.ApiVersion
}

func (c ClickhouseServiceAdapter) GetSupportedRoles() []string {
	if features := c.GetFeatures(); !features[FeatureMultiUsers] {
		return []string{AdminRole}
	}
	return c.roles
}

func (c ClickhouseServiceAdapter) GetFeatures() map[string]bool {
	return c.features
}

func (c ClickhouseServiceAdapter) createMetadata(ctx context.Context, dbName string, metadata map[string]interface{}) error {
	log := utils.GetLogger(false)
	utils.AddLoggerContext(log, ctx)

	log.Info(fmt.Sprintf("Insert metadata to %s for %s", DBaaSMetadata, dbName))

	conn, err := c.GetConnectionToDb(dbName)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Exec(createMetadataQuery())
	if err != nil {
		log.Error("Couldn't create metadata table", zap.Error(err))
		return err
	}

	if metadata == nil {
		log.Warn("metadata is nill during createMetadata")
		return nil
	}

	metadataJson, errParse := json.Marshal(metadata)
	if errParse != nil {
		log.Error("Error during marshal metadata", zap.Error(err))
		return errParse
	}

	_, err = conn.Exec(insertMetadataQuery(string(metadataJson)))
	if err != nil {
		log.Error("Couldn't insert data in metadata table", zap.Error(err))
		return err
	}

	return nil
}

func (c ClickhouseServiceAdapter) getConnectionProperties(dbName, username, role, password string) dao.ConnectionProperties {
	url := fmt.Sprintf("clickhouse://%s:%d/%s", c.GetHost(), c.GetPort(), dbName) //todo user pass https://clickhouse.com/docs/en/integrations/go/clickhouse-go/database-sql-api
	connectionProps := dao.ConnectionProperties{
		"name":     dbName,
		"url":      url,
		"host":     c.GetHost(),
		"port":     c.GetPort(),
		"username": username,
		"password": password,
		"role":     role,
	}
	return connectionProps
}

func (c ClickhouseServiceAdapter) dropDatabase(ctx context.Context, conn cluster.Conn, dbName string) error {
	return c.dropResource(ctx, conn, dbName, DbKind)
}

func (c ClickhouseServiceAdapter) dropUser(ctx context.Context, conn cluster.Conn, username string) error {
	return c.dropResource(ctx, conn, username, UserKind)
}

func (c ClickhouseServiceAdapter) dropResource(ctx context.Context, conn cluster.Conn, resourceName, resourceKind string) error {
	log := utils.GetLogger(false)
	utils.AddLoggerContext(log, ctx)

	var query string
	if resourceKind == DbKind {
		query = dropDatabaseQuery(resourceName)
	} else {
		query = dropUserQuery(resourceName)
	}

	_, err := conn.Exec(query)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to delete %s resource", resourceName), zap.Error(err))
		return err
	}

	return nil
}

func (c ClickhouseServiceAdapter) dropUsers(ctx context.Context, conn cluster.Conn, users map[string]string) {
	for user := range users {
		_ = c.dropUser(ctx, conn, user)
	}
}

func validateDbIdentifierParam(paramName string, paramValue string, pattern string) bool {
	if paramValue != "" {
		matched, _ := regexp.MatchString(pattern, paramValue)
		return matched
	}
	return true
}

func (generator UUIDGenerator) Generate() string {
	uuidName := uuid.NewV4()
	uuidString := uuidName.String()
	return strings.ReplaceAll(uuidString, "-", "")
}

func Contains(array []string, str string) bool {
	for _, a := range array {
		if a == str {
			return true
		}
	}
	return false
}
