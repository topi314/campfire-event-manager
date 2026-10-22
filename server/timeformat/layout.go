package timeformat

import (
	"fmt"
	"time"
)

var monthAbbr = [...]string{
	"Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
}

// Pieces is the date and clock half of a Settings time-format preset.
type Pieces struct {
	Date string
	Time string
}

// Layout renders the date and clock for a preset id. ok is false for unknown ids.
func Layout(id string, t time.Time) (Pieces, bool) {
	if !Valid(id) {
		return Pieces{}, false
	}
	mon := monthAbbr[int(t.Month())-1]
	day := t.Day()
	dd := fmt.Sprintf("%02d", day)
	mm := fmt.Sprintf("%02d", int(t.Month()))
	yyyy := fmt.Sprintf("%04d", t.Year())
	dayStr := fmt.Sprintf("%d", day)
	date := fmt.Sprintf("%s %s %s", dayStr, mon, yyyy)
	clock := clock24(t.Hour(), t.Minute(), ":")
	switch id {
	case "mdy-12":
		date = fmt.Sprintf("%s %d, %s", mon, day, yyyy)
		clock = clock12(t.Hour(), t.Minute(), true)
	case "mdy-slash-12":
		date = fmt.Sprintf("%s/%s/%s", mm, dd, yyyy)
		clock = clock12(t.Hour(), t.Minute(), true)
	case "dmy-12":
		date = fmt.Sprintf("%s %s %s", dayStr, mon, yyyy)
		clock = clock12(t.Hour(), t.Minute(), false)
	case "dmy-slash-12":
		date = fmt.Sprintf("%s/%s/%s", dd, mm, yyyy)
		clock = clock12(t.Hour(), t.Minute(), false)
	case "dmy-24":
		date = fmt.Sprintf("%s %s %s", dayStr, mon, yyyy)
		clock = clock24(t.Hour(), t.Minute(), ":")
	case "dmy-dot-24":
		date = fmt.Sprintf("%s.%s.%s", dd, mm, yyyy)
		clock = clock24(t.Hour(), t.Minute(), ":")
	case "dmy-slash-24":
		date = fmt.Sprintf("%s/%s/%s", dd, mm, yyyy)
		clock = clock24(t.Hour(), t.Minute(), ":")
	case "dmy-dash-24":
		date = fmt.Sprintf("%s-%s-%s", dd, mm, yyyy)
		clock = clock24(t.Hour(), t.Minute(), ":")
	case "dmy-dot-time":
		date = fmt.Sprintf("%s.%s.%s", dd, mm, yyyy)
		clock = clock24(t.Hour(), t.Minute(), ".")
	case "dmy-h":
		date = fmt.Sprintf("%s %s %s", dayStr, mon, yyyy)
		clock = clock24(t.Hour(), t.Minute(), "h")
	case "ymd-24":
		date = fmt.Sprintf("%s-%s-%s", yyyy, mm, dd)
		clock = clock24(t.Hour(), t.Minute(), ":")
	case "ymd-slash-24":
		date = fmt.Sprintf("%s/%s/%s", yyyy, mm, dd)
		clock = clock24(t.Hour(), t.Minute(), ":")
	case "ymd-dot-24":
		date = fmt.Sprintf("%s.%s.%s", yyyy, mm, dd)
		clock = clock24(t.Hour(), t.Minute(), ":")
	case "hm-dmy":
		date = fmt.Sprintf("%s %s %s", dayStr, mon, yyyy)
		clock = clock24(t.Hour(), t.Minute(), ":")
	case "hm-dmy-slash":
		date = fmt.Sprintf("%s/%s/%s", dd, mm, yyyy)
		clock = clock24(t.Hour(), t.Minute(), ":")
	}
	return Pieces{Date: date, Time: clock}, true
}

func clock12(hours, minutes int, upper bool) string {
	pm := hours >= 12
	h := hours % 12
	if h == 0 {
		h = 12
	}
	mark := "pm"
	if upper {
		mark = "PM"
	}
	if !pm {
		if upper {
			mark = "AM"
		} else {
			mark = "am"
		}
	}
	return fmt.Sprintf("%d:%02d %s", h, minutes, mark)
}

func clock24(hours, minutes int, sep string) string {
	if sep == "h" {
		return fmt.Sprintf("%02d h %02d", hours, minutes)
	}
	return fmt.Sprintf("%02d%s%02d", hours, sep, minutes)
}
