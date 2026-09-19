package database

import (
	"encoding/json"
	"time"
)

type DiscordUser struct {
	ID          string    `db:"discord_user_id" json:"id"`
	Username    string    `db:"discord_user_username" json:"username"`
	DisplayName string    `db:"discord_user_display_name" json:"displayName"`
	AvatarURL   string    `db:"discord_user_avatar_url" json:"avatarUrl"`
	ImportedAt  time.Time `db:"discord_user_imported_at" json:"importedAt"`
	TimeZone    *string   `db:"discord_user_timezone" json:"timeZone,omitempty"`
}

type Session struct {
	ID        string    `db:"session_id"`
	CreatedAt time.Time `db:"session_created_at"`
	ExpiresAt time.Time `db:"session_expires_at"`
	UserID    string    `db:"session_user_id"`
	Admin     bool      `db:"session_admin"`
}

type SessionWithUser struct {
	Session
	DiscordUser
}

type MeetupTemplate struct {
	ID                 int64           `db:"template_id" json:"id"`
	DiscordUserID      string          `db:"template_discord_user_id" json:"discordUserId"`
	Name               string          `db:"template_name" json:"name"`
	Payload            json.RawMessage `db:"template_payload" json:"payload"`
	PublishedAt        *time.Time      `db:"template_published_at" json:"publishedAt"`
	PublishDescription *string         `db:"template_publish_description" json:"publishDescription,omitempty"`
	Language           *string         `db:"template_language" json:"language,omitempty"`
	OriginID           *int64          `db:"template_origin_id" json:"originTemplateId"`
	// Synced means a clone follows its origin until the owner edits it.
	Synced             bool            `db:"template_synced" json:"synced"`
	CreatedAt          time.Time       `db:"template_created_at" json:"createdAt"`
	UpdatedAt          time.Time       `db:"template_updated_at" json:"updatedAt"`

	// Populated by joins for API responses (not DB columns on the main row).
	Publisher *TemplatePublisher `db:"-" json:"publisher,omitempty"`
	Origin    *TemplateOrigin    `db:"-" json:"origin,omitempty"`
	LikeCount int                `db:"-" json:"likeCount"`
	LikedByMe bool               `db:"-" json:"likedByMe"`
	Likers    []TemplatePublisher `db:"-" json:"likers,omitempty"`
}

type TemplatePublisher struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
}

type TemplateOrigin struct {
	ID        int64              `json:"id"`
	Name      string             `json:"name"`
	Publisher *TemplatePublisher `json:"publisher,omitempty"`
}

type MeetupDraftItem struct {
	ID            int64           `db:"draft_id" json:"id"`
	ClubID        string          `db:"draft_club_id" json:"clubId"`
	DiscordUserID string          `db:"draft_discord_user_id" json:"discordUserId"`
	Payload       json.RawMessage `db:"draft_payload" json:"payload"`
	CreatedAt     time.Time       `db:"draft_created_at" json:"createdAt"`
	UpdatedAt     time.Time       `db:"draft_updated_at" json:"updatedAt"`
	Creator       *DraftCreator   `db:"-" json:"creator,omitempty"`
}

type DraftCreator struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
}
