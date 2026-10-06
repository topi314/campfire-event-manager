-- name: ListTemplatesByUser :many
SELECT
    t.template_id,
    t.template_discord_user_id,
    t.template_name,
    t.template_payload,
    t.template_published_at,
    t.template_publish_description,
    t.template_language,
    t.template_origin_id,
    t.template_synced,
    t.template_created_at,
    t.template_updated_at,
    o.template_id AS origin_id,
    o.template_name AS origin_name,
    o.template_payload AS origin_payload,
    ou.discord_user_id AS origin_publisher_id,
    ou.discord_user_username AS origin_publisher_username,
    ou.discord_user_display_name AS origin_publisher_display_name,
    ou.discord_user_avatar_url AS origin_publisher_avatar_url
FROM meetup_templates t
LEFT JOIN meetup_templates o ON o.template_id = t.template_origin_id
LEFT JOIN discord_users ou ON ou.discord_user_id = o.template_discord_user_id
WHERE t.template_discord_user_id = $1
ORDER BY t.template_updated_at DESC;

-- name: GetTemplateForUser :one
SELECT
    t.template_id,
    t.template_discord_user_id,
    t.template_name,
    t.template_payload,
    t.template_published_at,
    t.template_publish_description,
    t.template_language,
    t.template_origin_id,
    t.template_synced,
    t.template_created_at,
    t.template_updated_at,
    o.template_id AS origin_id,
    o.template_name AS origin_name,
    o.template_payload AS origin_payload,
    ou.discord_user_id AS origin_publisher_id,
    ou.discord_user_username AS origin_publisher_username,
    ou.discord_user_display_name AS origin_publisher_display_name,
    ou.discord_user_avatar_url AS origin_publisher_avatar_url
FROM meetup_templates t
LEFT JOIN meetup_templates o ON o.template_id = t.template_origin_id
LEFT JOIN discord_users ou ON ou.discord_user_id = o.template_discord_user_id
WHERE t.template_id = $1 AND t.template_discord_user_id = $2;

-- name: GetTemplateByID :one
SELECT
    t.template_id,
    t.template_discord_user_id,
    t.template_name,
    t.template_payload,
    t.template_published_at,
    t.template_publish_description,
    t.template_language,
    t.template_origin_id,
    t.template_synced,
    t.template_created_at,
    t.template_updated_at
FROM meetup_templates t
WHERE t.template_id = $1;

-- name: CreateTemplate :one
INSERT INTO meetup_templates (
    template_discord_user_id,
    template_name,
    template_payload,
    template_origin_id,
    template_synced,
    template_language,
    template_created_at,
    template_updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING
    template_id,
    template_discord_user_id,
    template_name,
    template_payload,
    template_published_at,
    template_publish_description,
    template_language,
    template_origin_id,
    template_synced,
    template_created_at,
    template_updated_at;

-- name: UpdateTemplate :one
UPDATE meetup_templates
SET template_name = $1,
    template_payload = $2,
    template_language = $3,
    template_synced = FALSE,
    template_updated_at = $6
WHERE template_id = $4 AND template_discord_user_id = $5
RETURNING
    template_id,
    template_discord_user_id,
    template_name,
    template_payload,
    template_published_at,
    template_publish_description,
    template_language,
    template_origin_id,
    template_synced,
    template_created_at,
    template_updated_at;

-- name: PublishTemplate :one
UPDATE meetup_templates
SET template_published_at = COALESCE(template_published_at, sqlc.arg('now')),
    template_publish_description = sqlc.narg('description'),
    template_language = sqlc.narg('language'),
    template_updated_at = sqlc.arg('now')
WHERE template_id = sqlc.arg('id') AND template_discord_user_id = sqlc.arg('user_id')
RETURNING
    template_id,
    template_discord_user_id,
    template_name,
    template_payload,
    template_published_at,
    template_publish_description,
    template_language,
    template_origin_id,
    template_synced,
    template_created_at,
    template_updated_at;

-- name: UnpublishTemplate :one
UPDATE meetup_templates
SET template_published_at = NULL,
    template_updated_at = $3
WHERE template_id = $1 AND template_discord_user_id = $2
RETURNING
    template_id,
    template_discord_user_id,
    template_name,
    template_payload,
    template_published_at,
    template_publish_description,
    template_language,
    template_origin_id,
    template_synced,
    template_created_at,
    template_updated_at;

-- name: DeleteTemplate :execrows
DELETE FROM meetup_templates
WHERE template_id = $1 AND template_discord_user_id = $2;

-- name: ListPublishedTemplates :many
SELECT
    t.template_id,
    t.template_discord_user_id,
    t.template_name,
    t.template_payload,
    t.template_published_at,
    t.template_publish_description,
    t.template_language,
    t.template_origin_id,
    t.template_synced,
    t.template_created_at,
    t.template_updated_at,
    u.discord_user_id AS publisher_id,
    u.discord_user_username AS publisher_username,
    u.discord_user_display_name AS publisher_display_name,
    u.discord_user_avatar_url AS publisher_avatar_url,
    COALESCE((
        SELECT COUNT(*)::int FROM meetup_template_likes ml
        WHERE ml.template_id = t.template_id
    ), 0)::int AS like_count,
    (
        CASE
            WHEN sqlc.narg('viewer_id')::text IS NULL THEN FALSE
            ELSE EXISTS (
                SELECT 1 FROM meetup_template_likes ml
                WHERE ml.template_id = t.template_id
                  AND ml.discord_user_id = sqlc.narg('viewer_id')
            )
        END
    )::bool AS liked_by_me
FROM meetup_templates t
JOIN discord_users u ON u.discord_user_id = t.template_discord_user_id
WHERE t.template_published_at IS NOT NULL
  AND (
      sqlc.narg('query')::text IS NULL
      OR t.template_name ILIKE sqlc.narg('query')
      OR COALESCE(t.template_publish_description, '') ILIKE sqlc.narg('query')
  )
  AND (
      sqlc.narg('category')::text IS NULL
      OR COALESCE(t.template_payload->>'category', '') = sqlc.narg('category')
  )
  AND (
      sqlc.narg('language')::text IS NULL
      OR COALESCE(t.template_language, '') = sqlc.narg('language')
  )
  AND (
      sqlc.narg('publisher_id')::text IS NULL
      OR t.template_discord_user_id = sqlc.narg('publisher_id')
  )
  AND (
      sqlc.narg('publisher')::text IS NULL
      OR u.discord_user_username ILIKE sqlc.narg('publisher')
      OR u.discord_user_display_name ILIKE sqlc.narg('publisher')
  )
  AND (
      sqlc.narg('liked_by')::text IS NULL
      OR EXISTS (
          SELECT 1 FROM meetup_template_likes vl
          WHERE vl.template_id = t.template_id
            AND vl.discord_user_id = sqlc.narg('liked_by')
      )
  )
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'name_asc' THEN t.template_name END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'updated_desc' THEN t.template_updated_at END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'likes_desc' THEN (
      COALESCE((
          SELECT COUNT(*)::int FROM meetup_template_likes ml
          WHERE ml.template_id = t.template_id
      ), 0)
  ) END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'liked_desc' THEN (
      SELECT vl.liked_at FROM meetup_template_likes vl
      WHERE vl.template_id = t.template_id
        AND vl.discord_user_id = sqlc.narg('liked_by')
  ) END DESC NULLS LAST,
  CASE WHEN sqlc.arg('sort')::text IN ('published_desc', '') THEN t.template_published_at END DESC NULLS LAST,
  CASE WHEN sqlc.arg('sort')::text = 'name_asc' THEN t.template_id END ASC,
  t.template_id DESC;

-- name: LikeTemplate :execrows
INSERT INTO meetup_template_likes (template_id, discord_user_id, liked_at)
SELECT t.template_id, sqlc.arg('user_id'), (now() AT TIME ZONE 'utc')
FROM meetup_templates t
WHERE t.template_id = sqlc.arg('template_id') AND t.template_published_at IS NOT NULL
ON CONFLICT (template_id, discord_user_id) DO NOTHING;

-- name: IsTemplatePublished :one
SELECT (template_published_at IS NOT NULL)::bool AS published
FROM meetup_templates
WHERE template_id = $1;

-- name: UnlikeTemplate :exec
DELETE FROM meetup_template_likes
WHERE template_id = $1 AND discord_user_id = $2;

-- name: CountTemplateLikes :one
SELECT COUNT(*)::int AS count
FROM meetup_template_likes
WHERE template_id = $1;

-- name: TemplateLikedByUser :one
SELECT EXISTS (
    SELECT 1 FROM meetup_template_likes
    WHERE template_id = $1 AND discord_user_id = $2
) AS liked;

-- name: ListTemplateLikers :many
SELECT
    l.template_id,
    u.discord_user_id,
    u.discord_user_username,
    u.discord_user_display_name,
    u.discord_user_avatar_url
FROM meetup_template_likes l
JOIN discord_users u ON u.discord_user_id = l.discord_user_id
WHERE l.template_id = ANY(sqlc.arg('template_ids')::bigint[])
ORDER BY l.liked_at DESC, u.discord_user_username ASC;
