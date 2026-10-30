package placeholders

import (
	_ "embed"
	"encoding/json"
	"time"

	"github.com/topi314/campfire-event-manager/server/languages"
)

//go:embed calendar_names.json
var calendarNamesJSON []byte

type calendarNameRow struct {
	Months   []string `json:"months"`
	Weekdays []string `json:"weekdays"`
}

// calendarNameTable is long month and weekday names (Sunday = 0) per template language.
// Generated from Intl.DateTimeFormat so the server matches the create-page preview.
var calendarNameTable map[string]calendarNameRow

func init() {
	if err := json.Unmarshal(calendarNamesJSON, &calendarNameTable); err != nil {
		panic(err)
	}
	en, ok := calendarNameTable["en"]
	if !ok || len(en.Months) != 12 || len(en.Weekdays) != 7 {
		panic("calendar names: english row missing")
	}
}

func namesFor(local time.Time, lang string) (monthName, weekdayName string) {
	code := languages.Normalize(lang)
	if code == "" {
		code = "en"
	}
	set, ok := calendarNameTable[code]
	if !ok || len(set.Months) != 12 || len(set.Weekdays) != 7 {
		set = calendarNameTable["en"]
	}
	return set.Months[int(local.Month())-1], set.Weekdays[int(local.Weekday())]
}

func applyCalendarNames(out map[string]string, local time.Time, lang string) {
	monthName, weekdayName := namesFor(local, lang)
	out["monthName"] = monthName
	out["weekday"] = weekdayName
	for code := range calendarNameTable {
		month, weekday := namesFor(local, code)
		out["monthName:"+code] = month
		out["weekday:"+code] = weekday
	}
}

func clearCalendarNames(out map[string]string) {
	out["monthName"] = ""
	out["weekday"] = ""
	for code := range calendarNameTable {
		out["monthName:"+code] = ""
		out["weekday:"+code] = ""
	}
}
