package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/topi314/campfire-event-manager/server/auth"
	"github.com/topi314/campfire-event-manager/server/database"
	"github.com/topi314/campfire-event-manager/server/database/dbsqlc"
	"github.com/topi314/campfire-event-manager/server/pokemon"
)

const (
	maxIconBytes          = 8 << 20
	maxIconBatch          = 40
	maxPokemonOnIcon      = 30
	maxLibraryName        = 80
	maxLibraryDescription = 500
)

type iconLibraryResponse struct {
	ID            int64      `json:"id"`
	OwnerID       string     `json:"ownerId"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	IsDefault     bool       `json:"isDefault"`
	Published     bool       `json:"published"`
	Mine          bool       `json:"mine"`
	Saved         bool       `json:"saved"`
	OwnerName     string     `json:"ownerName,omitempty"`
	OwnerUsername string     `json:"ownerUsername,omitempty"`
	Previews      []string   `json:"previews"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	PublishedAt   *time.Time `json:"publishedAt,omitempty"`
}

type iconResponse struct {
	ID               int64     `json:"id"`
	LibraryID        int64     `json:"libraryId"`
	LibraryName      string    `json:"libraryName"`
	Filename         string    `json:"filename"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	Pokemon          []string  `json:"pokemon"`
	URL              string    `json:"url"`
	Rank             int       `json:"rank,omitempty"`
	OwnerID          string    `json:"ownerId,omitempty"`
	LibraryPublished bool      `json:"libraryPublished"`
	CreatedAt        time.Time `json:"createdAt,omitempty"`
}

func packDescription(w http.ResponseWriter, raw string) (string, bool) {
	description := strings.TrimSpace(raw)
	if len(description) > maxLibraryDescription {
		writeError(w, http.StatusBadRequest, "description must be 500 characters or fewer")
		return "", false
	}
	return description, true
}

func packResponse(row dbsqlc.IconLibrary, viewer string) iconLibraryResponse {
	return iconLibraryResponse{
		ID:          row.LibraryID,
		OwnerID:     row.LibraryDiscordUserID,
		Name:        row.LibraryName,
		Description: row.LibraryDescription,
		IsDefault:   row.LibraryIsDefault,
		Published:   row.LibraryPublishedAt.Valid,
		Mine:        row.LibraryDiscordUserID == viewer,
		Previews:    []string{},
		CreatedAt:   timeFrom(row.LibraryCreatedAt),
		UpdatedAt:   timeFrom(row.LibraryUpdatedAt),
		PublishedAt: timePtr(row.LibraryPublishedAt),
	}
}

func iconURL(id int64) string {
	return "/api/icons/" + strconv.FormatInt(id, 10)
}

func (s *Server) listIconLibraries(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	if _, err := s.db.EnsureDefaultIconLibrary(r.Context(), session.Session.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load packs")
		return
	}
	q := r.URL.Query()
	rows, err := s.db.ListVisibleIconLibraries(r.Context(), dbsqlc.ListVisibleIconLibrariesParams{
		ViewerID: session.Session.UserID,
		Query:    strings.TrimSpace(q.Get("q")),
		OwnerID:  strings.TrimSpace(q.Get("owner")),
		Category: strings.TrimSpace(q.Get("category")),
		Pokemon:  strings.TrimSpace(strings.ToLower(q.Get("pokemon"))),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load packs")
		return
	}
	previews, err := s.db.ListRecentIconPreviews(r.Context(), session.Session.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load packs")
		return
	}
	byLibrary := map[int64][]string{}
	for _, p := range previews {
		byLibrary[p.IconLibraryID] = append(byLibrary[p.IconLibraryID], iconURL(p.IconID))
	}
	out := make([]iconLibraryResponse, 0, len(rows))
	for _, row := range rows {
		item := packResponse(dbsqlc.IconLibrary{
			LibraryID:            row.LibraryID,
			LibraryDiscordUserID: row.LibraryDiscordUserID,
			LibraryName:          row.LibraryName,
			LibraryDescription:   row.LibraryDescription,
			LibraryIsDefault:     row.LibraryIsDefault,
			LibraryPublishedAt:   row.LibraryPublishedAt,
			LibraryCreatedAt:     row.LibraryCreatedAt,
			LibraryUpdatedAt:     row.LibraryUpdatedAt,
		}, session.Session.UserID)
		item.Saved = row.Saved
		item.OwnerName = row.DiscordUserDisplayName
		item.OwnerUsername = row.DiscordUserUsername
		item.Previews = byLibrary[row.LibraryID]
		if item.Previews == nil {
			item.Previews = []string{}
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"libraries": out})
}

func (s *Server) createIconLibrary(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len(name) > maxLibraryName {
		writeError(w, http.StatusBadRequest, "pack name is required (max 80 characters)")
		return
	}
	description, ok := packDescription(w, body.Description)
	if !ok {
		return
	}
	row, err := s.db.CreateIconLibrary(r.Context(), dbsqlc.CreateIconLibraryParams{
		LibraryDiscordUserID: session.Session.UserID,
		LibraryName:          name,
		LibraryDescription:   description,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pack")
		return
	}
	writeJSON(w, http.StatusCreated, packResponse(row, session.Session.UserID))
}

func (s *Server) renameIconLibrary(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	id, ok := pathID(w, r, "id", "pack")
	if !ok {
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == nil && body.Description == nil {
		writeError(w, http.StatusBadRequest, "nothing to update")
		return
	}
	params := dbsqlc.UpdateIconLibraryParams{
		LibraryID:            id,
		LibraryDiscordUserID: session.Session.UserID,
	}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" || len(name) > maxLibraryName {
			writeError(w, http.StatusBadRequest, "pack name is required (max 80 characters)")
			return
		}
		params.Name = database.Text(name)
	}
	if body.Description != nil {
		description, ok := packDescription(w, *body.Description)
		if !ok {
			return
		}
		params.Description = database.Text(description)
	}
	row, err := s.db.UpdateIconLibrary(r.Context(), params)
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "pack not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update pack")
		return
	}
	writeJSON(w, http.StatusOK, packResponse(row, session.Session.UserID))
}

func (s *Server) deleteIconLibrary(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	id, ok := pathID(w, r, "id", "pack")
	if !ok {
		return
	}
	paths, err := s.db.ListIconPathsByLibrary(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete pack")
		return
	}
	n, err := s.db.DeleteIconLibrary(r.Context(), dbsqlc.DeleteIconLibraryParams{
		LibraryID:            id,
		LibraryDiscordUserID: session.Session.UserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete pack")
		return
	}
	if n == 0 {
		lib, gerr := s.db.GetIconLibrary(r.Context(), id)
		if gerr == nil && lib.LibraryDiscordUserID == session.Session.UserID && lib.LibraryIsDefault {
			writeError(w, http.StatusBadRequest, "the default pack cannot be deleted")
			return
		}
		writeError(w, http.StatusNotFound, "pack not found")
		return
	}
	for _, p := range paths {
		s.removeIconFile(p.IconStoragePath)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) publishIconLibrary(w http.ResponseWriter, r *http.Request) {
	s.setIconLibraryPublished(w, r, true)
}

func (s *Server) unpublishIconLibrary(w http.ResponseWriter, r *http.Request) {
	s.setIconLibraryPublished(w, r, false)
}

func (s *Server) setIconLibraryPublished(w http.ResponseWriter, r *http.Request, publish bool) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	id, ok := pathID(w, r, "id", "pack")
	if !ok {
		return
	}
	lib, err := s.db.GetIconLibrary(r.Context(), id)
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "pack not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update pack")
		return
	}
	if lib.LibraryDiscordUserID != session.Session.UserID {
		writeError(w, http.StatusNotFound, "pack not found")
		return
	}
	if lib.LibraryIsDefault {
		writeError(w, http.StatusBadRequest, "the default pack cannot be published")
		return
	}
	if publish {
		row, err := s.db.PublishIconLibrary(r.Context(), dbsqlc.PublishIconLibraryParams{
			LibraryID:            id,
			LibraryDiscordUserID: session.Session.UserID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to publish pack")
			return
		}
		writeJSON(w, http.StatusOK, packResponse(row, session.Session.UserID))
		return
	}
	row, err := s.db.UnpublishIconLibrary(r.Context(), dbsqlc.UnpublishIconLibraryParams{
		LibraryID:            id,
		LibraryDiscordUserID: session.Session.UserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unpublish pack")
		return
	}
	writeJSON(w, http.StatusOK, packResponse(row, session.Session.UserID))
}

func (s *Server) saveIconLibrary(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	id, ok := pathID(w, r, "id", "pack")
	if !ok {
		return
	}
	lib, err := s.db.GetIconLibrary(r.Context(), id)
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "pack not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add pack")
		return
	}
	if lib.LibraryDiscordUserID == session.Session.UserID {
		writeError(w, http.StatusBadRequest, "you already own this pack")
		return
	}
	if !lib.LibraryPublishedAt.Valid {
		writeError(w, http.StatusNotFound, "pack not found")
		return
	}
	if err := s.db.SaveIconLibrary(r.Context(), dbsqlc.SaveIconLibraryParams{
		SaveDiscordUserID: session.Session.UserID,
		SaveLibraryID:     id,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add pack")
		return
	}
	item := packResponse(lib, session.Session.UserID)
	item.Saved = true
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) unsaveIconLibrary(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	id, ok := pathID(w, r, "id", "pack")
	if !ok {
		return
	}
	if err := s.db.UnsaveIconLibrary(r.Context(), dbsqlc.UnsaveIconLibraryParams{
		SaveDiscordUserID: session.Session.UserID,
		SaveLibraryID:     id,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove pack")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listLibraryIcons(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	id, ok := pathID(w, r, "id", "pack")
	if !ok {
		return
	}
	lib, err := s.db.GetIconLibrary(r.Context(), id)
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "pack not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load icons")
		return
	}
	if !packVisible(lib, session.Session.UserID) {
		writeError(w, http.StatusNotFound, "pack not found")
		return
	}
	rows, err := s.db.ListIconsByLibrary(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load icons")
		return
	}
	out := make([]iconResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, iconFromLibraryRow(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"icons": out})
}

func (s *Server) uploadIcons(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	id, ok := pathID(w, r, "id", "pack")
	if !ok {
		return
	}
	lib, err := s.db.GetIconLibrary(r.Context(), id)
	if err != nil || lib.LibraryDiscordUserID != session.Session.UserID {
		writeError(w, http.StatusNotFound, "pack not found")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "expected multipart form")
		return
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	if len(files) > maxIconBatch {
		writeError(w, http.StatusBadRequest, "too many files (max 40)")
		return
	}
	category := strings.TrimSpace(r.FormValue("category"))
	if len(category) > 80 {
		writeError(w, http.StatusBadRequest, "category is too long")
		return
	}
	sharedSlugs, err := normalizePokemonList(r.Form["pokemon"])
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	type iconUploadMeta struct {
		Name     string   `json:"name"`
		Category string   `json:"category"`
		Pokemon  []string `json:"pokemon"`
	}
	var metas []iconUploadMeta
	if raw := strings.TrimSpace(r.FormValue("meta")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &metas); err != nil {
			writeError(w, http.StatusBadRequest, "invalid icon details")
			return
		}
		if len(metas) != len(files) {
			writeError(w, http.StatusBadRequest, "icon details must match the files")
			return
		}
	}
	type fileMeta struct {
		name     string
		category string
		slugs    []string
	}
	prepared := make([]fileMeta, len(files))
	for i := range files {
		item := fileMeta{category: category, slugs: sharedSlugs}
		if metas != nil {
			item.name = strings.TrimSpace(metas[i].Name)
			item.category = strings.TrimSpace(metas[i].Category)
			slugs, err := normalizePokemonList(metas[i].Pokemon)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			item.slugs = slugs
		}
		if len(item.name) > maxLibraryName || len(item.category) > 80 {
			writeError(w, http.StatusBadRequest, "name or category is too long")
			return
		}
		prepared[i] = item
	}
	saved := make([]iconResponse, 0, len(files))
	var problems []string
	for i, hdr := range files {
		meta := prepared[i]
		f, err := hdr.Open()
		if err != nil {
			problems = append(problems, hdr.Filename+": failed to read")
			continue
		}
		data, err := io.ReadAll(io.LimitReader(f, maxIconBytes+1))
		f.Close()
		if err != nil || len(data) == 0 {
			problems = append(problems, hdr.Filename+": empty or unreadable")
			continue
		}
		if len(data) > maxIconBytes {
			problems = append(problems, hdr.Filename+": larger than 8MB")
			continue
		}
		contentType := hdr.Header.Get("Content-Type")
		ct, ext, ok := sniffIcon(contentType, data)
		if !ok {
			problems = append(problems, hdr.Filename+": must be jpeg, png, or webp")
			continue
		}
		filename := path.Base(strings.TrimSpace(hdr.Filename))
		if filename == "" || filename == "." || filename == "/" {
			filename = "icon" + ext
		}
		row, err := s.db.InsertIcon(r.Context(), dbsqlc.InsertIconParams{
			IconLibraryID:   id,
			IconFilename:    filename,
			IconName:        meta.name,
			IconContentType: ct,
			IconStoragePath: "",
			IconCategory:    database.Text(meta.category),
		})
		if err != nil {
			problems = append(problems, filename+": failed to store")
			continue
		}
		rel := filepath.ToSlash(filepath.Join("icons", session.Session.UserID, strconv.FormatInt(id, 10), strconv.FormatInt(row.IconID, 10)+ext))
		if err := s.writeIconFile(rel, data); err != nil {
			_, _ = s.db.DeleteIconForOwner(r.Context(), dbsqlc.DeleteIconForOwnerParams{
				IconID:               row.IconID,
				LibraryDiscordUserID: session.Session.UserID,
			})
			problems = append(problems, filename+": failed to save file")
			continue
		}
		if err := s.db.SetIconStoragePath(r.Context(), dbsqlc.SetIconStoragePathParams{
			IconID:          row.IconID,
			IconStoragePath: rel,
		}); err != nil {
			s.removeIconFile(rel)
			_, _ = s.db.DeleteIconForOwner(r.Context(), dbsqlc.DeleteIconForOwnerParams{
				IconID:               row.IconID,
				LibraryDiscordUserID: session.Session.UserID,
			})
			problems = append(problems, filename+": failed to store")
			continue
		}
		if err := replaceIconPokemon(r, s, row.IconID, meta.slugs); err != nil {
			problems = append(problems, filename+": saved without species")
		}
		saved = append(saved, iconResponse{
			ID:          row.IconID,
			LibraryID:   id,
			LibraryName: lib.LibraryName,
			Filename:    filename,
			Name:        meta.name,
			Category:    meta.category,
			Pokemon:     meta.slugs,
			URL:         iconURL(row.IconID),
		})
	}
	if len(saved) == 0 && len(problems) > 0 {
		writeError(w, http.StatusBadRequest, problems[0])
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"icons": saved, "errors": problems})
}

func (s *Server) getIcon(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	id, ok := pathID(w, r, "id", "icon")
	if !ok {
		return
	}
	row, err := s.db.GetIcon(r.Context(), id)
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "icon not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load icon")
		return
	}
	if !iconVisible(row.LibraryDiscordUserID, row.LibraryPublishedAt, session.Session.UserID) {
		writeError(w, http.StatusNotFound, "icon not found")
		return
	}
	full, err := s.iconAbsPath(row.IconStoragePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load icon")
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		writeError(w, http.StatusNotFound, "icon not found")
		return
	}
	w.Header().Set("Content-Type", row.IconContentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) updateIcon(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	id, ok := pathID(w, r, "id", "icon")
	if !ok {
		return
	}
	row, err := s.db.GetIcon(r.Context(), id)
	if err != nil || row.LibraryDiscordUserID != session.Session.UserID {
		writeError(w, http.StatusNotFound, "icon not found")
		return
	}
	var body struct {
		Name     *string  `json:"name"`
		Category *string  `json:"category"`
		Pokemon  []string `json:"pokemon"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	iconName := strings.TrimSpace(row.IconName)
	if body.Name != nil {
		iconName = strings.TrimSpace(*body.Name)
		if len(iconName) > maxLibraryName {
			writeError(w, http.StatusBadRequest, "name is too long")
			return
		}
	}
	category := strings.TrimSpace(row.IconCategory.String)
	if body.Category != nil {
		category = strings.TrimSpace(*body.Category)
		if len(category) > 80 {
			writeError(w, http.StatusBadRequest, "category is too long")
			return
		}
	}
	if err := s.db.UpdateIconFields(r.Context(), dbsqlc.UpdateIconFieldsParams{
		IconID:       id,
		IconName:     iconName,
		IconCategory: database.Text(category),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update icon")
		return
	}
	slugs := nonEmpty(row.PokemonSlugs)
	if body.Pokemon != nil {
		slugs, err = normalizePokemonList(body.Pokemon)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := replaceIconPokemon(r, s, id, slugs); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update icon")
			return
		}
	}
	writeJSON(w, http.StatusOK, iconResponse{
		ID:          row.IconID,
		LibraryID:   row.IconLibraryID,
		LibraryName: row.LibraryName,
		Filename:    row.IconFilename,
		Name:        iconName,
		Category:    category,
		Pokemon:     slugs,
		URL:         iconURL(row.IconID),
	})
}

func (s *Server) deleteIcon(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	id, ok := pathID(w, r, "id", "icon")
	if !ok {
		return
	}
	rel, err := s.db.DeleteIconForOwner(r.Context(), dbsqlc.DeleteIconForOwnerParams{
		IconID:               id,
		LibraryDiscordUserID: session.Session.UserID,
	})
	if err != nil {
		if database.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "icon not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete icon")
		return
	}
	s.removeIconFile(rel)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listIcons(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	q := r.URL.Query()
	var packID int64
	if raw := strings.TrimSpace(q.Get("libraryId")); raw != "" {
		var err error
		packID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || packID < 0 {
			writeError(w, http.StatusBadRequest, "invalid pack id")
			return
		}
	}
	rows, err := s.db.ListAccessibleIcons(r.Context(), dbsqlc.ListAccessibleIconsParams{
		ViewerID:  session.Session.UserID,
		LibraryID: packID,
		Category:  strings.TrimSpace(q.Get("category")),
		Pokemon:   strings.TrimSpace(strings.ToLower(q.Get("pokemon"))),
		Query:     strings.TrimSpace(q.Get("q")),
		MineOnly:  q.Get("scope") == "mine",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list icons")
		return
	}
	out := make([]iconResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, iconFromAccessibleRow(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"icons": out})
}

func (s *Server) matchIcons(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	q := r.URL.Query()
	libraryID, err := strconv.ParseInt(strings.TrimSpace(q.Get("libraryId")), 10, 64)
	if err != nil || libraryID <= 0 {
		writeError(w, http.StatusBadRequest, "libraryId is required")
		return
	}
	lib, err := s.db.GetIconLibrary(r.Context(), libraryID)
	if err != nil || !packVisible(lib, session.Session.UserID) {
		writeError(w, http.StatusNotFound, "pack not found")
		return
	}
	inMine, err := s.db.IconLibraryInMyPacks(r.Context(), dbsqlc.IconLibraryInMyPacksParams{
		SaveDiscordUserID: session.Session.UserID,
		LibraryID:         libraryID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to match icons")
		return
	}
	if !inMine {
		writeJSON(w, http.StatusOK, map[string]any{"icons": []iconResponse{}})
		return
	}
	rows, err := s.db.ListIconsByLibrary(r.Context(), libraryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to match icons")
		return
	}
	category := strings.TrimSpace(q.Get("category"))
	eventName := strings.TrimSpace(q.Get("eventName"))
	var eventSlugs []string
	for _, sp := range pokemon.ExtractSpecies(eventName, category, nil) {
		if sp != nil && sp.Slug != "" {
			eventSlugs = append(eventSlugs, sp.Slug)
		}
	}
	if extra := strings.TrimSpace(q.Get("pokemon")); extra != "" {
		if sp := pokemon.BySlug(extra); sp != nil {
			eventSlugs = append(eventSlugs, sp.Slug)
		}
	}
	type ranked struct {
		row  dbsqlc.ListIconsByLibraryRow
		rank int
	}
	var hits []ranked
	for _, row := range rows {
		rank, ok := iconMatchRank(category, eventName, row.IconCategory.String, row.PokemonSlugs, eventSlugs)
		if ok {
			hits = append(hits, ranked{row: row, rank: rank})
		}
	}
	// Best rank first; stable enough for a small pack.
	for i := 1; i < len(hits); i++ {
		j := i
		for j > 0 && hits[j].rank > hits[j-1].rank {
			hits[j], hits[j-1] = hits[j-1], hits[j]
			j--
		}
	}
	out := make([]iconResponse, 0, len(hits))
	for _, hit := range hits {
		item := iconFromLibraryRow(hit.row)
		item.Rank = hit.rank
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"icons": out})
}

func (s *Server) searchPokemon(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var found []pokemon.Species
	if q == "" {
		found = pokemon.List()
	} else {
		found = pokemon.Search(q, 20)
	}
	out := make([]map[string]string, 0, len(found))
	for _, sp := range found {
		out = append(out, map[string]string{"slug": sp.Slug, "name": sp.Name})
	}
	writeJSON(w, http.StatusOK, map[string]any{"pokemon": out})
}

func packVisible(lib dbsqlc.IconLibrary, viewer string) bool {
	return lib.LibraryDiscordUserID == viewer || lib.LibraryPublishedAt.Valid
}

func iconVisible(owner string, published pgtype.Timestamp, viewer string) bool {
	return owner == viewer || published.Valid
}

func iconFromLibraryRow(row dbsqlc.ListIconsByLibraryRow) iconResponse {
	return iconResponse{
		ID:               row.IconID,
		LibraryID:        row.IconLibraryID,
		LibraryName:      row.LibraryName,
		Filename:         row.IconFilename,
		Name:             row.IconName,
		Category:         row.IconCategory.String,
		Pokemon:          nonEmpty(row.PokemonSlugs),
		URL:              iconURL(row.IconID),
		OwnerID:          row.LibraryDiscordUserID,
		LibraryPublished: row.LibraryPublishedAt.Valid,
		CreatedAt:        timeFrom(row.IconCreatedAt),
	}
}

func iconFromAccessibleRow(row dbsqlc.ListAccessibleIconsRow) iconResponse {
	return iconResponse{
		ID:               row.IconID,
		LibraryID:        row.IconLibraryID,
		LibraryName:      row.LibraryName,
		Filename:         row.IconFilename,
		Name:             row.IconName,
		Category:         row.IconCategory.String,
		Pokemon:          nonEmpty(row.PokemonSlugs),
		URL:              iconURL(row.IconID),
		OwnerID:          row.LibraryDiscordUserID,
		LibraryPublished: row.LibraryPublishedAt.Valid,
		CreatedAt:        timeFrom(row.IconCreatedAt),
	}
}

func nonEmpty(slugs []string) []string {
	if slugs == nil {
		return []string{}
	}
	return slugs
}

func normalizePokemonList(raw []string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		sp := pokemon.BySlug(item)
		if sp == nil {
			sp = pokemon.MatchEnglish(item)
		}
		if sp == nil {
			return nil, fmt.Errorf("unknown pokémon %q", item)
		}
		if _, ok := seen[sp.Slug]; ok {
			continue
		}
		seen[sp.Slug] = struct{}{}
		out = append(out, sp.Slug)
		if len(out) > maxPokemonOnIcon {
			return nil, fmt.Errorf("too many pokémon (max %d)", maxPokemonOnIcon)
		}
	}
	return out, nil
}

func replaceIconPokemon(r *http.Request, s *Server, iconID int64, slugs []string) error {
	if err := s.db.DeleteIconPokemon(r.Context(), iconID); err != nil {
		return err
	}
	for _, slug := range slugs {
		if err := s.db.InsertIconPokemon(r.Context(), dbsqlc.InsertIconPokemonParams{
			IconID:      iconID,
			PokemonSlug: slug,
		}); err != nil {
			return err
		}
	}
	return nil
}

func sniffIcon(contentType string, data []byte) (string, string, bool) {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch ct {
	case "image/jpeg", "image/jpg", "image/pjpeg":
		return "image/jpeg", ".jpg", true
	case "image/png":
		return "image/png", ".png", true
	case "image/webp":
		return "image/webp", ".webp", true
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp", ".webp", true
	}
	detected := http.DetectContentType(data)
	switch detected {
	case "image/jpeg":
		return "image/jpeg", ".jpg", true
	case "image/png":
		return "image/png", ".png", true
	default:
		return "", "", false
	}
}

func (s *Server) writeIconFile(rel string, data []byte) error {
	full, err := s.iconAbsPath(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

func (s *Server) removeIconFile(rel string) {
	full, err := s.iconAbsPath(rel)
	if err != nil || strings.TrimSpace(rel) == "" {
		return
	}
	_ = os.Remove(full)
}

func (s *Server) iconAbsPath(rel string) (string, error) {
	root, err := filepath.Abs(s.cfg.DataDir)
	if err != nil {
		return "", err
	}
	rel = filepath.Clean(filepath.FromSlash(strings.TrimSpace(rel)))
	if rel == "." || rel == "" || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid icon path")
	}
	full := filepath.Join(root, rel)
	rootPrefix := root + string(os.PathSeparator)
	if !strings.HasPrefix(full, rootPrefix) {
		return "", fmt.Errorf("invalid icon path")
	}
	return full, nil
}

func (s *Server) readIconBytes(rel string) ([]byte, error) {
	full, err := s.iconAbsPath(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func viewerID(r *http.Request) string {
	session, ok := auth.GetSession(r)
	if !ok {
		return ""
	}
	return session.Session.UserID
}

func pathID(w http.ResponseWriter, r *http.Request, name, label string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue(name)), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid "+label+" id")
		return 0, false
	}
	return id, true
}

func parseIconID(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	const marker = "/api/icons/"
	i := strings.Index(raw, marker)
	if i < 0 {
		return 0, false
	}
	rest := raw[i+len(marker):]
	if j := strings.IndexAny(rest, "?#/"); j >= 0 {
		rest = rest[:j]
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
