#!/bin/bash

echo "[$(date +%Y-%m-%dT%H:%M:%S)] Starting custom entrypoint .."
if [ "$(id -u)" = "0" ]
then
    echo "[$(date +%Y-%m-%dT%H:%M:%S)] Starting as root user, running custom bootstrap .."
    CLICKHOUSE_USER="${CLICKHOUSE_USER:-default}"
    CLICKHOUSE_PASSWORD="${CLICKHOUSE_PASSWORD:-}"
    CLICKHOUSE_CONFIG="${CLICKHOUSE_CONFIG:-/etc/clickhouse-server/config.xml}"
    if [ -n "$(ls /docker-entrypoint-initdb.d/)" ] || [ -n "$CLICKHOUSE_DB" ]; then
        # port is needed to check if clickhouse-server is ready for connections
        HTTP_PORT="$(clickhouse extract-from-config --config-file "$CLICKHOUSE_CONFIG" --key=http_port)"

        # Listen only on localhost until the initialization is done
        /usr/bin/clickhouse-server --config-file="$CLICKHOUSE_CONFIG" -- --listen_host=127.0.0.1 &
        pid="$!"

        # check if clickhouse is ready to accept connections
        # will try to send ping clickhouse via http_port (max 12 retries by default, with 1 sec timeout and 1 sec delay between retries)
        tries=${CLICKHOUSE_INIT_TIMEOUT:-12}
        while ! wget --spider -T 1 -q "http://127.0.0.1:$HTTP_PORT/ping" 2>/dev/null; do
            if [ "$tries" -le "0" ]; then
                echo >&2 'ClickHouse init process failed.'
                exit 1
            fi
            tries=$(( tries-1 ))
            sleep 1
        done

        clickhouseclient=( clickhouse-client --multiquery --host "127.0.0.1" -u "$CLICKHOUSE_USER" --password "$CLICKHOUSE_PASSWORD" )

        echo

        # create default database, if defined
        if [ -n "$CLICKHOUSE_DB" ]; then
            echo "$0: create database '$CLICKHOUSE_DB'"
            "${clickhouseclient[@]}" -q "CREATE DATABASE IF NOT EXISTS $CLICKHOUSE_DB";
        fi

        for f in /docker-entrypoint-initdb.d/*; do
            case "$f" in
                *.sh)
                    if [ -x "$f" ]; then
                        echo "$0: running $f"
                        "$f"
                    else
                        echo "$0: sourcing $f"
                        # shellcheck source=/dev/null
                        . "$f"
                    fi
                    ;;
                *.sql)    echo "$0: running $f"; "${clickhouseclient[@]}" < "$f" ; echo ;;
                *.sql.gz) echo "$0: running $f"; gunzip -c "$f" | "${clickhouseclient[@]}"; echo ;;
                *)        echo "$0: ignoring $f" ;;
            esac
            echo
        done

        if ! kill -s TERM "$pid" || ! wait "$pid"; then
            echo >&2 'Finishing of ClickHouse init process failed.'
            exit 1
        fi

        if [[ $# -lt 1 ]] || [[ "$1" == "--"* ]]; then
            # Watchdog is launched by default, but does not send SIGINT to the main process,
            # so the container can't be finished by ctrl+c
            CLICKHOUSE_WATCHDOG_ENABLE=${CLICKHOUSE_WATCHDOG_ENABLE:-0}
            export CLICKHOUSE_WATCHDOG_ENABLE
            exec /usr/bin/clickhouse-server --config-file="$CLICKHOUSE_CONFIG" "$@"
        fi
    fi
else
  echo "[$(date +%Y-%m-%dT%H:%M:%S)] Starting as non-root user, omitting custom bootstrap .."
  /entrypoint.sh
fi