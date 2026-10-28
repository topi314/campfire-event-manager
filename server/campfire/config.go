package campfire

import (
	"encoding/base64"
	"fmt"
	"strings"
)

type Config struct {
	URL              string `toml:"url"`
	MaxRetries       int    `toml:"max_retries"`
	RealityChannelID string `toml:"reality_channel_id"`
	// TokenSyncKey is a required base64-encoded 32-byte AES key for account token sync.
	TokenSyncKey string `toml:"token_sync_key"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n  URL: %s\n  MaxRetries: %d\n  RealityChannelID: %s\n  TokenSyncKey: (set)",
		c.URL, c.MaxRetries, c.RealityChannelID)
}

// TokenSyncAESKey decodes the required 32-byte key.
func (c Config) TokenSyncAESKey() ([]byte, error) {
	raw := strings.TrimSpace(c.TokenSyncKey)
	if raw == "" {
		return nil, fmt.Errorf("campfire.token_sync_key is required")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("campfire.token_sync_key must be 32 bytes encoded as standard base64")
	}
	return key, nil
}

const DefaultURL = "https://niantic-social-api.nianticlabs.com/graphql"
const DefaultRealityChannelID = "da83476a-c4da-4312-a610-a4f2fc2c37f0"

func (c *Config) Normalize() {
	if strings.TrimSpace(c.URL) == "" {
		c.URL = DefaultURL
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if strings.TrimSpace(c.RealityChannelID) == "" {
		c.RealityChannelID = DefaultRealityChannelID
	}
}
