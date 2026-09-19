package database

import (
	"context"
	"fmt"
)

func (d *Database) UpsertDiscordUser(ctx context.Context, user DiscordUser) error {
	query := `
		INSERT INTO discord_users (discord_user_id, discord_user_username, discord_user_display_name, discord_user_avatar_url)
		VALUES (:discord_user_id, :discord_user_username, :discord_user_display_name, :discord_user_avatar_url)
		ON CONFLICT (discord_user_id) DO UPDATE
		SET discord_user_username     = EXCLUDED.discord_user_username,
		    discord_user_display_name = EXCLUDED.discord_user_display_name,
		    discord_user_avatar_url   = EXCLUDED.discord_user_avatar_url,
		    discord_user_imported_at  = now()
	`
	if _, err := d.db.NamedExecContext(ctx, query, user); err != nil {
		return fmt.Errorf("failed to upsert discord user: %w", err)
	}
	return nil
}

func (d *Database) SetDiscordUserTimeZone(ctx context.Context, userID, timeZone string) error {
	res, err := d.db.ExecContext(ctx, `
		UPDATE discord_users
		SET discord_user_timezone = $2
		WHERE discord_user_id = $1
	`, userID, timeZone)
	if err != nil {
		return fmt.Errorf("failed to set discord user timezone: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to set discord user timezone: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("discord user %s not found", userID)
	}
	return nil
}

func (d *Database) GetDiscordUser(ctx context.Context, userID string) (*DiscordUser, error) {
	query := `
		SELECT discord_user_id, discord_user_username, discord_user_display_name,
		       discord_user_avatar_url, discord_user_imported_at, discord_user_timezone
		FROM discord_users
		WHERE discord_user_id = $1
	`
	var u DiscordUser
	if err := d.db.GetContext(ctx, &u, query, userID); err != nil {
		return nil, err
	}
	return &u, nil
}
