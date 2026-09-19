package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"golang.org/x/oauth2"

	"github.com/topi314/campfire-event-manager/server/auth"
	"github.com/topi314/campfire-event-manager/server/database"
)

func (s *Server) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		path := r.URL.Path

		if strings.HasPrefix(path, "/auth/login") || path == "/api/health" || path == "/api/config" {
			next.ServeHTTP(w, r)
			return
		}

		var session *database.SessionWithUser
		for _, cookie := range r.CookiesNamed("session") {
			sess, err := s.db.GetSession(ctx, cookie.Value)
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, database.ErrSessionExpired) {
					slog.ErrorContext(ctx, "failed to get session", slog.Any("error", err))
				}
				continue
			}
			session = sess
			break
		}

		if session != nil {
			r = r.WithContext(auth.SetSession(ctx, *session))
		}
		next.ServeHTTP(w, r)
	})
}

type handlerFunc func(http.ResponseWriter, *http.Request)

func (s *Server) requireAuth(next handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := auth.GetSession(r)
		if !ok || session.Session.ID == "" {
			if wantsJSON(r) {
				writeError(w, http.StatusUnauthorized, "not authenticated")
				return
			}
			rd := r.URL.Path
			if r.URL.RawQuery != "" {
				rd += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, "/auth/login?rd="+url.QueryEscape(rd), http.StatusFound)
			return
		}
		next(w, r)
	}
}

func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json") ||
		strings.HasPrefix(r.URL.Path, "/api/") ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	redirect := s.resolveAppRedirect(r.URL.Query().Get("rd"))
	state := s.auth.NewState(redirect)
	scopes := strings.Join(s.auth.Config().Scopes, " ")
	opts := []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("scope", scopes)}
	expiration := time.Now().Add(auth.MaxLoginFlowDuration)
	addOauthCookie(w, state, expiration)
	http.Redirect(w, r, s.auth.Config().AuthCodeURL(state, opts...), http.StatusTemporaryRedirect)
}

func (s *Server) loginCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	oauthState, _ := r.Cookie("oauthstate")
	state := query.Get("state")
	code := query.Get("code")

	if oauthState == nil || state != oauthState.Value {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	redirectURL, ok := s.auth.GetState(state)
	if !ok {
		http.Error(w, "Unknown OAuth state", http.StatusBadRequest)
		return
	}

	token, err := s.auth.Config().Exchange(ctx, code)
	if err != nil {
		slog.ErrorContext(ctx, "failed to exchange OAuth code", slog.Any("error", err))
		http.Error(w, "Failed to exchange OAuth code", http.StatusInternalServerError)
		return
	}

	user, err := s.getDiscordUser(ctx, token.AccessToken)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get Discord user", slog.Any("error", err))
		http.Error(w, "Failed to get user info from Discord", http.StatusInternalServerError)
		return
	}

	cfg := s.auth.DiscordConfig()
	if !slices.Contains(cfg.Whitelist, user.ID.String()) {
		var member bool
		for _, guildID := range cfg.GuildIDs {
			ok, err := s.isDiscordGuildMember(ctx, token.AccessToken, guildID)
			if err != nil {
				slog.ErrorContext(ctx, "failed to check guild membership", slog.String("guild_id", guildID), slog.Any("error", err))
				http.Error(w, "Failed to check Discord guild membership", http.StatusInternalServerError)
				return
			}
			if ok {
				member = true
				break
			}
		}
		if !member {
			http.Error(w, "You are not whitelisted or a member of the required Discord guilds (Community Ambassadors)", http.StatusForbidden)
			return
		}
	}

	admin := slices.Contains(cfg.Admins, user.ID.String())
	now := time.Now()
	expiration := now.AddDate(1, 0, 0)
	sessionID := auth.RandomStr(32)

	if err = s.db.UpsertDiscordUser(ctx, database.DiscordUser{
		ID:          user.ID.String(),
		Username:    user.Username,
		DisplayName: user.EffectiveName(),
		AvatarURL:   user.EffectiveAvatarURL(),
	}); err != nil {
		slog.ErrorContext(ctx, "failed to upsert discord user", slog.Any("error", err))
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	if err = s.db.CreateSession(ctx, database.Session{
		ID:        sessionID,
		CreatedAt: now,
		ExpiresAt: expiration,
		UserID:    user.ID.String(),
		Admin:     admin,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to create session", slog.Any("error", err))
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	addSessionCookie(w, sessionID, expiration)
	http.Redirect(w, r, s.resolveAppRedirect(redirectURL), http.StatusFound)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	for _, cookie := range r.CookiesNamed("session") {
		_ = s.db.DeleteSession(ctx, cookie.Value)
	}
	clearSessionCookie(w)
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	http.Redirect(w, r, s.cfg.Server.AppOrigin()+"/", http.StatusFound)
}

// resolveAppRedirect turns a relative rd path (or allowed absolute URL) into the
// SPA origin. Prevents open redirects while sending Nuxt-dev users to :3000.
func (s *Server) resolveAppRedirect(rd string) string {
	defaultPath := "/create"
	app := s.cfg.Server.AppOrigin()
	rd = strings.TrimSpace(rd)
	if rd == "" {
		return app + defaultPath
	}

	if strings.HasPrefix(rd, "http://") || strings.HasPrefix(rd, "https://") {
		if s.allowedAppRedirect(rd) {
			return rd
		}
		return app + defaultPath
	}

	if !strings.HasPrefix(rd, "/") {
		rd = "/" + rd
	}
	// Only allow same-origin path redirects (no protocol-relative //evil).
	if strings.HasPrefix(rd, "//") {
		return app + defaultPath
	}
	return app + rd
}

func (s *Server) allowedAppRedirect(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	origin := u.Scheme + "://" + u.Host
	if isAllowedOrigin(origin, s.cfg.Server.CORSOriginList()) {
		return true
	}
	pub := strings.TrimRight(strings.TrimSpace(s.cfg.Server.PublicURL), "/")
	app := s.cfg.Server.AppOrigin()
	return strings.EqualFold(origin, pub) || strings.EqualFold(origin, app)
}

func (s *Server) apiMe(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          session.DiscordUser.ID,
		"username":    session.DiscordUser.Username,
		"displayName": session.DiscordUser.DisplayName,
		"avatarUrl":   session.DiscordUser.AvatarURL,
		"admin":       session.Session.Admin,
		"timeZone":    session.DiscordUser.TimeZone,
	})
}

func (s *Server) apiUpdateMe(w http.ResponseWriter, r *http.Request) {
	session, _ := auth.GetSession(r)
	var body struct {
		TimeZone string `json:"timeZone"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	tz := strings.TrimSpace(body.TimeZone)
	if !validTimeZone(tz) {
		writeError(w, http.StatusBadRequest, "invalid timezone")
		return
	}
	if err := s.db.SetDiscordUserTimeZone(r.Context(), session.DiscordUser.ID, tz); err != nil {
		slog.ErrorContext(r.Context(), "failed to save timezone", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to save timezone")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"timeZone": tz})
}

func validTimeZone(tz string) bool {
	if tz == "" || len(tz) > 64 || strings.EqualFold(tz, "Local") {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

func (s *Server) getDiscordUser(ctx context.Context, accessToken string) (*discord.OAuth2User, error) {
	rq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	rq.Header.Set("Authorization", "Bearer "+accessToken)

	rs, err := s.httpClient.Do(rq)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer rs.Body.Close()
	if rs.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", rs.StatusCode)
	}
	var user discord.OAuth2User
	if err = json.NewDecoder(rs.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &user, nil
}

func (s *Server) isDiscordGuildMember(ctx context.Context, accessToken, guildID string) (bool, error) {
	rq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/users/@me/guilds/"+guildID+"/member", nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}
	rq.Header.Set("Authorization", "Bearer "+accessToken)

	rs, err := s.httpClient.Do(rq)
	if err != nil {
		return false, fmt.Errorf("failed to do request: %w", err)
	}
	defer rs.Body.Close()
	switch rs.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status code: %d", rs.StatusCode)
	}
}

func addOauthCookie(w http.ResponseWriter, state string, expiration time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oauthstate",
		Value:    state,
		Expires:  expiration,
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Path:     "/auth/login/callback",
	})
}

func removeOauthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oauthstate",
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Path:     "/auth/login/callback",
	})
}

func addSessionCookie(w http.ResponseWriter, session string, expiration time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    session,
		Expires:  expiration,
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Path:     "/",
	})
	removeOauthCookie(w)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
		Path:     "/",
	})
}
