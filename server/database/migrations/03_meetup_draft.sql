ALTER TABLE meetup_queue RENAME TO meetup_draft;

ALTER TABLE meetup_draft RENAME COLUMN queue_id TO draft_id;
ALTER TABLE meetup_draft RENAME COLUMN queue_discord_user_id TO draft_discord_user_id;
ALTER TABLE meetup_draft RENAME COLUMN queue_payload TO draft_payload;
ALTER TABLE meetup_draft RENAME COLUMN queue_created_at TO draft_created_at;
ALTER TABLE meetup_draft RENAME COLUMN queue_updated_at TO draft_updated_at;

ALTER INDEX meetup_queue_user_idx RENAME TO meetup_draft_user_idx;

ALTER SEQUENCE IF EXISTS meetup_queue_queue_id_seq RENAME TO meetup_draft_draft_id_seq;
