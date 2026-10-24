CREATE TABLE icon_libraries (
    library_id BIGSERIAL PRIMARY KEY,
    library_discord_user_id TEXT NOT NULL REFERENCES discord_users (discord_user_id) ON DELETE CASCADE,
    library_name TEXT NOT NULL,
    library_is_default BOOLEAN NOT NULL DEFAULT FALSE,
    library_published_at TIMESTAMP,
    library_created_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    library_updated_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc')
);

CREATE UNIQUE INDEX icon_libraries_one_default_idx
    ON icon_libraries (library_discord_user_id)
    WHERE library_is_default;

CREATE INDEX icon_libraries_published_idx
    ON icon_libraries (library_published_at)
    WHERE library_published_at IS NOT NULL;

CREATE TABLE icons (
    icon_id BIGSERIAL PRIMARY KEY,
    icon_library_id BIGINT NOT NULL REFERENCES icon_libraries (library_id) ON DELETE CASCADE,
    icon_filename TEXT NOT NULL DEFAULT '',
    icon_content_type TEXT NOT NULL,
    icon_storage_path TEXT NOT NULL DEFAULT '',
    icon_category TEXT,
    icon_created_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc'),
    icon_updated_at TIMESTAMP NOT NULL DEFAULT (now() AT TIME ZONE 'utc')
);

CREATE INDEX icons_library_idx ON icons (icon_library_id);

CREATE TABLE icon_pokemon (
    icon_id BIGINT NOT NULL REFERENCES icons (icon_id) ON DELETE CASCADE,
    pokemon_slug TEXT NOT NULL,
    PRIMARY KEY (icon_id, pokemon_slug)
);
