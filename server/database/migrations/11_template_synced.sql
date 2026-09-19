-- Cloned templates stay linked to their origin until the owner edits them.
ALTER TABLE meetup_templates
    ADD COLUMN IF NOT EXISTS template_synced BOOLEAN NOT NULL DEFAULT FALSE;

-- Existing clones keep a frozen copy (treat as already customized).
-- New clones set template_synced = TRUE in CreateTemplateClone.
