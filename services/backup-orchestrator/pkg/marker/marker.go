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
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/utils"
	"go.uber.org/zap"
)

const (
	markerSentinel = "current"
	markerDatabase = "default"

	createTableSQL = `
CREATE TABLE IF NOT EXISTS default.backup_restore_markers ON CLUSTER '{cluster}' (
	sentinel String,
	marker   String,
	written_at DateTime DEFAULT now()
) ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/default/backup_restore_markers/{uuid}', '{replica}')
ORDER BY sentinel`

	upsertSQL = `INSERT INTO default.backup_restore_markers (sentinel, marker) VALUES (?, ?)`

	selectSQL = `SELECT marker FROM default.backup_restore_markers FINAL WHERE sentinel = ? LIMIT 1`
)

var log = utils.GetLogger()

// Set writes the marker value into ClickHouse-native storage.
// Called by the base backup daemon via marker_set_cmd.
func Set(marker string) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err = db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("create marker table: %w", err)
	}
	if _, err = db.Exec(upsertSQL, markerSentinel, marker); err != nil {
		return fmt.Errorf("write marker: %w", err)
	}
	log.Info("marker written to ClickHouse", zap.String("marker", marker))
	return nil
}

// Get reads the current marker from ClickHouse-native storage and prints it to stdout.
// Called by the base backup daemon via marker_get_cmd; stdout is the return value.
func Get() (string, error) {
	db, err := openDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	var value string
	row := db.QueryRow(selectSQL, markerSentinel)
	if err = row.Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("read marker: %w", err)
	}
	return value, nil
}

func openDB() (*sql.DB, error) {
	host := fmt.Sprintf("clickhouse-cluster.%s", utils.GetNameSpace())
	port := utils.GetDbPort()

	db := clickhouse.OpenDB(&clickhouse.Options{
		Addr: []string{host + ":" + port},
		Auth: clickhouse.Auth{
			Database: markerDatabase,
			Username: utils.GetClickhouseUserName(),
			Password: utils.GetClusterPassword(),
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 10 * time.Second,
		TLS:         utils.GetTlsConfig(),
	})

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("connect to ClickHouse: %w", err)
	}
	return db, nil
}
