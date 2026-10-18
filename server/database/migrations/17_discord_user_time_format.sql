ALTER TABLE discord_users
    ADD COLUMN IF NOT EXISTS discord_user_time_format VARCHAR(16) NULL;
