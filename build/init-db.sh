#!/bin/bash
set -e

CLICKHOUSE_USER="${CH_USER:-default}"
CLICKHOUSE_PASSWORD="${CH_PASSWORD:-crknet}"

echo "[$(date +%Y-%m-%dT%H:%M:%S)] Starting initial db script..."

createDBquery=$(cat /opt/sqlscripts/create_db_prom.sql)

function process_table {
    if [ -d "/var/lib/clickhouse/data/prom/$1" ];
        then
            echo "[$(date +%Y-%m-%dT%H:%M:%S)] Table $1 exists and will be attached."
            processTableQuery=$(cat /opt/sqlscripts/attach_table_$1.sql)
            processTableQuery=${processTableQuery//\$TTL/$3}
        else
            echo "[$(date +%Y-%m-%dT%H:%M:%S)] Table $1 does not exist and will be created."
            processTableQuery=$(cat /opt/sqlscripts/create_table_$1.sql)
            processTableQuery=${processTableQuery//\$TTL/$3}
        fi
    echo "[$(date +%Y-%m-%dT%H:%M:%S)] Processing table $1."
    clickhouse-client -h 127.0.0.1 --user "${CLICKHOUSE_USER}" --password "${CLICKHOUSE_PASSWORD}" --query="$processTableQuery"
    clickhouse-client -h 127.0.0.1 --user "${CLICKHOUSE_USER}" --password "${CLICKHOUSE_PASSWORD}" --query="ALTER TABLE prom.$1 MODIFY TTL $2 + INTERVAL $3;"
}

if [[ "${RUN_MIGRATION}" =~ ^[Ff]alse$ ]]; then
    echo "[$(date +%Y-%m-%dT%H:%M:%S)] Run Migration set as ${RUN_MIGRATION}, skipping execution of sql init scripts, running clickhouse-copier task."
    /docker-entrypoint-initdb.d/clickhouse-copier.shell &
    return
fi

echo "[$(date +%Y-%m-%dT%H:%M:%S)] Running initial scripts..."
echo "[$(date +%Y-%m-%dT%H:%M:%S)] Creating db Prom."

clickhouse-client -h 127.0.0.1 --user "${CLICKHOUSE_USER}" --password "${CLICKHOUSE_PASSWORD}" --query="$createDBquery"

process_table samples date "$SAMPLES_TTL"
process_table time_series date "$TIME_SERIES_TTL"
process_table alarms raisedDate "$ALARMS_TTL"
process_table link_metrics timestamp "$LINK_METRICS_TTL"

echo "[$(date +%Y-%m-%dT%H:%M:%S)] Initialization finished."