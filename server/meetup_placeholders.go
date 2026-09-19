package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/topi314/campfire-event-manager/server/auth"
	"github.com/topi314/campfire-event-manager/server/placeholders"
)

// meetupPlaceholderMeta is optional context sent with create/edit so the
// server can resolve {{tokens}} before talking to Campfire.
type meetupPlaceholderMeta struct {
	ClubName          string            `json:"clubName"`
	LiveEventName     string            `json:"liveEventName"`
	Category          string            `json:"category"`
	TimeZone          string            `json:"timeZone"`
	PlaceholderValues map[string]string `json:"placeholderValues"`
}

func parseMeetupTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (s *Server) resolveTimeZone(r *http.Request, explicit string) string {
	tz := strings.TrimSpace(explicit)
	if tz != "" {
		if _, err := time.LoadLocation(tz); err == nil {
			return tz
		}
	}
	if session, ok := auth.GetSession(r); ok && session.DiscordUser.TimeZone != nil {
		saved := strings.TrimSpace(*session.DiscordUser.TimeZone)
		if saved != "" {
			if _, err := time.LoadLocation(saved); err == nil {
				return saved
			}
		}
	}
	return "UTC"
}

func (s *Server) applyMeetupPlaceholders(
	r *http.Request,
	name, details, address, cover string,
	lat, lng *float64,
	eventTime, eventEndTime string,
	meta meetupPlaceholderMeta,
) (string, string, string, string) {
	ctx := placeholders.Context{
		ClubName:      meta.ClubName,
		LiveEventName: meta.LiveEventName,
		Category:      meta.Category,
		Title:         name,
		Address:       address,
		Latitude:      lat,
		Longitude:     lng,
		TimeZone:      s.resolveTimeZone(r, meta.TimeZone),
	}
	if t, ok := parseMeetupTime(eventTime); ok {
		ctx.EventTime = t
	}
	if t, ok := parseMeetupTime(eventEndTime); ok {
		ctx.EventEndTime = t
		ctx.HasEndTime = true
	}
	out := placeholders.ResolveAndApply(placeholders.TextFields{
		Name:          name,
		Details:       details,
		Address:       address,
		CoverPhotoURL: cover,
	}, ctx, meta.PlaceholderValues)
	return strings.TrimSpace(out.Name), out.Details, out.Address, out.CoverPhotoURL
}
