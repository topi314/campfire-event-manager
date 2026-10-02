CREATE TABLE cover_images (
    cover_id BIGSERIAL PRIMARY KEY,
    cover_discord_user_id TEXT NOT NULL REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    cover_filename TEXT NOT NULL DEFAULT '',
    cover_content_type TEXT NOT NULL DEFAULT 'image/jpeg',
    cover_data BYTEA NOT NULL,
    cover_created_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc')
);

CREATE INDEX cover_images_user_idx ON cover_images (cover_discord_user_id);
