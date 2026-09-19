-- Scope drafts to a Campfire club so club admins can share them.
-- draft_discord_user_id remains the creator.

ALTER TABLE meetup_draft
    ADD COLUMN IF NOT EXISTS draft_club_id VARCHAR NULL;

UPDATE meetup_draft
SET draft_club_id = NULLIF(BTRIM(draft_payload->>'clubId'), '')
WHERE draft_club_id IS NULL;

DELETE FROM meetup_draft
WHERE draft_club_id IS NULL;

ALTER TABLE meetup_draft
    ALTER COLUMN draft_club_id SET NOT NULL;

DROP INDEX IF EXISTS meetup_draft_user_idx;

CREATE INDEX IF NOT EXISTS meetup_draft_club_idx ON meetup_draft (draft_club_id);
CREATE INDEX IF NOT EXISTS meetup_draft_creator_idx ON meetup_draft (draft_discord_user_id);
