package database

import (
	"fmt"
	"net/url"
)

type Config struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Username string `toml:"username"`
	Password string `toml:"password"`
	Database string `toml:"database"`
	SSLMode  string `toml:"ssl_mode"`
}

func (c Config) DataSourceName() string {
	ssl := c.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.Username, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   c.Database,
	}
	q := u.Query()
	q.Set("sslmode", ssl)
	q.Set("TimeZone", "UTC")
	u.RawQuery = q.Encode()
	return u.String()
}

func (c Config) String() string {
	return fmt.Sprintf("\n  Host: %s\n  Port: %d\n  Username: %s\n  Database: %s\n  SSLMode: %s",
		c.Host, c.Port, c.Username, c.Database, c.SSLMode)
}
