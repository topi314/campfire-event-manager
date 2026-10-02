package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"

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
		filename := strings.TrimSpace(img.CoverFilename)
		if filename == "" {
			filename = "cover.jpg"
		}
		return &campfire.AvatarUpload{
			Filename:    filename,
			ContentType: img.CoverContentType,
			Data:        img.CoverData,
		}, nil
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
		writeError(w, http.StatusBadGateway, err.Error())
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
		writeError(w, http.StatusBadGateway, err.Error())
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
		writeError(w, http.StatusBadGateway, err.Error())
		return
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
			writeError(w, http.StatusBadGateway, err.Error())
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
		writeError(w, http.StatusBadGateway, err.Error())
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
	events, cursor, hasNext, err := s.campfire.ActiveEvents(r.Context(), token, clubID, first, after)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":      events,
		"endCursor":   cursor,
		"hasNextPage": hasNext,
	})
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
		Latitude                     *float64          `json:"latitude"`
		Longitude                    *float64          `json:"longitude"`
		ClubName                     string            `json:"clubName"`
		LiveEventName                string            `json:"liveEventName"`
		Category                     string            `json:"category"`
		TimeZone                     string            `json:"timeZone"`
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
			PlaceholderValues: body.PlaceholderValues,
		},
	)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
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
		Avatar:                       avatar,
	}

	event, err := s.campfire.CreateActivityReminder(r.Context(), token, createInput)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if event == nil || event.ID == "" {
		writeError(w, http.StatusBadGateway, "createActivityReminder returned no event")
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
		HasEventPhotoChanged         *bool             `json:"hasEventPhotoChanged"`
		ClubName                     string            `json:"clubName"`
		LiveEventName                string            `json:"liveEventName"`
		Category                     string            `json:"category"`
		TimeZone                     string            `json:"timeZone"`
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
			PlaceholderValues: body.PlaceholderValues,
		},
	)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
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
		CampfireLiveEventID:          strings.TrimSpace(body.CampfireLiveEventID),
		HasEventPhotoChanged:         body.HasEventPhotoChanged,
		Avatar:                       avatar,
	}

	event, err := s.campfire.EditEvent(r.Context(), token, input)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
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
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if !okDel {
		writeError(w, http.StatusBadGateway, "deleteEvent did not succeed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "id": eventID})
}
