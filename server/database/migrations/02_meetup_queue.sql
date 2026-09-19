CREATE TABLE meetup_queue
(
    queue_id              BIGSERIAL PRIMARY KEY,
    queue_discord_user_id VARCHAR   NOT NULL REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    queue_payload         JSONB     NOT NULL DEFAULT '{}'::jsonb,
    queue_created_at      TIMESTAMP NOT NULL DEFAULT now(),
    queue_updated_at      TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX meetup_queue_user_idx ON meetup_queue (queue_discord_user_id);
