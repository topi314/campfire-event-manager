package timeformat

// IDs match frontend/utils/timeFormats.ts.
var allowed = map[string]struct{}{
	"mdy-12":       {},
	"mdy-slash-12": {},
	"dmy-12":       {},
	"dmy-slash-12": {},
	"dmy-24":       {},
	"dmy-dot-24":   {},
	"dmy-slash-24": {},
	"dmy-dash-24":  {},
	"dmy-dot-time": {},
	"dmy-h":        {},
	"ymd-24":       {},
	"ymd-slash-24": {},
	"ymd-dot-24":   {},
	"hm-dmy":       {},
	"hm-dmy-slash": {},
}

func Valid(id string) bool {
	_, ok := allowed[id]
	return ok
}

// IDs returns every preset id.
func IDs() []string {
	out := make([]string, 0, len(allowed))
	for id := range allowed {
		out = append(out, id)
	}
	return out
}
