package server

import (
	"strings"
	"testing"
	"time"
)

func TestResolveMeetupCampfireTimesDuration(t *testing.T) {
	s := &Server{}
	cases := []struct {
		name    string
		start   string
		end     string
		wantErr string
	}{
		{
			name:  "minimum",
			start: "2026-10-22T18:00",
			end:   "2026-10-22T18:15",
		},
		{
			name:  "exactly 24 hours",
			start: "2026-10-22T18:00",
			end:   "2026-10-23T18:00",
		},
		{
			name:    "longer than 24 hours",
			start:   "2026-10-22T18:00",
			end:     "2026-10-23T18:01",
			wantErr: "meetup cannot be longer than 24 hours",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := s.resolveMeetupCampfireTimes(nil, tc.start, tc.end, "UTC")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestMeetupStartInPast(t *testing.T) {
	now := time.Date(2026, 11, 19, 19, 42, 0, 0, time.UTC)
	past, err := time.Parse(time.RFC3339, "2026-11-19T17:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if !meetupStartInPast(past, now) {
		t.Fatal("start 2h42m ago should be rejected")
	}
	if !meetupStartInPast(now.Add(-time.Second), now) {
		t.Fatal("start one second ago should be rejected")
	}
	if meetupStartInPast(now, now) {
		t.Fatal("start exactly now should be allowed")
	}
	if meetupStartInPast(now.Add(time.Hour), now) {
		t.Fatal("future start should be allowed")
	}
}
