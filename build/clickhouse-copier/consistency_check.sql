WITH
    (SELECT min(timestamp) FROM prom.samples) AS min_date,
    (SELECT max(timestamp) FROM prom.samples) AS max_date,
    samples AS (
        SELECT
            toStartOfDay(timestamp) AS day,
            count()                 AS count
        FROM
            prom.samples
        WHERE
                timestamp >= min_date
          AND timestamp <= max_date
        GROUP BY day
        ORDER BY day),
    samples_replacing AS (
        SELECT
            toStartOfDay(timestamp) AS day,
            count()                 AS count
        FROM
            prom.samples_replacing
        WHERE
                timestamp >= min_date
          AND timestamp <= max_date
        GROUP BY day
        ORDER BY day
    )
SELECT
    samples.day,
    samples.count as samples_count,
    samples_replacing.count as samples_replacing_count,
    samples_replacing_count/samples_count as consistency
FROM samples JOIN samples_replacing ON samples.day = samples_replacing.day;
