package campfire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	Message    string         `json:"message"`
	Path       []any          `json:"path,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
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
			if isRetryable(err) && attempt < c.retries-1 {
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

	op := operationName(query)
	resp, err := c.http.Do(req)
	if err != nil {
		logCampfireExchange(ctx, op, body, nil, 0, err)
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logCampfireExchange(ctx, op, body, nil, resp.StatusCode, err)
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable {
		err := upstreamStatusError(resp.StatusCode)
		logCampfireExchange(ctx, op, body, data, resp.StatusCode, err)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("graphql http %s: %s", resp.Status, truncate(data, 400))
		logCampfireExchange(ctx, op, body, data, resp.StatusCode, err)
		return nil, err
	}

	var parsed gqlResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		err = fmt.Errorf("decode graphql: %w", err)
		logCampfireExchange(ctx, op, body, data, resp.StatusCode, err)
		return nil, err
	}
	if len(parsed.Errors) > 0 {
		msgs := make([]string, 0, len(parsed.Errors))
		for _, e := range parsed.Errors {
			msgs = append(msgs, formatGqlErr(e))
		}
		joined := strings.Join(msgs, "; ")
		if parsed.Data == nil || bytes.Equal(parsed.Data, []byte("null")) {
			err := fmt.Errorf("graphql: %s", joined)
			logCampfireExchange(ctx, op, body, data, resp.StatusCode, err)
			return nil, err
		}
		slog.WarnContext(ctx, "graphql partial errors",
			slog.String("operation", op),
			slog.String("request", string(body)),
			slog.String("response", string(data)),
		)
	}
	if parsed.Data == nil || bytes.Equal(parsed.Data, []byte("null")) {
		err := fmt.Errorf("empty graphql data")
		logCampfireExchange(ctx, op, body, data, resp.StatusCode, err)
		return nil, err
	}
	return parsed.Data, nil
}

// campfireHTTPError is a failed Campfire HTTP call with the raw response kept for logging.
type campfireHTTPError struct {
	status int
	body   []byte
	err    error
}

func (e *campfireHTTPError) Error() string { return e.err.Error() }

func (e *campfireHTTPError) Unwrap() error { return e.err }

func logCampfireExchange(ctx context.Context, operation string, request, response []byte, status int, err error) {
	slog.ErrorContext(ctx, "campfire graphql request failed",
		slog.String("operation", operation),
		slog.Int("status", status),
		slog.String("request", string(request)),
		slog.String("response", string(response)),
		slog.Any("err", err),
	)
}

func formatGqlErr(e gqlErr) string {
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = "graphql error"
	}
	var extra []string
	for _, key := range []string{"code", "errorCode", "detail", "reason", "error"} {
		if v, ok := e.Extensions[key]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" && s != msg {
				extra = append(extra, key+"="+s)
			}
		}
	}
	if ex, ok := e.Extensions["exception"].(map[string]any); ok {
		if m, ok := ex["message"].(string); ok {
			m = strings.TrimSpace(m)
			if m != "" && m != msg {
				extra = append(extra, m)
			}
		}
	}
	if len(extra) == 0 && len(e.Extensions) > 0 {
		b, err := json.Marshal(e.Extensions)
		if err == nil && len(b) > 2 {
			extra = append(extra, truncate(b, 300))
		}
	}
	if len(extra) == 0 {
		return msg
	}
	return msg + " (" + strings.Join(extra, "; ") + ")"
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

// UnavailableError means Campfire returned 502 or 503.
type UnavailableError struct {
	Status int
}

func (e *UnavailableError) Error() string {
	return fmt.Sprintf("campfire status %d", e.Status)
}

// UnavailableMessage is the user-facing text for a Campfire 502 or 503.
const UnavailableMessage = "Campfire is having issues. Try again in a bit."

func IsUnavailable(err error) bool {
	var unavailable *UnavailableError
	return errors.As(err, &unavailable)
}

func upstreamStatusError(status int) error {
	if status == http.StatusBadGateway || status == http.StatusServiceUnavailable {
		return &UnavailableError{Status: status}
	}
	return fmt.Errorf("status %d", status)
}

func isRetryable(err error) bool {
	if IsUnavailable(err) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "DeadlineExceeded")
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
