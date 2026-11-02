-- name: IncrementStat :exec
INSERT INTO stats (stat_kind, stat_count)
VALUES ($1, $2)
ON CONFLICT (stat_kind) DO UPDATE
SET stat_count = stats.stat_count + EXCLUDED.stat_count;

-- name: ListStats :many
SELECT stat_kind, stat_count
FROM stats;

-- name: InventoryCounts :one
SELECT
    (SELECT count(*) FROM meetup_templates)::bigint AS templates,
    (SELECT count(*) FROM meetup_templates WHERE template_published_at IS NOT NULL)::bigint AS templates_published,
    (SELECT count(*) FROM meetup_draft)::bigint AS drafts;
