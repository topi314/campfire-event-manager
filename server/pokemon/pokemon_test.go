package pokemon

import (
	"strings"
	"testing"

	"github.com/topi314/campfire-event-manager/internal/eventcategory"
)

func TestSquirtleGermanName(t *testing.T) {
	sp := MatchEnglish("Squirtle")
	if sp == nil {
		t.Fatal("Squirtle not found")
	}
	if got := LocalizedName(sp, "de"); got != "Schiggy" {
		t.Fatalf("de=%q want Schiggy", got)
	}
}

func TestGogoatCPGoldens(t *testing.T) {
	sp := MatchEnglish("Gogoat")
	if sp == nil {
		t.Fatal("Gogoat not found")
	}
	cases := []struct {
		level, iv, want int
	}{
		{15, 15, 1199},
		{20, 15, 1598},
		{25, 15, 1998},
		{15, 10, 1142},
		{20, 10, 1522},
		{20, 11, 1537}, // super mega raid floor
		{25, 11, 1922}, // weather-boosted super mega raid floor
		{20, 6, 1463},  // shadow raid floor
		{25, 6, 1829},  // weather-boosted shadow raid floor
	}
	for _, c := range cases {
		if got := CP(sp, c.level, c.iv); got != c.want {
			t.Errorf("CP(L%d iv%d)=%d want %d", c.level, c.iv, got, c.want)
		}
	}
}

func TestExtractRaidHourMulti(t *testing.T) {
	got := ExtractSpecies("Raid Hour: Squirtle, Wartortle", "Raid Hour", eventcategory.All)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].Name != "Squirtle" || got[1].Name != "Wartortle" {
		t.Fatalf("names=%s, %s", got[0].Name, got[1].Name)
	}
}

func TestExtractSuperMegaAndShadow(t *testing.T) {
	got := ExtractSpecies("Falinks Super Mega Raid Day", "Super Mega Raid Day", eventcategory.All)
	if len(got) != 1 || got[0].Name != "Falinks" {
		t.Fatalf("super mega=%v", got)
	}
	got = ExtractSpecies("Shadow Groudon Raid Day 1", "Shadow Raid Day", eventcategory.All)
	if len(got) != 1 || got[0].Name != "Groudon" {
		t.Fatalf("shadow groudon=%v", got)
	}
	got = ExtractSpecies("Entei Shadow Raid Day", "Shadow Raid Day", eventcategory.All)
	if len(got) != 1 || got[0].Name != "Entei" {
		t.Fatalf("entei shadow=%v", got)
	}
	got = ExtractSpecies("Shadow Lugia, Shadow Ho-Oh Raid Hour", "Shadow Raid Day", eventcategory.All)
	if len(got) != 2 {
		t.Fatalf("shadow hour len=%d want 2", len(got))
	}
}

func TestExtractSkipGOFest(t *testing.T) {
	got := ExtractSpecies("GO Fest: Squirtle", "GO Fest", eventcategory.All)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}

func TestExtractMegaForm(t *testing.T) {
	got := ExtractSpecies("Raid Hour: Mega Charizard X", "Raid Hour", eventcategory.All)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	if got[0].Name != "Mega Charizard X" {
		t.Fatalf("name=%q", got[0].Name)
	}
}

func TestExtractGiratinaOriginForme(t *testing.T) {
	got := ExtractSpecies("Giratina (Origin Forme) Raid Hour", "Raid Hour", eventcategory.All)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	if got[0].Name != "Giratina (Origin Forme)" {
		t.Fatalf("name=%q", got[0].Name)
	}
	// Origin has higher attack than Altered (225 vs 187).
	if got[0].BaseAttack < 200 {
		t.Fatalf("expected Origin stats, atk=%d", got[0].BaseAttack)
	}
}

func TestExtractGiratinaBareIsAltered(t *testing.T) {
	got := ExtractSpecies("Raid Hour: Giratina", "Raid Hour", eventcategory.All)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	if got[0].Name != "Giratina (Altered Forme)" {
		t.Fatalf("name=%q want Altered", got[0].Name)
	}
}

func TestResolveIndexedAndUnitDE(t *testing.T) {
	m := ResolveEventPokemon("Raid Hour: Squirtle, Wartortle", "Raid Hour", "de", eventcategory.All)
	if m["eventPokemon"] != "Schiggy, Schillok" {
		t.Fatalf("eventPokemon=%q", m["eventPokemon"])
	}
	if m["eventPokemon[1]"] != "Schiggy" {
		t.Fatalf("[1]=%q", m["eventPokemon[1]"])
	}
	if m["eventPokemon[3]"] != "" {
		t.Fatalf("OOB should be empty, got %q", m["eventPokemon[3]"])
	}
	if got := m["eventPokemonCeilingRaid[1]"]; got == "" {
		t.Fatalf("raid[1] empty")
	}
	for _, r := range m["eventPokemonCeilingRaid[1]"] {
		if r < '0' || r > '9' {
			t.Fatalf("raid[1]=%q want digits only", m["eventPokemonCeilingRaid[1]"])
		}
	}
	if got := m["eventPokemonFloorSuperMegaRaid[1]"]; got == "" {
		t.Fatalf("super mega floor[1] empty")
	}
	if got := m["eventPokemonFloorSuperMegaRaidWeather[1]"]; got == "" {
		t.Fatalf("super mega weather floor[1] empty")
	}
	if got := m["eventPokemonFloorShadowRaid[1]"]; got == "" {
		t.Fatalf("shadow floor[1] empty")
	}
	if got := m["eventPokemonFloorShadowRaidWeather[1]"]; got == "" {
		t.Fatalf("shadow weather floor[1] empty")
	}
	if m["eventPokemonFloorSuperMegaRaid[1]"] == m["eventPokemonFloorRaid[1]"] {
		t.Fatalf("super mega floor should differ from raid floor (11 vs 10 IV)")
	}
	if m["eventPokemonFloorShadowRaid[1]"] == m["eventPokemonFloorRaid[1]"] {
		t.Fatalf("shadow floor should differ from raid floor (6 vs 10 IV)")
	}
}

func TestMatchLocalizedName(t *testing.T) {
	sp := MatchEnglish("Schiggy")
	if sp == nil || sp.Name != "Squirtle" {
		t.Fatalf("Schiggy → %+v", sp)
	}
}

func TestEnrichFromManualNames(t *testing.T) {
	// Title extraction fails (non-extractable category), manual name still fills CP.
	values := ResolveEventPokemon("GO Fest: Squirtle", "GO Fest", "en", eventcategory.All)
	if values["eventPokemonCeilingRaid"] != "" {
		t.Fatalf("expected empty auto resolve, got %q", values["eventPokemonCeilingRaid"])
	}
	overrides := map[string]string{"eventPokemon": "Squirtle"}
	values["eventPokemon"] = "Squirtle"
	EnrichFromNameOverrides(values, overrides, "en")
	if values["eventPokemon"] != "Squirtle" {
		t.Fatalf("name=%q", values["eventPokemon"])
	}
	if got := values["eventPokemonCeilingRaid"]; got == "" {
		t.Fatalf("ceiling raid empty")
	}
	for _, r := range values["eventPokemonCeilingRaid"] {
		if r < '0' || r > '9' {
			t.Fatalf("ceiling raid=%q want digits only", values["eventPokemonCeilingRaid"])
		}
	}
	if values["eventPokemon[1]"] != "Squirtle" {
		t.Fatalf("[1]=%q", values["eventPokemon[1]"])
	}
}

func TestEnrichFromIndexedGermanName(t *testing.T) {
	values := ResolveEventPokemon("", "Raid Hour", "de", eventcategory.All)
	overrides := map[string]string{"eventPokemon[1]": "Schiggy"}
	values["eventPokemon[1]"] = "Schiggy"
	EnrichFromNameOverrides(values, overrides, "de")
	if got := values["eventPokemonCeilingRaid[1]"]; got == "" {
		t.Fatalf("raid[1] empty")
	}
	for _, r := range values["eventPokemonCeilingRaid[1]"] {
		if r < '0' || r > '9' {
			t.Fatalf("raid[1]=%q want digits only", values["eventPokemonCeilingRaid[1]"])
		}
	}
	if values["eventPokemon[1]"] != "Schiggy" {
		t.Fatalf("kept manual name=%q", values["eventPokemon[1]"])
	}
	if values["eventPokemon[1]:en"] != "Squirtle" {
		t.Fatalf("pinned en=%q", values["eventPokemon[1]:en"])
	}
	if values["eventPokemon:en"] != "Squirtle" {
		t.Fatalf("pinned list en=%q", values["eventPokemon:en"])
	}
}

func TestResolveEventPokemonPinnedLanguage(t *testing.T) {
	m := ResolveEventPokemon("Raid Hour: Squirtle, Wartortle", "Raid Hour", "de", eventcategory.All)
	if !strings.Contains(m["eventPokemon"], "Schiggy") {
		t.Fatalf("meetup lang=%q", m["eventPokemon"])
	}
	if m["eventPokemon:en"] != "Squirtle, Wartortle" {
		t.Fatalf("en=%q", m["eventPokemon:en"])
	}
	if m["eventPokemon[2]:en"] != "Wartortle" {
		t.Fatalf("[2]:en=%q", m["eventPokemon[2]:en"])
	}
	if m["eventPokemon[1]:de"] == "" || m["eventPokemon[1]:de"] == "Squirtle" {
		t.Fatalf("[1]:de=%q", m["eventPokemon[1]:de"])
	}
	if _, ok := m["eventPokemonCeilingRaid:en"]; ok {
		t.Fatalf("ceiling should not be language-pinned")
	}
}

func TestUnmatchedNamePinsTypedString(t *testing.T) {
	values := map[string]string{"eventPokemon": "NotARealMon"}
	overrides := map[string]string{"eventPokemon": "NotARealMon"}
	EnrichFromNameOverrides(values, overrides, "de")
	if values["eventPokemon:en"] != "NotARealMon" {
		t.Fatalf("pinned en=%q", values["eventPokemon:en"])
	}
	if values["eventPokemon"] != "NotARealMon" {
		t.Fatalf("name=%q", values["eventPokemon"])
	}
}
