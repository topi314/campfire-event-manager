package server

import "testing"

func TestIconMatchRank(t *testing.T) {
	cases := []struct {
		name     string
		category string
		iconCat  string
		icon     []string
		event    []string
		rank     int
		ok       bool
	}{
		{"both", "Community Day", "Community Day", []string{"bulbasaur", "ivysaur"}, []string{"ivysaur"}, 3, true},
		{"species only", "Community Day", "", []string{"bulbasaur"}, []string{"bulbasaur"}, 2, true},
		{"category only", "Raid Hour", "Raid Hour", nil, []string{"mewtwo"}, 1, true},
		{"wrong category", "Raid Hour", "Community Day", []string{"mewtwo"}, []string{"mewtwo"}, 0, false},
		{"wrong species", "Raid Hour", "Raid Hour", []string{"mew"}, []string{"mewtwo"}, 0, false},
		{"untagged", "Raid Hour", "", nil, []string{"mewtwo"}, 0, false},
		{"species icon without event species", "Raid Hour", "", []string{"mew"}, nil, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rank, ok := iconMatchRank(tc.category, "", tc.iconCat, tc.icon, tc.event)
			if rank != tc.rank || ok != tc.ok {
				t.Fatalf("rank=%d ok=%v want %d %v", rank, ok, tc.rank, tc.ok)
			}
		})
	}
}
