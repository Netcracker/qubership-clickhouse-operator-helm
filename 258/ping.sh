#!/bin/bash
set -e

CONN_FILE=/tmp/connection

if [[ ! -f "$CONN_FILE" ]]; then
clickhouse-client --query "SELECT count(*) FROM system.zookeeper WHERE path = '/clickhouse'"
touch $CONN_FILE
fi

CODE=$(wget --server-response localhost:8123/ping 2>&1 -O- | awk '/^  HTTP/{print $2}')

if [[ $CODE != "200" ]]; then
exit 1
fi

exit 0