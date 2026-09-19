CREATE TABLE discord_users
(
    discord_user_id           VARCHAR PRIMARY KEY,
    discord_user_username     VARCHAR UNIQUE NOT NULL,
    discord_user_display_name VARCHAR        NOT NULL,
    discord_user_avatar_url   VARCHAR        NOT NULL,
    discord_user_imported_at  TIMESTAMP      NOT NULL DEFAULT now()
);

CREATE TABLE sessions
(
    session_id         VARCHAR(64) PRIMARY KEY,
    session_created_at TIMESTAMP NOT NULL DEFAULT now(),
    session_expires_at TIMESTAMP NOT NULL,
    session_user_id    VARCHAR   NOT NULL REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    session_admin      BOOLEAN   NOT NULL DEFAULT FALSE
);

CREATE TABLE meetup_templates
(
    template_id              BIGSERIAL PRIMARY KEY,
    template_discord_user_id VARCHAR   NOT NULL REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    template_name            VARCHAR   NOT NULL,
    template_payload         JSONB     NOT NULL DEFAULT '{}'::jsonb,
    template_created_at      TIMESTAMP NOT NULL DEFAULT now(),
    template_updated_at      TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX meetup_templates_user_idx ON meetup_templates (template_discord_user_id);
