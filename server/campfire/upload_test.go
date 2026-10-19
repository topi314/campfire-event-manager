package campfire

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoMultipartPNGBecomesJPEGAvatarPart(t *testing.T) {
	var body []byte
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	defer srv.Close()

	src := image.NewNRGBA(image.Rect(0, 0, 2000, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 1000; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 255, A: 255})
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, src); err != nil {
		t.Fatal(err)
	}

	c := New(Config{URL: srv.URL, MaxRetries: 1})
	if _, err := c.doMultipart(context.Background(), "token", "CreateActivityReminderMutation", "query { x }", map[string]any{}, "variables.input.avatarFile", pngBuf.Bytes()); err != nil {
		t.Fatal(err)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])

	ops, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if ops.FormName() != "operations" {
		t.Fatalf("first part %q", ops.FormName())
	}

	mapPart, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if mapPart.FormName() != "map" {
		t.Fatalf("map part %q", mapPart.FormName())
	}
	mapJSON, err := io.ReadAll(mapPart)
	if err != nil {
		t.Fatal(err)
	}
	var fileMap map[string][]string
	if err := json.Unmarshal(mapJSON, &fileMap); err != nil {
		t.Fatal(err)
	}
	gotPaths := fileMap["0"]
	if len(gotPaths) != 1 || gotPaths[0] != "variables.input.avatarFile" {
		t.Fatalf("map = %s", mapJSON)
	}

	filePart, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if filePart.FormName() != "0" {
		t.Fatalf("file part %q", filePart.FormName())
	}
	if filePart.FileName() != "cover.jpg" {
		t.Fatalf("filename %q", filePart.FileName())
	}
	if filePart.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("content type %q", filePart.Header.Get("Content-Type"))
	}
	jpegBytes, err := io.ReadAll(filePart)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(jpegBytes))
	if err != nil {
		t.Fatalf("uploaded part is not jpeg: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 2000 || b.Dy() != 100 {
		t.Fatalf("size = %dx%d", b.Dx(), b.Dy())
	}
	opaque := img.At(100, b.Dy()/2)
	r, g, bl, _ := opaque.RGBA()
	if r > 0x2000 || g > 0x2000 || bl < 0xc000 {
		t.Fatalf("opaque pixel = #%04x%04x%04x", r, g, bl)
	}
	transparent := img.At(1500, b.Dy()/2)
	tr, tg, tb, _ := transparent.RGBA()
	if tr < 0xc000 || tg < 0xc000 || tb < 0xc000 {
		t.Fatalf("flattened pixel = #%04x%04x%04x", tr, tg, tb)
	}
}

func TestDoMultipartKeepsSmallBaselineJPEG(t *testing.T) {
	var body []byte
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	defer srv.Close()

	src := image.NewRGBA(image.Rect(0, 0, 8, 8))
	src.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, src, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	original := jpegBuf.Bytes()

	c := New(Config{URL: srv.URL, MaxRetries: 1})
	if _, err := c.doMultipart(context.Background(), "", "EditEventMutation", "query { x }", map[string]any{}, "variables.input.avatarFile", original); err != nil {
		t.Fatal(err)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	if _, err := mr.NextPart(); err != nil {
		t.Fatal(err)
	}
	if _, err := mr.NextPart(); err != nil {
		t.Fatal(err)
	}
	filePart, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(filePart)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("jpeg was re-encoded (%d vs %d bytes)", len(got), len(original))
	}
}
