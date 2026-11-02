-- Global action counters. No per-user rows.

CREATE TABLE IF NOT EXISTS stats
(
    stat_kind  VARCHAR PRIMARY KEY,
    stat_count BIGINT NOT NULL DEFAULT 0
);
