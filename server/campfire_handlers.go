package server

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/topi314/campfire-event-manager/server/campfire"
)

func (s *Server) requireCampfireToken(r *http.Request) (string, bool) {
	token := campfire.BearerFromRequest(r)
	return token, token != ""
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

func (s *Server) campfireMapObjects(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}
	q := r.URL.Query()
	south, err1 := strconv.ParseFloat(q.Get("south"), 64)
	west, err2 := strconv.ParseFloat(q.Get("west"), 64)
	north, err3 := strconv.ParseFloat(q.Get("north"), 64)
	east, err4 := strconv.ParseFloat(q.Get("east"), 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		writeError(w, http.StatusBadRequest, "south, west, north, east query params required")
		return
	}
	if south >= north || west >= east {
		writeError(w, http.StatusBadRequest, "invalid bounds")
		return
	}
	// Reject world-sized queries.
	if north-south > 0.5 || east-west > 0.5 {
		writeError(w, http.StatusBadRequest, "bounds too large; zoom in")
		return
	}
	pois, err := s.campfire.MapObjectsInBounds(r.Context(), token, south, west, north, east)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pois)
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
		MapObjectID                  string            `json:"mapObjectId"`
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
	mapObjectID := strings.TrimSpace(body.MapObjectID)
	if clubID == "" || name == "" || eventTime == "" {
		writeError(w, http.StatusBadRequest, "clubId, name, and eventTime are required")
		return
	}
	if body.Latitude == nil || body.Longitude == nil {
		if mapObjectID == "" {
			writeError(w, http.StatusBadRequest, "pick a location on the map")
			return
		}
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
		body.EventEndTime,
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
		avatar, err = s.campfire.FetchImage(r.Context(), cover)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to load cover image: "+err.Error())
			return
		}
	}

	// createPoiMeetup has no location field; dropId anchors on a POI or "" for freeform.
	createInput := campfire.CreatePoiEventInput{
		ClubID:                       clubID,
		Name:                         name,
		Details:                      details,
		EventTime:                    eventTime,
		EventEndTime:                 strings.TrimSpace(body.EventEndTime),
		DropID:                       mapObjectID,
		Address:                      address,
		PlaceID:                      strings.TrimSpace(body.PlaceID),
		CommentsPermissions:          comments,
		AllInvited:                   body.AllInvited,
		UserIDs:                      userIDs,
		Game:                         campfire.GamePGO,
		CampfireLiveEventID:          strings.TrimSpace(body.CampfireLiveEventID),
		CreatedByCommunityAmbassador: body.CreatedByCommunityAmbassador,
	}

	event, err := s.campfire.CreatePoiMeetup(r.Context(), token, createInput)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if event == nil || event.ID == "" {
		writeError(w, http.StatusBadGateway, "createPoiMeetup returned no event")
		return
	}

	// Freeform coordinates are applied via editEvent. A cover is attached as
	// avatarFile on that same edit; a bare coverPhotoUrl does not stick.
	lat, lng := 0.0, 0.0
	if body.Latitude != nil && body.Longitude != nil {
		lat, lng = *body.Latitude, *body.Longitude
	} else if event.Location != "" {
		if parsedLat, parsedLng, ok := campfire.ParseLocationString(event.Location); ok {
			lat, lng = parsedLat, parsedLng
		}
	}
	photoChanged := avatar != nil
	editInput := campfire.EditEventInput{
		EventID:                      event.ID,
		Name:                         name,
		Details:                      details,
		EventTime:                    eventTime,
		EventEndTime:                 strings.TrimSpace(body.EventEndTime),
		Location:                     campfire.FormatEditLocation(lat, lng),
		Address:                      address,
		PlaceID:                      strings.TrimSpace(body.PlaceID),
		CoverPhotoURL:                "",
		CommentsPermissions:          comments,
		AllInvited:                   body.AllInvited,
		UserIDs:                      userIDs,
		CampfireLiveEventID:          strings.TrimSpace(body.CampfireLiveEventID),
		HasEventPhotoChanged:         &photoChanged,
		CreatedByCommunityAmbassador: body.CreatedByCommunityAmbassador,
		Avatar:                       avatar,
	}
	edited, err := s.campfire.EditEvent(r.Context(), token, editInput)
	if err != nil {
		_, _ = s.campfire.DeleteEvent(r.Context(), token, event.ID)
		writeError(w, http.StatusBadGateway, "created meetup but failed to set location: "+err.Error())
		return
	}
	if edited != nil {
		event = edited
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

	name, details, address, cover := s.applyMeetupPlaceholders(
		r,
		name,
		body.Details,
		body.Address,
		body.CoverPhotoURL,
		body.Latitude,
		body.Longitude,
		eventTime,
		body.EventEndTime,
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
		avatar, err = s.campfire.FetchImage(r.Context(), cover)
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
		EventEndTime:                 strings.TrimSpace(body.EventEndTime),
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

func (s *Server) campfireUploadImage(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}

	const maxMem = 9 << 20
	if err := r.ParseMultipartForm(maxMem); err != nil {
		writeError(w, http.StatusBadRequest, "expected multipart form with file: "+err.Error())
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 8<<20+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "empty file")
		return
	}
	if len(data) > 8<<20 {
		writeError(w, http.StatusBadRequest, "file too large (max 8MB)")
		return
	}

	contentType := hdr.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	rec, err := s.campfire.UploadImage(r.Context(), token, hdr.Filename, contentType, data)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	url := rec.PermanentURL
	if url == "" {
		url = rec.TransientURL
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"photoId":      rec.PhotoID,
		"url":          url,
		"permanentUrl": rec.PermanentURL,
		"transientUrl": rec.TransientURL,
	})
}

func (s *Server) campfireRemoveImage(w http.ResponseWriter, r *http.Request) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
		return
	}

	photoID := strings.TrimSpace(r.URL.Query().Get("photoId"))
	if photoID == "" {
		var body struct {
			PhotoID string `json:"photoId"`
		}
		if err := decodeJSON(r, &body); err == nil {
			photoID = strings.TrimSpace(body.PhotoID)
		}
	}
	if photoID == "" {
		writeError(w, http.StatusBadRequest, "missing photoId")
		return
	}

	if err := s.campfire.RemoveImage(r.Context(), token, photoID); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
