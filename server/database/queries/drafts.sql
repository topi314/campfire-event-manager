-- name: ListDraftsByClubIDs :many
SELECT
    d.draft_id,
    d.draft_club_id,
    d.draft_discord_user_id,
    d.draft_payload,
    d.draft_created_at,
    d.draft_updated_at,
    u.discord_user_id AS creator_id,
    u.discord_user_username AS creator_username,
    u.discord_user_display_name AS creator_display_name,
    u.discord_user_avatar_url AS creator_avatar_url
FROM meetup_draft d
LEFT JOIN discord_users u ON u.discord_user_id = d.draft_discord_user_id
WHERE d.draft_club_id = ANY(sqlc.arg('club_ids')::text[])
ORDER BY d.draft_created_at ASC;

-- name: GetDraft :one
SELECT
    d.draft_id,
    d.draft_club_id,
    d.draft_discord_user_id,
    d.draft_payload,
    d.draft_created_at,
    d.draft_updated_at,
    u.discord_user_id AS creator_id,
    u.discord_user_username AS creator_username,
    u.discord_user_display_name AS creator_display_name,
    u.discord_user_avatar_url AS creator_avatar_url
FROM meetup_draft d
LEFT JOIN discord_users u ON u.discord_user_id = d.draft_discord_user_id
WHERE d.draft_id = $1;

-- name: CreateDraft :one
INSERT INTO meetup_draft (
    draft_club_id,
    draft_discord_user_id,
    draft_payload,
    draft_created_at,
    draft_updated_at
) VALUES ($1, $2, $3, $4, $5)
RETURNING
    draft_id,
    draft_club_id,
    draft_discord_user_id,
    draft_payload,
    draft_created_at,
    draft_updated_at;

-- name: UpdateDraft :one
UPDATE meetup_draft
SET draft_payload = $1,
    draft_club_id = $2,
    draft_updated_at = $4
WHERE draft_id = $3
RETURNING
    draft_id,
    draft_club_id,
    draft_discord_user_id,
    draft_payload,
    draft_created_at,
    draft_updated_at;

-- name: DeleteDraft :execrows
DELETE FROM meetup_draft
WHERE draft_id = $1;

-- name: ClearDraftsByClubIDs :exec
DELETE FROM meetup_draft
WHERE draft_club_id = ANY(sqlc.arg('club_ids')::text[]);
