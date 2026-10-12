package campfire

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// DiscordChannel is a Discord guild channel from club Discord settings.
type DiscordChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// ClubDiscordSettings is the Discord integration state for a club.
type ClubDiscordSettings struct {
	Available        bool             `json:"available"`
	DiscordGuildID   string           `json:"discordGuildId,omitempty"`
	DiscordChannelID string           `json:"discordChannelId,omitempty"`
	Channels         []DiscordChannel `json:"channels"`
}

type discordChannelsResp struct {
	Club struct {
		ID                  string `json:"id"`
		ClubDiscordSettings *struct {
			DiscordGuildID   string `json:"discordGuildId"`
			DiscordChannelID string `json:"discordChannelId"`
			DiscordGuild     *struct {
				ID       string `json:"id"`
				Channels []struct {
					ID   string          `json:"id"`
					Name string          `json:"name"`
					Type json.RawMessage `json:"type"`
				} `json:"channels"`
			} `json:"discordGuild"`
		} `json:"clubDiscordSettings"`
	} `json:"club"`
}

func discordTypeString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	var i int
	if err := json.Unmarshal(raw, &i); err == nil {
		return fmt.Sprintf("%d", i)
	}
	return strings.Trim(string(raw), `"`)
}

// DiscordSettings returns Discord channel options for a club.
// Available is false when settings or guild channels are missing.
func (c *Client) DiscordSettings(ctx context.Context, token, clubID string) (ClubDiscordSettings, error) {
	out := ClubDiscordSettings{Channels: []DiscordChannel{}}
	clubID = strings.TrimSpace(clubID)
	if clubID == "" {
		return out, nil
	}

	var resp discordChannelsResp
	if err := c.Do(ctx, token, queryDiscordChannels, discordChannelsVars{ID: clubID}, &resp); err != nil {
		return out, err
	}

	settings := resp.Club.ClubDiscordSettings
	if settings == nil || settings.DiscordGuild == nil {
		return out, nil
	}

	channels := make([]DiscordChannel, 0, len(settings.DiscordGuild.Channels))
	for _, ch := range settings.DiscordGuild.Channels {
		id := strings.TrimSpace(ch.ID)
		if id == "" {
			continue
		}
		channels = append(channels, DiscordChannel{
			ID:   id,
			Name: ch.Name,
			Type: discordTypeString(ch.Type),
		})
	}
	if len(channels) == 0 {
		return out, nil
	}

	out.Available = true
	out.DiscordGuildID = strings.TrimSpace(settings.DiscordGuildID)
	if out.DiscordGuildID == "" {
		out.DiscordGuildID = strings.TrimSpace(settings.DiscordGuild.ID)
	}
	out.DiscordChannelID = strings.TrimSpace(settings.DiscordChannelID)
	out.Channels = channels
	return out, nil
}
