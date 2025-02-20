ATTACH TABLE IF NOT EXISTS prom.alarms
(
    `raisedTime` DateTime,
    `clearedTime` Nullable(DateTime),
    `raisedDate` Date,
    `alarmName` String,
    `alarmId` String,
    `objectId` UUID,
    `objectName` String,
    `severity` String
)
    ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/1/alarms1', '{replica}')
        PARTITION BY raisedDate
        ORDER BY (raisedTime, alarmName, objectId, severity)
        TTL raisedDate + INTERVAL $TTL
        SETTINGS index_granularity = 8192;