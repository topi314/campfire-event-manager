package campfire

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"
)

const maxUploadBytes = 8 << 20 // 8 MiB

// doMultipart sends a GraphQL multipart request with one file mapped to filePath
// (e.g. "variables.input.avatarFile") per the GraphQL multipart request spec.
func (c *Client) doMultipart(ctx context.Context, token, operation, query string, variables any, filePath, filename, contentType string, data []byte) (json.RawMessage, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	ops := struct {
		Query         string `json:"query"`
		OperationName string `json:"operationName"`
		Variables     any    `json:"variables"`
	}{
		Query:         query,
		OperationName: operation,
		Variables:     variables,
	}
	opsJSON, err := json.Marshal(ops)
	if err != nil {
		return nil, err
	}
	if err := w.WriteField("operations", string(opsJSON)); err != nil {
		return nil, err
	}
	if err := w.WriteField("map", `{"0":["`+filePath+`"]}`); err != nil {
		return nil, err
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="0"; filename="%s"`, escapeQuotes(filename)))
	h.Set("Content-Type", contentType)
	part, err := w.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return c.roundTripMultipart(ctx, token, operation, w.FormDataContentType(), body.Bytes())
}

func (c *Client) roundTripMultipart(ctx context.Context, token, operation, contentType string, body []byte) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "campfire-event-manager/1.0")
	if operation != "" {
		req.Header.Set("X-APOLLO-OPERATION-NAME", operation)
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
	}
	if parsed.Data == nil || bytes.Equal(parsed.Data, []byte("null")) {
		return nil, fmt.Errorf("empty graphql data")
	}
	return parsed.Data, nil
}

func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

// FetchImage downloads an external https cover so it can be attached as avatarFile
// (e.g. when editing a meetup that already has a Campfire CDN cover URL).
func (c *Client) FetchImage(ctx context.Context, rawURL string) (*AvatarUpload, error) {
	u, err := safeImageURL(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/*,*/*")
	req.Header.Set("User-Agent", "campfire-event-manager/1.0")

	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		if _, err := safeImageURL(req.URL.String()); err != nil {
			return err
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("load cover image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("load cover image: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxUploadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("load cover image: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("cover image is empty")
	}
	if len(data) > maxUploadBytes {
		return nil, fmt.Errorf("cover image is too large")
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(contentType, "image/") {
		contentType = sniffImageType(data)
	}
	if !strings.HasPrefix(contentType, "image/") {
		return nil, fmt.Errorf("cover url is not an image")
	}
	ext := "jpg"
	switch contentType {
	case "image/png":
		ext = "png"
	case "image/webp":
		ext = "webp"
	case "image/gif":
		ext = "gif"
	}
	return &AvatarUpload{
		Filename:    "cover." + ext,
		ContentType: contentType,
		Data:        data,
	}, nil
}

func safeImageURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("invalid cover url")
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("cover url must be https")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return nil, fmt.Errorf("cover url not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && !ip.IsGlobalUnicast() {
		return nil, fmt.Errorf("cover url not allowed")
	}
	return u, nil
}

func sniffImageType(data []byte) string {
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "image/png"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "image/gif"
	}
	return ""
}
