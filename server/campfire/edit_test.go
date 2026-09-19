package campfire

import "testing"

func TestParseLocationString(t *testing.T) {
	lat, lng, ok := ParseLocationString("[8.6821, 50.1109]")
	if !ok {
		t.Fatal("expected ok")
	}
	if lat != 50.1109 || lng != 8.6821 {
		t.Fatalf("got lat=%v lng=%v", lat, lng)
	}

	lat, lng, ok = ParseLocationString("[ -0.1276 , 51.5074 ]")
	if !ok || lat != 51.5074 || lng != -0.1276 {
		t.Fatalf("got ok=%v lat=%v lng=%v", ok, lat, lng)
	}

	if _, _, ok := ParseLocationString(""); ok {
		t.Fatal("empty should fail")
	}
	if _, _, ok := ParseLocationString("not-a-point"); ok {
		t.Fatal("invalid should fail")
	}
}

func TestFormatEditLocation(t *testing.T) {
	got := FormatEditLocation(50.1109, 8.6821)
	want := "[8.6821, 50.1109]"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeCommentsPermissions(t *testing.T) {
	cases := map[string]string{
		"ORGANIZERS_ONLY": "ORGANIZERS_ONLY",
		"HOST_ONLY":       "ORGANIZERS_ONLY",
		"ALL_INVITED":     "ALL_INVITED",
		"ATTENDEES":       "ALL_INVITED",
		"EVERYONE":        "ALL_INVITED",
		"anyone":          "ALL_INVITED",
		"NO_ONE":          "NO_ONE",
		"DISABLED":        "NO_ONE",
		"nobody":          "NO_ONE",
		"BOGUS":           "",
		"":                "",
	}
	for in, want := range cases {
		if got := NormalizeCommentsPermissions(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}
