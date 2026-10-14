-- Per-field overrides for clones linked to an origin template.
ALTER TABLE meetup_templates
    ADD COLUMN IF NOT EXISTS template_overrides JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Fully synced clones keep empty overrides (default).
-- Previously unsynced clones: treat current name/language/payload as full overrides.
-- Meetup title (payload.name) is stored as overrides.title so it does not clash with template name.
UPDATE meetup_templates t
SET template_overrides = (
    (COALESCE(t.template_payload, '{}'::jsonb) - 'name')
    || CASE
        WHEN COALESCE(t.template_payload, '{}'::jsonb) ? 'name'
            THEN jsonb_build_object('title', t.template_payload->'name')
        ELSE '{}'::jsonb
    END
    || jsonb_build_object('name', t.template_name)
    || CASE
        WHEN t.template_language IS NOT NULL AND btrim(t.template_language) <> ''
            THEN jsonb_build_object('language', t.template_language)
        ELSE '{}'::jsonb
    END
)
WHERE t.template_origin_id IS NOT NULL
  AND t.template_synced = FALSE;

-- Keep template_synced aligned: linked + empty overrides => synced.
UPDATE meetup_templates
SET template_synced = (template_origin_id IS NOT NULL AND template_overrides = '{}'::jsonb)
WHERE template_origin_id IS NOT NULL;
