package languages

import "strings"

// TemplateLanguages are ISO 639-1 codes accepted when publishing templates.
var TemplateLanguages = map[string]string{
	"en": "English",
	"de": "German",
	"fr": "French",
	"es": "Spanish",
	"nl": "Dutch",
	"it": "Italian",
	"pt": "Portuguese",
	"pl": "Polish",
	"cs": "Czech",
	"sk": "Slovak",
	"hu": "Hungarian",
	"ro": "Romanian",
	"sv": "Swedish",
	"da": "Danish",
	"nb": "Norwegian",
	"fi": "Finnish",
	"tr": "Turkish",
	"uk": "Ukrainian",
	"ru": "Russian",
	"ja": "Japanese",
	"ko": "Korean",
	"zh": "Chinese",
}

// Normalize returns a lowercased code if it is allowed, otherwise "".
func Normalize(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if _, ok := TemplateLanguages[c]; ok {
		return c
	}
	return ""
}

// Valid reports whether code is an allowed template language.
func Valid(code string) bool {
	return Normalize(code) != ""
}
