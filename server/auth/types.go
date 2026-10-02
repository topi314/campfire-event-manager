package auth

import "time"

type DiscordUser struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	AvatarURL   string    `json:"avatarUrl"`
	ImportedAt  time.Time `json:"importedAt"`
	TimeZone    *string   `json:"timeZone,omitempty"`
}

type Session struct {
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time
	UserID    string
	Admin     bool
}

type SessionWithUser struct {
	Session
	DiscordUser
}
