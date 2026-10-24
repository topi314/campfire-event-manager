package server

import "strings"

// iconMatchRank scores an icon against a live event.
// 3 = category and a listed species, 2 = species with no category, 1 = category with no species.
// Icons that name a different category or a species list that misses the event are excluded.
func iconMatchRank(eventCategory, eventName string, iconCategory string, iconSlugs []string, eventSlugs []string) (int, bool) {
	eventCategory = strings.TrimSpace(eventCategory)
	iconCategory = strings.TrimSpace(iconCategory)
	hasSpecies := len(iconSlugs) > 0
	hasCategory := iconCategory != ""

	if hasCategory && !strings.EqualFold(iconCategory, eventCategory) {
		return 0, false
	}
	speciesHit := false
	if hasSpecies {
		if len(eventSlugs) == 0 {
			return 0, false
		}
		set := map[string]struct{}{}
		for _, slug := range eventSlugs {
			slug = strings.TrimSpace(strings.ToLower(slug))
			if slug != "" {
				set[slug] = struct{}{}
			}
		}
		for _, slug := range iconSlugs {
			slug = strings.TrimSpace(strings.ToLower(slug))
			if _, ok := set[slug]; ok {
				speciesHit = true
				break
			}
		}
		if !speciesHit {
			return 0, false
		}
	}

	_ = eventName
	switch {
	case hasCategory && speciesHit && eventCategory != "":
		return 3, true
	case speciesHit && !hasCategory:
		return 2, true
	case hasCategory && !hasSpecies && eventCategory != "":
		return 1, true
	default:
		return 0, false
	}
}
