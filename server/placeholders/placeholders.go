package placeholders

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/topi314/campfire-event-manager/server/pokemon"
	"github.com/topi314/campfire-event-manager/server/timeformat"
)

// Match {{key}}, {{key[1]}}, {{eventPokemon[1]:de}}, and {{date:dmy-dot-24}}.
var tokenRE = regexp.MustCompile(`\{\{\s*([a-zA-Z][a-zA-Z0-9_-]*(?:\[[0-9]+\])?(?::[a-zA-Z0-9]+(?:-[a-zA-Z0-9]+)*)?)\s*\}\}`)

var suffixRE = regexp.MustCompile(`(?i):([a-z0-9]+(?:-[a-z0-9]+)*)$`)

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
	"eventPokemonFloorResearch",
	"eventPokemonCeilingEgg",
	"eventPokemonFloorEgg",
	"eventPokemonCeilingRaid",
	"eventPokemonFloorRaid",
	"eventPokemonCeilingRaidWeather",
	"eventPokemonFloorRaidWeather",
	"eventPokemonFloorSuperMegaRaid",
	"eventPokemonFloorSuperMegaRaidWeather",
	"eventPokemonFloorShadowRaid",
	"eventPokemonFloorShadowRaidWeather",
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
	// Language is the template language code (for eventPokemon translation).
	Language string
	// CategoryPatterns maps category → phrases used to strip titles when extracting species.
	CategoryPatterns map[string][]string
}

// NormalizeKey lowercases a placeholder key for comparison (includes [i]).
func NormalizeKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func splitSuffix(key string) (rest, suffix string, has bool) {
	key = strings.TrimSpace(key)
	m := suffixRE.FindStringSubmatchIndex(key)
	if m == nil {
		return key, "", false
	}
	return key[:m[0]], strings.ToLower(key[m[2]:m[3]]), true
}

func isTimeLayout(base string) bool {
	switch strings.ToLower(strings.TrimSpace(base)) {
	case "date", "starttime", "endtime":
		return true
	default:
		return false
	}
}

func keepSuffix(base, suffix string) bool {
	if suffix == "" {
		return false
	}
	if strings.EqualFold(base, "eventPokemon") {
		_, _, ok := pokemon.SplitLang(base + ":" + suffix)
		return ok
	}
	return isTimeLayout(base) && timeformat.Valid(suffix)
}

// baseKey strips a trailing [digits] index.
// A :lang pin is stripped for eventPokemon names, and a time-format pin for date/startTime/endTime.
// Any other suffix is left in place so the key is not treated as that builtin.
func baseKey(key string) string {
	rest, suffix, has := splitSuffix(key)
	base, _, _ := pokemon.SplitIndex(rest)
	if has && !strings.EqualFold(strings.TrimSpace(base), "eventPokemon") && !isTimeLayout(base) {
		return strings.TrimSpace(key)
	}
	_ = suffix
	return base
}

// CanonicalKey returns the documented spelling for built-ins; otherwise the trimmed key.
func CanonicalKey(key string) string {
	trimmed := strings.TrimSpace(key)
	rest, suffix, hasSuffix := splitSuffix(trimmed)
	base, idx, has := pokemon.SplitIndex(rest)
	c, ok := builtinByNorm[NormalizeKey(base)]
	if !ok {
		return trimmed
	}
	if hasSuffix && !keepSuffix(c, suffix) {
		if strings.EqualFold(c, "eventPokemon") || isTimeLayout(c) {
			if has {
				return fmt.Sprintf("%s[%d]", c, idx)
			}
			return c
		}
		return trimmed
	}
	out := c
	if has {
		out = fmt.Sprintf("%s[%d]", c, idx)
	}
	if hasSuffix && keepSuffix(c, suffix) {
		out += ":" + suffix
	}
	return out
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
		if v, ok := lookupPinnedLang(values, key); ok {
			return v
		}
		if v, ok := lookupUnscoped(values, key); ok {
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
		for _, id := range timeformat.IDs() {
			pieces, ok := timeformat.Layout(id, local)
			if !ok {
				continue
			}
			out["date:"+id] = pieces.Date
			out["startTime:"+id] = pieces.Time
		}
	} else {
		for _, id := range timeformat.IDs() {
			out["date:"+id] = ""
			out["startTime:"+id] = ""
		}
	}
	if ctx.HasEndTime && !ctx.EventEndTime.IsZero() {
		endLocal := ctx.EventEndTime.In(loc)
		out["endTime"] = endLocal.Format("15:04")
		for _, id := range timeformat.IDs() {
			pieces, ok := timeformat.Layout(id, endLocal)
			if !ok {
				continue
			}
			out["endTime:"+id] = pieces.Time
		}
	} else {
		for _, id := range timeformat.IDs() {
			out["endTime:"+id] = ""
		}
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
// Manual eventPokemon name overrides recompute dependent ceiling/floor placeholders.
func ResolveAndApply(fields TextFields, ctx Context, custom map[string]string) TextFields {
	values := MergeValues(ResolveBuiltins(ctx), custom)
	pokemon.EnrichFromNameOverrides(values, custom, ctx.Language)
	return ApplyToFields(fields, values)
}

// lookupPinnedLang resolves eventPokemon:zh-CN via the region prefix, then English.
func lookupPinnedLang(values map[string]string, key string) (string, bool) {
	rest, lang, hasLang := pokemon.SplitLang(key)
	if !hasLang {
		return "", false
	}
	base, _, _ := pokemon.SplitIndex(rest)
	if !strings.EqualFold(strings.TrimSpace(base), "eventPokemon") {
		return "", false
	}
	if i := strings.IndexByte(lang, '-'); i > 0 {
		if v, ok := Lookup(values, rest+":"+lang[:i]); ok {
			return v, true
		}
	}
	if v, ok := Lookup(values, rest+":en"); ok {
		return v, true
	}
	return "", false
}

// lookupUnscoped maps an unknown pin on eventPokemon or a time token back to the unscoped value.
func lookupUnscoped(values map[string]string, key string) (string, bool) {
	rest, suffix, has := splitSuffix(key)
	if !has || suffix == "" {
		return "", false
	}
	base, idx, hasIdx := pokemon.SplitIndex(rest)
	if !strings.EqualFold(strings.TrimSpace(base), "eventPokemon") && !isTimeLayout(base) {
		return "", false
	}
	if keepSuffix(base, suffix) {
		return "", false
	}
	unscoped := base
	if c, ok := builtinByNorm[NormalizeKey(base)]; ok {
		unscoped = c
	}
	if hasIdx {
		unscoped = fmt.Sprintf("%s[%d]", unscoped, idx)
	}
	return Lookup(values, unscoped)
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
