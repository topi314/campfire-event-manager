package server

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/topi314/campfire-event-manager/server/auth"
)

const maxCoverBytes = 8 << 20

func (s *Server) uploadCover(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
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

	data, err := io.ReadAll(io.LimitReader(file, maxCoverBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "empty file")
		return
	}
	if len(data) > maxCoverBytes {
		writeError(w, http.StatusBadRequest, "file too large (max 8MB)")
		return
	}

	contentType := hdr.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(contentType, "image/") {
		writeError(w, http.StatusBadRequest, "file must be an image")
		return
	}

	filename := path.Base(strings.TrimSpace(hdr.Filename))
	if filename == "" || filename == "." || filename == "/" {
		filename = "cover.jpg"
	}

	img, err := s.db.InsertCoverImage(r.Context(), session.Session.UserID, filename, contentType, data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store cover image")
		return
	}

	url := coverAPIPath(img.ID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          img.ID,
		"url":         url,
		"contentType": img.ContentType,
		"filename":    img.Filename,
	})
}

func (s *Server) getCover(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid cover id")
		return
	}
	img, err := s.db.GetCoverImage(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "cover not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load cover image")
		return
	}
	w.Header().Set("Content-Type", img.ContentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Length", strconv.Itoa(len(img.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(img.Data)
}

func (s *Server) deleteCover(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.GetSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid cover id")
		return
	}
	if err := s.db.DeleteCoverImage(r.Context(), id, session.Session.UserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "cover not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete cover image")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func coverAPIPath(id int64) string {
	return "/api/covers/" + strconv.FormatInt(id, 10)
}

// parseCoverImageID extracts an id from "/api/covers/{id}" (optionally absolute).
func parseCoverImageID(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	const marker = "/api/covers/"
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
