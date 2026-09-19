ALTER TABLE discord_users
    ADD COLUMN IF NOT EXISTS discord_user_timezone VARCHAR NULL;
