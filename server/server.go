package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/topi314/campfire-event-manager/frontend"
	"github.com/topi314/campfire-event-manager/server/auth"
	"github.com/topi314/campfire-event-manager/server/campfire"
	"github.com/topi314/campfire-event-manager/server/database"
)

type Server struct {
	cfg        Config
	db         *database.Database
	auth       *auth.Auth
	campfire   *campfire.Client
	httpClient *http.Client
	http       *http.Server
}

func New(cfg Config) (*Server, error) {
	db, err := database.New(cfg.Database)
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:      cfg,
		db:       db,
		auth:     auth.New(cfg.DiscordAuth, cfg.Server.PublicURL),
		campfire: campfire.New(cfg.Campfire),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}

	handler, err := s.Handler()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	s.http = &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s, nil
}

func (s *Server) Start() {
	go func() {
		slog.Info("listening", slog.String("addr", s.cfg.Server.Addr))
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", slog.Any("err", err))
		}
	}()
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.http.Shutdown(ctx); err != nil {
		slog.Error("error while shutting down server", slog.Any("err", err))
	}
	if err := s.db.Close(); err != nil {
		slog.Error("error while closing database", slog.Any("err", err))
	}
}

func (s *Server) Handler() (http.Handler, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/config", s.clientConfig)
	mux.HandleFunc("GET /api/geocode", s.requireAuth(s.geocodeSearch))
	mux.HandleFunc("GET /auth/login", s.login)
	mux.HandleFunc("GET /auth/login/callback", s.loginCallback)
	mux.HandleFunc("POST /auth/logout", s.logout)
	mux.HandleFunc("POST /logout", s.logout) // alias
	mux.HandleFunc("GET /api/me", s.requireAuth(s.apiMe))
	mux.HandleFunc("PATCH /api/me", s.requireAuth(s.apiUpdateMe))

	mux.HandleFunc("GET /api/templates", s.requireAuth(s.listTemplates))
	mux.HandleFunc("POST /api/templates", s.requireAuth(s.createTemplate))
	mux.HandleFunc("POST /api/templates/import", s.requireAuth(s.importTemplates))
	mux.HandleFunc("GET /api/templates/shared", s.requireAuth(s.sharedTemplates))
	mux.HandleFunc("GET /api/templates/{id}", s.requireAuth(s.getTemplate))
	mux.HandleFunc("PUT /api/templates/{id}", s.requireAuth(s.updateTemplate))
	mux.HandleFunc("DELETE /api/templates/{id}", s.requireAuth(s.deleteTemplate))
	mux.HandleFunc("POST /api/templates/{id}/publish", s.requireAuth(s.publishTemplate))
	mux.HandleFunc("POST /api/templates/{id}/unpublish", s.requireAuth(s.unpublishTemplate))
	mux.HandleFunc("POST /api/templates/{id}/clone", s.requireAuth(s.cloneTemplate))
	mux.HandleFunc("POST /api/templates/{id}/like", s.requireAuth(s.likeTemplate))
	mux.HandleFunc("DELETE /api/templates/{id}/like", s.requireAuth(s.unlikeTemplate))
	mux.HandleFunc("GET /api/profiles/{userId}", s.requireAuth(s.getProfile))

	mux.HandleFunc("GET /api/drafts", s.requireAuth(s.listDrafts))
	mux.HandleFunc("POST /api/drafts", s.requireAuth(s.createDraft))
	mux.HandleFunc("DELETE /api/drafts", s.requireAuth(s.clearDrafts))
	mux.HandleFunc("PUT /api/drafts/{id}", s.requireAuth(s.updateDraft))
	mux.HandleFunc("DELETE /api/drafts/{id}", s.requireAuth(s.deleteDraft))

	mux.HandleFunc("GET /api/campfire/me", s.requireAuth(s.campfireMe))
	mux.HandleFunc("GET /api/campfire/clubs", s.requireAuth(s.campfireClubs))
	mux.HandleFunc("GET /api/campfire/live-events", s.requireAuth(s.campfireLiveEvents))
	mux.HandleFunc("POST /api/campfire/meetup-schedule", s.requireAuth(s.campfireMeetupSchedule))
	mux.HandleFunc("GET /api/campfire/clubs/{clubId}/members", s.requireAuth(s.campfireClubMembers))
	mux.HandleFunc("GET /api/campfire/clubs/{clubId}/events", s.requireAuth(s.campfireClubEvents))
	mux.HandleFunc("POST /api/campfire/meetups", s.requireAuth(s.campfireCreateMeetup))
	mux.HandleFunc("PUT /api/campfire/meetups/{eventId}", s.requireAuth(s.campfireEditMeetup))
	mux.HandleFunc("DELETE /api/campfire/meetups/{eventId}", s.requireAuth(s.campfireDeleteMeetup))

	mux.HandleFunc("POST /api/covers", s.requireAuth(s.uploadCover))
	mux.HandleFunc("GET /api/covers/{id}", s.requireAuth(s.getCover))
	mux.HandleFunc("DELETE /api/covers/{id}", s.requireAuth(s.deleteCover))

	spa, err := frontend.Handler()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", spa)

	return s.cors(s.sessionMiddleware(mux)), nil
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) clientConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"cartoApiKey": s.cfg.Basemaps.CartoKey()})
}
