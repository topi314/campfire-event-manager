package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/topi314/campfire-event-manager/server/auth"
	"github.com/topi314/campfire-event-manager/server/database"
	"github.com/topi314/campfire-event-manager/server/languages"
)

type templateBody struct {
	Name    string          `json:"name"`
	Payload json.RawMessage `json:"payload"`
}

func (s *Server) listTemplates(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	templates, err := s.db.ListTemplates(r.Context(), session.Session.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

func (s *Server) getTemplate(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	t, err := s.db.GetTemplate(r.Context(), session.Session.UserID, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get template")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) createTemplate(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	var body templateBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	t, err := s.db.CreateTemplate(r.Context(), session.Session.UserID, body.Name, body.Payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create template")
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) updateTemplate(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body templateBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	t, err := s.db.UpdateTemplate(r.Context(), session.Session.UserID, id, body.Name, body.Payload)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update template")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.DeleteTemplate(r.Context(), session.Session.UserID, id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete template")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) publishTemplate(w http.ResponseWriter, r *http.Request) {
	s.setTemplatePublish(w, r, true)
}

func (s *Server) unpublishTemplate(w http.ResponseWriter, r *http.Request) {
	s.setTemplatePublish(w, r, false)
}

func (s *Server) setTemplatePublish(w http.ResponseWriter, r *http.Request, published bool) {
	session, _ := auth.GetSession(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	description := ""
	language := ""
	if published {
		var body struct {
			Description string `json:"description"`
			Language    string `json:"language"`
		}
		if err := decodeJSON(r, &body); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		description = strings.TrimSpace(body.Description)
		if len(description) > 500 {
			writeError(w, http.StatusBadRequest, "description must be 500 characters or fewer")
			return
		}
		language = languages.Normalize(body.Language)
		if language == "" {
			writeError(w, http.StatusBadRequest, "language is required")
			return
		}
	}

	t, err := s.db.SetTemplatePublished(r.Context(), session.Session.UserID, id, published, description, language)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update publish state")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) sharedTemplates(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	f := database.SharedFilter{
		Query:        r.URL.Query().Get("q"),
		Category:     r.URL.Query().Get("category"),
		Language:     r.URL.Query().Get("language"),
		PublisherID:  r.URL.Query().Get("publisherId"),
		Publisher:    r.URL.Query().Get("creator"),
		Sort:         r.URL.Query().Get("sort"),
		ViewerUserID: session.Session.UserID,
	}
	templates, err := s.db.ListPublishedTemplates(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list shared templates")
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

func (s *Server) likeTemplate(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.LikeTemplate(r.Context(), session.Session.UserID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to like template")
		return
	}
	count, likedByMe, likers, err := s.db.GetTemplateLikeState(r.Context(), session.Session.UserID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load likes")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"likeCount": count,
		"likedByMe": likedByMe,
		"likers":    likers,
	})
}

func (s *Server) unlikeTemplate(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.UnlikeTemplate(r.Context(), session.Session.UserID, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlike template")
		return
	}
	count, likedByMe, likers, err := s.db.GetTemplateLikeState(r.Context(), session.Session.UserID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load likes")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"likeCount": count,
		"likedByMe": likedByMe,
		"likers":    likers,
	})
}

func (s *Server) cloneTemplate(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	src, err := s.db.GetTemplateByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load template")
		return
	}
	isOwner := src.DiscordUserID == session.Session.UserID
	if !isOwner && src.PublishedAt == nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	originID := src.ID
	// Prefer the ultimate origin when cloning a clone.
	if src.OriginID != nil {
		originID = *src.OriginID
	}
	payload := src.Payload
	if src.PublishedAt != nil {
		// Shared templates never include the publisher's pin.
		payload = database.StripSharedFieldsFromPayload(src.Payload)
	}
	t, err := s.db.CreateTemplateClone(
		r.Context(),
		session.Session.UserID,
		src.Name,
		payload,
		&originID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clone template")
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("userId"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing user id")
		return
	}
	user, err := s.db.GetDiscordUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}
	f := database.SharedFilter{
		Query:        r.URL.Query().Get("q"),
		Category:     r.URL.Query().Get("category"),
		Language:     r.URL.Query().Get("language"),
		Sort:         r.URL.Query().Get("sort"),
		ViewerUserID: "",
	}
	view := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("view")))
	if view == "liked" {
		f.LikedByUserID = userID
		if f.Sort == "" {
			f.Sort = "liked_desc"
		}
	} else {
		f.PublisherID = userID
	}
	if session, ok := auth.GetSession(r); ok {
		f.ViewerUserID = session.Session.UserID
	}
	templates, err := s.db.ListPublishedTemplates(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list published templates")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":          user.ID,
			"username":    user.Username,
			"displayName": user.DisplayName,
			"avatarUrl":   user.AvatarURL,
		},
		"templates": templates,
	})
}

func (s *Server) importTemplates(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		// also accept single "file"
		files = r.MultipartForm.File["file"]
	}
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "no files uploaded")
		return
	}

	type imported struct {
		Name string `json:"name"`
		ID   int64  `json:"id"`
	}
	var created []imported
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to open upload")
			return
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read upload")
			return
		}

		name := strings.TrimSuffix(fh.Filename, ".json")
		payload := json.RawMessage(data)

		// Support { "name": "...", "payload": {...} } or bare payload object.
		var wrapper struct {
			Name    string          `json:"name"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Payload) > 0 {
			if wrapper.Name != "" {
				name = wrapper.Name
			}
			payload = wrapper.Payload
		} else if !json.Valid(data) {
			writeError(w, http.StatusBadRequest, "invalid json in "+fh.Filename)
			return
		}

		if strings.TrimSpace(name) == "" {
			name = "Imported template"
		}
		t, err := s.db.CreateTemplate(r.Context(), session.Session.UserID, name, payload)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to import template")
			return
		}
		created = append(created, imported{Name: t.Name, ID: t.ID})
	}
	writeJSON(w, http.StatusCreated, created)
}
