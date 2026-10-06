package placeholders

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/topi314/campfire-event-manager/server/pokemon"
)

// Match {{key}} or {{key[1]}} tokens.
var tokenRE = regexp.MustCompile(`\{\{\s*([a-zA-Z][a-zA-Z0-9_-]*(?:\[[0-9]+\])?)\s*\}\}`)

// BuiltinKeys lists documented built-in placeholder names (canonical casing, unindexed).
var BuiltinKeys = []string{
	"club",
	"liveEvent",
	"category",
	"title",
	"address",
	"lat",
	"lng",
	"date",
	"dateShort",
	"weekday",
	"startTime",
	"endTime",
	"timezone",
	"eventPokemon",
	"eventPokemonCeilingResearch",
	"eventPokemonCeilingRaid",
	"eventPokemonCeilingEgg",
	"eventPokemonCeilingRaidWeather",
	"eventPokemonFloorResearch",
	"eventPokemonFloorRaid",
	"eventPokemonFloorEgg",
	"eventPokemonFloorRaidWeather",
}

var builtinByNorm = func() map[string]string {
	m := make(map[string]string, len(BuiltinKeys))
	for _, k := range BuiltinKeys {
		m[strings.ToLower(k)] = k
	}
	return m
}()

// Context holds values used to fill built-in placeholders.
type Context struct {
	ClubName      string
	LiveEventName string
	Category      string
	Title         string
	Address       string
	Latitude      *float64
	Longitude     *float64
	EventTime     time.Time
	EventEndTime  time.Time
	HasEndTime    bool
	TimeZone      string
	// Language is the template language code (for eventPokemon translation + CP unit).
	Language string
	// CategoryPatterns maps category → phrases used to strip titles when extracting species.
	CategoryPatterns map[string][]string
}

// NormalizeKey lowercases a placeholder key for comparison (includes [i]).
func NormalizeKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

// baseKey strips a trailing [digits] index for builtin identity checks.
func baseKey(key string) string {
	base, _, has := pokemon.SplitIndex(key)
	if has {
		return base
	}
	return strings.TrimSpace(key)
}

// CanonicalKey returns the documented spelling for built-ins; otherwise the trimmed key.
func CanonicalKey(key string) string {
	trimmed := strings.TrimSpace(key)
	base, idx, has := pokemon.SplitIndex(trimmed)
	if c, ok := builtinByNorm[NormalizeKey(base)]; ok {
		if has {
			return fmt.Sprintf("%s[%d]", c, idx)
		}
		return c
	}
	return trimmed
}

// IsBuiltin reports whether key is a built-in placeholder (case-insensitive; index ignored).
func IsBuiltin(key string) bool {
	_, ok := builtinByNorm[NormalizeKey(baseKey(key))]
	return ok
}

// Lookup returns values[key] with case-insensitive fallback.
func Lookup(values map[string]string, key string) (string, bool) {
	if values == nil {
		return "", false
	}
	if v, ok := values[key]; ok {
		return v, true
	}
	norm := NormalizeKey(key)
	for k, v := range values {
		if NormalizeKey(k) == norm {
			return v, true
		}
	}
	return "", false
}

// Apply substitutes {{tokens}} in text using values (case-insensitive keys).
// Unknown tokens are left unchanged, except event-pokemon builtins (incl. OOB indexes)
// which resolve to empty when missing.
func Apply(text string, values map[string]string) string {
	if text == "" || !strings.Contains(text, "{{") {
		return text
	}
	return tokenRE.ReplaceAllStringFunc(text, func(match string) string {
		sub := tokenRE.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		key := sub[1]
		if v, ok := Lookup(values, key); ok {
			return v
		}
		base := baseKey(key)
		if pokemon.IsEventPokemonBase(base) {
			return ""
		}
		return match
	})
}

// ResolveBuiltins builds the built-in placeholder map from ctx.
func ResolveBuiltins(ctx Context) map[string]string {
	loc := loadLocation(ctx.TimeZone)
	out := map[string]string{
		"club":      strings.TrimSpace(ctx.ClubName),
		"liveEvent": strings.TrimSpace(ctx.LiveEventName),
		"category":  strings.TrimSpace(ctx.Category),
		"title":     strings.TrimSpace(ctx.Title),
		"address":   strings.TrimSpace(ctx.Address),
		"timezone":  strings.TrimSpace(ctx.TimeZone),
		"lat":       "",
		"lng":       "",
		"date":      "",
		"dateShort": "",
		"weekday":   "",
		"startTime": "",
		"endTime":   "",
	}
	if ctx.Latitude != nil {
		out["lat"] = formatCoord(*ctx.Latitude)
	}
	if ctx.Longitude != nil {
		out["lng"] = formatCoord(*ctx.Longitude)
	}
	if !ctx.EventTime.IsZero() {
		local := ctx.EventTime.In(loc)
		out["date"] = local.Format("Jan 2, 2006")
		out["dateShort"] = local.Format("2006-01-02")
		out["weekday"] = local.Format("Monday")
		out["startTime"] = local.Format("15:04")
	}
	if ctx.HasEndTime && !ctx.EventEndTime.IsZero() {
		out["endTime"] = ctx.EventEndTime.In(loc).Format("15:04")
	}

	patterns := ctx.CategoryPatterns
	ep := pokemon.ResolveEventPokemon(ctx.LiveEventName, ctx.Category, ctx.Language, patterns)
	for k, v := range ep {
		out[k] = v
	}
	return out
}

// MergeValues returns builtins overridden by custom (case-insensitive; custom wins).
func MergeValues(builtins map[string]string, custom map[string]string) map[string]string {
	out := make(map[string]string, len(builtins)+len(custom))
	for k, v := range builtins {
		out[k] = v
	}
	for k, v := range custom {
		if strings.TrimSpace(k) == "" {
			continue
		}
		canon := CanonicalKey(k)
		out[canon] = v
		if canon != k {
			out[k] = v
		}
	}
	return out
}

// TextFields are meetup string fields that may contain placeholders.
type TextFields struct {
	Name          string
	Details       string
	Address       string
	CoverPhotoURL string
}

// ApplyToFields resolves placeholders in meetup text fields.
func ApplyToFields(fields TextFields, values map[string]string) TextFields {
	return TextFields{
		Name:          Apply(fields.Name, values),
		Details:       Apply(fields.Details, values),
		Address:       Apply(fields.Address, values),
		CoverPhotoURL: Apply(fields.CoverPhotoURL, values),
	}
}

// ResolveAndApply builds builtins from ctx, merges custom values, and applies to fields.
func ResolveAndApply(fields TextFields, ctx Context, custom map[string]string) TextFields {
	values := MergeValues(ResolveBuiltins(ctx), custom)
	return ApplyToFields(fields, values)
}

func loadLocation(tz string) *time.Location {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func formatCoord(v float64) string {
	s := fmt.Sprintf("%g", v)
	return s
}
