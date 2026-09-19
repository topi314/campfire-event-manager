package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/topi314/campfire-event-manager/server/auth"
)

type draftBody struct {
	Payload json.RawMessage `json:"payload"`
}

func draftClubIDFromPayload(payload json.RawMessage) string {
	var meta struct {
		ClubID string `json:"clubId"`
	}
	if err := json.Unmarshal(payload, &meta); err != nil {
		return ""
	}
	return strings.TrimSpace(meta.ClubID)
}

func (s *Server) adminClubIDs(r *http.Request) ([]string, error) {
	token, ok := s.requireCampfireToken(r)
	if !ok {
		return nil, errMissingCampfireToken
	}
	clubs, err := s.campfire.Clubs(r.Context(), token)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	seen := make(map[string]struct{})
	for _, c := range clubs {
		if !c.AmIAdmin {
			continue
		}
		id := strings.TrimSpace(c.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Server) requireAdminOfClub(r *http.Request, clubID string) (string, int, string) {
	clubID = strings.TrimSpace(clubID)
	if clubID == "" {
		return "", http.StatusBadRequest, "clubId is required"
	}
	ids, err := s.adminClubIDs(r)
	if err != nil {
		if errors.Is(err, errMissingCampfireToken) {
			return "", http.StatusBadRequest, "missing Campfire Authorization Bearer token"
		}
		return "", http.StatusBadGateway, err.Error()
	}
	for _, id := range ids {
		if id == clubID {
			return clubID, 0, ""
		}
	}
	return "", http.StatusForbidden, "you must be a club admin to manage drafts for this club"
}

var errMissingCampfireToken = errors.New("missing campfire token")

func (s *Server) listDrafts(w http.ResponseWriter, r *http.Request) {
	ids, err := s.adminClubIDs(r)
	if err != nil {
		if errors.Is(err, errMissingCampfireToken) {
			writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	items, err := s.db.ListDraftsByClubIDs(r.Context(), ids)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list drafts")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createDraft(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	var body draftBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(body.Payload) == 0 || !json.Valid(body.Payload) {
		writeError(w, http.StatusBadRequest, "payload is required")
		return
	}
	clubID := draftClubIDFromPayload(body.Payload)
	if _, status, msg := s.requireAdminOfClub(r, clubID); status != 0 {
		writeError(w, status, msg)
		return
	}
	item, err := s.db.CreateDraft(r.Context(), clubID, session.Session.UserID, body.Payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create draft")
		return
	}
	if full, err := s.db.GetDraft(r.Context(), item.ID); err == nil {
		item = full
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) updateDraft(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := s.db.GetDraft(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "draft not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load draft")
		return
	}
	if _, status, msg := s.requireAdminOfClub(r, existing.ClubID); status != 0 {
		writeError(w, status, msg)
		return
	}

	var body draftBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(body.Payload) == 0 || !json.Valid(body.Payload) {
		writeError(w, http.StatusBadRequest, "payload is required")
		return
	}
	clubID := draftClubIDFromPayload(body.Payload)
	if clubID == "" {
		clubID = existing.ClubID
	}
	if clubID != existing.ClubID {
		if _, status, msg := s.requireAdminOfClub(r, clubID); status != 0 {
			writeError(w, status, msg)
			return
		}
	}

	item, err := s.db.UpdateDraft(r.Context(), id, clubID, body.Payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			writeError(w, http.StatusNotFound, "draft not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update draft")
		return
	}
	if full, err := s.db.GetDraft(r.Context(), item.ID); err == nil {
		item = full
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) deleteDraft(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	existing, err := s.db.GetDraft(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "draft not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load draft")
		return
	}
	if _, status, msg := s.requireAdminOfClub(r, existing.ClubID); status != 0 {
		writeError(w, status, msg)
		return
	}
	if err := s.db.DeleteDraft(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "draft not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete draft")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) clearDrafts(w http.ResponseWriter, r *http.Request) {
	ids, err := s.adminClubIDs(r)
	if err != nil {
		if errors.Is(err, errMissingCampfireToken) {
			writeError(w, http.StatusBadRequest, "missing Campfire Authorization Bearer token")
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if err := s.db.ClearDraftsByClubIDs(r.Context(), ids); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear drafts")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
