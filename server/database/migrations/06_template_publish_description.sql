ALTER TABLE meetup_templates
    ADD COLUMN IF NOT EXISTS template_publish_description TEXT NULL;
