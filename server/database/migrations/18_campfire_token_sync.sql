ALTER TABLE discord_users
    ADD COLUMN IF NOT EXISTS discord_user_campfire_token BYTEA NULL;

ALTER TABLE discord_users
    ADD COLUMN IF NOT EXISTS discord_user_campfire_token_sync BOOLEAN NOT NULL DEFAULT FALSE;
