package server

import (
	"strings"
	"testing"
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
