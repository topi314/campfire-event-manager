package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type CoverImage struct {
	ID            int64     `db:"cover_id" json:"id"`
	DiscordUserID string    `db:"cover_discord_user_id" json:"discordUserId"`
	Filename      string    `db:"cover_filename" json:"filename"`
	ContentType   string    `db:"cover_content_type" json:"contentType"`
	Data          []byte    `db:"cover_data" json:"-"`
	CreatedAt     time.Time `db:"cover_created_at" json:"createdAt"`
}

func (d *Database) InsertCoverImage(ctx context.Context, userID, filename, contentType string, data []byte) (*CoverImage, error) {
	const q = `
		INSERT INTO cover_images (
			cover_discord_user_id,
			cover_filename,
			cover_content_type,
			cover_data
		) VALUES ($1, $2, $3, $4)
		RETURNING
			cover_id,
			cover_discord_user_id,
			cover_filename,
			cover_content_type,
			cover_created_at
	`
	var img CoverImage
	err := d.db.QueryRowxContext(ctx, q, userID, filename, contentType, data).Scan(
		&img.ID,
		&img.DiscordUserID,
		&img.Filename,
		&img.ContentType,
		&img.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert cover image: %w", err)
	}
	img.Data = data
	return &img, nil
}

func (d *Database) GetCoverImage(ctx context.Context, id int64) (*CoverImage, error) {
	const q = `
		SELECT
			cover_id,
			cover_discord_user_id,
			cover_filename,
			cover_content_type,
			cover_data,
			cover_created_at
		FROM cover_images
		WHERE cover_id = $1
	`
	var img CoverImage
	if err := d.db.GetContext(ctx, &img, q, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("get cover image: %w", err)
	}
	return &img, nil
}

func (d *Database) DeleteCoverImage(ctx context.Context, id int64, userID string) error {
	const q = `
		DELETE FROM cover_images
		WHERE cover_id = $1 AND cover_discord_user_id = $2
	`
	res, err := d.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return fmt.Errorf("delete cover image: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete cover image: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
