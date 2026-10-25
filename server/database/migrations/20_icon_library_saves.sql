CREATE TABLE icon_library_saves (
    save_discord_user_id TEXT NOT NULL REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    save_library_id BIGINT NOT NULL REFERENCES icon_libraries (library_id) ON DELETE CASCADE,
    save_created_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    PRIMARY KEY (save_discord_user_id, save_library_id)
);

CREATE INDEX icon_library_saves_library_idx ON icon_library_saves (save_library_id);
