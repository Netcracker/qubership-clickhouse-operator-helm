CREATE TABLE IF NOT EXISTS prom.samples_replacing ON CLUSTER '{cluster}'
(
    `date`        Date,
    `timestamp`   DateTime,
    `fingerprint` FixedString(32),
    `value`       Float64,
    `insert_timestamp` DateTime DEFAULT now()
)
    ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/1/samples_replacing', '{replica}', insert_timestamp)
        PARTITION BY date
        ORDER BY (fingerprint, date, timestamp)
        $TTL_SETTING
        SETTINGS index_granularity = 8192;