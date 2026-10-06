package placeholders

import (
	"strings"
	"testing"
	"time"
)

func TestApplyCaseInsensitive(t *testing.T) {
	values := map[string]string{
		"liveEvent": "Community Day",
		"address":   "Hauptbahnhof",
	}
	got := Apply("Meet at {{Address}} for {{LIVEEVENT}}", values)
	want := "Meet at Hauptbahnhof for Community Day"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestApplyLeavesUnknown(t *testing.T) {
	got := Apply("Hello {{city}}", map[string]string{"club": "X"})
	if got != "Hello {{city}}" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveBuiltins(t *testing.T) {
	lat, lng := 50.11, 8.68
	start := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC) // 14:00 Europe/Berlin (CEST)
	end := start.Add(3 * time.Hour)
	ctx := Context{
		ClubName:      "PoGo OF",
		LiveEventName: "Community Day",
		Category:      "Community Day",
		Title:         "{{liveEvent}} — Downtown",
		Address:       "Main St",
		Latitude:      &lat,
		Longitude:     &lng,
		EventTime:     start,
		EventEndTime:  end,
		HasEndTime:    true,
		TimeZone:      "Europe/Berlin",
	}
	vals := ResolveBuiltins(ctx)
	if vals["club"] != "PoGo OF" {
		t.Fatalf("club=%q", vals["club"])
	}
	if vals["startTime"] != "14:00" {
		t.Fatalf("startTime=%q", vals["startTime"])
	}
	if vals["endTime"] != "17:00" {
		t.Fatalf("endTime=%q", vals["endTime"])
	}
	if vals["dateShort"] != "2026-10-02" {
		t.Fatalf("dateShort=%q", vals["dateShort"])
	}
	if vals["lat"] != "50.11" || vals["lng"] != "8.68" {
		t.Fatalf("coords lat=%q lng=%q", vals["lat"], vals["lng"])
	}
	if vals["title"] != "{{liveEvent}} — Downtown" {
		t.Fatalf("title=%q", vals["title"])
	}
}

func TestResolveAndApply(t *testing.T) {
	lat := 1.5
	ctx := Context{
		ClubName:  "Club",
		Title:     "Title {{club}}",
		Address:   "Addr",
		Latitude:  &lat,
		EventTime: time.Date(2026, 1, 15, 13, 0, 0, 0, time.UTC),
		TimeZone:  "UTC",
	}
	fields := ResolveAndApply(TextFields{
		Name:    "Title {{club}}",
		Details: "At {{address}} ({{lat}}) custom={{city}}",
		Address: "Addr",
	}, ctx, map[string]string{"city": "Berlin"})
	if fields.Name != "Title Club" {
		t.Fatalf("name=%q", fields.Name)
	}
	if fields.Details != "At Addr (1.5) custom=Berlin" {
		t.Fatalf("details=%q", fields.Details)
	}
}

func TestApplyIndexedEventPokemon(t *testing.T) {
	ctx := Context{
		LiveEventName:    "Raid Hour: Squirtle, Wartortle",
		Category:         "Raid Hour",
		Language:         "en",
		CategoryPatterns: map[string][]string{"Raid Hour": {"Raid Hour"}},
	}
	fields := ResolveAndApply(TextFields{
		Details: "{{eventPokemon[1]}} / {{eventPokemonCeilingRaid[1]}} / {{eventPokemon[9]}}",
	}, ctx, nil)
	if !strings.Contains(fields.Details, "Squirtle") {
		t.Fatalf("details=%q", fields.Details)
	}
	if !strings.Contains(fields.Details, " CP") {
		t.Fatalf("expected CP unit, got %q", fields.Details)
	}
	// OOB index → empty (trailing slash space)
	if strings.Contains(fields.Details, "{{") {
		t.Fatalf("expected OOB emptied, got %q", fields.Details)
	}
}
