package eventcategory

import (
	"slices"
	"strings"
)

const (
	Other   = "Other"
	NoEvent = "No Event"
)

// All maps category → substrings that identify the category in a live-event title.
// More specific raid types must appear before Raid Day / Raid Hour in Ordered
// (e.g. "Super Mega Raid Day" also contains "Mega Raid" / "Raid Day").
var All = map[string][]string{
	"Raid Day": {
		"Raid Day",
		"Mega Raid",
		"Elite Raid",   // "Elite Raids"
		"Mega Ascension",
	},
	"Raid Hour": {"Raid Hour"},
	"Super Mega Raid Day": {
		"Super Mega Raid Day",
		"Super Mega Raids",
		"Super Mega Raid",
	},
	// Super Mega / Shadow are Raid Day variants for template fallbacks.
	"Shadow Raid Day": {
		"Shadow Raid Day",
		"Shadow Raid Hour",
		"Shadow Raid",
		"Shadow",
	},
	"Max Monday":     {"Max Monday"},
	"Research Day":   {"Research Day"},
	"Catch Mastery":  {"Catch Mastery"},
	"Hatch Day":      {"Hatch Day"},
	"Community Day": {"Community Day"},
	"Community Day Classic": {
		"Community Day Classic",
		"Community Classic Day",
		"Community Classic",
	},
	"Spotlight Hour": {"Spotlight Hour", "SL Hour"},
	// "Max Finale" is after GO Fest in Ordered so "GO Fest … Max Finale" stays GO Fest.
	"Max Battle Day": {
		"Max Battle Day",
		"Max Battle Weekend",
		"Max Battle",
		"Max Weekend",
		"Max Finale",
		"Gigantamax",
		"GMAX",
	},
	"GO Tour":           {"GO Tour"},
	"GO Fest":           {"GO Fest"},
	"GO Wild Area":      {"GOWA", "GO Wild Area"},
	"Friendship Friday": {"Friendship Friday"},
}

// StripExtra are additional title fragments removed when extracting species names.
// Not used for category matching (so "Raid Day" here cannot steal normal Raid Days).
var StripExtra = map[string][]string{
	"Shadow Raid Day": {
		"Raid Day", "Raid Hour", "Weekend", "Day 1", "Day 2", "(Day 1)", "(Day 2)",
	},
	"Super Mega Raid Day": {
		"Ultra Unlock:",
		"Ultra Unlock",
	},
}

var Ordered = []string{
	"GO Wild Area",
	"GO Fest",
	"GO Tour",
	"Community Day Classic",
	"Community Day",
	"Max Battle Day",
	"Research Day",
	"Catch Mastery",
	"Hatch Day",
	"Friendship Friday",
	"Super Mega Raid Day",
	"Shadow Raid Day",
	"Raid Day",
	"Raid Hour",
	"Max Monday",
	"Spotlight Hour",
}

// CatalogEntry is a built-in category for API clients (template dropdowns, etc.).
type CatalogEntry struct {
	Name           string   `json:"name"`
	FixedLocalHour bool     `json:"fixedLocalHour"`
	Fallbacks      []string `json:"fallbacks"`
}

// ClientCatalog returns Ordered presets with metadata (single source for the UI).
func ClientCatalog() []CatalogEntry {
	out := make([]CatalogEntry, 0, len(Ordered))
	for _, name := range Ordered {
		fb := CategoryFallbacks(name)
		if fb == nil {
			fb = []string{}
		}
		out = append(out, CatalogEntry{
			Name:           name,
			FixedLocalHour: IsFixedLocalHour(name),
			Fallbacks:      fb,
		})
	}
	return out
}

// Clock is a wall-clock time-of-day.
type Clock struct {
	Hour   int
	Minute int
}

// DefaultClock returns the usual meetup hours for a live-event category.
// Research / Hatch / Catch Mastery: 11:00–17:00. Raid Day (+ Super Mega / Shadow)
// and Max Battle Day: 14:00–17:00. Raid Hour / Spotlight / Max Monday: 18:00–19:00.
func DefaultClock(category string) (start, end Clock) {
	switch strings.TrimSpace(category) {
	case "Raid Hour", "Spotlight Hour", "Max Monday":
		return Clock{18, 0}, Clock{19, 0}
	case "Research Day", "Hatch Day", "Catch Mastery":
		return Clock{11, 0}, Clock{17, 0}
	case "Raid Day", "Super Mega Raid Day", "Shadow Raid Day", "Max Battle Day":
		return Clock{14, 0}, Clock{17, 0}
	default:
		// Community Day, etc.
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
	case "Super Mega Raid Day", "Shadow Raid Day":
		return []string{"Raid Day"}
	case "Catch Mastery":
		return []string{"Research Day"}
	default:
		return nil
	}
}

// ExtractPatterns returns phrases to strip from a title when extracting species
// (match patterns plus StripExtra), longest first.
func ExtractPatterns(category string) []string {
	category = strings.TrimSpace(category)
	seen := map[string]struct{}{}
	var out []string
	add := func(phrases []string) {
		for _, p := range phrases {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			k := strings.ToLower(p)
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, p)
		}
	}
	add(All[category])
	add(StripExtra[category])
	slices.SortFunc(out, func(a, b string) int {
		if len(a) == len(b) {
			return strings.Compare(strings.ToLower(a), strings.ToLower(b))
		}
		return len(b) - len(a) // longer first
	})
	return out
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
