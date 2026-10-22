package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/topi314/campfire-event-manager/server/auth"
	"github.com/topi314/campfire-event-manager/server/database"
	"github.com/topi314/campfire-event-manager/server/database/dbsqlc"
	"github.com/topi314/campfire-event-manager/server/languages"
)

type templateBody struct {
	Name             string          `json:"name"`
	Payload          json.RawMessage `json:"payload"`
	Language         string          `json:"language"`
	OverriddenFields []string        `json:"overriddenFields"`
}

func (s *Server) listTemplates(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	rows, err := s.db.ListTemplatesByUser(r.Context(), session.Session.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}
	templates := make([]MeetupTemplate, 0, len(rows))
	for _, row := range rows {
		templates = append(templates, templateWithOrigin(row))
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
	row, err := s.db.GetTemplateForUser(r.Context(), dbsqlc.GetTemplateForUserParams{
		TemplateID:            id,
		TemplateDiscordUserID: session.Session.UserID,
	})
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get template")
		return
	}
	writeJSON(w, http.StatusOK, templateWithOriginGet(row))
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
	lang := languages.Normalize(body.Language)
	if lang == "" {
		writeError(w, http.StatusBadRequest, "language is required")
		return
	}
	t, err := s.createTemplateRow(r.Context(), session.Session.UserID, body.Name, body.Payload, lang, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create template")
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) createTemplateRow(ctx context.Context, userID, name string, payload json.RawMessage, language string, originID *int64) (MeetupTemplate, error) {
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	overrides := json.RawMessage(`{}`)
	synced := originID != nil
	now := time.Now().UTC()
	row, err := s.db.CreateTemplate(ctx, dbsqlc.CreateTemplateParams{
		TemplateDiscordUserID: userID,
		TemplateName:          name,
		TemplatePayload:       payload,
		TemplateOriginID:      database.Int8Ptr(originID),
		TemplateSynced:        synced,
		TemplateOverrides:     overrides,
		TemplateLanguage:      database.Text(languages.Normalize(language)),
		TemplateCreatedAt:     database.Ts(now),
		TemplateUpdatedAt:     database.Ts(now),
	})
	if err != nil {
		return MeetupTemplate{}, err
	}
	t := templateFromCreate(row)
	if originID != nil {
		full, err := s.db.GetTemplateForUser(ctx, dbsqlc.GetTemplateForUserParams{
			TemplateID:            t.ID,
			TemplateDiscordUserID: userID,
		})
		if err == nil {
			return templateWithOriginGet(full), nil
		}
	}
	return t, nil
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
	lang := languages.Normalize(body.Language)
	if lang == "" {
		writeError(w, http.StatusBadRequest, "language is required")
		return
	}
	if len(body.Payload) == 0 {
		body.Payload = json.RawMessage(`{}`)
	}

	existing, err := s.db.GetTemplateForUser(r.Context(), dbsqlc.GetTemplateForUserParams{
		TemplateID:            id,
		TemplateDiscordUserID: session.Session.UserID,
	})
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load template")
		return
	}

	name := body.Name
	payload := body.Payload
	overrides := json.RawMessage(`{}`)
	synced := false
	if existing.TemplateOriginID.Valid {
		built, err := buildOverridesFromFields(body.OverriddenFields, body.Name, lang, body.Payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid overrides")
			return
		}
		overrides = built
		synced = isEmptyOverrides(overrides)
		// Keep a materialized snapshot for fallback if the origin disappears.
		originLang := stringPtr(existing.OriginLanguage)
		merged := mergeLinkedTemplate(
			body.Name, &lang, body.Payload, overrides,
			textString(existing.OriginName), originLang, json.RawMessage(existing.OriginPayload),
			existing.OriginID.Valid,
		)
		name, lang, payload = materializeEffective(merged)
	}

	now := time.Now().UTC()
	_, err = s.db.UpdateTemplate(r.Context(), dbsqlc.UpdateTemplateParams{
		TemplateName:          name,
		TemplatePayload:       payload,
		TemplateLanguage:      database.Text(lang),
		TemplateOverrides:     overrides,
		TemplateSynced:        synced,
		TemplateID:            id,
		TemplateDiscordUserID: session.Session.UserID,
		TemplateUpdatedAt:     database.Ts(now),
	})
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update template")
		return
	}
	full, err := s.db.GetTemplateForUser(r.Context(), dbsqlc.GetTemplateForUserParams{
		TemplateID:            id,
		TemplateDiscordUserID: session.Session.UserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load template")
		return
	}
	writeJSON(w, http.StatusOK, templateWithOriginGet(full))
}

func (s *Server) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	n, err := s.db.DeleteTemplate(r.Context(), dbsqlc.DeleteTemplateParams{
		TemplateID:            id,
		TemplateDiscordUserID: session.Session.UserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete template")
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "template not found")
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
	if published {
		var body struct {
			Description string `json:"description"`
			Language    string `json:"language"` // optional override; prefer stored template language
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
	}

	now := time.Now().UTC()
	var t MeetupTemplate
	if published {
		existing, err := s.db.GetTemplateForUser(r.Context(), dbsqlc.GetTemplateForUserParams{
			TemplateID:            id,
			TemplateDiscordUserID: session.Session.UserID,
		})
		if err != nil {
			if database.IsNotFound(err) {
				writeError(w, http.StatusNotFound, "template not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to load template")
			return
		}
		language := ""
		if existing.TemplateLanguage.Valid {
			language = languages.Normalize(existing.TemplateLanguage.String)
		}
		if language == "" {
			writeError(w, http.StatusBadRequest, "set a language on the template before publishing")
			return
		}
		if existing.TemplateOriginID.Valid {
			writeError(w, http.StatusBadRequest, "unlink from the original before publishing")
			return
		}
		row, err := s.db.PublishTemplate(r.Context(), dbsqlc.PublishTemplateParams{
			Now:         database.Ts(now),
			Description: database.Text(description),
			Language:    database.Text(language),
			ID:          id,
			UserID:      session.Session.UserID,
		})
		if err != nil {
			if database.IsNotFound(err) {
				writeError(w, http.StatusNotFound, "template not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to update publish state")
			return
		}
		t = templateFromPublish(row)
	} else {
		row, err := s.db.UnpublishTemplate(r.Context(), dbsqlc.UnpublishTemplateParams{
			TemplateID:            id,
			TemplateDiscordUserID: session.Session.UserID,
			TemplateUpdatedAt:     database.Ts(now),
		})
		if err != nil {
			if database.IsNotFound(err) {
				writeError(w, http.StatusNotFound, "template not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to update publish state")
			return
		}
		t = templateFromUnpublish(row)
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) sharedTemplates(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	templates, err := s.listPublishedTemplates(r.Context(), SharedFilter{
		Query:        r.URL.Query().Get("q"),
		Category:     r.URL.Query().Get("category"),
		Language:     r.URL.Query().Get("language"),
		PublisherID:  r.URL.Query().Get("publisherId"),
		Publisher:    r.URL.Query().Get("creator"),
		Sort:         r.URL.Query().Get("sort"),
		ViewerUserID: session.Session.UserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list shared templates")
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

func (s *Server) sharedTemplateFacets(w http.ResponseWriter, r *http.Request) {
	categories, err := s.db.ListPublishedCategories(r.Context(), database.Text(r.URL.Query().Get("publisherId")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list categories")
		return
	}
	if categories == nil {
		categories = []string{}
	}
	rows, err := s.db.ListPublishedCreators(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list creators")
		return
	}
	creators := make([]UserRef, 0, len(rows))
	for _, row := range rows {
		creators = append(creators, UserRef{
			ID:          row.DiscordUserID,
			Username:    row.DiscordUserUsername,
			DisplayName: row.DiscordUserDisplayName,
			AvatarURL:   row.DiscordUserAvatarUrl,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
		"creators":   creators,
	})
}

func (s *Server) likeTemplate(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.likeTemplateRecord(r.Context(), session.Session.UserID, id); err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to like template")
		return
	}
	count, likedByMe, likers, err := s.templateLikeState(r.Context(), session.Session.UserID, id)
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
	if err := s.db.UnlikeTemplate(r.Context(), dbsqlc.UnlikeTemplateParams{
		TemplateID:    id,
		DiscordUserID: session.Session.UserID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlike template")
		return
	}
	count, likedByMe, likers, err := s.templateLikeState(r.Context(), session.Session.UserID, id)
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
	srcRow, err := s.db.GetTemplateByID(r.Context(), id)
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load template")
		return
	}
	src := templateFromByID(srcRow)
	isOwner := src.DiscordUserID == session.Session.UserID
	if !isOwner && src.PublishedAt == nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	originID := src.ID
	if src.OriginID != nil {
		originID = *src.OriginID
	}
	// Snapshot name/language/payload for fallback; live values come from origin while linked.
	t, err := s.createTemplateRow(
		r.Context(),
		session.Session.UserID,
		src.Name,
		src.Payload,
		ptrString(src.Language),
		&originID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clone template")
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) unlinkTemplate(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := s.db.GetTemplateForUser(r.Context(), dbsqlc.GetTemplateForUserParams{
		TemplateID:            id,
		TemplateDiscordUserID: session.Session.UserID,
	})
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load template")
		return
	}
	if !existing.TemplateOriginID.Valid {
		writeError(w, http.StatusBadRequest, "template is not linked to an original")
		return
	}
	linked := templateWithOriginGet(existing)
	name, lang, payload := materializeEffective(mergedTemplate{
		Name:     linked.Name,
		Language: linked.Language,
		Payload:  linked.Payload,
	})
	now := time.Now().UTC()
	row, err := s.db.UnlinkTemplate(r.Context(), dbsqlc.UnlinkTemplateParams{
		TemplateName:          name,
		TemplatePayload:       payload,
		TemplateLanguage:      database.Text(lang),
		TemplateID:            id,
		TemplateDiscordUserID: session.Session.UserID,
		TemplateUpdatedAt:     database.Ts(now),
	})
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to unlink template")
		return
	}
	writeJSON(w, http.StatusOK, templateFromUnlink(row))
}

func ptrString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.PathValue("userId"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing user id")
		return
	}
	userRow, err := s.db.GetDiscordUser(r.Context(), userID)
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}
	user := auth.DiscordUserFrom(userRow)
	f := SharedFilter{
		Query:        r.URL.Query().Get("q"),
		Category:     r.URL.Query().Get("category"),
		Language:     r.URL.Query().Get("language"),
		Sort:         r.URL.Query().Get("sort"),
		ViewerUserID: "",
	}
	view := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("view")))
	session, hasSession := auth.GetSession(r)
	if hasSession {
		f.ViewerUserID = session.Session.UserID
	}
	if view == "liked" {
		if !hasSession || session.Session.UserID != userID {
			writeError(w, http.StatusForbidden, "liked templates are private")
			return
		}
		f.LikedByUserID = userID
		if f.Sort == "" {
			f.Sort = "liked_desc"
		}
	} else {
		f.PublisherID = userID
	}
	templates, err := s.listPublishedTemplates(r.Context(), f)
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
		files = r.MultipartForm.File["file"]
	}
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "no files uploaded")
		return
	}

	existing, err := s.db.ListTemplatesByUser(r.Context(), session.Session.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}
	byID := make(map[int64]string, len(existing)) // id → current language
	for _, row := range existing {
		lang := ""
		if row.TemplateLanguage.Valid {
			lang = strings.TrimSpace(row.TemplateLanguage.String)
		}
		byID[row.TemplateID] = lang
	}

	type imported struct {
		Name    string `json:"name"`
		ID      int64  `json:"id"`
		Updated bool   `json:"updated"`
	}
	var results []imported
	createdCount := 0
	updatedCount := 0

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

		entries, err := parseImportEntries(fh.Filename, data)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error()+" in "+fh.Filename)
			return
		}

		for _, entry := range entries {
			name := strings.TrimSpace(entry.Name)
			if name == "" {
				name = "Imported template"
			}
			payload := entry.Payload
			if len(payload) == 0 {
				payload = json.RawMessage(`{}`)
			}
			lang := languages.Normalize(entry.Language)

			if existingLang, ok := byID[entry.ID]; ok && entry.ID > 0 {
				if lang == "" {
					lang = languages.Normalize(existingLang)
				}
				now := time.Now().UTC()
				cur, err := s.db.GetTemplateForUser(r.Context(), dbsqlc.GetTemplateForUserParams{
					TemplateID:            entry.ID,
					TemplateDiscordUserID: session.Session.UserID,
				})
				if err != nil {
					writeError(w, http.StatusInternalServerError, "failed to load template")
					return
				}
				overrides := json.RawMessage(`{}`)
				synced := false
				if cur.TemplateOriginID.Valid {
					// Keep the import as local overrides so it is not wiped by origin merge.
					built, err := buildOverridesFromFields(allOverrideFieldIDs(), name, lang, payload)
					if err != nil {
						writeError(w, http.StatusInternalServerError, "failed to build overrides")
						return
					}
					overrides = built
				}
				row, err := s.db.UpdateTemplate(r.Context(), dbsqlc.UpdateTemplateParams{
					TemplateName:          name,
					TemplatePayload:       payload,
					TemplateLanguage:      database.Text(lang),
					TemplateOverrides:     overrides,
					TemplateSynced:        synced,
					TemplateID:            entry.ID,
					TemplateDiscordUserID: session.Session.UserID,
					TemplateUpdatedAt:     database.Ts(now),
				})
				if err != nil {
					writeError(w, http.StatusInternalServerError, "failed to update template")
					return
				}
				t := templateFromUpdate(row)
				byID[t.ID] = ptrString(t.Language)
				results = append(results, imported{Name: t.Name, ID: t.ID, Updated: true})
				updatedCount++
				continue
			}

			t, err := s.createTemplateRow(r.Context(), session.Session.UserID, name, payload, lang, nil)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to import template")
				return
			}
			byID[t.ID] = ptrString(t.Language)
			results = append(results, imported{Name: t.Name, ID: t.ID, Updated: false})
			createdCount++
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"templates": results,
		"created":   createdCount,
		"updated":   updatedCount,
	})
}
