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
	"path"
	"strings"
	"time"
)

type ImageRecord struct {
	PhotoID      string `json:"photoId"`
	TransientURL string `json:"transientUrl"`
	PermanentURL string `json:"permanentUrl"`
	ExpiresAt    string `json:"expiresAt"`
}

type uploadFileResp struct {
	UploadFile struct {
		ImageRecord *ImageRecord `json:"imageRecord"`
	} `json:"uploadFile"`
}

type processStagingResp struct {
	ProcessStagingImage struct {
		ImageRecord *ImageRecord `json:"imageRecord"`
	} `json:"processStagingImage"`
}

const maxUploadBytes = 8 << 20 // 8 MiB

// UploadImage uploads an image to Campfire and returns a usable cover URL (permanent preferred).
func (c *Client) UploadImage(ctx context.Context, token, filename, contentType string, data []byte) (*ImageRecord, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty file")
	}
	if len(data) > maxUploadBytes {
		return nil, fmt.Errorf("file too large (max %d bytes)", maxUploadBytes)
	}
	filename = path.Base(strings.TrimSpace(filename))
	if filename == "" || filename == "." || filename == "/" {
		filename = "cover.jpg"
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	rec, err := c.uploadFileMultipart(ctx, token, filename, contentType, data)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(rec.PermanentURL) != "" {
		return rec, nil
	}

	// Some responses only give a signed upload URL — PUT the bytes, then finalize.
	if rec.TransientURL != "" && looksLikeUploadURL(rec.TransientURL) {
		if err := c.putBytes(ctx, token, rec.TransientURL, contentType, data); err != nil {
			return nil, fmt.Errorf("upload to staging url: %w", err)
		}
	}

	if rec.PhotoID != "" {
		processed, err := c.processStagingImage(ctx, token, rec.PhotoID)
		if err != nil {
			if preferURL(rec) != "" {
				return rec, nil
			}
			return nil, err
		}
		return processed, nil
	}

	if preferURL(rec) == "" {
		return nil, fmt.Errorf("upload returned no image url")
	}
	return rec, nil
}

func preferURL(rec *ImageRecord) string {
	if rec == nil {
		return ""
	}
	if u := strings.TrimSpace(rec.PermanentURL); u != "" {
		return u
	}
	return strings.TrimSpace(rec.TransientURL)
}

// RemoveImage best-effort deletes an uploaded image by photoId via the Campfire images REST API.
// Cover removal in the UI only needs to clear the URL; remote delete failures are ignored.
func (c *Client) RemoveImage(ctx context.Context, token, photoID string) error {
	photoID = strings.TrimSpace(photoID)
	if photoID == "" {
		return fmt.Errorf("missing photoId")
	}
	base := strings.TrimSuffix(c.url, "/graphql")
	base = strings.TrimRight(base, "/")
	if base == "" {
		base = "https://niantic-social-api.nianticlabs.com"
	}
	endpoint := base + "/images/" + photoID

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "campfire-event-manager/1.0")
	token = strings.TrimSpace(token)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := c.http
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		// Network blip — cover URL can still be cleared client-side.
		return nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
	return nil
}

func looksLikeUploadURL(u string) bool {
	u = strings.ToLower(u)
	return strings.Contains(u, "upload") || strings.Contains(u, "put") || strings.Contains(u, "s3") || strings.Contains(u, "storage")
}

func (c *Client) uploadFileMultipart(ctx context.Context, token, filename, contentType string, data []byte) (*ImageRecord, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	ops := map[string]any{
		"query": mutationUploadFile,
		"variables": map[string]any{
			"input": map[string]any{
				"file": nil,
			},
		},
	}
	opsJSON, err := json.Marshal(ops)
	if err != nil {
		return nil, err
	}
	if err := w.WriteField("operations", string(opsJSON)); err != nil {
		return nil, err
	}
	if err := w.WriteField("map", `{"0":["variables.input.file"]}`); err != nil {
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

	raw, err := c.roundTripMultipart(ctx, token, "UploadFileMutation", w.FormDataContentType(), body.Bytes())
	if err != nil {
		return nil, err
	}
	var resp uploadFileResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode uploadFile: %w", err)
	}
	if resp.UploadFile.ImageRecord == nil {
		return nil, fmt.Errorf("uploadFile returned no imageRecord")
	}
	return resp.UploadFile.ImageRecord, nil
}

func (c *Client) processStagingImage(ctx context.Context, token, photoID string) (*ImageRecord, error) {
	vars := map[string]any{
		"input": map[string]any{
			"photoId": photoID,
		},
	}
	var resp processStagingResp
	if err := c.Do(ctx, token, mutationProcessStagingImage, vars, &resp); err != nil {
		return nil, err
	}
	if resp.ProcessStagingImage.ImageRecord == nil {
		return nil, fmt.Errorf("processStagingImage returned no imageRecord")
	}
	return resp.ProcessStagingImage.ImageRecord, nil
}

func (c *Client) putBytes(ctx context.Context, token, url, contentType string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(data))
	// Some CDNs want the session; others reject Authorization on signed URLs.
	token = strings.TrimSpace(token)
	if token != "" && !strings.Contains(strings.ToLower(url), "x-amz-") && !strings.Contains(url, "Signature=") {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := c.http
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PUT %s: %s", resp.Status, truncate(b, 200))
	}
	return nil
}

func (c *Client) doMultipart(ctx context.Context, token, operation, query string, variables map[string]any, filePath, filename, contentType string, data []byte) (json.RawMessage, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	ops := map[string]any{
		"query":         query,
		"operationName": operation,
		"variables":     variables,
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
	if operation == "" {
		operation = "UploadFileMutation"
	}
	req.Header.Set("X-APOLLO-OPERATION-NAME", operation)
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

// FetchImage downloads a cover image so it can be attached as editEvent avatarFile.
// Campfire does not keep a coverPhotoUrl unless the bytes are uploaded with the edit.
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
