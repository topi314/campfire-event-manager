package campfire

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/topi314/campfire-event-manager/internal/meetuptime"
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
	DiscordChannelID             string `json:"discordChannelId,omitempty"`
	ClubID                       string `json:"clubId"`
	Creator                      struct {
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	} `json:"creator"`
}

type eventFeedPage struct {
	Edges []struct {
		Node   json.RawMessage `json:"node"`
		Cursor string          `json:"cursor"`
	} `json:"edges"`
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
}

type activeEventsResp struct {
	Club struct {
		ID           string        `json:"id"`
		Name         string        `json:"name"`
		ActiveFeed   eventFeedPage `json:"activeFeed"`
		ArchivedFeed eventFeedPage `json:"archivedFeed"`
	} `json:"club"`
}

type activeFeedNode struct {
	Typename string `json:"__typename"`
	ClubEvent
}

// ActiveEventsResult is one page of club meetups.
type ActiveEventsResult struct {
	Events      []ClubEvent
	ClubID      string
	ClubName    string
	EndCursor   string
	HasNextPage bool
}

// ActiveEvents returns one page of club meetups.
// archived selects the past feed (archivedFeed); otherwise the upcoming/ongoing feed.
// When lite is true, skips heavy fields (details, permissions) for list UIs.
// The past feed is always the lite query. Location stays so the events list can show a map pin.
// Discord channels are loaded separately via DiscordSettings.
func (c *Client) ActiveEvents(ctx context.Context, token, clubID string, first int, after string, lite bool, archived bool) (ActiveEventsResult, error) {
	out := ActiveEventsResult{
		Events: []ClubEvent{},
	}
	if first <= 0 {
		first = 50
	}
	query := queryActiveEvents
	if archived {
		query = queryArchivedEventsLite
	} else if lite {
		query = queryActiveEventsLite
	}
	var resp activeEventsResp
	if err := c.Do(ctx, token, query, activeEventsVars{
		ClubID: clubID,
		First:  first,
		After:  after,
	}, &resp); err != nil {
		return out, err
	}
	feed := resp.Club.ActiveFeed
	if archived {
		feed = resp.Club.ArchivedFeed
	}
	out.ClubID = resp.Club.ID
	out.ClubName = strings.TrimSpace(resp.Club.Name)
	out.Events = make([]ClubEvent, 0, len(feed.Edges))
	for _, e := range feed.Edges {
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
		// Campfire often omits Z on UTC instants; normalize so clients don't
		// misread them as browser-local wall clocks.
		ev.EventTime = meetuptime.NormalizeCampfireInstantRFC3339(ev.EventTime)
		ev.EventEndTime = meetuptime.NormalizeCampfireInstantRFC3339(ev.EventEndTime)
		out.Events = append(out.Events, ev)
	}
	out.EndCursor = feed.PageInfo.EndCursor
	out.HasNextPage = feed.PageInfo.HasNextPage
	return out, nil
}

// NullableLiveEventID is a Campfire live-event id on editEvent.
// An empty value marshals as null so an existing link can be cleared.
type NullableLiveEventID string

func (id NullableLiveEventID) MarshalJSON() ([]byte, error) {
	if strings.TrimSpace(string(id)) == "" {
		return []byte("null"), nil
	}
	return json.Marshal(string(id))
}

// EditEventInput is the GraphQL input for editEvent (from Campfire client).
// Several fields must be present even when empty (coverPhotoUrl, details, address, userIds);
// do not mark those with omitempty.
type EditEventInput struct {
	EventID             string   `json:"eventId"`
	Name                string   `json:"name"`
	Details             string   `json:"details"`
	EventTime           string   `json:"eventTime,omitempty"`
	EventEndTime        string   `json:"eventEndTime,omitempty"`
	Location            string   `json:"location,omitempty"` // "[lng, lat]"
	Address             string   `json:"address"`
	PlaceID             string   `json:"placeId,omitempty"`
	CoverPhotoURL       string   `json:"coverPhotoUrl"`
	CommentsPermissions string   `json:"commentsPermissions,omitempty"`
	AllInvited          *bool    `json:"allInvited,omitempty"`
	UserIDs             []string `json:"userIds"`
	// Nil omits the field. A non-nil empty id is sent as null so the link can be removed.
	CampfireLiveEventID          *NullableLiveEventID `json:"campfireLiveEventId,omitempty"`
	HasEventPhotoChanged         *bool                `json:"hasEventPhotoChanged,omitempty"`
	CreatedByCommunityAmbassador *bool                `json:"createdByCommunityAmbassador,omitempty"`
	DiscordChannelID             string               `json:"discordChannelId,omitempty"`
	// Avatar is uploaded with the edit as input.avatarFile. Campfire ignores
	// coverPhotoUrl unless this file is attached and hasEventPhotoChanged is set.
	Avatar *AvatarUpload `json:"-"`
}

// AvatarUpload is a cover image attached to editEvent via the GraphQL multipart spec.
// Bytes are re-encoded to JPEG before upload.
type AvatarUpload struct {
	Data []byte
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
	var resp editEventResp
	if err := c.Do(ctx, token, mutationEditEvent, editEventVars{Input: input}, &resp); err != nil {
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

	data, err := c.doMultipart(ctx, token, "EditEventMutation", mutationEditEvent, editEventMultipartVars{
		Input: editEventMultipartInput{EditEventInput: input},
	}, "variables.input.avatarFile", avatar.Data)
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
	var resp deleteEventResp
	if err := c.Do(ctx, token, mutationDeleteEvent, deleteEventVars{
		Input: DeleteEventInput{EventID: eventID},
	}, &resp); err != nil {
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
