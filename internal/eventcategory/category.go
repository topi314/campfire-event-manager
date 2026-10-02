package eventcategory

import (
	"slices"
	"strings"
)

const (
	Other   = "Other"
	NoEvent = "No Event"
)

var All = map[string][]string{
	"Raid Day":          {"Raid Day", "Mega Raid"},
	"Raid Hour":         {"Raid Hour"},
	"Max Monday":        {"Max Monday"},
	"Research Day":      {"Research Day"},
	"Hatch Day":         {"Hatch Day"},
	"Community Day":         {"Community Day"},
	"Community Day Classic": {"Community Day Classic", "Community Classic Day", "Community Classic"},
	"Spotlight Hour":        {"Spotlight Hour"},
	"Max Battle Day":    {"Max Battle Day", "Max Battle Weekend", "Max Battle", "Max Weekend", "Gigantamax", "GMAX"},
	"GO Tour":           {"GO Tour"},
	"GO Fest":           {"GO Fest"},
	"GO Wild Area":      {"GOWA", "GO Wild Area"},
	"Friendship Friday": {"Friendship Friday"},
}

var Ordered = []string{
	"GO Wild Area",
	"GO Fest",
	"GO Tour",
	"Community Day Classic",
	"Community Day",
	"Max Battle Day",
	"Research Day",
	"Hatch Day",
	"Friendship Friday",
	"Raid Day",
	"Raid Hour",
	"Max Monday",
	"Spotlight Hour",
}

// Clock is a wall-clock time-of-day.
type Clock struct {
	Hour   int
	Minute int
}

// DefaultClock returns the usual meetup hours for a live-event category.
// Local-hour events (Raid Hour, Spotlight Hour, Max Monday) are always 18:00–19:00
// in the player's timezone — Campfire's startTimestamp is a worldwide window.
func DefaultClock(category string) (start, end Clock) {
	switch strings.TrimSpace(category) {
	case "Raid Hour", "Spotlight Hour", "Max Monday":
		return Clock{18, 0}, Clock{19, 0}
	default:
		return Clock{14, 0}, Clock{17, 0}
	}
}

// IsFixedLocalHour reports categories whose play time is always 18:00–19:00 local.
func IsFixedLocalHour(category string) bool {
	switch strings.TrimSpace(category) {
	case "Raid Hour", "Spotlight Hour", "Max Monday":
		return true
	default:
		return false
	}
}

// CategoryFallbacks returns alternate categories when no template matches exactly.
func CategoryFallbacks(category string) []string {
	switch strings.TrimSpace(category) {
	case "Community Day Classic":
		return []string{"Community Day"}
	case "Community Day":
		return []string{"Community Day Classic"}
	default:
		return nil
	}
}

// CategoryMatchOrder is exact category first, then fallbacks.
func CategoryMatchOrder(liveCategory string) []string {
	cat := strings.TrimSpace(liveCategory)
	if cat == "" {
		return nil
	}
	return append([]string{cat}, CategoryFallbacks(cat)...)
}

func FromName(eventName string) string {
	eventName = strings.ToLower(strings.TrimSpace(eventName))
	if eventName == "" {
		return NoEvent
	}
	for _, category := range Ordered {
		for _, pattern := range All[category] {
			if strings.Contains(eventName, strings.ToLower(pattern)) {
				return category
			}
		}
	}
	return Other
}

func IsPreset(name string) bool {
	if name == Other || name == NoEvent {
		return true
	}
	_, ok := All[name]
	return ok
}

func Format(name string) string {
	if name == "" || IsPreset(name) {
		return name
	}
	return name + " (Custom)"
}

// Sort orders preset categories by Ordered, then Other, then No Event,
// then custom categories alphabetically.
func Sort(categories []string) []string {
	if len(categories) == 0 {
		return categories
	}

	index := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		index[category] = struct{}{}
	}

	sorted := make([]string, 0, len(categories))
	for _, category := range Ordered {
		if _, ok := index[category]; ok {
			sorted = append(sorted, category)
			delete(index, category)
		}
	}
	if _, ok := index[Other]; ok {
		sorted = append(sorted, Other)
		delete(index, Other)
	}
	if _, ok := index[NoEvent]; ok {
		sorted = append(sorted, NoEvent)
		delete(index, NoEvent)
	}

	customs := make([]string, 0, len(index))
	for category := range index {
		customs = append(customs, category)
	}
	slices.Sort(customs)
	return append(sorted, customs...)
}
