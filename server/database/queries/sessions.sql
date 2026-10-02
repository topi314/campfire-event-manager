-- name: GetSessionWithUser :one
SELECT
    s.session_id,
    s.session_created_at,
    s.session_expires_at,
    s.session_user_id,
    s.session_admin,
    u.discord_user_id,
    u.discord_user_username,
    u.discord_user_display_name,
    u.discord_user_avatar_url,
    u.discord_user_imported_at,
    u.discord_user_timezone
FROM sessions s
LEFT JOIN discord_users u ON s.session_user_id = u.discord_user_id
WHERE s.session_id = $1;

-- name: CreateSession :exec
INSERT INTO sessions (
    session_id,
    session_created_at,
    session_expires_at,
    session_user_id,
    session_admin
) VALUES ($1, $2, $3, $4, $5);

-- name: DeleteSession :exec
DELETE FROM sessions WHERE session_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE session_expires_at < now();
