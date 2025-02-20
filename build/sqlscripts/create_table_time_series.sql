CREATE TABLE IF NOT EXISTS prom.time_series
(
    `date`        Date,
    `fingerprint` FixedString(32),
    `uid`         UUID,
    `metric`      String,
    `labels`      String,
    `job`         String
)
    ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/1/time_series', '{replica}')
        PARTITION BY date
        PRIMARY KEY (uid, metric, fingerprint)
        ORDER BY (uid, metric, fingerprint)
        TTL date + INTERVAL $TTL
        SETTINGS index_granularity = 8192;