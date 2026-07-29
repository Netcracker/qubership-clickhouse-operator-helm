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

package marker

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/utils"
	"go.uber.org/zap"
)

var log = utils.GetLogger()

const (
	markerSentinel = "current"
	markerDatabase = "default"
	createTableSQL = `
CREATE TABLE IF NOT EXISTS default.backup_restore_markers ON CLUSTER '{cluster}'
(
	sentinel String,
	marker String,
	written_at DateTime DEFAULT now()
)
ENGINE = ReplicatedMergeTree(
	'/clickhouse/tables/{shard}/default/backup_restore_markers',
	'{replica}'
)
ORDER BY sentinel`

	deleteSQL = `
ALTER TABLE default.backup_restore_markers
DELETE WHERE sentinel = ?`

	insertSQL = `
INSERT INTO default.backup_restore_markers
(
	sentinel,
	marker,
	written_at
)
VALUES
(
	?,
	?,
	now()
)`

	selectSQL = `
SELECT marker
FROM default.backup_restore_markers
WHERE sentinel = ?
ORDER BY written_at DESC
LIMIT 1
SETTINGS select_sequential_consistency = 1`
)

var (
	dbInstance *sql.DB
	dbOnce     sync.Once

	tableOnce sync.Once
	tableErr  error
)

// Set writes marker
func Set(marker string) error {

	db, err := getDB()
	if err != nil {
		return err
	}

	// create table only once per pod
	tableOnce.Do(func() {
		_, tableErr = db.Exec(createTableSQL)
	})

	if tableErr != nil {
		return fmt.Errorf("create marker table: %w", tableErr)
	}

	// delete existing marker
	if _, err = db.Exec(deleteSQL, markerSentinel); err != nil {
		return fmt.Errorf("delete marker: %w", err)
	}

	// wait until mutation completes
	if err = waitForMutation(db); err != nil {
		return err
	}

	// insert new marker
	if _, err = db.Exec(insertSQL, markerSentinel, marker); err != nil {
		return fmt.Errorf("insert marker: %w", err)
	}

	log.Info(
		"marker written",
		zap.String("marker", marker),
	)

	return nil
}

// Wait for delete mutation
func waitForMutation(db *sql.DB) error {

	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)

	defer ticker.Stop()

	for {
		select {

		case <-timeout:
			return fmt.Errorf(
				"timeout waiting for delete mutation",
			)

		case <-ticker.C:

			var pending uint64

			err := db.QueryRow(`
SELECT count()
FROM system.mutations
WHERE database='default'
AND table='backup_restore_markers'
AND is_done=0
`).Scan(&pending)

			if err != nil {
				return fmt.Errorf(
					"check mutation: %w",
					err,
				)
			}

			if pending == 0 {
				return nil
			}
		}
	}
}

// Get marker
func Get() (string, error) {

	db, err := getDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	for i := 0; i < 10; i++ {

		var value string

		err = db.QueryRow(
			selectSQL,
			markerSentinel,
		).Scan(&value)

		if err == nil {
			return value, nil
		}

		if err == sql.ErrNoRows {
			time.Sleep(1 * time.Second)
			continue
		}

		return "", fmt.Errorf("read marker: %w", err)
	}

	return "", nil
}

// singleton DB connection
func getDB() (*sql.DB, error) {

	var err error

	dbOnce.Do(func() {

		host := fmt.Sprintf(
			"clickhouse-cluster.%s",
			utils.GetNameSpace(),
		)

		port := utils.GetDbPort()

		dbInstance = clickhouse.OpenDB(&clickhouse.Options{

			Addr: []string{
				host + ":" + port,
			},

			Auth: clickhouse.Auth{
				Database: markerDatabase,
				Username: utils.GetClickhouseUserName(),
				Password: utils.GetClusterPassword(),
			},

			Settings: clickhouse.Settings{
				"max_execution_time": 60,
			},

			DialTimeout: 10 * time.Second,

			TLS: utils.GetTlsConfig(),
		})

		err = dbInstance.Ping()
	})

	if err != nil {
		return nil, fmt.Errorf(
			"connect ClickHouse: %w",
			err,
		)
	}

	return dbInstance, nil
}
