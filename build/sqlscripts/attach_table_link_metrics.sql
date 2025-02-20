ATTACH TABLE IF NOT EXISTS prom.link_metrics
(
    `date`          Date,
    `timestamp`     DateTime,
    `link_id`       String,
    `link_name`     String,
    `groupId`       String,
    `entryType`     String,
    `aLocationName` String,
    `zLocationName` String,
    `aLocationId`   String,
    `zLocationId`   String,
    `bandwidth`     Float64,
    `utilization`   Float64,
    `azBitrate`     Float64,
    `zaBitrate`     Float64
) ENGINE = ReplicatedMergeTree('/clickhouse/tables/1/linkMetrics','{replica}')
    PARTITION BY date
    ORDER BY (date, timestamp)
    TTL timestamp + INTERVAL $TTL
    SETTINGS index_granularity = 8192
