ALTER TABLE meetup_templates
    ADD COLUMN template_published_at TIMESTAMP NULL,
    ADD COLUMN template_origin_id BIGINT NULL REFERENCES meetup_templates (template_id) ON DELETE SET NULL;

CREATE INDEX meetup_templates_published_idx
    ON meetup_templates (template_published_at DESC)
    WHERE template_published_at IS NOT NULL;

CREATE INDEX meetup_templates_origin_idx
    ON meetup_templates (template_origin_id)
    WHERE template_origin_id IS NOT NULL;

CREATE INDEX meetup_templates_category_idx
    ON meetup_templates ((template_payload ->> 'category'))
    WHERE template_published_at IS NOT NULL;
