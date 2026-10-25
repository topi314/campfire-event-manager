package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"
)

func TestParsePackZip(t *testing.T) {
	doc := packExportDoc{
		Version: 1,
		Packs: []packExportPack{{
			Name: "Raids",
			Icons: []packExportIcon{{
				Name:     "Pikachu",
				Filename: "pika.png",
				Category: "Raid",
				Pokemon:  []string{"pikachu"},
				File:     "packs/0/icons/0.png",
			}},
		}},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := writeZipFile(zw, packExportManifest, raw); err != nil {
		t.Fatal(err)
	}
	png := []byte{0x89, 'P', 'N', 'G'}
	if err := writeZipFile(zw, "packs/0/icons/0.png", png); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	packs, blobs, err := parsePackZip(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(packs) != 1 || packs[0].Name != "Raids" || len(packs[0].Icons) != 1 {
		t.Fatalf("packs = %#v", packs)
	}
	if !bytes.Equal(blobs["packs/0/icons/0.png"], png) {
		t.Fatalf("missing icon bytes")
	}
}

func TestParsePackZipRejectsPathEscape(t *testing.T) {
	doc := packExportDoc{
		Version: 1,
		Packs: []packExportPack{{
			Name: "Bad",
			Icons: []packExportIcon{{
				File: "../secret.png",
			}},
		}},
	}
	raw, _ := json.Marshal(doc)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	_ = writeZipFile(zw, packExportManifest, raw)
	_ = writeZipFile(zw, "secret.png", []byte("nope"))
	_ = zw.Close()

	if _, _, err := parsePackZip(buf.Bytes()); err == nil {
		t.Fatal("expected invalid path")
	}
}

func TestSafePackFilename(t *testing.T) {
	if got := safePackFilename("Raid Icons!"); got != "Raid_Icons" {
		t.Fatalf("got %q", got)
	}
	if got := safePackFilename("   "); got != "pack" {
		t.Fatalf("got %q", got)
	}
}
