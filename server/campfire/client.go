package campfire

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ensure net/http Request helpers compile

type Client struct {
	url     string
	http    *http.Client
	retries int
}

func New(cfg Config) *Client {
	cfg.Normalize()
	return &Client{
		url: cfg.URL,
		http: &http.Client{
			Timeout: 90 * time.Second,
		},
		retries: cfg.MaxRetries,
	}
}

type gqlReq struct {
	Query     string `json:"query"`
	Variables any    `json:"variables,omitempty"`
}

type gqlErr struct {
	Message string `json:"message"`
}

type gqlResp struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlErr        `json:"errors"`
}

func (c *Client) Do(ctx context.Context, token, query string, vars any, dest any) error {
	var last error
	for attempt := 0; attempt < c.retries; attempt++ {
		raw, err := c.roundTrip(ctx, token, query, vars)
		if err != nil {
			last = err
			if isRetryable(err) {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return err
		}
		if dest != nil {
			if err := json.Unmarshal(raw, dest); err != nil {
				return fmt.Errorf("decode graphql data: %w", err)
			}
		}
		return nil
	}
	if last == nil {
		last = fmt.Errorf("too many retries")
	}
	return last
}

func (c *Client) roundTrip(ctx context.Context, token, query string, vars any) (json.RawMessage, error) {
	body, err := json.Marshal(gqlReq{Query: query, Variables: vars})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "campfire-event-manager/1.0")
	if name := operationName(query); name != "" {
		req.Header.Set("X-APOLLO-OPERATION-NAME", name)
	}
	token = strings.TrimSpace(token)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusBadGateway {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graphql http %s: %s", resp.Status, truncate(data, 400))
	}

	var parsed gqlResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("decode graphql: %w", err)
	}
	if len(parsed.Errors) > 0 {
		msgs := make([]string, 0, len(parsed.Errors))
		for _, e := range parsed.Errors {
			msgs = append(msgs, e.Message)
		}
		joined := strings.Join(msgs, "; ")
		if parsed.Data == nil || bytes.Equal(parsed.Data, []byte("null")) {
			return nil, fmt.Errorf("graphql: %s", joined)
		}
		slog.WarnContext(ctx, "graphql partial errors", slog.String("errors", joined))
	}
	if parsed.Data == nil || bytes.Equal(parsed.Data, []byte("null")) {
		return nil, fmt.Errorf("empty graphql data")
	}
	return parsed.Data, nil
}

func operationName(query string) string {
	for _, line := range strings.Split(query, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "query ") || strings.HasPrefix(line, "mutation ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				name := strings.TrimRight(fields[1], "({")
				if name != "" {
					return name
				}
			}
		}
	}
	return ""
}

func isRetryable(err error) bool {
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "502") || strings.Contains(s, "DeadlineExceeded")
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

// IsAuthError reports whether a Campfire GraphQL/HTTP failure is an expired or
// rejected session token rather than a transient upstream problem.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "401"),
		strings.Contains(s, "unauthorized"),
		strings.Contains(s, "unauthenticated"),
		strings.Contains(s, "not authenticated"),
		strings.Contains(s, "invalid token"),
		strings.Contains(s, "jwt"),
		strings.Contains(s, "expired"):
		return true
	default:
		return false
	}
}

type tokenCtxKey struct{}

func WithToken(ctx context.Context, token string) context.Context {
	token = strings.TrimSpace(token)
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, tokenCtxKey{}, token)
}

func TokenFrom(ctx context.Context) string {
	t, _ := ctx.Value(tokenCtxKey{}).(string)
	return t
}

func BearerFromRequest(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if strings.HasPrefix(strings.ToLower(h), strings.ToLower(prefix)) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return strings.TrimSpace(h)
}
