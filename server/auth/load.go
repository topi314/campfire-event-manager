package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/topi314/campfire-event-manager/server/database"
	"github.com/topi314/campfire-event-manager/server/database/dbsqlc"
)

var ErrSessionExpired = errors.New("session expired")

func LoadSession(ctx context.Context, db *database.DB, sessionID string) (*SessionWithUser, error) {
	row, err := db.GetSessionWithUser(ctx, sessionID)
	if err != nil {
		return nil, database.MapError(err)
	}
	expires := timeFrom(row.SessionExpiresAt)
	if expires.Before(time.Now()) {
		return nil, ErrSessionExpired
	}
	userID := row.SessionUserID
	if row.DiscordUserID.Valid && row.DiscordUserID.String != "" {
		userID = row.DiscordUserID.String
	}
	return &SessionWithUser{
		Session: Session{
			ID:        row.SessionID,
			CreatedAt: timeFrom(row.SessionCreatedAt),
			ExpiresAt: expires,
			UserID:    row.SessionUserID,
			Admin:     row.SessionAdmin,
		},
		DiscordUser: DiscordUser{
			ID:          userID,
			Username:    textString(row.DiscordUserUsername),
			DisplayName: textString(row.DiscordUserDisplayName),
			AvatarURL:   textString(row.DiscordUserAvatarUrl),
			ImportedAt:  timeFrom(row.DiscordUserImportedAt),
			TimeZone:    stringPtr(row.DiscordUserTimezone),
		},
	}, nil
}

func DiscordUserFrom(u dbsqlc.DiscordUser) DiscordUser {
	return DiscordUser{
		ID:          u.DiscordUserID,
		Username:    u.DiscordUserUsername,
		DisplayName: u.DiscordUserDisplayName,
		AvatarURL:   u.DiscordUserAvatarUrl,
		ImportedAt:  timeFrom(u.DiscordUserImportedAt),
		TimeZone:    stringPtr(u.DiscordUserTimezone),
	}
}

func timeFrom(ts pgtype.Timestamp) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}

func stringPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func textString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}
