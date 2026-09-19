ALTER TABLE meetup_templates
    ADD COLUMN IF NOT EXISTS template_language VARCHAR(16) NULL;

CREATE INDEX IF NOT EXISTS meetup_templates_language_idx
    ON meetup_templates (template_language)
    WHERE template_published_at IS NOT NULL AND template_language IS NOT NULL;
