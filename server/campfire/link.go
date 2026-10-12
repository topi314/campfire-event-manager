package campfire

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type createLinkVars struct {
	Input createLinkInput `json:"input"`
}

type createLinkInput struct {
	EventID string `json:"eventId"`
}

type createLinkResp struct {
	CreateLink struct {
		Link struct {
			URL string `json:"url"`
		} `json:"link"`
	} `json:"createLink"`
}

// CreateEventLink returns Campfire's short share URL for an event (e.g. https://cmpf.re/xxxxxx).
func (c *Client) CreateEventLink(ctx context.Context, token, eventID string) (string, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return "", fmt.Errorf("eventId required")
	}
	var resp createLinkResp
	if err := c.Do(ctx, token, mutationCreateLink, createLinkVars{
		Input: createLinkInput{EventID: eventID},
	}, &resp); err != nil {
		return "", err
	}
	u := strings.TrimSpace(resp.CreateLink.Link.URL)
	if u == "" {
		return "", fmt.Errorf("createLink returned empty url")
	}
	return u, nil
}

// ShortIDFromShareURL extracts the short path id from a Campfire share URL
// (e.g. https://cmpf.re/vN87iT → vN87iT).
func ShortIDFromShareURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" || u.Path == "/" {
		parts := strings.Split(strings.Trim(raw, "/"), "/")
		if len(parts) == 0 {
			return ""
		}
		return parts[len(parts)-1]
	}
	return strings.Trim(u.Path, "/")
}
