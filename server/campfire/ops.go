package campfire

import (
	"context"
)

// GamePGO is the only game this app manages meetups for.
const GamePGO = "PGO"

type Badge struct {
	BadgeType string `json:"badgeType"`
	Alias     string `json:"alias"`
}

type Me struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	AvatarURL   string  `json:"avatarUrl"`
	Badges      []Badge `json:"badges"`
}

type meResp struct {
	Me *Me `json:"me"`
}

func (c *Client) Me(ctx context.Context, token string) (*Me, error) {
	var resp meResp
	if err := c.Do(ctx, token, queryMe, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Me, nil
}

type Club struct {
	ID                            string `json:"id"`
	Name                          string `json:"name"`
	Game                          string `json:"game"`
	AvatarURL                     string `json:"avatarUrl"`
	CreatedByCommunityAmbassador  bool   `json:"createdByCommunityAmbassador"`
	AmIAdmin                      bool   `json:"amIAdmin"`
	OnlyAllowAdminsToCreateEvents bool   `json:"onlyAllowAdminsToCreateEvents"`
	Members                       struct {
		TotalCount int `json:"totalCount"`
	} `json:"members"`
}

// CanCreateMeetups reports whether the current user may create meetups in this club.
func (c Club) CanCreateMeetups() bool {
	if c.OnlyAllowAdminsToCreateEvents {
		return c.AmIAdmin
	}
	return true
}

type clubsResp struct {
	Me struct {
		ID       string `json:"id"`
		MemberOf struct {
			Edges []struct {
				Node Club `json:"node"`
			} `json:"edges"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		} `json:"memberOf"`
	} `json:"me"`
}

// Clubs returns PGO clubs the current user belongs to where they may create meetups.
func (c *Client) Clubs(ctx context.Context, token string) ([]Club, error) {
	const pageSize = 100
	out := make([]Club, 0)
	after := ""
	for {
		var resp clubsResp
		if err := c.Do(ctx, token, queryClubs, clubsVars{First: pageSize, After: after}, &resp); err != nil {
			return nil, err
		}
		for _, e := range resp.Me.MemberOf.Edges {
			club := e.Node
			if club.Game != GamePGO {
				continue
			}
			if club.CanCreateMeetups() {
				out = append(out, club)
			}
		}
		if !resp.Me.MemberOf.PageInfo.HasNextPage || resp.Me.MemberOf.PageInfo.EndCursor == "" {
			break
		}
		after = resp.Me.MemberOf.PageInfo.EndCursor
	}
	return out, nil
}
