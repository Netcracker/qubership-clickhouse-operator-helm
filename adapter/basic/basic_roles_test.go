package basic

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/dao"
	"github.com/stretchr/testify/assert"
)

func Test_SupportedRoles(t *testing.T) {
	cl := new(ClickhouseServiceAdapterMock)

	// check multiusers
	feature := map[string]bool{FeatureMultiUsers: true}
	sa := NewServiceAdapter(cl, apiVersion, roles, feature)

	expectedRoles := roles
	supportedRoles := sa.GetSupportedRoles()

	assert.ElementsMatch(t, expectedRoles, supportedRoles)

	// check singleuser
	features := map[string]bool{FeatureMultiUsers: false}
	sa = NewServiceAdapter(cl, apiVersion, roles, features)

	expectedRoles = []string{"admin"}
	supportedRoles = sa.GetSupportedRoles()

	assert.ElementsMatch(t, expectedRoles, supportedRoles)
}

func Test_GetVersion(t *testing.T) {
	cl := new(ClickhouseServiceAdapterMock)

	// check multiusers
	feature := map[string]bool{FeatureMultiUsers: true}
	sa := NewServiceAdapter(cl, apiVersion, roles, feature)

	expectedversion := dao.ApiVersion("v2")
	version := sa.GetVersion()

	assert.Equal(t, expectedversion, version)

}

func Test_GetFeatures(t *testing.T) {
	cl := new(ClickhouseServiceAdapterMock)

	// check multiusers
	features := map[string]bool{FeatureMultiUsers: true}
	sa := NewServiceAdapter(cl, apiVersion, roles, features)

	expectedFeatures := features
	supportedFeatures := sa.GetFeatures()
	fmt.Printf("supportedFeatures: %v\n", supportedFeatures)

	if !reflect.DeepEqual(expectedFeatures, supportedFeatures) {
		t.Errorf("Expected features %v, but got %v", expectedFeatures, supportedFeatures)
	}
}

func TestCreateRoles(t *testing.T) {
	ca := new(ClickhouseServiceAdapterMock)
	conn := new(ClickHouseConnMock)
	result := new(MockResult)
	rows := new(MockRows)
	userex := "dbaas_testuser"
	passwordex := "testuser"
	ca.On("GetConnection").Return(conn, nil)
	conn.On("Close").Return(nil)

	ca.On("GetUser").Return(user)
	ca.On("GetHost").Return(host)
	ca.On("GetPort").Return(port)
	generator := new(GeneratorMock)
	generator.On("Generate").Return(user)
	conn.On("Query", getUserQuery(userex)).Return(rows, nil)
	rows.On("Close").Return(nil)
	rows.On("Next").Return(true).Times(3)

	conn.On("Exec", grantRWQuery(database, userex)).Return(result, nil)
	conn.On("Exec", GrantDictionaryReloadQuery(userex)).Return(result, nil)
	conn.On("Exec", GrantAdminRemoteQuery(userex)).Return(result, nil)
	conn.On("Exec", grantAdminQuery(database, userex)).Return(result, nil)
	conn.On("Exec", changeUserPassword(userex, passwordex)).Return(result, nil)

	sa := NewServiceAdapter(ca, apiVersion, roles, map[string]bool{FeatureMultiUsers: true})
	sa.Generator = generator
	rolesReq := []dao.AdditionalRole{{Id: "123", DbName: database, ConnectionProperties: []dao.ConnectionProperties{{
		"name":     database,
		"url":      fmt.Sprintf("clickhouse://%s:%d/%s", host, port, database),
		"host":     host,
		"port":     port,
		"username": user,
		"password": password,
		"role":     RORole,
	}},
		Resources: []dao.DbResource{{Kind: DbKind, Name: database}, {Kind: UserKind, Name: user}},
	}}

	succ, failure := sa.CreateRoles(context.TODO(), rolesReq)
	fmt.Printf("CreateRoles succ: %v\n", succ)
	fmt.Printf("CreateRoles failure: %v\n", failure)
	assert.Equal(t, 2, len(succ[0].ConnectionProperties))
	assert.Equal(t, 4, len(succ[0].Resources))
	assert.Nil(t, failure)

}

func Test_CreateRoles_Failed(t *testing.T) {
	ca := new(ClickhouseServiceAdapterMock)
	conn := new(ClickHouseConnMock)
	result := new(MockResult)
	rows := new(MockRows)
	userex := "dbaas_testuser"
	passwordex := "testuser"
	ca.On("GetConnection").Return(conn, nil)
	conn.On("Close").Return(nil)

	ca.On("GetUser").Return(user)
	ca.On("GetHost").Return(host)
	ca.On("GetPort").Return(port)
	generator := new(GeneratorMock)
	generator.On("Generate").Return(user)
	conn.On("Query", getUserQuery(userex)).Return(rows, nil)
	rows.On("Close").Return(nil)
	rows.On("Next").Return(true).Times(3)
	expectedErrorC := errors.New("can't set database grants")
	conn.On("Exec", grantRWQuery(database, userex)).Return(result, expectedErrorC)
	conn.On("Exec", GrantDictionaryReloadQuery(userex)).Return(result, nil)
	conn.On("Exec", GrantAdminRemoteQuery(userex)).Return(result, expectedErrorC)
	conn.On("Exec", grantAdminQuery(database, userex)).Return(result, expectedErrorC)
	conn.On("Exec", changeUserPassword(userex, passwordex)).Return(result, nil)

	sa := NewServiceAdapter(ca, apiVersion, roles, map[string]bool{FeatureMultiUsers: true})
	sa.Generator = generator
	rolesReq := []dao.AdditionalRole{{Id: "123", DbName: database, ConnectionProperties: []dao.ConnectionProperties{{
		"name":     database,
		"url":      fmt.Sprintf("clickhouse://%s:%d/%s", host, port, database),
		"host":     host,
		"port":     port,
		"username": user,
		"password": password,
		"role":     RORole,
	}},
		Resources: []dao.DbResource{{Kind: DbKind, Name: database}, {Kind: UserKind, Name: user}},
	}}
	succ, failure := sa.CreateRoles(context.TODO(), rolesReq)
	fmt.Printf("CreateRoles succ: %v\n", succ)
	fmt.Printf("CreateRoles failure: %v\n", failure)
	assert.NotEmpty(t, succ)
	assert.NotNil(t, failure)
	assert.Empty(t, succ[0].ConnectionProperties)
	assert.Empty(t, succ[0].Resources)
	assert.Equal(t, "error during roles creation", failure.Message)

}
