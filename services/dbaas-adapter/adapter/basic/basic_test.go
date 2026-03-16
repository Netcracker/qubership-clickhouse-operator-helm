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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Netcracker/qubership-clickhouse-dbaas-adapter/adapter/cluster"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/dao"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	roles    = []string{AdminRole, RWRole, RORole}
	features = map[string]bool{}
	log      = utils.GetLogger(false)
)

const (
	apiVersion = dao.ApiVersion("v2")

	cKey  = "classifier"
	nsKey = "namespace"
	msKey = "microserviceName"
)

var (
	host     = "test_host"
	port     = 8529
	database = "test_db"
	user     = "testuser"
	password = "password"
)

type ClickhouseServiceAdapterMock struct {
	mock.Mock
	cluster.ClusterAdapter
}

type ClickHouseConnMock struct {
	mock.Mock
	cluster.Conn
}

type MockRows struct {
	mock.Mock
	cluster.Rows
}

type MockResult struct {
	mock.Mock
	sql.Result
}

type MockUser struct {
	mock.Mock
	dao.CreatedUser
}

type GeneratorMock struct {
	mock.Mock
	Generator
}

func (c *ClickhouseServiceAdapterMock) GetHost() string {
	args := c.Called()
	return args.String(0)
}

func (c *ClickhouseServiceAdapterMock) GetPort() int {
	args := c.Called()
	return args.Int(0)
}

func (c *ClickhouseServiceAdapterMock) GetDatabase() string {
	args := c.Called()
	return args.String(0)
}

func (c *ClickhouseServiceAdapterMock) GetHealth() string {
	args := c.Called()
	return args.String(0)
}

func (ca *ClickhouseServiceAdapterMock) GetConnectionToDb(database string) (cluster.Conn, error) {
	args := ca.Called(database)
	return args.Get(0).(cluster.Conn), args.Error(1)
}

func (ca *ClickhouseServiceAdapterMock) GetConnection() (cluster.Conn, error) {
	args := ca.Called()
	return args.Get(0).(cluster.Conn), args.Error(1)
}

func (db *ClickHouseConnMock) Query(query string, args ...any) (cluster.Rows, error) {
	ret := db.Called(append([]interface{}{query}, args...)...)
	return ret.Get(0).(cluster.Rows), ret.Error(1)

}

func (db *ClickHouseConnMock) Exec(query string, args ...any) (sql.Result, error) {
	ret := db.Called(append([]interface{}{query}, args...)...)

	return ret.Get(0).(sql.Result), ret.Error(1)

}

func (m *ClickHouseConnMock) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Mock the Next method
func (m *MockRows) Next() bool {
	args := m.Called()
	return args.Bool(0)
}

// Mock the Scan method
func (m *MockRows) Scan(dest ...interface{}) error {
	args := m.Called(dest)
	return args.Error(0)
}

// Mock the Close method
func (m *MockRows) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockResult) RowsAffected() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *GeneratorMock) Generate() string {
	args := m.Called()
	return args.String(0)
}

func TestDropResource_Database(t *testing.T) {
	ca := new(ClickhouseServiceAdapterMock)
	conn := new(ClickHouseConnMock)
	result := new(MockResult)
	ctx := context.TODO()

	ca.On("GetConnection").Return(conn, nil)
	conn.On("Close").Return(nil)

	resource := []dao.DbResource{
		{Kind: DbKind,
			Name: database,
		},
	}

	conn.On("Exec", dropDatabaseQuery(database)).Return(result, nil)
	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)
	dbResource := sa.DropResources(ctx, resource)
	fmt.Printf("dropResource : %v\n", dbResource)
	assert.Equal(t, dao.DELETED, dbResource[0].Status)

}

func TestDropResource_User(t *testing.T) {
	ctx := context.TODO()
	ca := new(ClickhouseServiceAdapterMock)
	conn := new(ClickHouseConnMock)
	result := new(MockResult)

	ca.On("GetConnection").Return(conn, nil)
	conn.On("Close").Return(nil)

	resource := []dao.DbResource{
		{Kind: UserKind,
			Name: user,
		},
	}

	conn.On("Exec", dropUserQuery(user)).Return(result, nil)
	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)
	dbResource := sa.DropResources(ctx, resource)
	fmt.Printf("dropResource : %v\n", dbResource)
	assert.Equal(t, dao.DELETED, dbResource[0].Status)

}
func TestDropResource_UserFailed(t *testing.T) {
	ctx := context.TODO()
	ca := new(ClickhouseServiceAdapterMock)
	conn := new(ClickHouseConnMock)
	result := new(MockResult)

	ca.On("GetConnection").Return(conn, nil)
	conn.On("Close").Return(nil)

	resource := []dao.DbResource{
		{Kind: UserKind,
			Name: user,
		},
	}
	expectedErrorC := errors.New("error while execute drop user")
	conn.On("Exec", dropUserQuery(user)).Return(result, expectedErrorC)
	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)
	dbResource := sa.DropResources(ctx, resource)
	fmt.Printf("dropResource : %v\n", dbResource)
	assert.Equal(t, dao.DELETE_FAILED, dbResource[0].Status)
	assert.Equal(t, "error while execute drop user", dbResource[0].ErrorMessage)

}

func TestDropResource_DatabaseFailed(t *testing.T) {
	ctx := context.TODO()
	ca := new(ClickhouseServiceAdapterMock)
	conn := new(ClickHouseConnMock)
	result := new(MockResult)

	ca.On("GetConnection").Return(conn, nil)
	conn.On("Close").Return(nil)

	resource := []dao.DbResource{
		{Kind: DbKind,
			Name: database,
		},
	}
	expectedErrorC := errors.New("error while execute drop database")
	conn.On("Exec", dropDatabaseQuery(database)).Return(result, expectedErrorC)
	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)
	dbResource := sa.DropResources(ctx, resource)
	fmt.Printf("dropResource : %v\n", dbResource)
	assert.Equal(t, dao.DELETE_FAILED, dbResource[0].Status)
	assert.Equal(t, "error while execute drop database", dbResource[0].ErrorMessage)

}

func TestCreateUser_UserExist(t *testing.T) {
	ctx := context.TODO()
	ca := new(ClickhouseServiceAdapterMock)
	conn := new(ClickHouseConnMock)
	rows := new(MockRows)
	result := new(MockResult)
	ca.On("GetConnection").Return(conn, nil)
	ca.On("GetConnectionToDb", database).Return(conn, nil)
	fmt.Printf("my ac from connect func connection: %v\n", conn)

	conn.On("Close").Return(nil)
	userRole := AdminRole
	var err error
	features := map[string]bool{FeatureMultiUsers: true}
	userCreateRequest := dao.UserCreateRequest{DbName: database, Role: userRole, Password: password}

	ca.On("GetUser").Return(user)
	ca.On("GetHost").Return(host)
	ca.On("GetPort").Return(port)
	conn.On("Query", getUserQuery(user)).Return(rows, nil)
	conn.On("Query", getMetadataQuery()).Return(rows, nil)
	conn.On("Exec", GrantDictionaryReloadQuery(user)).Return(result, nil)
	conn.On("Exec", GrantAdminRemoteQuery(user)).Return(result, nil)
	conn.On("Exec", grantAdminQuery(database, user)).Return(result, nil)
	conn.On("Exec", createUserQuery(user, password, false)).Return(result, nil)
	conn.On("Exec", changeUserPassword(user, password)).Return(result, nil)
	rows.On("Next").Return(true).Times(3)
	rows.On("Close").Return(nil)
	rows.On("Scan", mock.Anything).Return(nil)
	rows.On("Next").Return(false).Times(6)

	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)
	createdUser, err := sa.CreateUser(ctx, user, userCreateRequest)
	assert.Empty(t, err)
	fmt.Printf("CreateUser user name: %v\n", createdUser.Name)
	fmt.Printf("CreateUser user: %v\n", createdUser)
	assert.Equal(t, database, createdUser.Name)
	assert.Equal(t, []dao.DbResource{{Kind: DbKind, Name: database}, {Kind: UserKind, Name: user}}, createdUser.Resources)
	assert.Equal(t, dao.ConnectionProperties{
		"name":     database,
		"url":      fmt.Sprintf("clickhouse://%s:%d/%s", host, port, database),
		"host":     host,
		"port":     port,
		"username": user,
		"password": password,
		"role":     userRole,
	}, createdUser.ConnectionProperties)
}

func TestCreateUser_MultiusersTrue(t *testing.T) {
	ctx := context.TODO()

	//ro_user
	ca := new(ClickhouseServiceAdapterMock)
	conn := new(ClickHouseConnMock)
	rows := new(MockRows)
	result := new(MockResult)
	ca.On("GetConnection").Return(conn, nil)
	ca.On("GetConnectionToDb", database).Return(conn, nil)

	conn.On("Close").Return(nil)

	var err error
	userCreateRequest := dao.UserCreateRequest{DbName: database, Role: RORole, Password: password}

	ca.On("GetUser").Return(user)
	ca.On("GetHost").Return(host)
	ca.On("GetPort").Return(port)
	conn.On("Query", getUserQuery(user)).Return(rows, nil)
	conn.On("Query", getMetadataQuery()).Return(rows, nil)
	conn.On("Exec", GrantDictionaryReloadQuery(user)).Return(result, nil)
	//ROQuery
	conn.On("Exec", grantROQuery(database, user)).Return(result, nil)
	conn.On("Exec", createUserQuery(user, password, false)).Return(result, nil)
	conn.On("Exec", changeUserPassword(user, password)).Return(result, nil)
	rows.On("Next").Return(true).Times(3)
	rows.On("Close").Return(nil)
	rows.On("Scan", mock.Anything).Return(nil)
	rows.On("Next").Return(false).Times(6)

	mu := NewServiceAdapter(ca, apiVersion, roles, map[string]bool{FeatureMultiUsers: true}, false)
	// userCreateRequest = dao.UserCreateRequest{DbName: database, Role: RORole, Password: password}
	createdUser, err := mu.CreateUser(ctx, user, userCreateRequest)
	assert.Empty(t, err)
	fmt.Printf("CreateUser user name: %v\n", createdUser.Name)
	fmt.Printf("CreateUser user: %v\n", createdUser)
	assert.Equal(t, database, createdUser.Name)
	assert.Equal(t, []dao.DbResource{{Kind: DbKind, Name: database}, {Kind: UserKind, Name: user}}, createdUser.Resources)
	assert.Equal(t, dao.ConnectionProperties{
		"name":     database,
		"url":      fmt.Sprintf("clickhouse://%s:%d/%s", host, port, database),
		"host":     host,
		"port":     port,
		"username": user,
		"password": password,
		"role":     RORole,
	}, createdUser.ConnectionProperties)

	//RW_user
	ca = new(ClickhouseServiceAdapterMock)
	conn = new(ClickHouseConnMock)
	rows = new(MockRows)
	result = new(MockResult)
	ca.On("GetConnection").Return(conn, nil)
	ca.On("GetConnectionToDb", database).Return(conn, nil)

	conn.On("Close").Return(nil)
	userCreateRequest = dao.UserCreateRequest{DbName: database, Role: RWRole, Password: password}

	ca.On("GetUser").Return(user)
	ca.On("GetHost").Return(host)
	ca.On("GetPort").Return(port)
	conn.On("Query", getUserQuery(user)).Return(rows, nil)
	conn.On("Query", getMetadataQuery()).Return(rows, nil)
	conn.On("Exec", GrantDictionaryReloadQuery(user)).Return(result, nil)
	//RWQuery
	conn.On("Exec", grantRWQuery(database, user)).Return(result, nil)
	conn.On("Exec", createUserQuery(user, password, false)).Return(result, nil)
	conn.On("Exec", changeUserPassword(user, password)).Return(result, nil)
	rows.On("Next").Return(true).Times(3)
	rows.On("Close").Return(nil)
	rows.On("Scan", mock.Anything).Return(nil)
	rows.On("Next").Return(false).Times(6)

	mu = NewServiceAdapter(ca, apiVersion, roles, map[string]bool{FeatureMultiUsers: true}, false)
	// userCreateRequest = dao.UserCreateRequest{DbName: database, Role: "rw", Password: password}
	createdUser, err = mu.CreateUser(ctx, user, userCreateRequest)
	assert.Empty(t, err)
	fmt.Printf("CreateUser user name: %v\n", createdUser.Name)
	fmt.Printf("CreateUser user: %v\n", createdUser)
	assert.Equal(t, database, createdUser.Name)
	assert.Equal(t, []dao.DbResource{{Kind: DbKind, Name: database}, {Kind: UserKind, Name: user}}, createdUser.Resources)
	assert.Equal(t, dao.ConnectionProperties{
		"name":     database,
		"url":      fmt.Sprintf("clickhouse://%s:%d/%s", host, port, database),
		"host":     host,
		"port":     port,
		"username": user,
		"password": password,
		"role":     RWRole,
	}, createdUser.ConnectionProperties)

}

func Test_GetDatabases(t *testing.T) {
	expectedDBNames := []string{"test_db_1", "test_db_2"}
	ctx := context.TODO()

	ca := new(ClickhouseServiceAdapterMock)
	conn := new(ClickHouseConnMock)
	rows := new(MockRows)
	ca.On("GetConnection").Return(conn, nil)

	conn.On("Query", getDatabasesQuery()).Return(rows, nil)
	conn.On("Close").Return(nil)

	rows.On("Next").Return(true).Times(2)
	rows.On("Next").Return(false).Once()
	rows.On("Close").Return(nil)

	for _, roles := range expectedDBNames {
		roles := roles
		rows.On("Scan", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			arg := args.Get(0).([]interface{})
			strArg := arg[0].(*string)
			*strArg = roles
		}).Once()
	}

	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)
	databases := sa.GetDatabases(ctx)
	assert.ElementsMatch(t, expectedDBNames, databases)
}

func TestGetDefaultCreateRequest(t *testing.T) {
	ca := new(ClickhouseServiceAdapterMock)
	//conn := new(ClickHouseConnMock)

	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)
	request := sa.GetDefaultCreateRequest()
	assert.Equal(t, dao.DbCreateRequest{}, request)

}

func TestDescribeDatabases(t *testing.T) {
	ctx := context.TODO()

	ca := new(ClickhouseServiceAdapterMock)
	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)
	databaseDesc := sa.DescribeDatabases(ctx, []string{database}, false, false)
	fmt.Printf("databaseDesc: %v\n", databaseDesc)
	assert.NotNil(t, databaseDesc)

}

func TestCreateDatabaseNilPrefix(t *testing.T) {

	ctx := context.TODO()

	userName := "dbaas_user0"
	userPassword := "password0"

	dbName := "test_database_name"

	namespace := "test-namespace"
	msName := "test-microservice"

	classifier := map[string]interface{}{nsKey: namespace, msKey: msName}

	genDBNameWithoutTS := fmt.Sprintf("%s_%s", msName, namespace)
	dbArgMatcher := mock.MatchedBy(func(dbName string) bool { return strings.Contains(dbName, genDBNameWithoutTS) })

	metadata := map[string]interface{}{
		"roles": map[string]string{
			"dbaas_user0": "admin",
		},
		cKey: classifier,
	}

	metadataJson, errParse := json.Marshal(metadata)
	if errParse != nil {
		log.Error("Error during marshal metadata")
		return
	}

	ca := new(ClickhouseServiceAdapterMock)
	ac := new(ClickHouseConnMock)

	result := new(MockResult)
	generator := new(GeneratorMock)

	ca.On("GetConnection").Return(ac, nil)
	ca.On("GetConnectionToDb", dbArgMatcher).Return(ac, nil)
	ca.On("GetConnectionToDbWithUser", dbArgMatcher, userName, userPassword).Return(ac, nil)

	request := dao.DbCreateRequest{
		Metadata: metadata, NamePrefix: nil, Password: userPassword,
		DbName: dbName, Settings: map[string]interface{}{},
		Username: userName,
	}

	ac.On("Close").Return(nil)
	ca.On("GetHost").Return(host)
	ca.On("GetPort").Return(port)

	// generate names
	for i := range roles {
		generator.On("Generate").Return(fmt.Sprintf("user%d", i)).Once()
		generator.On("Generate").Return(fmt.Sprintf("password%d", i)).Once()
	}

	ac.On("Exec", mock.MatchedBy(func(query string) bool {
		genEmpty := createDatabaseQuery("")
		queryArr := strings.Split(query, "\"")
		emptyQueryArr := strings.Split(genEmpty, "\"")
		return queryArr[0] == emptyQueryArr[0] && strings.Contains(queryArr[1], genDBNameWithoutTS)
	})).Return(result, nil)

	ac.On("Exec", createUserQuery(userName, userPassword, false)).Return(result, nil)

	ac.On("Exec", mock.MatchedBy(func(query string) bool {
		genEmpty := grantAdminQuery("", userName)
		queryArr := strings.Split(query, "\"")
		emptyQueryArr := strings.Split(genEmpty, "\"")
		return queryArr[0] == emptyQueryArr[0] && strings.Contains(queryArr[1], genDBNameWithoutTS)
	})).Return(result, nil)
	ac.On("Exec", GrantAdminRemoteQuery(userName)).Return(result, nil)
	ac.On("Exec", GrantDictionaryReloadQuery(userName)).Return(result, nil)
	ac.On("Exec", createMetadataQuery()).Return(result, nil)
	ac.On("Exec", insertMetadataQuery(string(metadataJson))).Return(result, nil)

	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)
	sa.Generator = generator

	dbNameresult, dbDesc, err := sa.CreateDatabase(ctx, request)
	assert.Nil(t, err)
	assert.Contains(t, dbNameresult, genDBNameWithoutTS)

	cp := dbDesc.ConnectionProperties[0]

	expCp := dao.ConnectionProperties{
		"host":     host,
		"name":     cp["name"],
		"url":      fmt.Sprintf("clickhouse://%s:%d/%s", host, port, cp["name"]),
		"port":     port,
		"username": userName,
		"password": userPassword,
		"role":     AdminRole,
	}

	assert.Equal(t, expCp, cp)
}

func TestCreateDatabaseMultiuser(t *testing.T) {

	ctx := context.TODO()

	userName := "user"
	userPassword := "password"

	dbName := "test_database_name"

	namespace := "test-namespace"
	msName := "test-microservice"

	classifier := map[string]interface{}{nsKey: namespace, msKey: msName}

	genDBNameWithoutTS := fmt.Sprintf("%s_%s", msName, namespace)
	dbArgMatcher := mock.MatchedBy(func(dbName string) bool { return strings.Contains(dbName, genDBNameWithoutTS) })

	metadata := map[string]interface{}{
		"roles": map[string]string{
			"dbaas_user0": "admin",
			"dbaas_user1": "rw",
			"dbaas_user2": "ro",
		},
		cKey: classifier,
	}

	metadataJson, errParse := json.Marshal(metadata)
	if errParse != nil {
		log.Error("Error during marshal metadata")
		return
	}

	ca := new(ClickhouseServiceAdapterMock)
	ac := new(ClickHouseConnMock)

	result := new(MockResult)
	generator := new(GeneratorMock)
	ca.On("GetConnection").Return(ac, nil)
	ca.On("GetConnectionToDb", dbArgMatcher).Return(ac, nil)
	ca.On("GetConnectionToDbWithUser", dbArgMatcher, userName, userPassword).Return(ac, nil)

	request := dao.DbCreateRequest{
		Metadata: metadata, NamePrefix: nil, Password: userPassword,
		DbName: dbName, Settings: map[string]interface{}{},
		Username: "",
	}

	ac.On("Close").Return(nil)
	ca.On("GetHost").Return(host)
	ca.On("GetPort").Return(port)

	ac.On("Exec", mock.MatchedBy(func(query string) bool {
		genEmpty := createDatabaseQuery("")
		queryArr := strings.Split(query, "\"")
		emptyQueryArr := strings.Split(genEmpty, "\"")
		return queryArr[0] == emptyQueryArr[0] && strings.Contains(queryArr[1], genDBNameWithoutTS)
	})).Return(result, nil)

	// generate names
	for i := range roles {
		generator.On("Generate").Return(fmt.Sprintf("user%d", i)).Once()
		generator.On("Generate").Return(fmt.Sprintf("password%d", i)).Once()

		userName, password := fmt.Sprintf("dbaas_user%d", i), fmt.Sprintf("password%d", i)

		ac.On("Exec", createUserQuery(userName, password, false)).Return(result, nil)

		ac.On("Exec", mock.MatchedBy(func(query string) bool {
			genEmpty := grantAdminQuery("", userName)
			queryArr := strings.Split(query, "\"")
			emptyQueryArr := strings.Split(genEmpty, "\"")
			return queryArr[0] == emptyQueryArr[0] && strings.Contains(queryArr[1], genDBNameWithoutTS)
		})).Return(result, nil)

		ac.On("Exec", GrantAdminRemoteQuery(userName)).Return(result, nil)
		ac.On("Exec", GrantDictionaryReloadQuery(userName)).Return(result, nil)

		ac.On("Exec", mock.MatchedBy(func(query string) bool {
			genEmpty := grantROQuery("", userName)
			queryArr := strings.Split(query, "\"")
			emptyQueryArr := strings.Split(genEmpty, "\"")
			return queryArr[0] == emptyQueryArr[0] && strings.Contains(queryArr[1], genDBNameWithoutTS)
		})).Return(result, nil)

		ac.On("Exec", mock.MatchedBy(func(query string) bool {
			genEmpty := grantRWQuery("", userName)
			queryArr := strings.Split(query, "\"")
			emptyQueryArr := strings.Split(genEmpty, "\"")
			return queryArr[0] == emptyQueryArr[0] && strings.Contains(queryArr[1], genDBNameWithoutTS)
		})).Return(result, nil)

		ac.On("Exec", createMetadataQuery()).Return(result, nil)
		ac.On("Exec", insertMetadataQuery(string(metadataJson))).Return(result, nil)
	}
	sa := NewServiceAdapter(ca, apiVersion, roles, map[string]bool{FeatureMultiUsers: true}, false)
	sa.Generator = generator

	dbNameresult, dbDesc, err := sa.CreateDatabase(ctx, request)
	assert.Nil(t, err)
	assert.Contains(t, dbNameresult, genDBNameWithoutTS)

	for i, r := range roles {
		found := false
		for _, cp := range dbDesc.ConnectionProperties {
			if cp["role"] != r {
				continue
			}
			found = true
			expCp := dao.ConnectionProperties{
				"host":     host,
				"name":     cp["name"],
				"url":      fmt.Sprintf("clickhouse://%s:%d/%s", host, port, cp["name"]),
				"port":     port,
				"username": fmt.Sprintf("dbaas_user%d", i),
				"password": fmt.Sprintf("password%d", i),
				"role":     r,
			}
			assert.Equal(t, expCp, cp)
			assert.Contains(t, cp["name"], genDBNameWithoutTS)
		}
		assert.True(t, found)
	}
}

func TestUpdateMetadata(t *testing.T) {
	ctx := context.TODO()

	ca := new(ClickhouseServiceAdapterMock)

	conn := new(ClickHouseConnMock)

	ca.On("GetConnectionToDb", database).Return(conn, nil)
	conn.On("Close").Return(nil)

	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)
	result := new(MockResult)
	metadata := map[string]interface{}{"test": "data"}
	conn.On("Exec", createMetadataQuery()).Return(new(MockResult), nil).Once()
	conn.On("Exec", deleteMetadataQuery()).Return(new(MockResult), nil).Once()

	metadataJson, _ := json.Marshal(metadata)
	conn.On("Exec", insertMetadataQuery(string(metadataJson))).Return(result, nil).Once()

	sa.UpdateMetadata(ctx, metadata, database)
	conn.AssertNumberOfCalls(t, "Exec", 3)
}

func TestUpdateMetadata_PanicOnError(t *testing.T) {
	ctx := context.TODO()
	ca := new(ClickhouseServiceAdapterMock)
	conn := new(ClickHouseConnMock)

	ca.On("GetConnectionToDb", database).Return(conn, nil)
	conn.On("Close").Return(nil)

	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)
	result := new(MockResult)
	metadata := map[string]interface{}{"test": "data"}

	expectedErrorCreate := errors.New("unit testing error while execute createMetadata query")
	conn.On("Exec", createMetadataQuery()).Return(result, expectedErrorCreate).Once()
	assert.PanicsWithError(t, expectedErrorCreate.Error(), func() {
		sa.UpdateMetadata(ctx, metadata, database)
	})
	conn.On("Exec", createMetadataQuery()).Return(result, nil).Once()

	conn.On("Exec", deleteMetadataQuery()).Return(result, nil).Once()

	metadataJson, _ := json.Marshal(metadata)
	expectedErrorInsert := errors.New("unit testing error while updating metadata")
	conn.On("Exec", insertMetadataQuery(string(metadataJson))).Return(result, expectedErrorInsert).Once()

	assert.PanicsWithError(t, expectedErrorInsert.Error(), func() {
		sa.UpdateMetadata(ctx, metadata, database)
	})
}

func Test_GetMetadataInternal(t *testing.T) {

	ctx := context.TODO()

	ca := new(ClickhouseServiceAdapterMock)
	conn := new(ClickHouseConnMock)
	rows := new(MockRows)

	metadata := map[string]interface{}{
		"roles1": "test1",
		"roles2": "test2",
	}
	ca.On("GetConnectionToDb", database).Return(conn, nil)
	conn.On("Close").Return(nil)
	conn.On("Query", getMetadataQuery()).Return(rows, nil)

	rows.On("Close").Return(nil)
	rows.On("Next").Return(true).Times(2)

	rows.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]interface{})
		strArg := arg[0].(*string)
		metadataJson, _ := json.Marshal(metadata)
		*strArg = string(metadataJson)
	}).Return(nil)

	rows.On("Next").Return(false).Times(1)

	sa := NewServiceAdapter(ca, apiVersion, roles, features, false)

	data, err := sa.GetMetadataInternal(ctx, database)
	assert.NoError(t, err)
	assert.Equal(t, metadata, data)
}
