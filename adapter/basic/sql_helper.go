package basic

import (
	"fmt"
	"strings"
)

const (
	DbPrefixPattern      = `^(_|[a-z])[\w\d_]{0,29}$`
	DbIdentifiersPattern = `^(_|[a-z])[\w\d_-]{0,62}$`
)

func GrantDictionaryReloadQuery(user string) string {
	return fmt.Sprintf("GRANT ON CLUSTER '{cluster}' SYSTEM RELOAD DICTIONARY ON *.* to \"%s\"", escapeString(user))
}

func GetAdminUserQuery(database string) string {
	return fmt.Sprintf("SELECT cast(user_name, 'String') FROM system.grants WHERE database='%s' and access_type='TRUNCATE' and user_name like 'dbaas_%%';", escapeString(database))
}

func GetRWUserQuery(database string) string {
	return fmt.Sprintf("SELECT user_name FROM system.grants WHERE database='%s' and access_type='ALTER UPDATE' and user_name like 'dbaas_%%' and user_name not in (%s);", escapeString(database), GetAdminUserQuery(database))
}

func GetROUserQuery(database string) string {
	return fmt.Sprintf("SELECT user_name FROM system.grants WHERE database='%s' and access_type='SELECT' and user_name like 'dbaas_%%' and user_name not in (%s) and user_name not in (%s);", escapeString(database), GetAdminUserQuery(database), GetRWUserQuery(database))
}

func createDatabaseQuery(dbName string) string {
	return fmt.Sprintf("create database \"%s\" on cluster '{cluster}'", escapeString(dbName))
}

func createUserQuery(username, password string) string {
	return fmt.Sprintf("CREATE USER \"%s\" ON CLUSTER '{cluster}' IDENTIFIED WITH sha256_password BY '%s'", escapeString(username), escapeString(password))
}

func grantAdminQuery(dbName, user string) string {
	return fmt.Sprintf("GRANT ON CLUSTER '{cluster}' ALL ON \"%s\".* TO \"%s\"", escapeString(dbName), escapeString(user))
}

func grantROQuery(dbName, user string) string {
	return fmt.Sprintf("GRANT ON CLUSTER '{cluster}' SELECT ON \"%s\".* TO \"%s\"", escapeString(dbName), escapeString(user))
}

func grantRWQuery(dbName, user string) string {
	return fmt.Sprintf("GRANT ON CLUSTER '{cluster}' SELECT, INSERT, ALTER UPDATE, ALTER DELETE ON \"%s\".* TO \"%s\"", escapeString(dbName), escapeString(user)) //TODO CHECK
}

func createMetadataQuery() string {
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s ON CLUSTER '{cluster}' (key String, value String) ENGINE = ReplicatedMergeTree() ORDER BY key", DBaaSMetadata)
}

func insertMetadataQuery(metadata string) string {
	return fmt.Sprintf("INSERT INTO %s values('%s','%s')", DBaaSMetadata, Metadata, escapeString(metadata))
}

func getMetadataQuery() string {
	return fmt.Sprintf("SELECT %s from %s", Metadata, DBaaSMetadata)
}

func dropDatabaseQuery(dbName string) string {
	return fmt.Sprintf("DROP DATABASE IF EXISTS \"%s\" ON CLUSTER '{cluster}' SYNC", escapeString(dbName))
}

func dropUserQuery(user string) string {
	return fmt.Sprintf("DROP USER IF EXISTS \"%s\" ON CLUSTER '{cluster}'", escapeString(user))
}

func getDatabasesQuery() string {
	return "SELECT name FROM system.databases"
}

func getUserQuery(user string) string {
	return fmt.Sprintf("SELECT name FROM system.users where name='%s'", escapeString(user))
}

func changeUserPassword(user, password string) string {
	return fmt.Sprintf("ALTER USER \"%s\" IDENTIFIED WITH sha256_password BY '%s'", escapeString(user), escapeString(password))
}

func deleteMetadataQuery() string {
	return fmt.Sprintf("ALTER TABLE %s ON CLUSTER '{cluster}' DELETE WHERE key='%s'", DBaaSMetadata, Metadata)
}

func escapeString(input string) string {
	res := strings.ReplaceAll(input, `'`, `''`)
	return strings.ReplaceAll(res, `"`, `""`)
}

func GrantAdminRemoteQuery(user string) string {
	return fmt.Sprintf("GRANT ON CLUSTER '{cluster}' REMOTE ON  *.*  TO \"%s\"", escapeString(user))
}
