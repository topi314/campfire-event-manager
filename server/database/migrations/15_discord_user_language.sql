ALTER TABLE discord_users
    ADD COLUMN IF NOT EXISTS discord_user_language VARCHAR(16) NULL;
