package server

import (
	"encoding/json"
	"testing"
)

func TestParseImportEntriesSingleWrapper(t *testing.T) {
	entries, err := parseImportEntries("ignored.json", []byte(`{
		"id": 42,
		"name": "Community Day",
		"language": "de",
		"payload": {"name": "Title", "details": "Hello"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("len=%d", len(entries))
	}
	if entries[0].ID != 42 || entries[0].Name != "Community Day" || entries[0].Language != "de" {
		t.Fatalf("%+v", entries[0])
	}
	var payload map[string]any
	if err := json.Unmarshal(entries[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["name"] != "Title" {
		t.Fatalf("payload=%v", payload)
	}
}

func TestParseImportEntriesBundleAndArray(t *testing.T) {
	bundle, err := parseImportEntries("bundle.json", []byte(`{
		"templates": [
			{"name": "A", "payload": {"details": "a"}},
			{"name": "B", "language": "en", "payload": {"details": "b"}}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle) != 2 || bundle[0].Name != "A" || bundle[1].Name != "B" {
		t.Fatalf("%+v", bundle)
	}

	arr, err := parseImportEntries("list.json", []byte(`[
		{"name": "X", "payload": {"details": "x"}},
		{"name": "Y", "payload": {"details": "y"}}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(arr) != 2 || arr[0].Name != "X" || arr[1].Name != "Y" {
		t.Fatalf("%+v", arr)
	}
}

func TestParseImportEntriesBarePayloadUsesFilename(t *testing.T) {
	entries, err := parseImportEntries("My Template.json", []byte(`{
		"name": "Meetup title",
		"details": "Body"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "My Template" {
		t.Fatalf("%+v", entries[0])
	}
}
