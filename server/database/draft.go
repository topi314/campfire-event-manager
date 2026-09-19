package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const draftColumns = `
	d.draft_id,
	d.draft_club_id,
	d.draft_discord_user_id,
	d.draft_payload,
	d.draft_created_at,
	d.draft_updated_at
`

func (d *Database) ListDraftsByClubIDs(ctx context.Context, clubIDs []string) ([]MeetupDraftItem, error) {
	if len(clubIDs) == 0 {
		return []MeetupDraftItem{}, nil
	}
	query := `
		SELECT ` + draftColumns + `,
			u.discord_user_id AS creator_id,
			u.discord_user_username AS creator_username,
			u.discord_user_display_name AS creator_display_name,
			u.discord_user_avatar_url AS creator_avatar_url
		FROM meetup_draft d
		LEFT JOIN discord_users u ON u.discord_user_id = d.draft_discord_user_id
		WHERE d.draft_club_id IN (?)
		ORDER BY d.draft_created_at ASC
	`
	q, args, err := sqlx.In(query, clubIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to build draft list query: %w", err)
	}
	rows, err := d.db.QueryxContext(ctx, d.db.Rebind(q), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list drafts: %w", err)
	}
	defer rows.Close()

	out := make([]MeetupDraftItem, 0)
	for rows.Next() {
		item, err := scanDraftWithCreator(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *Database) GetDraft(ctx context.Context, id int64) (*MeetupDraftItem, error) {
	query := `
		SELECT ` + draftColumns + `,
			u.discord_user_id AS creator_id,
			u.discord_user_username AS creator_username,
			u.discord_user_display_name AS creator_display_name,
			u.discord_user_avatar_url AS creator_avatar_url
		FROM meetup_draft d
		LEFT JOIN discord_users u ON u.discord_user_id = d.draft_discord_user_id
		WHERE d.draft_id = $1
	`
	rows, err := d.db.QueryxContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	item, err := scanDraftWithCreator(rows)
	if err != nil {
		return nil, err
	}
	return &item, rows.Err()
}

func (d *Database) CreateDraft(ctx context.Context, clubID, creatorUserID string, payload json.RawMessage) (*MeetupDraftItem, error) {
	clubID = strings.TrimSpace(clubID)
	if clubID == "" {
		return nil, fmt.Errorf("club id is required")
	}
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	now := time.Now().UTC()
	query := `
		INSERT INTO meetup_draft (draft_club_id, draft_discord_user_id, draft_payload, draft_created_at, draft_updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING draft_id, draft_club_id, draft_discord_user_id, draft_payload, draft_created_at, draft_updated_at
	`
	var item MeetupDraftItem
	if err := d.db.GetContext(ctx, &item, query, clubID, creatorUserID, payload, now, now); err != nil {
		return nil, fmt.Errorf("failed to create draft: %w", err)
	}
	return &item, nil
}

func (d *Database) UpdateDraft(ctx context.Context, id int64, clubID string, payload json.RawMessage) (*MeetupDraftItem, error) {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	now := time.Now().UTC()
	query := `
		UPDATE meetup_draft
		SET draft_payload = $1,
		    draft_club_id = $2,
		    draft_updated_at = $4
		WHERE draft_id = $3
		RETURNING draft_id, draft_club_id, draft_discord_user_id, draft_payload, draft_created_at, draft_updated_at
	`
	var item MeetupDraftItem
	if err := d.db.GetContext(ctx, &item, query, payload, strings.TrimSpace(clubID), id, now); err != nil {
		return nil, err
	}
	return &item, nil
}

func (d *Database) DeleteDraft(ctx context.Context, id int64) error {
	res, err := d.db.ExecContext(ctx, `
		DELETE FROM meetup_draft
		WHERE draft_id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("failed to delete draft: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("draft not found")
	}
	return nil
}

func (d *Database) ClearDraftsByClubIDs(ctx context.Context, clubIDs []string) error {
	if len(clubIDs) == 0 {
		return nil
	}
	query := `DELETE FROM meetup_draft WHERE draft_club_id IN (?)`
	q, args, err := sqlx.In(query, clubIDs)
	if err != nil {
		return fmt.Errorf("failed to build clear drafts query: %w", err)
	}
	if _, err := d.db.ExecContext(ctx, d.db.Rebind(q), args...); err != nil {
		return fmt.Errorf("failed to clear drafts: %w", err)
	}
	return nil
}

type draftScanner interface {
	Scan(dest ...any) error
}

func scanDraftWithCreator(row draftScanner) (MeetupDraftItem, error) {
	var item MeetupDraftItem
	var creatorID, username, displayName, avatar sql.NullString
	if err := row.Scan(
		&item.ID,
		&item.ClubID,
		&item.DiscordUserID,
		&item.Payload,
		&item.CreatedAt,
		&item.UpdatedAt,
		&creatorID,
		&username,
		&displayName,
		&avatar,
	); err != nil {
		return item, err
	}
	if creatorID.Valid && creatorID.String != "" {
		item.Creator = &DraftCreator{
			ID:          creatorID.String,
			Username:    username.String,
			DisplayName: displayName.String,
			AvatarURL:   avatar.String,
		}
	}
	return item, nil
}
