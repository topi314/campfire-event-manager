package server

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/topi314/campfire-event-manager/internal/eventcategory"
	"github.com/topi314/campfire-event-manager/server/campfire"
)

func (s *Server) requireCampfireToken(r *http.Request) (string, bool) {
	token := campfire.BearerFromRequest(r)
	return token, token != ""
}

// resolveCoverAvatar returns cover bytes for avatarFile on create/edit.
// Local covers (/api/covers/{id}) are loaded from our DB; https URLs are fetched.
func (s *Server) resolveCoverAvatar(ctx context.Context, coverURL string) (*campfire.AvatarUpload, error) {
	coverURL = strings.TrimSpace(coverURL)
	if id, ok := parseCoverImageID(coverURL); ok {
		img, err := s.db.GetCoverImage(ctx, id)
		if err != nil {
			return nil, err
		}
		return &campfire.AvatarUpload{Data: img.CoverData}, nil
	}
	return s.campfire.FetchImage(ctx, coverURL)
}

func (s *Server) campfireMe(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}
	me, err := s.campfire.Me(r.Context(), token)
	if err != nil {
		if campfire.IsAuthError(err) {
			writeError(w, http.StatusUnauthorized, "invalid Campfire token")
			return
		}
		writeCampfireError(w, err)
		return
	}
	if me == nil {
		writeError(w, http.StatusUnauthorized, "invalid Campfire token")
		return
	}
	writeJSON(w, http.StatusOK, me)
}

func (s *Server) campfireClubs(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}
	clubs, err := s.campfire.Clubs(r.Context(), token)
	if err != nil {
		writeCampfireError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, clubs)
}

func (s *Server) campfireLiveEvents(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}
	events, err := s.campfire.LiveEvents(r.Context(), token)
	if err != nil {
		writeCampfireError(w, err)
		return
	}
	for i := range events {
		cat := eventcategory.FromName(events[i].EventName)
		events[i].Category = cat
		events[i].FixedLocalHour = eventcategory.IsFixedLocalHour(cat)
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) campfireClubMembers(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}
	clubID := strings.TrimSpace(r.PathValue("clubId"))
	if clubID == "" {
		writeError(w, http.StatusBadRequest, "clubId required")
		return
	}
	q := r.URL.Query()
	if search := strings.TrimSpace(q.Get("q")); search != "" {
		members, err := s.campfire.SearchClubMembers(r.Context(), token, clubID, search)
		if err != nil {
			writeCampfireError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"members": members, "hasNextPage": false})
		return
	}
	first := 50
	if n, err := strconv.Atoi(q.Get("first")); err == nil && n > 0 && n <= 100 {
		first = n
	}
	after := q.Get("after")
	members, cursor, hasNext, err := s.campfire.ClubMembers(r.Context(), token, clubID, first, after)
	if err != nil {
		writeCampfireError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"members":     members,
		"endCursor":   cursor,
		"hasNextPage": hasNext,
	})
}

func (s *Server) campfireClubEvents(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}
	clubID := strings.TrimSpace(r.PathValue("clubId"))
	if clubID == "" {
		writeError(w, http.StatusBadRequest, "clubId required")
		return
	}
	q := r.URL.Query()
	first := 50
	if n, err := strconv.Atoi(q.Get("first")); err == nil && n > 0 && n <= 100 {
		first = n
	}
	after := q.Get("after")
	lite := q.Get("lite") == "1" || strings.EqualFold(q.Get("lite"), "true")
	archived := q.Get("feed") == "past"
	res, err := s.campfire.ActiveEvents(r.Context(), token, clubID, first, after, lite, archived)
	if err != nil {
		writeCampfireError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":      res.Events,
		"clubId":      res.ClubID,
		"clubName":    res.ClubName,
		"endCursor":   res.EndCursor,
		"hasNextPage": res.HasNextPage,
	})
}

func (s *Server) campfireClubDiscord(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}
	clubID := strings.TrimSpace(r.PathValue("clubId"))
	if clubID == "" {
		writeError(w, http.StatusBadRequest, "clubId required")
		return
	}
	settings, err := s.campfire.DiscordSettings(r.Context(), token, clubID)
	if err != nil {
		if campfire.IsAuthError(err) {
			writeError(w, http.StatusUnauthorized, "invalid Campfire token")
			return
		}
		writeCampfireError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) campfireGetMeetup(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}
	eventID := strings.TrimSpace(r.PathValue("eventId"))
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "eventId required")
		return
	}
	ev, err := s.campfire.Event(r.Context(), token, eventID)
	if err != nil {
		if campfire.IsAuthError(err) {
			writeError(w, http.StatusUnauthorized, "invalid Campfire token")
			return
		}
		writeCampfireError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ev)
}

func (s *Server) campfireCreateMeetup(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}

	var body struct {
		ClubID                       string            `json:"clubId"`
		Name                         string            `json:"name"`
		Details                      string            `json:"details"`
		EventTime                    string            `json:"eventTime"`
		EventEndTime                 string            `json:"eventEndTime"`
		Address                      string            `json:"address"`
		PlaceID                      string            `json:"placeId"`
		CoverPhotoURL                string            `json:"coverPhotoUrl"`
		CommentsPermissions          string            `json:"commentsPermissions"`
		AllInvited                   *bool             `json:"allInvited"`
		InviteeIDs                   []string          `json:"inviteeIds"`
		CampfireLiveEventID          string            `json:"campfireLiveEventId"`
		CreatedByCommunityAmbassador *bool             `json:"createdByCommunityAmbassador"`
		DiscordChannelID             string            `json:"discordChannelId"`
		Latitude                     *float64          `json:"latitude"`
		Longitude                    *float64          `json:"longitude"`
		ClubName                     string            `json:"clubName"`
		LiveEventName                string            `json:"liveEventName"`
		Category                     string            `json:"category"`
		TimeZone                     string            `json:"timeZone"`
		Language                     string            `json:"language"`
		PlaceholderValues            map[string]string `json:"placeholderValues"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	clubID := strings.TrimSpace(body.ClubID)
	name := strings.TrimSpace(body.Name)
	eventTime := strings.TrimSpace(body.EventTime)
	if clubID == "" || name == "" || eventTime == "" {
		writeError(w, http.StatusBadRequest, "clubId, name, and eventTime are required")
		return
	}
	if body.Latitude == nil || body.Longitude == nil {
		writeError(w, http.StatusBadRequest, "pick a location on the map")
		return
	}

	eventTime, eventEndTime, err := s.resolveMeetupCampfireTimes(r, eventTime, body.EventEndTime, body.TimeZone)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid event time: "+err.Error())
		return
	}

	name, details, address, cover := s.applyMeetupPlaceholders(
		r,
		name,
		body.Details,
		body.Address,
		body.CoverPhotoURL,
		body.Latitude,
		body.Longitude,
		eventTime,
		eventEndTime,
		meetupPlaceholderMeta{
			ClubName:          body.ClubName,
			LiveEventName:     body.LiveEventName,
			Category:          body.Category,
			TimeZone:          body.TimeZone,
			Language:          body.Language,
			PlaceholderValues: body.PlaceholderValues,
		},
	)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if msg := meetupTextOverLimit(name, details); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	userIDs := body.InviteeIDs
	if userIDs == nil {
		userIDs = []string{}
	}
	comments := campfire.NormalizeCommentsPermissions(body.CommentsPermissions)
	cover = strings.TrimSpace(cover)

	var avatar *campfire.AvatarUpload
	if cover != "" {
		var err error
		avatar, err = s.resolveCoverAvatar(r.Context(), cover)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to load cover image: "+err.Error())
			return
		}
	}

	// Freeform meetups: one createActivityReminder with location + optional avatarFile
	// (same as the Campfire app). No follow-up editEvent.
	// discordChannelId is trusted from the client (validated against club Discord channels in the UI).
	createInput := campfire.CreateActivityReminderInput{
		ClubID:                       clubID,
		Name:                         name,
		Details:                      details,
		EventTime:                    eventTime,
		EventEndTime:                 eventEndTime,
		Location:                     campfire.FormatEditLocation(*body.Latitude, *body.Longitude),
		Address:                      address,
		PlaceID:                      strings.TrimSpace(body.PlaceID),
		CoverPhotoURL:                "",
		CommentsPermissions:          comments,
		AllInvited:                   body.AllInvited,
		UserIDs:                      userIDs,
		CampfireLiveEventID:          strings.TrimSpace(body.CampfireLiveEventID),
		CreatedByCommunityAmbassador: body.CreatedByCommunityAmbassador,
		DiscordChannelID:             strings.TrimSpace(body.DiscordChannelID),
		Avatar:                       avatar,
	}

	event, err := s.campfire.CreateActivityReminder(r.Context(), token, createInput)
	if err != nil {
		slog.ErrorContext(r.Context(), "create activity reminder failed", slog.Any("err", err), slog.String("clubId", clubID))
		if campfire.IsUnavailable(err) {
			writeError(w, http.StatusBadGateway, campfire.UnavailableMessage)
			return
		}
		writeError(w, http.StatusBadGateway, "Could not create the meetup. Try again, or reach out on Discord if it keeps happening.")
		return
	}
	if event == nil || event.ID == "" {
		slog.ErrorContext(r.Context(), "create activity reminder returned no event", slog.String("clubId", clubID))
		writeError(w, http.StatusBadGateway, "Could not create the meetup. Try again, or reach out on Discord if it keeps happening.")
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) campfireEditMeetup(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}
	eventID := strings.TrimSpace(r.PathValue("eventId"))
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "eventId required")
		return
	}

	var body struct {
		Name                         string            `json:"name"`
		Details                      string            `json:"details"`
		EventTime                    string            `json:"eventTime"`
		EventEndTime                 string            `json:"eventEndTime"`
		Latitude                     *float64          `json:"latitude"`
		Longitude                    *float64          `json:"longitude"`
		Address                      string            `json:"address"`
		PlaceID                      string            `json:"placeId"`
		CoverPhotoURL                string            `json:"coverPhotoUrl"`
		CommentsPermissions          string            `json:"commentsPermissions"`
		AllInvited                   *bool             `json:"allInvited"`
		CreatedByCommunityAmbassador *bool             `json:"createdByCommunityAmbassador"`
		CampfireLiveEventID          string            `json:"campfireLiveEventId"`
		ClearCampfireLiveEvent       bool              `json:"clearCampfireLiveEvent"`
		HasEventPhotoChanged         *bool             `json:"hasEventPhotoChanged"`
		DiscordChannelID             string            `json:"discordChannelId"`
		ClubName                     string            `json:"clubName"`
		LiveEventName                string            `json:"liveEventName"`
		Category                     string            `json:"category"`
		TimeZone                     string            `json:"timeZone"`
		Language                     string            `json:"language"`
		PlaceholderValues            map[string]string `json:"placeholderValues"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	name := strings.TrimSpace(body.Name)
	eventTime := strings.TrimSpace(body.EventTime)
	if name == "" || eventTime == "" {
		writeError(w, http.StatusBadRequest, "name and eventTime are required")
		return
	}
	if body.Latitude == nil || body.Longitude == nil {
		writeError(w, http.StatusBadRequest, "pick a location on the map")
		return
	}

	eventTime, eventEndTime, err := s.resolveMeetupCampfireTimes(r, eventTime, body.EventEndTime, body.TimeZone)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid event time: "+err.Error())
		return
	}

	name, details, address, cover := s.applyMeetupPlaceholders(
		r,
		name,
		body.Details,
		body.Address,
		body.CoverPhotoURL,
		body.Latitude,
		body.Longitude,
		eventTime,
		eventEndTime,
		meetupPlaceholderMeta{
			ClubName:          body.ClubName,
			LiveEventName:     body.LiveEventName,
			Category:          body.Category,
			TimeZone:          body.TimeZone,
			Language:          body.Language,
			PlaceholderValues: body.PlaceholderValues,
		},
	)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if msg := meetupTextOverLimit(name, details); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	comments := campfire.NormalizeCommentsPermissions(body.CommentsPermissions)
	userIDs := []string{}
	cover = strings.TrimSpace(cover)
	var avatar *campfire.AvatarUpload
	photoChanged := body.HasEventPhotoChanged != nil && *body.HasEventPhotoChanged
	if photoChanged && cover != "" {
		var err error
		avatar, err = s.resolveCoverAvatar(r.Context(), cover)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to load cover image: "+err.Error())
			return
		}
		cover = ""
	}
	input := campfire.EditEventInput{
		EventID:                      eventID,
		Name:                         name,
		Details:                      details,
		EventTime:                    eventTime,
		EventEndTime:                 eventEndTime,
		Location:                     campfire.FormatEditLocation(*body.Latitude, *body.Longitude),
		Address:                      address,
		PlaceID:                      strings.TrimSpace(body.PlaceID),
		CoverPhotoURL:                cover,
		CommentsPermissions:          comments,
		AllInvited:                   body.AllInvited,
		UserIDs:                      userIDs,
		CreatedByCommunityAmbassador: body.CreatedByCommunityAmbassador,
		HasEventPhotoChanged:         body.HasEventPhotoChanged,
		DiscordChannelID:             strings.TrimSpace(body.DiscordChannelID),
		Avatar:                       avatar,
	}
	if liveID := strings.TrimSpace(body.CampfireLiveEventID); liveID != "" {
		id := campfire.NullableLiveEventID(liveID)
		input.CampfireLiveEventID = &id
	} else if body.ClearCampfireLiveEvent {
		id := campfire.NullableLiveEventID("")
		input.CampfireLiveEventID = &id
	}

	event, err := s.campfire.EditEvent(r.Context(), token, input)
	if err != nil {
		writeCampfireError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (s *Server) campfireDeleteMeetup(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}
	eventID := strings.TrimSpace(r.PathValue("eventId"))
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "eventId required")
		return
	}

	okDel, err := s.campfire.DeleteEvent(r.Context(), token, eventID)
	if err != nil {
		writeCampfireError(w, err)
		return
	}
	if !okDel {
		writeError(w, http.StatusBadGateway, "deleteEvent did not succeed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "id": eventID})
}

func (s *Server) campfireMeetupShareLink(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}
	eventID := strings.TrimSpace(r.PathValue("eventId"))
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "eventId required")
		return
	}
	shareURL, err := s.campfire.CreateEventLink(r.Context(), token, eventID)
	if err != nil {
		writeCampfireError(w, err)
		return
	}
	shortID := campfire.ShortIDFromShareURL(shareURL)
	if shortID == "" {
		writeError(w, http.StatusBadGateway, "createLink returned url without short id")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"url":     shareURL,
		"shortId": shortID,
	})
}
