CREATE TABLE IF NOT EXISTS meetup_template_likes (
    template_id BIGINT NOT NULL REFERENCES meetup_templates (template_id) ON DELETE CASCADE,
    discord_user_id TEXT NOT NULL REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    liked_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    PRIMARY KEY (template_id, discord_user_id)
);

CREATE INDEX meetup_template_likes_user_idx
    ON meetup_template_likes (discord_user_id);

CREATE INDEX meetup_template_likes_template_liked_at_idx
    ON meetup_template_likes (template_id, liked_at DESC);
