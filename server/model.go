package server

import (
	"encoding/json"
	"time"
)

type MeetupTemplate struct {
	ID                 int64           `json:"id"`
	DiscordUserID      string          `json:"discordUserId"`
	Name               string          `json:"name"`
	Payload            json.RawMessage `json:"payload"`
	PublishedAt        *time.Time      `json:"publishedAt"`
	PublishDescription *string         `json:"publishDescription,omitempty"`
	Language           *string         `json:"language,omitempty"`
	OriginID           *int64          `json:"originTemplateId"`
	Synced             bool            `json:"synced"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`

	Publisher *UserRef        `json:"publisher,omitempty"`
	Origin    *TemplateOrigin `json:"origin,omitempty"`
	LikeCount int             `json:"likeCount"`
	LikedByMe bool            `json:"likedByMe"`
	Likers    []UserRef       `json:"likers,omitempty"`
}

// UserRef is a public Discord user snippet (publisher, liker, draft creator).
type UserRef struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
}

type TemplateOrigin struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Publisher *UserRef `json:"publisher,omitempty"`
}

type MeetupDraftItem struct {
	ID            int64           `json:"id"`
	ClubID        string          `json:"clubId"`
	DiscordUserID string          `json:"discordUserId"`
	Payload       json.RawMessage `json:"payload"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	Creator       *UserRef        `json:"creator,omitempty"`
}

type SharedFilter struct {
	Query         string
	Category      string
	Language      string
	PublisherID   string
	Publisher     string
	LikedByUserID string
	Sort          string
	ViewerUserID  string
}
