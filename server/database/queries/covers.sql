-- name: InsertCoverImage :one
INSERT INTO cover_images (
    cover_discord_user_id,
    cover_filename,
    cover_content_type,
    cover_data
) VALUES ($1, $2, $3, $4)
RETURNING
    cover_id,
    cover_discord_user_id,
    cover_filename,
    cover_content_type,
    cover_created_at;

-- name: GetCoverImage :one
SELECT
    cover_id,
    cover_discord_user_id,
    cover_filename,
    cover_content_type,
    cover_data,
    cover_created_at
FROM cover_images
WHERE cover_id = $1;

-- name: DeleteCoverImage :execrows
DELETE FROM cover_images
WHERE cover_id = $1 AND cover_discord_user_id = $2;
