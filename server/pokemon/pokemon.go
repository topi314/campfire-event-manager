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
			for _, loc := range sp.Names {
				add(loc)
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

func formatCP(cp int) string {
	return fmt.Sprintf("%d", cp)
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
	species := ExtractSpecies(liveEventName, category, categoryPatterns)
	display := make([]string, len(species))
	for i, sp := range species {
		display[i] = LocalizedName(sp, language)
	}
	return buildEventPokemonValues(species, display, language)
}

// EnrichFromNameOverrides recomputes eventPokemon* CP placeholders from manually
// provided eventPokemon / eventPokemon[i] names. Name keys already present in values
// are left unchanged; derived indexes and all ceiling/floor keys are refreshed.
// No-op when nameOverrides contains no eventPokemon name keys with values.
func EnrichFromNameOverrides(values map[string]string, nameOverrides map[string]string, language string) {
	if values == nil {
		return
	}
	display := collectEventPokemonNames(nameOverrides)
	if len(display) == 0 {
		return
	}
	species := make([]*Species, len(display))
	for i, name := range display {
		species[i] = MatchEnglish(name)
	}
	built := buildEventPokemonValues(species, display, language)
	for k, v := range built {
		if IsEventPokemonNameKey(k) {
			if lookupEventPokemonName(nameOverrides, k) != "" {
				continue
			}
		}
		values[k] = v
	}
}

func lookupEventPokemonName(m map[string]string, key string) string {
	if m == nil {
		return ""
	}
	if v := strings.TrimSpace(m[key]); v != "" {
		return v
	}
	want := strings.ToLower(strings.TrimSpace(key))
	for k, v := range m {
		if strings.ToLower(strings.TrimSpace(k)) == want {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// collectEventPokemonNames returns ordered display names from eventPokemon overrides.
// Indexed keys win when any eventPokemon[i] is set; otherwise the unindexed list is split.
func collectEventPokemonNames(overrides map[string]string) []string {
	if overrides == nil {
		return nil
	}
	byIdx := map[int]string{}
	maxIdx := 0
	unindexed := ""
	for k, v := range overrides {
		if !IsEventPokemonNameKey(k) {
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		base, idx, has := SplitIndex(k)
		_ = base
		if has {
			if idx < 1 {
				continue
			}
			byIdx[idx] = v
			if idx > maxIdx {
				maxIdx = idx
			}
			continue
		}
		unindexed = v
	}
	if maxIdx > 0 {
		out := make([]string, 0, maxIdx)
		for i := 1; i <= maxIdx; i++ {
			out = append(out, byIdx[i]) // empty string if a gap
		}
		return out
	}
	if unindexed == "" {
		return nil
	}
	parts := strings.Split(unindexed, ", ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func buildEventPokemonValues(species []*Species, displayNames []string, language string) map[string]string {
	out := make(map[string]string, 32)
	for _, k := range EventPokemonBaseKeys {
		out[k] = ""
	}
	n := len(species)
	if len(displayNames) < n {
		n = len(displayNames)
	}
	if n == 0 {
		return out
	}

	lang := language
	names := make([]string, n)
	ceilingResearch := make([]string, n)
	ceilingRaid := make([]string, n)
	ceilingEgg := make([]string, n)
	ceilingRaidWeather := make([]string, n)
	floorResearch := make([]string, n)
	floorRaid := make([]string, n)
	floorEgg := make([]string, n)
	floorRaidWeather := make([]string, n)

	for i := 0; i < n; i++ {
		sp := species[i]
		name := strings.TrimSpace(displayNames[i])
		if name == "" && sp != nil {
			name = LocalizedName(sp, lang)
		}
		names[i] = name
		if sp != nil {
			ceilingResearch[i] = formatCP(CP(sp, 15, 15))
			ceilingRaid[i] = formatCP(CP(sp, 20, 15))
			ceilingEgg[i] = formatCP(CP(sp, 20, 15))
			ceilingRaidWeather[i] = formatCP(CP(sp, 25, 15))
			floorResearch[i] = formatCP(CP(sp, 15, 10))
			floorRaid[i] = formatCP(CP(sp, 20, 10))
			floorEgg[i] = formatCP(CP(sp, 20, 10))
			floorRaidWeather[i] = formatCP(CP(sp, 25, 10))
		}

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
