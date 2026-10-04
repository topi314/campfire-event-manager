package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/topi314/campfire-event-manager/internal/meetuptime"
)

// minMeetupDuration is the shortest allowed meetup length.
const minMeetupDuration = 15 * time.Minute

type meetupScheduleRequest struct {
	TimeZone  string `json:"timeZone"`
	Date      string `json:"date"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	LiveEvent *struct {
		EventName      string `json:"eventName"`
		StartTimestamp string `json:"startTimestamp"`
		EndTimestamp   string `json:"endTimestamp"`
		LocalStartTime string `json:"localStartTime"`
		LocalEndTime   string `json:"localEndTime"`
	} `json:"liveEvent"`
}

// campfireMeetupSchedule resolves wall-clock meetup start/end for the create form.
// Clocks come from template HH:mm or category defaults; live events only supply the day.
func (s *Server) campfireMeetupSchedule(w http.ResponseWriter, r *http.Request) {
	var body meetupScheduleRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	tz := s.resolveTimeZone(r, body.TimeZone)
	in := meetuptime.Input{
		TimeZone:  tz,
		Date:      strings.TrimSpace(body.Date),
		StartTime: strings.TrimSpace(body.StartTime),
		EndTime:   strings.TrimSpace(body.EndTime),
	}
	if body.LiveEvent != nil {
		in.Live = &meetuptime.LiveEventFields{
			EventName:      body.LiveEvent.EventName,
			StartTimestamp: body.LiveEvent.StartTimestamp,
			EndTimestamp:   body.LiveEvent.EndTimestamp,
			LocalStartTime: body.LiveEvent.LocalStartTime,
			LocalEndTime:   body.LiveEvent.LocalEndTime,
		}
	}

	sched, err := meetuptime.Resolve(in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sched)
}

// resolveMeetupCampfireTimes converts create/edit wall clocks to RFC3339 UTC.
// End time is required and must be at least minMeetupDuration after start.
func (s *Server) resolveMeetupCampfireTimes(r *http.Request, eventTime, eventEndTime, timeZone string) (string, string, error) {
	tz := s.resolveTimeZone(r, timeZone)
	start, err := meetuptime.WallToUTC(eventTime, tz)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(eventEndTime) == "" {
		return "", "", fmt.Errorf("end time is required")
	}
	end, err := meetuptime.WallToUTC(eventEndTime, tz)
	if err != nil {
		return "", "", err
	}
	if !end.After(start) {
		return "", "", fmt.Errorf("end time must be after start time")
	}
	if end.Sub(start) < minMeetupDuration {
		return "", "", fmt.Errorf("meetup must be at least %d minutes long", int(minMeetupDuration/time.Minute))
	}
	return meetuptime.FormatRFC3339(start), meetuptime.FormatRFC3339(end), nil
}
