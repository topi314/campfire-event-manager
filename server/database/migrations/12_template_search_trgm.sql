-- Substring ILIKE on browse search/creator; B-tree can't help, trigram can.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS meetup_templates_name_trgm_idx
    ON meetup_templates
    USING gin (template_name gin_trgm_ops)
    WHERE template_published_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS meetup_templates_publish_desc_trgm_idx
    ON meetup_templates
    USING gin (template_publish_description gin_trgm_ops)
    WHERE template_published_at IS NOT NULL
      AND template_publish_description IS NOT NULL;

CREATE INDEX IF NOT EXISTS discord_users_username_trgm_idx
    ON discord_users
    USING gin (discord_user_username gin_trgm_ops);

CREATE INDEX IF NOT EXISTS discord_users_display_name_trgm_idx
    ON discord_users
    USING gin (discord_user_display_name gin_trgm_ops);
