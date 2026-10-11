package eventcategory

import "testing"

func TestFromNameSuperMegaAndShadow(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Falinks Super Mega Raid Day", "Super Mega Raid Day"},
		{"Raichu Super Mega Raid Day", "Super Mega Raid Day"},
		{"Super Mega Raid Day", "Super Mega Raid Day"},
		{"Super Mega Raids", "Super Mega Raid Day"},
		{"Ultra Unlock: Starmie Super Mega Raid Day", "Super Mega Raid Day"},
		{"Staraptor Super Mega Raid Day", "Super Mega Raid Day"},
		{"Camerupt Mega Raid Day", "Raid Day"},
		{"Mega Gallade Mega Raid Day", "Raid Day"},
		{"Rayquaza Mega Raid Day", "Raid Day"},
		{"Falinks Raid Day", "Raid Day"},
		{"Shadow Raid Day", "Shadow Raid Day"},
		{"Entei Shadow Raid Day", "Shadow Raid Day"},
		{"Kyogre Shadow Raid Day 1", "Shadow Raid Day"},
		{"Shadow Groudon Raid Day 1", "Shadow Raid Day"},
		{"Shadow Groudon Raid Day 2", "Shadow Raid Day"},
		{"Shadow Lugia Weekend (Day 1)", "Shadow Raid Day"},
		{"Shadow Lugia, Shadow Ho-Oh Raid Hour", "Shadow Raid Day"},
		{"Articuno Raid Hour", "Raid Hour"},
		{"Groudon Raid Hour", "Raid Hour"},
	}
	for _, c := range cases {
		if got := FromName(c.name); got != c.want {
			t.Errorf("FromName(%q)=%q want %q", c.name, got, c.want)
		}
	}
}

func TestExtractPatternsStripShadowTitle(t *testing.T) {
	phrases := ExtractPatterns("Shadow Raid Day")
	if len(phrases) == 0 {
		t.Fatal("no extract patterns")
	}
	if phrases[0] != "Shadow Raid Day" && phrases[0] != "Shadow Raid Hour" {
		t.Fatalf("expected a long Shadow Raid* phrase first, got %q", phrases[0])
	}
}

func TestClientCatalogIncludesSuperMegaAndShadow(t *testing.T) {
	cat := ClientCatalog()
	if len(cat) != len(Ordered) {
		t.Fatalf("len=%d want %d", len(cat), len(Ordered))
	}
	names := map[string]CatalogEntry{}
	for _, e := range cat {
		names[e.Name] = e
	}
	for _, want := range []string{"Super Mega Raid Day", "Shadow Raid Day", "Raid Day"} {
		if _, ok := names[want]; !ok {
			t.Fatalf("missing %q in catalog", want)
		}
	}
	if got := names["Shadow Raid Day"].Fallbacks; len(got) != 1 || got[0] != "Raid Day" {
		t.Fatalf("Shadow Raid Day fallbacks=%v want [Raid Day]", got)
	}
	if got := names["Super Mega Raid Day"].Fallbacks; len(got) != 1 || got[0] != "Raid Day" {
		t.Fatalf("Super Mega Raid Day fallbacks=%v want [Raid Day]", got)
	}
	if names["Raid Hour"].FixedLocalHour != true {
		t.Fatal("Raid Hour should be fixedLocalHour")
	}
}

func TestFromNamePatternExtras(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Elite Raids", "Raid Day"},
		{"Mega Ascension - Friday", "Raid Day"},
		{"Max Finale Leadup - Day 1", "Max Battle Day"},
		{"Pokémon GO Fest 2025: Max Finale Day 1", "GO Fest"}, // GO Fest wins over Max Finale
		{"Oricorio Catch Mastery", "Catch Mastery"},
		{"Phantump Catch Mastery", "Catch Mastery"},
		{"Aspiring Dragons Research Day", "Research Day"},
		{"Teddiursa, Combee SL Hour", "Spotlight Hour"},
		{"Teddiursa, Combee SL Hour + Nihilego Raid Hour", "Raid Hour"},
		{"Groudon Raid hour", "Raid Hour"}, // case
		{"Kyogre Raid hour", "Raid Hour"},
	}
	for _, c := range cases {
		if got := FromName(c.name); got != c.want {
			t.Errorf("FromName(%q)=%q want %q", c.name, got, c.want)
		}
	}
}
