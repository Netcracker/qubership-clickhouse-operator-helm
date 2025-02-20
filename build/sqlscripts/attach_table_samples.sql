ATTACH TABLE IF NOT EXISTS prom.samples
(
    `date`        Date,
    `timestamp`   DateTime,
    `fingerprint` FixedString(32),
    `value`       Float64
)
    ENGINE = ReplicatedMergeTree('/clickhouse/tables/1/samples', '{replica}')
        PARTITION BY date
        ORDER BY (fingerprint, date, timestamp)
        TTL date + INTERVAL $TTL
        SETTINGS index_granularity = 8192;
