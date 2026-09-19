package campfire

import (
	"context"
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
	vars := map[string]any{
		"input": map[string]any{"game": GamePGO},
	}
	var resp liveEventsResp
	if err := c.Do(ctx, token, queryLiveEvents, vars, &resp); err != nil {
		return nil, err
	}
	events := resp.MarketableCampfireLiveEvents
	if events == nil {
		events = []LiveEvent{}
	}
	return events, nil
}

type MapPOI struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"`
	Name     string  `json:"name"`
	ImageURL string  `json:"imageUrl,omitempty"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
}

type rawMapObject struct {
	ID            string `json:"id"`
	MapObjectType string `json:"mapObjectType"`
	PgoGym        *struct {
		Name     string `json:"name"`
		ImageURL string `json:"imageUrl"`
		Location LatLng `json:"location"`
	} `json:"pgoGym"`
	PgoPowerspot *struct {
		Name     string `json:"name"`
		Location LatLng `json:"location"`
	} `json:"pgoPowerspot"`
	PgoPokestop *struct {
		Name     string `json:"name"`
		ImageURL string `json:"imageUrl"`
		Location LatLng `json:"location"`
	} `json:"pgoPokestop"`
}

type mapObjectsResp struct {
	MapObjectsInLatLngBounds []rawMapObject `json:"mapObjectsInLatLngBounds"`
}

// MapObjectsInBounds loads gyms/stops/powerspots in the given viewport.
func (c *Client) MapObjectsInBounds(ctx context.Context, token string, south, west, north, east float64) ([]MapPOI, error) {
	vars := map[string]any{
		"mapObjectsInput": map[string]any{
			"latLngBounds": map[string]any{
				"southwest": map[string]float64{"latitude": south, "longitude": west},
				"northeast": map[string]float64{"latitude": north, "longitude": east},
			},
		},
	}
	var resp mapObjectsResp
	if err := c.Do(ctx, token, queryMapObjects, vars, &resp); err != nil {
		// Fallback: flat LatLngBounds field names used by some Campfire ops.
		vars = map[string]any{
			"mapObjectsInput": map[string]any{
				"latLngBounds": map[string]float64{
					"latitudeSouthwest":  south,
					"longitudeSouthwest": west,
					"latitudeNortheast":  north,
					"longitudeNortheast": east,
				},
			},
		}
		if err2 := c.Do(ctx, token, queryMapObjects, vars, &resp); err2 != nil {
			return nil, fmt.Errorf("%v; fallback: %w", err, err2)
		}
	}

	out := make([]MapPOI, 0, len(resp.MapObjectsInLatLngBounds))
	for _, mo := range resp.MapObjectsInLatLngBounds {
		poi := MapPOI{ID: mo.ID, Type: mo.MapObjectType}
		switch {
		case mo.PgoGym != nil:
			poi.Type = "gym"
			poi.Name = mo.PgoGym.Name
			poi.ImageURL = mo.PgoGym.ImageURL
			poi.Lat = mo.PgoGym.Location.Latitude
			poi.Lng = mo.PgoGym.Location.Longitude
		case mo.PgoPowerspot != nil:
			poi.Type = "powerspot"
			poi.Name = mo.PgoPowerspot.Name
			poi.Lat = mo.PgoPowerspot.Location.Latitude
			poi.Lng = mo.PgoPowerspot.Location.Longitude
		case mo.PgoPokestop != nil:
			poi.Type = "pokestop"
			poi.Name = mo.PgoPokestop.Name
			poi.ImageURL = mo.PgoPokestop.ImageURL
			poi.Lat = mo.PgoPokestop.Location.Latitude
			poi.Lng = mo.PgoPokestop.Location.Longitude
		default:
			continue
		}
		if poi.ID == "" || (poi.Lat == 0 && poi.Lng == 0) {
			continue
		}
		if poi.Name == "" {
			poi.Name = poi.Type
		}
		out = append(out, poi)
	}
	return out, nil
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
	vars := map[string]any{
		"clubId": clubID,
		"first":  first,
	}
	if after != "" {
		vars["after"] = after
	}
	var resp clubMembersResp
	if err := c.Do(ctx, token, queryClubMembers, vars, &resp); err != nil {
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
	vars := map[string]any{"clubId": clubID, "search": search}
	var resp searchMembersResp
	if err := c.Do(ctx, token, querySearchClubMembers, vars, &resp); err != nil {
		return nil, err
	}
	members := resp.Club.MemberSearch
	if members == nil {
		members = []ClubMember{}
	}
	return members, nil
}

// CreatePoiEventInput is the GraphQL input for createPoiMeetup.
// Campfire rejects location/coverPhotoUrl/mapObjectId here — freeform pins
// are applied afterwards with editEvent (location as "[lng, lat]").
// DropID is required by the schema; pass a map-object id to anchor on a
// gym/stop, or "" for a freeform pin.
type CreatePoiEventInput struct {
	ClubID                       string   `json:"clubId"`
	Name                         string   `json:"name"`
	Details                      string   `json:"details"`
	EventTime                    string   `json:"eventTime"`
	EventEndTime                 string   `json:"eventEndTime,omitempty"`
	DropID                       string   `json:"dropId"`
	Address                      string   `json:"address"`
	PlaceID                      string   `json:"placeId,omitempty"`
	CommentsPermissions          string   `json:"commentsPermissions,omitempty"`
	AllInvited                   *bool    `json:"allInvited,omitempty"`
	UserIDs                      []string `json:"userIds"`
	Game                         string   `json:"game"`
	CampfireLiveEventID          string   `json:"campfireLiveEventId,omitempty"`
	CreatedByCommunityAmbassador *bool    `json:"createdByCommunityAmbassador,omitempty"`
	DiscordChannelID             string   `json:"discordChannelId,omitempty"`
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
	PhotoID       string `json:"photoId,omitempty"`
}

type createMeetupResp struct {
	CreatePoiMeetup struct {
		Event *CreatedEvent `json:"event"`
	} `json:"createPoiMeetup"`
}

func (c *Client) CreatePoiMeetup(ctx context.Context, token string, input CreatePoiEventInput) (*CreatedEvent, error) {
	vars := map[string]any{"input": input}
	var resp createMeetupResp
	if err := c.Do(ctx, token, mutationCreatePoiMeetup, vars, &resp); err != nil {
		return nil, err
	}
	return resp.CreatePoiMeetup.Event, nil
}
