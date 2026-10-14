package server

import (
	"encoding/json"
	"testing"
)

func TestMergeLinkedFullySynced(t *testing.T) {
	originPayload := json.RawMessage(`{"details":"from origin","latitude":1,"allInvited":true}`)
	lang := "en"
	m := mergeLinkedTemplate(
		"local", &lang, json.RawMessage(`{}`), json.RawMessage(`{}`),
		"Origin Name", &lang, originPayload, true,
	)
	if m.Name != "Origin Name" {
		t.Fatalf("name=%q", m.Name)
	}
	if !m.Synced {
		t.Fatal("expected synced")
	}
	if len(m.OverriddenFields) != 0 {
		t.Fatalf("fields=%v", m.OverriddenFields)
	}
	pm := parsePayloadMap(m.Payload)
	if pm["details"] != "from origin" {
		t.Fatalf("payload=%v", pm)
	}
	if pm["latitude"] != float64(1) {
		t.Fatalf("latitude missing: %v", pm)
	}
}

func TestMergeLinkedWithOverrides(t *testing.T) {
	originPayload := json.RawMessage(`{"details":"origin","latitude":1,"longitude":2}`)
	lang := "en"
	overrides := json.RawMessage(`{"details":"mine","latitude":9,"longitude":8,"address":"A","placeId":null,"locationJitterMeters":0}`)
	m := mergeLinkedTemplate(
		"local", &lang, json.RawMessage(`{}`), overrides,
		"Origin", &lang, originPayload, true,
	)
	if m.Synced {
		t.Fatal("expected not fully synced")
	}
	pm := parsePayloadMap(m.Payload)
	if pm["details"] != "mine" {
		t.Fatalf("details=%v", pm["details"])
	}
	if pm["latitude"] != float64(9) {
		t.Fatalf("lat=%v", pm["latitude"])
	}
	ids := m.OverriddenFields
	if !containsStr(ids, "details") || !containsStr(ids, "location") {
		t.Fatalf("overriddenFields=%v", ids)
	}
}

func TestBuildOverridesFromFields(t *testing.T) {
	payload := json.RawMessage(`{"name":"Meet","details":"D","latitude":1.5,"longitude":2.5,"address":"X"}`)
	raw, err := buildOverridesFromFields(
		[]string{"name", "title", "details", "location"},
		"Tpl", "de", payload,
	)
	if err != nil {
		t.Fatal(err)
	}
	m := parseOverrideMap(raw)
	if m["name"] != "Tpl" {
		t.Fatalf("name=%v", m["name"])
	}
	if m["language"] != nil {
		t.Fatalf("unexpected language")
	}
	if m["title"] != "Meet" {
		t.Fatalf("title=%v", m["title"])
	}
	if m["details"] != "D" {
		t.Fatalf("details=%v", m["details"])
	}
	if m["latitude"] != float64(1.5) {
		t.Fatalf("lat=%v", m["latitude"])
	}
}

func containsStr(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
