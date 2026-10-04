package auth

import (
	"fmt"
	"strings"
)

type Config struct {
	ClientID     string   `toml:"client_id"`
	ClientSecret string   `toml:"client_secret"`
	GuildIDs     []string `toml:"guild_ids"`
	Whitelist    []string `toml:"whitelist"`
	Admins       []string `toml:"admins"`
	// BugHexe Discord user IDs get the Bug Hexe easter egg in the SPA.
	BugHexe []string `toml:"bug_hexe"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n  ClientID: %s\n  ClientSecret: %s\n  GuildIDs: %s\n  Whitelist: %s\n  Admins: %s\n  BugHexe: %s",
		c.ClientID,
		strings.Repeat("*", len(c.ClientSecret)),
		strings.Join(c.GuildIDs, ", "),
		strings.Join(c.Whitelist, ", "),
		strings.Join(c.Admins, ", "),
		strings.Join(c.BugHexe, ", "),
	)
}
