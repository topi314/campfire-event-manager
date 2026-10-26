package campfire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"golang.org/x/image/webp"
)

func init() {
	image.RegisterFormat("webp", "RIFF????WEBP", webp.Decode, webp.DecodeConfig)
}

const maxUploadBytes = 8 << 20 // 8 MiB

// WhatsApp skips share cards that are large PNGs. Campfire emits a PNG card
// when the meetup photo is a PNG, and a JPEG card when the photo is a JPEG.
const avatarJPEGQuality = 85

// doMultipart sends a GraphQL multipart request with one file mapped to filePath
// (e.g. "variables.input.avatarFile") per the GraphQL multipart request spec.
// Non-JPEG files are re-encoded to JPEG so the share card stays a JPEG.
func (c *Client) doMultipart(ctx context.Context, token, operation, query string, variables any, filePath string, data []byte) (json.RawMessage, error) {
	prepared, err := prepareAvatarJPEG(data)
	if err != nil {
		return nil, err
	}
	data = prepared

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
	h.Set("Content-Disposition", `form-data; name="0"; filename="cover.jpg"`)
	h.Set("Content-Type", "image/jpeg")
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
	raw, err := c.roundTripMultipart(ctx, token, operation, w.FormDataContentType(), body.Bytes())
	if err != nil {
		status, resp := 0, []byte(nil)
		var httpErr *campfireHTTPError
		if errors.As(err, &httpErr) {
			status = httpErr.status
			resp = httpErr.body
		}
		logCampfireExchange(ctx, operation, opsJSON, resp, status, err)
		return nil, err
	}
	return raw, nil
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
		return nil, &campfireHTTPError{err: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &campfireHTTPError{status: resp.StatusCode, err: err}
	}
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("graphql http %s: %s", resp.Status, truncate(data, 400))
		if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable {
			err = upstreamStatusError(resp.StatusCode)
		}
		return nil, &campfireHTTPError{
			status: resp.StatusCode,
			body:   data,
			err:    err,
		}
	}

	var parsed gqlResp
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, &campfireHTTPError{
			status: resp.StatusCode,
			body:   data,
			err:    fmt.Errorf("decode graphql: %w", err),
		}
	}
	if len(parsed.Errors) > 0 {
		msgs := make([]string, 0, len(parsed.Errors))
		for _, e := range parsed.Errors {
			msgs = append(msgs, formatGqlErr(e))
		}
		joined := strings.Join(msgs, "; ")
		if parsed.Data == nil || bytes.Equal(parsed.Data, []byte("null")) {
			return nil, &campfireHTTPError{
				status: resp.StatusCode,
				body:   data,
				err:    fmt.Errorf("graphql: %s", joined),
			}
		}
		slog.WarnContext(ctx, "graphql partial errors",
			slog.String("operation", operation),
			slog.String("response", string(data)),
		)
	}
	if parsed.Data == nil || bytes.Equal(parsed.Data, []byte("null")) {
		return nil, &campfireHTTPError{
			status: resp.StatusCode,
			body:   data,
			err:    fmt.Errorf("empty graphql data"),
		}
	}
	return parsed.Data, nil
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
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("cover url is not an image")
	}
	return &AvatarUpload{Data: data}, nil
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

// prepareAvatarJPEG leaves JPEG bytes unchanged. Other images are flattened
// onto white and encoded as JPEG.
func prepareAvatarJPEG(data []byte) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode cover image: %w", err)
	}
	if format == "jpeg" {
		return data, nil
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, flattenOpaque(img), &jpeg.Options{Quality: avatarJPEGQuality}); err != nil {
		return nil, fmt.Errorf("encode cover image: %w", err)
	}
	return buf.Bytes(), nil
}

func flattenOpaque(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}
