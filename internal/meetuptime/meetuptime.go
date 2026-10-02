package meetuptime

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/topi314/campfire-event-manager/internal/eventcategory"
)

var (
	clockRE     = regexp.MustCompile(`^(\d{1,2}):(\d{2})(?::\d{2})?$`)
	wallDateRE  = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2}))?)?$`)
	hasOffsetRE = regexp.MustCompile(`(?i)(z|[+-]\d{2}:?\d{2})$`)
)

// Clock is HH:mm wall clock.
type Clock struct {
	Hour   int
	Minute int
}

// Day is a calendar date.
type Day struct {
	Year  int
	Month time.Month
	Day   int
}

// LiveEventFields are the Campfire live-event bits needed for scheduling.
type LiveEventFields struct {
	EventName      string
	StartTimestamp string
	EndTimestamp   string
	LocalStartTime string
	LocalEndTime   string
}

// Input drives schedule resolution for the create form / create API.
type Input struct {
	TimeZone string
	Live     *LiveEventFields
	// Optional calendar day YYYY-MM-DD (e.g. keep an existing meetup's day on edit).
	Date string
	// Optional template (or explicit) clocks. HH:mm or offset-less datetime.
	StartTime string
	EndTime   string
}

// Schedule is wall-clock meetup times ready for datetime-local inputs / conversion.
type Schedule struct {
	Category     string `json:"category"`
	EventTime    string `json:"eventTime"`    // YYYY-MM-DDTHH:mm
	EventEndTime string `json:"eventEndTime"` // YYYY-MM-DDTHH:mm
}

// Resolve builds wall-clock start/end for a meetup.
// Live events supply the calendar day; clocks come from template HH:mm, then
// category defaults. Absolute Campfire window timestamps are never used as clocks.
func Resolve(in Input) (Schedule, error) {
	loc, err := LoadLocation(in.TimeZone)
	if err != nil {
		return Schedule{}, err
	}

	category := ""
	var day Day
	if d, ok := ParseDay(in.Date); ok {
		day = d
	}
	if in.Live != nil && strings.TrimSpace(in.Live.EventName) != "" {
		category = eventcategory.FromName(in.Live.EventName)
		if day.Year == 0 {
			day = CalendarDayFromLive(*in.Live, loc)
		}
	} else if day.Year == 0 {
		now := time.Now().In(loc)
		day = Day{Year: now.Year(), Month: now.Month(), Day: now.Day()}
	}

	defStart, defEnd := eventcategory.DefaultClock(category)
	startClock := Clock{defStart.Hour, defStart.Minute}
	endClock := Clock{defEnd.Hour, defEnd.Minute}

	fixed := eventcategory.IsFixedLocalHour(category)
	if !fixed {
		if c, ok := ParseWallClock(in.StartTime); ok {
			startClock = c
		} else if in.Live != nil {
			if c, ok := ParseWallClock(in.Live.LocalStartTime); ok {
				startClock = c
			}
		}
		if c, ok := ParseWallClock(in.EndTime); ok {
			endClock = c
		} else if in.Live != nil {
			if c, ok := ParseWallClock(in.Live.LocalEndTime); ok {
				endClock = c
			}
		}
	} else {
		// Template clocks still win for fixed-hour categories when explicitly set
		// (e.g. a Raid Hour template with 18:00). Otherwise use category defaults —
		// never Campfire's worldwide window.
		if c, ok := ParseWallClock(in.StartTime); ok {
			startClock = c
		}
		if c, ok := ParseWallClock(in.EndTime); ok {
			endClock = c
		}
	}

	start := Combine(day, startClock)
	endDay := day
	if !fixed && in.Live != nil {
		if d, ok := ParseDay(in.Live.LocalEndTime); ok {
			endDay = d
		}
	}
	end := Combine(endDay, endClock)
	if !end.After(start) {
		end = end.Add(24 * time.Hour)
	}

	return Schedule{
		Category:     category,
		EventTime:    FormatWall(start),
		EventEndTime: FormatWall(end),
	}, nil
}

// LoadLocation loads an IANA zone or UTC.
func LoadLocation(tz string) (*time.Location, error) {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("invalid timeZone %q", tz)
	}
	return loc, nil
}

// ParseWallClock accepts HH:mm or offset-less YYYY-MM-DDTHH:mm (rejects Z/offsets).
func ParseWallClock(raw string) (Clock, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || hasOffsetRE.MatchString(raw) {
		return Clock{}, false
	}
	if m := clockRE.FindStringSubmatch(raw); m != nil {
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		if h > 23 || min > 59 {
			return Clock{}, false
		}
		return Clock{Hour: h, Minute: min}, true
	}
	if m := wallDateRE.FindStringSubmatch(raw); m != nil && m[4] != "" {
		h, _ := strconv.Atoi(m[4])
		min, _ := strconv.Atoi(m[5])
		if h > 23 || min > 59 {
			return Clock{}, false
		}
		return Clock{Hour: h, Minute: min}, true
	}
	return Clock{}, false
}

// ParseDay reads YYYY-MM-DD from the start of a string.
func ParseDay(raw string) (Day, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Day{}, false
	}
	m := wallDateRE.FindStringSubmatch(raw)
	if m == nil {
		// Also accept date-only prefix on longer strings.
		if len(raw) >= 10 && raw[4] == '-' && raw[7] == '-' {
			y, err1 := strconv.Atoi(raw[0:4])
			mo, err2 := strconv.Atoi(raw[5:7])
			d, err3 := strconv.Atoi(raw[8:10])
			if err1 == nil && err2 == nil && err3 == nil && mo >= 1 && mo <= 12 && d >= 1 && d <= 31 {
				return Day{Year: y, Month: time.Month(mo), Day: d}, true
			}
		}
		return Day{}, false
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d, _ := strconv.Atoi(m[3])
	if mo < 1 || mo > 12 || d < 1 || d > 31 {
		return Day{}, false
	}
	return Day{Year: y, Month: time.Month(mo), Day: d}, true
}

// CalendarDayFromLive prefers localStartTime's date, then the instant in loc.
func CalendarDayFromLive(ev LiveEventFields, loc *time.Location) Day {
	if d, ok := ParseDay(ev.LocalStartTime); ok {
		return d
	}
	if t, ok := parseInstant(ev.StartTimestamp); ok {
		local := t.In(loc)
		return Day{Year: local.Year(), Month: local.Month(), Day: local.Day()}
	}
	if d, ok := ParseDay(ev.StartTimestamp); ok {
		return d
	}
	now := time.Now().In(loc)
	return Day{Year: now.Year(), Month: now.Month(), Day: now.Day()}
}

func parseInstant(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Combine builds a wall-clock time.Time in a dummy fixed zone (UTC label, wall values).
func Combine(day Day, clock Clock) time.Time {
	return time.Date(day.Year, day.Month, day.Day, clock.Hour, clock.Minute, 0, 0, time.UTC)
}

// FormatWall formats as datetime-local YYYY-MM-DDTHH:mm.
func FormatWall(t time.Time) string {
	return t.Format("2006-01-02T15:04")
}

// WallToUTC interprets a wall-clock string (datetime-local or RFC3339) in loc.
// Absolute timestamps (Z/offset) are returned as-is.
func WallToUTC(raw, timeZone string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if t, ok := parseInstant(raw); ok {
		return t.UTC(), nil
	}

	loc, err := LoadLocation(timeZone)
	if err != nil {
		return time.Time{}, err
	}

	// datetime-local / naive
	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	} {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q", raw)
}

// FormatRFC3339 formats t as RFC3339 UTC.
func FormatRFC3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
