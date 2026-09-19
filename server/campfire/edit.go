package campfire

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ClubEvent is a meetup from a club's active feed (upcoming/ongoing).
type ClubEvent struct {
	ID                           string `json:"id"`
	Name                         string `json:"name"`
	Address                      string `json:"address"`
	Location                     string `json:"location"`
	CoverPhotoURL                string `json:"coverPhotoUrl"`
	Details                      string `json:"details"`
	EventTime                    string `json:"eventTime"`
	EventEndTime                 string `json:"eventEndTime"`
	CommentsPermissions          string `json:"commentsPermissions"`
	AllInvited                   bool   `json:"allInvited"`
	CreatedByCommunityAmbassador bool   `json:"createdByCommunityAmbassador"`
	CampfireLiveEventID          string `json:"campfireLiveEventId"`
	ClubID                       string `json:"clubId"`
	Creator                      struct {
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	} `json:"creator"`
}

type activeEventsResp struct {
	Club struct {
		ID         string `json:"id"`
		ActiveFeed struct {
			Edges []struct {
				Node   json.RawMessage `json:"node"`
				Cursor string          `json:"cursor"`
			} `json:"edges"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"activeFeed"`
	} `json:"club"`
}

type activeFeedNode struct {
	Typename string `json:"__typename"`
	ClubEvent
}

// ActiveEvents returns upcoming/ongoing meetups for a club (one page).
func (c *Client) ActiveEvents(ctx context.Context, token, clubID string, first int, after string) ([]ClubEvent, string, bool, error) {
	if first <= 0 {
		first = 50
	}
	vars := map[string]any{
		"clubId": clubID,
		"first":  first,
	}
	if after != "" {
		vars["after"] = after
	}
	var resp activeEventsResp
	if err := c.Do(ctx, token, queryActiveEvents, vars, &resp); err != nil {
		return nil, "", false, err
	}
	out := make([]ClubEvent, 0, len(resp.Club.ActiveFeed.Edges))
	for _, e := range resp.Club.ActiveFeed.Edges {
		var node activeFeedNode
		if err := json.Unmarshal(e.Node, &node); err != nil {
			continue
		}
		if node.Typename != "" && node.Typename != "Event" {
			continue
		}
		if node.ID == "" {
			continue
		}
		ev := node.ClubEvent
		if ev.ClubID == "" {
			ev.ClubID = resp.Club.ID
		}
		out = append(out, ev)
	}
	return out, resp.Club.ActiveFeed.PageInfo.EndCursor, resp.Club.ActiveFeed.PageInfo.HasNextPage, nil
}

// EditEventInput is the GraphQL input for editEvent (from Campfire client).
// Several fields must be present even when empty (coverPhotoUrl, details, address, userIds);
// do not mark those with omitempty.
type EditEventInput struct {
	EventID                      string   `json:"eventId"`
	Name                         string   `json:"name"`
	Details                      string   `json:"details"`
	EventTime                    string   `json:"eventTime,omitempty"`
	EventEndTime                 string   `json:"eventEndTime,omitempty"`
	Location                     string   `json:"location,omitempty"` // "[lng, lat]"
	Address                      string   `json:"address"`
	PlaceID                      string   `json:"placeId,omitempty"`
	CoverPhotoURL                string   `json:"coverPhotoUrl"`
	CommentsPermissions          string   `json:"commentsPermissions,omitempty"`
	AllInvited                   *bool    `json:"allInvited,omitempty"`
	UserIDs                      []string `json:"userIds"`
	CampfireLiveEventID          string   `json:"campfireLiveEventId,omitempty"`
	HasEventPhotoChanged         *bool    `json:"hasEventPhotoChanged,omitempty"`
	DiscordChannelID             string   `json:"discordChannelId,omitempty"`
	CreatedByCommunityAmbassador *bool    `json:"createdByCommunityAmbassador,omitempty"`
	// Avatar is uploaded with the edit as input.avatarFile. Campfire ignores
	// coverPhotoUrl unless this file is attached and hasEventPhotoChanged is set.
	Avatar *AvatarUpload `json:"-"`
}

// AvatarUpload is a cover image attached to editEvent via the GraphQL multipart spec.
type AvatarUpload struct {
	Filename    string
	ContentType string
	Data        []byte
}

type editEventResp struct {
	EditEvent struct {
		Event *CreatedEvent `json:"event"`
	} `json:"editEvent"`
}

func (c *Client) EditEvent(ctx context.Context, token string, input EditEventInput) (*CreatedEvent, error) {
	if input.Avatar != nil && len(input.Avatar.Data) > 0 {
		return c.editEventWithAvatar(ctx, token, input)
	}
	vars := map[string]any{"input": input}
	var resp editEventResp
	if err := c.Do(ctx, token, mutationEditEvent, vars, &resp); err != nil {
		return nil, err
	}
	return resp.EditEvent.Event, nil
}

func (c *Client) editEventWithAvatar(ctx context.Context, token string, input EditEventInput) (*CreatedEvent, error) {
	avatar := input.Avatar
	input.Avatar = nil
	input.CoverPhotoURL = ""
	changed := true
	input.HasEventPhotoChanged = &changed

	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	fields["avatarFile"] = nil

	filename := strings.TrimSpace(avatar.Filename)
	if filename == "" {
		filename = "cover.jpg"
	}
	contentType := strings.TrimSpace(avatar.ContentType)
	if contentType == "" {
		contentType = "image/jpeg"
	}

	data, err := c.doMultipart(ctx, token, "EditEventMutation", mutationEditEvent, map[string]any{"input": fields}, "variables.input.avatarFile", filename, contentType, avatar.Data)
	if err != nil {
		return nil, err
	}
	var resp editEventResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode editEvent: %w", err)
	}
	return resp.EditEvent.Event, nil
}

type deleteEventResp struct {
	DeleteEvent struct {
		Success bool `json:"success"`
	} `json:"deleteEvent"`
}

func (c *Client) DeleteEvent(ctx context.Context, token, eventID string) (bool, error) {
	vars := map[string]any{"input": map[string]string{"eventId": eventID}}
	var resp deleteEventResp
	if err := c.Do(ctx, token, mutationDeleteEvent, vars, &resp); err != nil {
		return false, err
	}
	return resp.DeleteEvent.Success, nil
}

// FormatEditLocation builds the "[lng, lat]" string Campfire expects on edit.
func FormatEditLocation(lat, lng float64) string {
	return fmt.Sprintf("[%g, %g]", lng, lat)
}

// ParseLocationString parses Campfire event location ("[lng, lat]" or similar).
func ParseLocationString(s string) (lat, lng float64, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	a, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	b, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	// Campfire stores GeoJSON-style [lng, lat].
	return b, a, true
}
