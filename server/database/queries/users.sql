-- name: UpsertDiscordUser :exec
INSERT INTO discord_users (
    discord_user_id,
    discord_user_username,
    discord_user_display_name,
    discord_user_avatar_url
) VALUES ($1, $2, $3, $4)
ON CONFLICT (discord_user_id) DO UPDATE
SET discord_user_username     = EXCLUDED.discord_user_username,
    discord_user_display_name = EXCLUDED.discord_user_display_name,
    discord_user_avatar_url   = EXCLUDED.discord_user_avatar_url,
    discord_user_imported_at  = now();

-- name: SetDiscordUserTimeZone :execrows
UPDATE discord_users
SET discord_user_timezone = $2
WHERE discord_user_id = $1;

-- name: GetDiscordUser :one
SELECT
    discord_user_id,
    discord_user_username,
    discord_user_display_name,
    discord_user_avatar_url,
    discord_user_imported_at,
    discord_user_timezone
FROM discord_users
WHERE discord_user_id = $1;
