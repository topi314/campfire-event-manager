package server

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/topi314/campfire-event-manager/server/auth"
	"github.com/topi314/campfire-event-manager/server/campfire"
	"github.com/topi314/campfire-event-manager/server/database"
)

type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

type LogConfig struct {
	Level     slog.Level `toml:"level"`
	Format    LogFormat  `toml:"format"`
	AddSource bool       `toml:"add_source"`
}

type ServerConfig struct {
	Addr        string `toml:"addr"`
	PublicURL   string `toml:"public_url"`
	// AppURL is the browser origin for the SPA (e.g. http://localhost:3000 in Nuxt
	// dev). Post-login redirects go here. Empty means same origin as PublicURL
	// (embedded production build).
	AppURL      string `toml:"app_url"`
	CORSOrigins string `toml:"cors_origins"`
}

type Config struct {
	Log         LogConfig       `toml:"log"`
	Server      ServerConfig    `toml:"server"`
	Database    database.Config `toml:"database"`
	DiscordAuth auth.Config     `toml:"discord_auth"`
	Campfire    campfire.Config `toml:"campfire"`
	Basemaps    BasemapsConfig  `toml:"basemaps"`
}

type BasemapsConfig struct {
	CartoAPIKey string `toml:"carto_api_key"`
}

func (c BasemapsConfig) CartoKey() string {
	return strings.TrimSpace(c.CartoAPIKey)
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	meta, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		slog.Warn("unknown config keys", slog.Any("keys", undecoded))
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.Server.PublicURL == "" {
		cfg.Server.PublicURL = "http://localhost:8080"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = LogFormatText
	}
	return cfg, nil
}

func (c Config) String() string {
	return fmt.Sprintf("\n Log: %+v\n Server: %+v\n Database: %s\n DiscordAuth: %s\n Campfire: %s\n Basemaps: carto_api_key=%s",
		c.Log,
		c.Server,
		c.Database.String(),
		c.DiscordAuth.String(),
		c.Campfire.String(),
		maskSecret(c.Basemaps.CartoAPIKey),
	)
}

func maskSecret(s string) string {
	if s == "" {
		return "(empty)"
	}
	return strings.Repeat("*", len(s))
}

func (c ServerConfig) CORSOriginList() []string {
	parts := strings.Split(c.CORSOrigins, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// AppOrigin is where the browser should load the SPA after auth.
func (c ServerConfig) AppOrigin() string {
	if u := strings.TrimRight(strings.TrimSpace(c.AppURL), "/"); u != "" {
		return u
	}
	return strings.TrimRight(strings.TrimSpace(c.PublicURL), "/")
}
