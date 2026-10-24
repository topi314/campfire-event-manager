-- name: EnsureDefaultIconLibrary :one
INSERT INTO icon_libraries (
    library_discord_user_id,
    library_name,
    library_description,
    library_is_default
) VALUES ($1, 'My pack', '', TRUE)
ON CONFLICT (library_discord_user_id) WHERE library_is_default
DO UPDATE SET library_updated_at = icon_libraries.library_updated_at
RETURNING
    library_id,
    library_discord_user_id,
    library_name,
    library_is_default,
    library_published_at,
    library_created_at,
    library_updated_at,
    library_description;

-- name: ListVisibleIconLibraries :many
SELECT
    l.library_id,
    l.library_discord_user_id,
    l.library_name,
    l.library_description,
    l.library_is_default,
    l.library_published_at,
    l.library_created_at,
    l.library_updated_at,
    u.discord_user_display_name,
    u.discord_user_username,
    (s.save_library_id IS NOT NULL)::boolean AS saved
FROM icon_libraries l
JOIN discord_users u ON u.discord_user_id = l.library_discord_user_id
LEFT JOIN icon_library_saves s
    ON s.save_library_id = l.library_id
   AND s.save_discord_user_id = sqlc.arg(viewer_id)
WHERE (
        l.library_discord_user_id = sqlc.arg(viewer_id)
        OR l.library_published_at IS NOT NULL
    )
  AND (
        sqlc.arg(query)::text = ''
        OR l.library_name ILIKE ('%' || sqlc.arg(query) || '%')
        OR l.library_description ILIKE ('%' || sqlc.arg(query) || '%')
        OR u.discord_user_display_name ILIKE ('%' || sqlc.arg(query) || '%')
        OR u.discord_user_username ILIKE ('%' || sqlc.arg(query) || '%')
    )
  AND (
        sqlc.arg(owner_id)::text = ''
        OR l.library_discord_user_id = sqlc.arg(owner_id)
    )
  AND (
        sqlc.arg(category)::text = ''
        OR EXISTS (
            SELECT 1
            FROM icons i
            WHERE i.icon_library_id = l.library_id
              AND i.icon_category = sqlc.arg(category)
        )
    )
  AND (
        sqlc.arg(pokemon)::text = ''
        OR EXISTS (
            SELECT 1
            FROM icons i
            JOIN icon_pokemon p ON p.icon_id = i.icon_id
            WHERE i.icon_library_id = l.library_id
              AND p.pokemon_slug = sqlc.arg(pokemon)
        )
    )
ORDER BY
    CASE WHEN l.library_discord_user_id = sqlc.arg(viewer_id) THEN 0 ELSE 1 END,
    l.library_is_default DESC,
    l.library_name ASC;

-- name: SaveIconLibrary :exec
INSERT INTO icon_library_saves (
    save_discord_user_id,
    save_library_id
) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnsaveIconLibrary :exec
DELETE FROM icon_library_saves
WHERE save_discord_user_id = $1
  AND save_library_id = $2;

-- name: IconLibraryInMyPacks :one
SELECT EXISTS (
    SELECT 1
    FROM icon_libraries l
    LEFT JOIN icon_library_saves s
        ON s.save_library_id = l.library_id
       AND s.save_discord_user_id = $1
    WHERE l.library_id = $2
      AND (
            l.library_discord_user_id = $1
            OR s.save_library_id IS NOT NULL
      )
)::boolean AS in_my_packs;

-- name: CreateIconLibrary :one
INSERT INTO icon_libraries (
    library_discord_user_id,
    library_name,
    library_description
) VALUES ($1, $2, $3)
RETURNING
    library_id,
    library_discord_user_id,
    library_name,
    library_is_default,
    library_published_at,
    library_created_at,
    library_updated_at,
    library_description;

-- name: GetIconLibrary :one
SELECT
    library_id,
    library_discord_user_id,
    library_name,
    library_is_default,
    library_published_at,
    library_created_at,
    library_updated_at,
    library_description
FROM icon_libraries
WHERE library_id = $1;

-- name: UpdateIconLibrary :one
UPDATE icon_libraries
SET
    library_name = COALESCE(sqlc.narg(name), library_name),
    library_description = COALESCE(sqlc.narg(description), library_description),
    library_updated_at = (now() AT TIME ZONE 'utc')
WHERE library_id = sqlc.arg(library_id)
  AND library_discord_user_id = sqlc.arg(library_discord_user_id)
RETURNING
    library_id,
    library_discord_user_id,
    library_name,
    library_is_default,
    library_published_at,
    library_created_at,
    library_updated_at,
    library_description;

-- name: DeleteIconLibrary :execrows
DELETE FROM icon_libraries
WHERE library_id = $1
  AND library_discord_user_id = $2
  AND library_is_default = FALSE;

-- name: PublishIconLibrary :one
UPDATE icon_libraries
SET
    library_published_at = COALESCE(library_published_at, (now() AT TIME ZONE 'utc')),
    library_updated_at = (now() AT TIME ZONE 'utc')
WHERE library_id = $1
  AND library_discord_user_id = $2
  AND library_is_default = FALSE
RETURNING
    library_id,
    library_discord_user_id,
    library_name,
    library_is_default,
    library_published_at,
    library_created_at,
    library_updated_at,
    library_description;

-- name: UnpublishIconLibrary :one
UPDATE icon_libraries
SET
    library_published_at = NULL,
    library_updated_at = (now() AT TIME ZONE 'utc')
WHERE library_id = $1
  AND library_discord_user_id = $2
  AND library_is_default = FALSE
RETURNING
    library_id,
    library_discord_user_id,
    library_name,
    library_is_default,
    library_published_at,
    library_created_at,
    library_updated_at,
    library_description;

-- name: ListRecentIconPreviews :many
SELECT icon_library_id, icon_id
FROM (
    SELECT
        i.icon_library_id,
        i.icon_id,
        ROW_NUMBER() OVER (
            PARTITION BY i.icon_library_id
            ORDER BY i.icon_created_at DESC
        ) AS n
    FROM icons i
    JOIN icon_libraries l ON l.library_id = i.icon_library_id
    WHERE l.library_discord_user_id = $1
       OR l.library_published_at IS NOT NULL
) ranked
WHERE n <= 5
ORDER BY icon_library_id, n;

-- name: ListIconPathsByLibrary :many
SELECT icon_id, icon_storage_path
FROM icons
WHERE icon_library_id = $1;

-- name: InsertIcon :one
INSERT INTO icons (
    icon_library_id,
    icon_filename,
    icon_name,
    icon_content_type,
    icon_storage_path,
    icon_category
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING
    icon_id,
    icon_library_id,
    icon_filename,
    icon_name,
    icon_content_type,
    icon_storage_path,
    icon_category,
    icon_created_at,
    icon_updated_at;

-- name: SetIconStoragePath :exec
UPDATE icons
SET icon_storage_path = $2
WHERE icon_id = $1;

-- name: UpdateIconFields :exec
UPDATE icons
SET
    icon_name = $2,
    icon_category = $3,
    icon_updated_at = (now() AT TIME ZONE 'utc')
WHERE icon_id = $1;

-- name: DeleteIconPokemon :exec
DELETE FROM icon_pokemon
WHERE icon_id = $1;

-- name: InsertIconPokemon :exec
INSERT INTO icon_pokemon (icon_id, pokemon_slug)
VALUES ($1, $2);

-- name: DeleteIconForOwner :one
DELETE FROM icons i
USING icon_libraries l
WHERE i.icon_id = $1
  AND i.icon_library_id = l.library_id
  AND l.library_discord_user_id = $2
RETURNING i.icon_storage_path;

-- name: GetIcon :one
SELECT
    i.icon_id,
    i.icon_library_id,
    i.icon_filename,
    i.icon_name,
    i.icon_content_type,
    i.icon_storage_path,
    i.icon_category,
    i.icon_created_at,
    i.icon_updated_at,
    l.library_discord_user_id,
    l.library_name,
    l.library_published_at,
    COALESCE(
        ARRAY_AGG(p.pokemon_slug ORDER BY p.pokemon_slug) FILTER (WHERE p.pokemon_slug IS NOT NULL),
        '{}'::text[]
    )::text[] AS pokemon_slugs
FROM icons i
JOIN icon_libraries l ON l.library_id = i.icon_library_id
LEFT JOIN icon_pokemon p ON p.icon_id = i.icon_id
WHERE i.icon_id = $1
GROUP BY
    i.icon_id,
    l.library_discord_user_id,
    l.library_name,
    l.library_published_at;

-- name: ListIconsByLibrary :many
SELECT
    i.icon_id,
    i.icon_library_id,
    i.icon_filename,
    i.icon_name,
    i.icon_content_type,
    i.icon_storage_path,
    i.icon_category,
    i.icon_created_at,
    i.icon_updated_at,
    l.library_discord_user_id,
    l.library_name,
    l.library_published_at,
    COALESCE(
        ARRAY_AGG(p.pokemon_slug ORDER BY p.pokemon_slug) FILTER (WHERE p.pokemon_slug IS NOT NULL),
        '{}'::text[]
    )::text[] AS pokemon_slugs
FROM icons i
JOIN icon_libraries l ON l.library_id = i.icon_library_id
LEFT JOIN icon_pokemon p ON p.icon_id = i.icon_id
WHERE i.icon_library_id = $1
GROUP BY
    i.icon_id,
    l.library_discord_user_id,
    l.library_name,
    l.library_published_at
ORDER BY i.icon_created_at DESC;

-- name: ListAccessibleIcons :many
SELECT
    i.icon_id,
    i.icon_library_id,
    i.icon_filename,
    i.icon_name,
    i.icon_content_type,
    i.icon_storage_path,
    i.icon_category,
    i.icon_created_at,
    i.icon_updated_at,
    l.library_discord_user_id,
    l.library_name,
    l.library_published_at,
    COALESCE(
        ARRAY_AGG(p.pokemon_slug ORDER BY p.pokemon_slug) FILTER (WHERE p.pokemon_slug IS NOT NULL),
        '{}'::text[]
    )::text[] AS pokemon_slugs
FROM icons i
JOIN icon_libraries l ON l.library_id = i.icon_library_id
LEFT JOIN icon_pokemon p ON p.icon_id = i.icon_id
WHERE (
        l.library_discord_user_id = sqlc.arg(viewer_id)
        OR l.library_published_at IS NOT NULL
    )
  AND (sqlc.arg(library_id)::bigint = 0 OR i.icon_library_id = sqlc.arg(library_id))
  AND (sqlc.arg(category)::text = '' OR i.icon_category = sqlc.arg(category))
  AND (
        sqlc.arg(pokemon)::text = ''
        OR EXISTS (
            SELECT 1
            FROM icon_pokemon px
            WHERE px.icon_id = i.icon_id
              AND px.pokemon_slug = sqlc.arg(pokemon)
        )
    )
  AND (
        sqlc.arg(query)::text = ''
        OR i.icon_filename ILIKE ('%' || sqlc.arg(query) || '%')
        OR i.icon_name ILIKE ('%' || sqlc.arg(query) || '%')
        OR l.library_name ILIKE ('%' || sqlc.arg(query) || '%')
    )
  AND (
        NOT sqlc.arg(mine_only)::boolean
        OR l.library_discord_user_id = sqlc.arg(viewer_id)
        OR EXISTS (
            SELECT 1
            FROM icon_library_saves sv
            WHERE sv.save_library_id = l.library_id
              AND sv.save_discord_user_id = sqlc.arg(viewer_id)
        )
    )
GROUP BY
    i.icon_id,
    l.library_discord_user_id,
    l.library_name,
    l.library_published_at
ORDER BY l.library_name ASC, i.icon_filename ASC
LIMIT 400;
