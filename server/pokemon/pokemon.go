package pokemon

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"
)

//go:embed pokemon.json
var catalogJSON []byte

// Species is one matchable Pokémon / form with GO base stats and localized names.
type Species struct {
	ID          int               `json:"id"`
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Aliases     []string          `json:"aliases"`
	BaseAttack  int               `json:"baseAttack"`
	BaseDefense int               `json:"baseDefense"`
	BaseStamina int               `json:"baseStamina"`
	Names       map[string]string `json:"names"`
}

// CPM for whole levels used by encounter CP placeholders.
var cpmByLevel = map[int]float64{
	15: 0.5173939515931172,
	20: 0.597400009632111,
	25: 0.667934000491142,
}

// ExtractableCategories are live-event categories where titles usually name a species.
var ExtractableCategories = map[string]struct{}{
	"Community Day":         {},
	"Community Day Classic": {},
	"Raid Hour":             {},
	"Raid Day":              {},
	"Spotlight Hour":        {},
	"Research Day":          {},
	"Hatch Day":             {},
	"Max Monday":            {},
	"Max Battle Day":        {},
}

// EventPokemonBaseKeys are the unindexed event-pokemon builtin bases.
var EventPokemonBaseKeys = []string{
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

var (
	loadOnce sync.Once
	catalog  []Species
	byNorm   map[string]*Species // alias/name → species (longest aliases registered first)
	loadErr  error
)

func load() {
	loadOnce.Do(func() {
		if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
			loadErr = err
			return
		}
		type alias struct {
			text string
			sp   *Species
		}
		var aliases []alias
		for i := range catalog {
			sp := &catalog[i]
			seen := map[string]struct{}{}
			add := func(s string) {
				s = strings.TrimSpace(s)
				if s == "" {
					return
				}
				n := normalizeName(s)
				if _, ok := seen[n]; ok {
					return
				}
				seen[n] = struct{}{}
				aliases = append(aliases, alias{text: n, sp: sp})
			}
			add(sp.Name)
			for _, a := range sp.Aliases {
				add(a)
			}
		}
		sort.Slice(aliases, func(i, j int) bool {
			if len(aliases[i].text) != len(aliases[j].text) {
				return len(aliases[i].text) > len(aliases[j].text)
			}
			return aliases[i].text < aliases[j].text
		})
		byNorm = make(map[string]*Species, len(aliases))
		for _, a := range aliases {
			if _, ok := byNorm[a.text]; !ok {
				byNorm[a.text] = a.sp
			}
		}
	})
}

// Catalog returns the embedded species list.
func Catalog() ([]Species, error) {
	load()
	return catalog, loadErr
}

func normalizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		// Treat punctuation used in Campfire titles ("Origin Forme", etc.) as spaces.
		if unicode.IsSpace(r) || r == '-' || r == '_' || r == '(' || r == ')' || r == '[' || r == ']' || r == ',' || r == ':' || r == '/' {
			if !prevSpace && b.Len() > 0 {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// stripFormeWords drops trailing/standalone "forme"/"form" tokens for looser matching.
func stripFormeWords(n string) string {
	parts := strings.Fields(n)
	out := parts[:0]
	for _, p := range parts {
		if p == "forme" || p == "form" {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, " ")
}

// CombatUnit returns the localized CP abbreviation for a template language code.
func CombatUnit(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "de":
		return "WP"
	case "fr", "es", "it":
		return "PC"
	default:
		return "CP"
	}
}

// LocalizedName returns the species display name in lang (fallback en / Name).
func LocalizedName(sp *Species, lang string) string {
	if sp == nil {
		return ""
	}
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang != "" && sp.Names != nil {
		if v := strings.TrimSpace(sp.Names[lang]); v != "" {
			return v
		}
		// zh-CN → zh
		if i := strings.IndexByte(lang, '-'); i > 0 {
			if v := strings.TrimSpace(sp.Names[lang[:i]]); v != "" {
				return v
			}
		}
	}
	if sp.Names != nil {
		if v := strings.TrimSpace(sp.Names["en"]); v != "" {
			return v
		}
	}
	return sp.Name
}

// CP computes PoGo combat power at level with uniform IVs (atk=def=sta=iv).
func CP(sp *Species, level int, iv int) int {
	if sp == nil {
		return 0
	}
	cpm, ok := cpmByLevel[level]
	if !ok {
		return 0
	}
	atk := float64(sp.BaseAttack + iv)
	def := float64(sp.BaseDefense + iv)
	sta := float64(sp.BaseStamina + iv)
	raw := atk * math.Sqrt(def) * math.Sqrt(sta) * cpm * cpm / 10
	out := int(math.Floor(raw))
	if out < 10 {
		return 10
	}
	return out
}

func formatCP(cp int, lang string) string {
	return fmt.Sprintf("%d %s", cp, CombatUnit(lang))
}

// MatchEnglish looks up a single title segment against the catalog.
func MatchEnglish(segment string) *Species {
	load()
	if loadErr != nil {
		return nil
	}
	n := normalizeName(segment)
	if n == "" {
		return nil
	}
	if sp := lookupNorm(n); sp != nil {
		return sp
	}
	if stripped := stripFormeWords(n); stripped != n {
		if sp := lookupNorm(stripped); sp != nil {
			return sp
		}
	}
	// Fallback: strip Gigantamax / GMAX prefix (not every species has a GMAX entry).
	for _, prefix := range []string{"gigantamax ", "gmax ", "g-max "} {
		if strings.HasPrefix(n, prefix) {
			rest := strings.TrimSpace(n[len(prefix):])
			if sp := lookupNorm(rest); sp != nil {
				return sp
			}
			if stripped := stripFormeWords(rest); stripped != rest {
				if sp := lookupNorm(stripped); sp != nil {
					return sp
				}
			}
		}
	}
	return nil
}

func lookupNorm(n string) *Species {
	if n == "" {
		return nil
	}
	return byNorm[n]
}

// CategoryPatterns maps category → phrases to strip from titles (longest first preferred by caller).
func CategoryPatterns(category string, all map[string][]string) []string {
	patterns := append([]string{}, all[category]...)
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i]) > len(patterns[j])
	})
	return patterns
}

// ExtractSpecies returns matched species from a live event title for an extractable category.
func ExtractSpecies(liveEventName, category string, categoryPatterns map[string][]string) []*Species {
	load()
	if loadErr != nil {
		return nil
	}
	category = strings.TrimSpace(category)
	if _, ok := ExtractableCategories[category]; !ok {
		return nil
	}
	title := strings.TrimSpace(liveEventName)
	if title == "" {
		return nil
	}

	remainder := title
	lower := strings.ToLower(remainder)
	for _, phrase := range CategoryPatterns(category, categoryPatterns) {
		p := strings.ToLower(strings.TrimSpace(phrase))
		if p == "" {
			continue
		}
		if idx := strings.Index(lower, p); idx >= 0 {
			remainder = strings.TrimSpace(remainder[:idx] + remainder[idx+len(phrase):])
			lower = strings.ToLower(remainder)
			break // strip one category phrase
		}
	}

	remainder = strings.TrimSpace(remainder)
	remainder = strings.TrimLeft(remainder, ":—–-")
	remainder = strings.TrimSpace(remainder)
	if remainder == "" {
		return nil
	}

	parts := strings.Split(remainder, ", ")
	out := make([]*Species, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		sp := MatchEnglish(part)
		if sp == nil {
			continue
		}
		if _, ok := seen[sp.Slug]; ok {
			continue
		}
		seen[sp.Slug] = struct{}{}
		out = append(out, sp)
	}
	return out
}

// ResolveEventPokemon builds eventPokemon* placeholder values (incl. [1..N] indexes).
func ResolveEventPokemon(liveEventName, category, language string, categoryPatterns map[string][]string) map[string]string {
	out := make(map[string]string, 32)
	for _, k := range EventPokemonBaseKeys {
		out[k] = ""
	}

	species := ExtractSpecies(liveEventName, category, categoryPatterns)
	if len(species) == 0 {
		return out
	}

	lang := language
	names := make([]string, len(species))
	ceilingResearch := make([]string, len(species))
	ceilingRaid := make([]string, len(species))
	ceilingEgg := make([]string, len(species))
	ceilingRaidWeather := make([]string, len(species))
	floorResearch := make([]string, len(species))
	floorRaid := make([]string, len(species))
	floorEgg := make([]string, len(species))
	floorRaidWeather := make([]string, len(species))

	for i, sp := range species {
		names[i] = LocalizedName(sp, lang)
		ceilingResearch[i] = formatCP(CP(sp, 15, 15), lang)
		ceilingRaid[i] = formatCP(CP(sp, 20, 15), lang)
		ceilingEgg[i] = formatCP(CP(sp, 20, 15), lang)
		ceilingRaidWeather[i] = formatCP(CP(sp, 25, 15), lang)
		floorResearch[i] = formatCP(CP(sp, 15, 10), lang)
		floorRaid[i] = formatCP(CP(sp, 20, 10), lang)
		floorEgg[i] = formatCP(CP(sp, 20, 10), lang)
		floorRaidWeather[i] = formatCP(CP(sp, 25, 10), lang)

		idx := i + 1
		out[fmt.Sprintf("eventPokemon[%d]", idx)] = names[i]
		out[fmt.Sprintf("eventPokemonCeilingResearch[%d]", idx)] = ceilingResearch[i]
		out[fmt.Sprintf("eventPokemonCeilingRaid[%d]", idx)] = ceilingRaid[i]
		out[fmt.Sprintf("eventPokemonCeilingEgg[%d]", idx)] = ceilingEgg[i]
		out[fmt.Sprintf("eventPokemonCeilingRaidWeather[%d]", idx)] = ceilingRaidWeather[i]
		out[fmt.Sprintf("eventPokemonFloorResearch[%d]", idx)] = floorResearch[i]
		out[fmt.Sprintf("eventPokemonFloorRaid[%d]", idx)] = floorRaid[i]
		out[fmt.Sprintf("eventPokemonFloorEgg[%d]", idx)] = floorEgg[i]
		out[fmt.Sprintf("eventPokemonFloorRaidWeather[%d]", idx)] = floorRaidWeather[i]
	}

	join := func(parts []string) string { return strings.Join(parts, ", ") }
	out["eventPokemon"] = join(names)
	out["eventPokemonCeilingResearch"] = join(ceilingResearch)
	out["eventPokemonCeilingRaid"] = join(ceilingRaid)
	out["eventPokemonCeilingEgg"] = join(ceilingEgg)
	out["eventPokemonCeilingRaidWeather"] = join(ceilingRaidWeather)
	out["eventPokemonFloorResearch"] = join(floorResearch)
	out["eventPokemonFloorRaid"] = join(floorRaid)
	out["eventPokemonFloorEgg"] = join(floorEgg)
	out["eventPokemonFloorRaidWeather"] = join(floorRaidWeather)
	return out
}

// IsEventPokemonBase reports whether key (without index) is an event-pokemon builtin base.
func IsEventPokemonBase(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, b := range EventPokemonBaseKeys {
		if strings.ToLower(b) == k {
			return true
		}
	}
	return false
}

// IsEventPokemonNameKey is true for eventPokemon / eventPokemon[i] (editable name builtins).
func IsEventPokemonNameKey(key string) bool {
	base, _, _ := SplitIndex(key)
	return strings.EqualFold(strings.TrimSpace(base), "eventPokemon")
}

// SplitIndex splits "eventPokemon[2]" → ("eventPokemon", 2, true).
func SplitIndex(key string) (base string, index int, hasIndex bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", 0, false
	}
	lb := strings.LastIndexByte(key, '[')
	if lb < 0 || !strings.HasSuffix(key, "]") {
		return key, 0, false
	}
	inner := key[lb+1 : len(key)-1]
	if inner == "" {
		return key, 0, false
	}
	for _, r := range inner {
		if r < '0' || r > '9' {
			return key, 0, false
		}
	}
	n := 0
	for _, r := range inner {
		n = n*10 + int(r-'0')
	}
	return key[:lb], n, true
}
