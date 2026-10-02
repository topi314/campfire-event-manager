package server

import (
	"net/http"
	"strings"

	"github.com/topi314/campfire-event-manager/internal/meetuptime"
)

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
func (s *Server) resolveMeetupCampfireTimes(r *http.Request, eventTime, eventEndTime, timeZone string) (string, string, error) {
	tz := s.resolveTimeZone(r, timeZone)
	start, err := meetuptime.WallToUTC(eventTime, tz)
	if err != nil {
		return "", "", err
	}
	outStart := meetuptime.FormatRFC3339(start)
	outEnd := ""
	if strings.TrimSpace(eventEndTime) != "" {
		end, err := meetuptime.WallToUTC(eventEndTime, tz)
		if err != nil {
			return "", "", err
		}
		outEnd = meetuptime.FormatRFC3339(end)
	}
	return outStart, outEnd, nil
}
