package meetuptime

import (
	"testing"
	"time"
)

func TestResolveRaidHourIgnoresUTCWindow(t *testing.T) {
	sched, err := Resolve(Input{
		TimeZone: "Europe/Berlin",
		Live: &LiveEventFields{
			EventName:      "Raid Hour: Stuff",
			StartTimestamp: "2026-10-02T04:00:00.000Z",
			EndTimestamp:   "2026-10-02T05:00:00.000Z",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sched.Category != "Raid Hour" {
		t.Fatalf("category=%q", sched.Category)
	}
	if sched.EventTime != "2026-10-02T18:00" {
		t.Fatalf("start=%q want 2026-10-02T18:00", sched.EventTime)
	}
	if sched.EventEndTime != "2026-10-02T19:00" {
		t.Fatalf("end=%q want 2026-10-02T19:00", sched.EventEndTime)
	}
}

func TestResolveTemplateClocksOnLiveDay(t *testing.T) {
	sched, err := Resolve(Input{
		TimeZone: "Europe/Berlin",
		Live: &LiveEventFields{
			EventName:      "Community Day",
			StartTimestamp: "2026-10-11T07:00:00.000Z",
		},
		StartTime: "14:00",
		EndTime:   "17:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sched.EventTime != "2026-10-11T14:00" {
		t.Fatalf("start=%q", sched.EventTime)
	}
	if sched.EventEndTime != "2026-10-11T17:00" {
		t.Fatalf("end=%q", sched.EventEndTime)
	}
}

func TestResolveIgnoresLiveLocalTimesWhenTemplateEmpty(t *testing.T) {
	// Live clocks are skipped; Community Day falls back to category defaults (14:00–17:00).
	sched, err := Resolve(Input{
		TimeZone: "Europe/Berlin",
		Live: &LiveEventFields{
			EventName:      "Community Day: Squirtle",
			StartTimestamp: "2026-10-11T07:00:00.000Z",
			EndTimestamp:   "2026-10-11T10:00:00.000Z",
			LocalStartTime: "2026-10-11T11:00:00.000",
			LocalEndTime:   "2026-10-11T14:00:00.000",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sched.EventTime != "2026-10-11T14:00" {
		t.Fatalf("start=%q want 2026-10-11T14:00", sched.EventTime)
	}
	if sched.EventEndTime != "2026-10-11T17:00" {
		t.Fatalf("end=%q want 2026-10-11T17:00", sched.EventEndTime)
	}
}

func TestResolveIgnoresLiveTimestampsWhenNoTemplate(t *testing.T) {
	sched, err := Resolve(Input{
		TimeZone: "Europe/Berlin",
		Live: &LiveEventFields{
			EventName:      "Raid Day: Falinks",
			StartTimestamp: "2026-10-11T07:00:00.000Z",
			EndTimestamp:   "2026-10-11T10:00:00.000Z",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sched.EventTime != "2026-10-11T14:00" {
		t.Fatalf("start=%q want 2026-10-11T14:00 (category default)", sched.EventTime)
	}
	if sched.EventEndTime != "2026-10-11T17:00" {
		t.Fatalf("end=%q want 2026-10-11T17:00 (category default)", sched.EventEndTime)
	}
}

func TestResolveRaidHourKeepsDefaultsDespiteLocalTimes(t *testing.T) {
	sched, err := Resolve(Input{
		TimeZone: "Europe/Berlin",
		Live: &LiveEventFields{
			EventName:      "Raid Hour: Stuff",
			StartTimestamp: "2026-10-02T04:00:00.000Z",
			EndTimestamp:   "2026-10-02T05:00:00.000Z",
			LocalStartTime: "2026-10-02T17:00:00.000",
			LocalEndTime:   "2026-10-02T18:00:00.000",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sched.EventTime != "2026-10-02T18:00" {
		t.Fatalf("start=%q want 18:00 category default", sched.EventTime)
	}
	if sched.EventEndTime != "2026-10-02T19:00" {
		t.Fatalf("end=%q want 19:00 category default", sched.EventEndTime)
	}
}

func TestWallToUTC(t *testing.T) {
	got, err := WallToUTC("2026-10-02T18:00", "Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	// CEST = UTC+2
	want := time.Date(2026, 10, 2, 16, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestParseWallClockRejectsOffset(t *testing.T) {
	if _, ok := ParseWallClock("2026-10-02T04:00:00Z"); ok {
		t.Fatal("expected reject")
	}
	c, ok := ParseWallClock("18:00")
	if !ok || c.Hour != 18 {
		t.Fatalf("got %#v ok=%v", c, ok)
	}
}

func TestParseCampfireInstantTreatsNaiveAsUTC(t *testing.T) {
	got, ok := ParseCampfireInstant("2026-10-15T08:00:00.000")
	if !ok {
		t.Fatal("expected ok")
	}
	want := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	norm := NormalizeCampfireInstantRFC3339("2026-10-15T08:00:00.000")
	if norm != "2026-10-15T08:00:00Z" {
		t.Fatalf("norm=%q", norm)
	}
}

func TestWallToUTCStillTreatsNaiveAsWallClock(t *testing.T) {
	// Create/edit form values must stay wall-clock in the user zone — not UTC.
	got, err := WallToUTC("2026-10-15T18:00", "Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 15, 16, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
