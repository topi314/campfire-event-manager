package campfire

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type LiveEvent struct {
	ID                   string  `json:"id"`
	EventName            string  `json:"eventName"`
	StartTimestamp       string  `json:"startTimestamp"`
	EndTimestamp         string  `json:"endTimestamp"`
	LocalStartTime       string  `json:"localStartTime"`
	LocalEndTime         string  `json:"localEndTime"`
	ModalHeadingImageURL string  `json:"modalHeadingImageUrl"`
	Location             *LatLng `json:"location"`
}

type LatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type liveEventsResp struct {
	MarketableCampfireLiveEvents []LiveEvent `json:"marketableCampfireLiveEvents"`
}

func (c *Client) LiveEvents(ctx context.Context, token string) ([]LiveEvent, error) {
	var resp liveEventsResp
	if err := c.Do(ctx, token, queryLiveEvents, liveEventsVars{
		Input: marketableCampfireLiveEventsInput{Game: GamePGO},
	}, &resp); err != nil {
		return nil, err
	}
	events := resp.MarketableCampfireLiveEvents
	if events == nil {
		events = []LiveEvent{}
	}
	return events, nil
}

type ClubMember struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
}

type clubMembersResp struct {
	Club struct {
		ID      string `json:"id"`
		Members struct {
			Edges []struct {
				Node   ClubMember `json:"node"`
				Cursor string     `json:"cursor"`
			} `json:"edges"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"members"`
	} `json:"club"`
}

func (c *Client) ClubMembers(ctx context.Context, token, clubID string, first int, after string) ([]ClubMember, string, bool, error) {
	if first <= 0 {
		first = 50
	}
	var resp clubMembersResp
	if err := c.Do(ctx, token, queryClubMembers, clubMembersVars{
		ClubID: clubID,
		First:  first,
		After:  after,
	}, &resp); err != nil {
		return nil, "", false, err
	}
	members := make([]ClubMember, 0, len(resp.Club.Members.Edges))
	for _, e := range resp.Club.Members.Edges {
		members = append(members, e.Node)
	}
	return members, resp.Club.Members.PageInfo.EndCursor, resp.Club.Members.PageInfo.HasNextPage, nil
}

type searchMembersResp struct {
	Club struct {
		ID           string       `json:"id"`
		MemberSearch []ClubMember `json:"memberSearch"`
	} `json:"club"`
}

func (c *Client) SearchClubMembers(ctx context.Context, token, clubID, search string) ([]ClubMember, error) {
	var resp searchMembersResp
	if err := c.Do(ctx, token, querySearchClubMembers, searchClubMembersVars{
		ClubID: clubID,
		Search: search,
	}, &resp); err != nil {
		return nil, err
	}
	members := resp.Club.MemberSearch
	if members == nil {
		members = []ClubMember{}
	}
	return members, nil
}

// CreateActivityReminderInput is Campfire's freeform meetup create
// (what the app uses when not anchoring on a POI drop). Location is
// "[lng, lat]"; cover is multipart input.avatarFile — one call, no edit.
type CreateActivityReminderInput struct {
	ClubID                       string   `json:"clubId"`
	Name                         string   `json:"name"`
	Details                      string   `json:"details"`
	EventTime                    string   `json:"eventTime"`
	EventEndTime                 string   `json:"eventEndTime,omitempty"`
	Location                     string   `json:"location,omitempty"` // "[lng, lat]"
	Address                      string   `json:"address"`
	PlaceID                      string   `json:"placeId,omitempty"`
	CoverPhotoURL                string   `json:"coverPhotoUrl"`
	CommentsPermissions          string   `json:"commentsPermissions,omitempty"`
	AllInvited                   *bool    `json:"allInvited,omitempty"`
	UserIDs                      []string `json:"userIds"`
	CampfireLiveEventID          string        `json:"campfireLiveEventId,omitempty"`
	CreatedByCommunityAmbassador *bool         `json:"createdByCommunityAmbassador,omitempty"`
	Avatar                       *AvatarUpload `json:"-"`
}

// NormalizeCommentsPermissions maps UI / template values to Campfire's
// EventCommentsPermissions enum (ORGANIZERS_ONLY | ALL_INVITED | NO_ONE).
// Unknown values are cleared (Campfire defaults to ORGANIZERS_ONLY when omitted).
func NormalizeCommentsPermissions(v string) string {
	switch strings.TrimSpace(v) {
	case "ORGANIZERS_ONLY", "HOST_ONLY", "host_only", "host", "organizers_only":
		return "ORGANIZERS_ONLY"
	case "ALL_INVITED", "ATTENDEES", "ATTENDEES_ONLY", "EVERYONE", "anyone", "attendees":
		return "ALL_INVITED"
	case "NO_ONE", "DISABLED", "NONE", "OFF", "no_one", "nobody":
		return "NO_ONE"
	case "":
		return ""
	default:
		return ""
	}
}

type CreatedEvent struct {
	ID            string `json:"id"`
	ClubID        string `json:"clubId"`
	EventTime     string `json:"eventTime"`
	EventEndTime  string `json:"eventEndTime"`
	Location      string `json:"location"`
	Name          string `json:"name"`
	Details       string `json:"details"`
	Address       string `json:"address"`
	CoverPhotoURL string `json:"coverPhotoUrl,omitempty"`
}

type createActivityReminderResp struct {
	CreateActivityReminder struct {
		Event *CreatedEvent `json:"event"`
	} `json:"createActivityReminder"`
}

func (c *Client) CreateActivityReminder(ctx context.Context, token string, input CreateActivityReminderInput) (*CreatedEvent, error) {
	avatar := input.Avatar
	input.Avatar = nil
	input.CoverPhotoURL = ""

	if avatar != nil && len(avatar.Data) > 0 {
		filename := strings.TrimSpace(avatar.Filename)
		if filename == "" {
			filename = "cover.jpg"
		}
		contentType := strings.TrimSpace(avatar.ContentType)
		if contentType == "" {
			contentType = "image/jpeg"
		}
		data, err := c.doMultipart(
			ctx,
			token,
			"CreateActivityReminderMutation",
			mutationCreateActivityReminder,
			createActivityReminderMultipartVars{
				Input: createActivityReminderMultipartInput{CreateActivityReminderInput: input},
			},
			"variables.input.avatarFile",
			filename,
			contentType,
			avatar.Data,
		)
		if err != nil {
			return nil, err
		}
		var resp createActivityReminderResp
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("decode createActivityReminder: %w", err)
		}
		return resp.CreateActivityReminder.Event, nil
	}

	var resp createActivityReminderResp
	if err := c.Do(ctx, token, mutationCreateActivityReminder, createActivityReminderVars{Input: input}, &resp); err != nil {
		return nil, err
	}
	return resp.CreateActivityReminder.Event, nil
}
