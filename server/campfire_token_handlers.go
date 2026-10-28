package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/topi314/campfire-event-manager/server/auth"
	"github.com/topi314/campfire-event-manager/server/database/dbsqlc"
)

type campfireTokenBody struct {
	Sync  bool   `json:"sync"`
	Token string `json:"token"`
}

func normalizeCampfireToken(raw string) string {
	token := strings.TrimSpace(raw)
	if len(token) >= 7 && strings.EqualFold(token[:7], "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return strings.TrimSpace(strings.Trim(token, `"'`))
}

func (s *Server) campfireTokenKey(w http.ResponseWriter) ([]byte, bool) {
	key, err := s.cfg.Campfire.TokenSyncAESKey()
	if err != nil || len(key) != 32 {
		writeError(w, http.StatusInternalServerError, "token sync key is invalid")
		return nil, false
	}
	return key, true
}

func (s *Server) campfireTokenFromSession(ctx context.Context, session auth.Session) (sync bool, token string) {
	if !session.CampfireTokenSync || len(session.CampfireToken) == 0 {
		return session.CampfireTokenSync, ""
	}
	key, err := s.cfg.Campfire.TokenSyncAESKey()
	if err != nil {
		slog.ErrorContext(ctx, "token sync key is invalid", slog.Any("error", err))
		return session.CampfireTokenSync, ""
	}
	plain, err := openToken(key, session.CampfireToken)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decrypt campfire token", slog.Any("error", err))
		return true, ""
	}
	return true, plain
}

func (s *Server) apiPutCampfireToken(w http.ResponseWriter, r *http.Request) {
	key, ok := s.campfireTokenKey(w)
	if !ok {
		return
	}
	var body campfireTokenBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	session, _ := auth.GetSession(r)
	token := ""
	var sealed []byte
	if body.Sync {
		token = normalizeCampfireToken(body.Token)
		if token != "" {
			var err error
			sealed, err = sealToken(key, token)
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to encrypt campfire token", slog.Any("error", err))
				writeError(w, http.StatusInternalServerError, "failed to store token")
				return
			}
		}
	}
	n, err := s.db.SetCampfireTokenSync(r.Context(), dbsqlc.SetCampfireTokenSyncParams{
		DiscordUserID:                session.DiscordUser.ID,
		DiscordUserCampfireToken:     sealed,
		DiscordUserCampfireTokenSync: body.Sync,
	})
	if err != nil || n == 0 {
		slog.ErrorContext(r.Context(), "failed to save campfire token sync", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to save token sync")
		return
	}
	writeJSON(w, http.StatusOK, campfireTokenBody{Sync: body.Sync, Token: token})
}
