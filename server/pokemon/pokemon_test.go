package pokemon

import (
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
}
