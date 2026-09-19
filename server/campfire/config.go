package campfire

import (
	"fmt"
	"strings"
)

type Config struct {
	URL              string `toml:"url"`
	MaxRetries       int    `toml:"max_retries"`
	RealityChannelID string `toml:"reality_channel_id"`
}

func (c Config) String() string {
	return fmt.Sprintf("\n  URL: %s\n  MaxRetries: %d\n  RealityChannelID: %s",
		c.URL, c.MaxRetries, c.RealityChannelID)
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
