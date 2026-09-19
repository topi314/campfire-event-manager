-- Remove demo Shared publishers + templates seeded by the old 05 migration.
DELETE FROM discord_users
WHERE discord_user_id IN (
    '900000000000000001',
    '900000000000000002',
    '900000000000000003'
);
