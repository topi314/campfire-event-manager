package eventcategory

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Historic titles from production Campfire live events (one per line).
// Used to audit category matching coverage.
func loadHistoricNames(t *testing.T) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", "historic_live_events.txt")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open historic list: %v", err)
	}
	defer f.Close()
	var names []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		names = append(names, line)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return names
}

func TestHistoricCategoryCoverage(t *testing.T) {
	names := loadHistoricNames(t)
	byCat := map[string][]string{}
	for _, name := range names {
		cat := FromName(name)
		byCat[cat] = append(byCat[cat], name)
	}

	// Titles that should stay Other (one-offs / non-template events).
	allowedOther := map[string]struct{}{
		"?????": {},
		"Carnival of Flamigo: Cologne, Germany":          {},
		"City Experience: Paris":                         {},
		"City Safari: Amsterdam":                         {},
		"City Safari, Amsterdam: Day 2":                  {},
		"City Safari: Bangkok, Day 1":                    {},
		"City Safari: Bangkok, Day 2":                    {},
		"City Safari: Sydney, Day 1":                     {},
		"City Safari: Sydney, Day 2":                     {},
		"City Safari: Valencia, Day 1":                   {},
		"City Safari: Valencia, Day 2":                   {},
		"City Safari: Vancouver, Day 1":                  {},
		"City Safari: Vancouver, Day 2":                  {},
		"Community Ambassador Event":                     {},
		"GO Battle Weekend: Tales of Transformation Day 1": {},
		"GO Battle Weekend: Tales of Transformation Day 2": {},
		"Play! Pokémon": {},
		"Pokémon 30th Anniversary: All Out - Day 1": {},
		"Pokémon 30th Anniversary: All Out - Day 2": {},
		"Pokémon Go x Arizona Cardinals, 12/21":     {},
		"Road Trip: Berlin, Germany":                {},
		"Road Trip: Den Haag, Netherlands":          {},
		"Road Trip: Gamescom":                       {},
		"Road Trip: Manchester":                     {},
		"Road Trip: Paris, France":                  {},
		"Road Trip: Valencia, Spain":                {},
		"World Championship Series Celebrations":    {},
		"Worlds Watch Party - Friday":               {},
		"Worlds Watch Party - Saturday":             {},
		"Worlds Watch Party - Sunday":               {},
	}

	for _, name := range byCat[Other] {
		if _, ok := allowedOther[name]; !ok {
			t.Errorf("unexpected Other: %q", name)
		}
	}

	if t.Failed() || testing.Verbose() {
		t.Logf("total=%d Other=%d", len(names), len(byCat[Other]))
		for _, n := range byCat[Other] {
			t.Log("Other:", n)
		}
	}
}
