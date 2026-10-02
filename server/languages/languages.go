package languages

import "strings"

// TemplateLanguages are ISO 639-1 codes accepted when publishing templates.
var TemplateLanguages = map[string]string{
	"af": "Afrikaans",
	"am": "Amharic",
	"ar": "Arabic",
	"az": "Azerbaijani",
	"be": "Belarusian",
	"bg": "Bulgarian",
	"bn": "Bengali",
	"bs": "Bosnian",
	"ca": "Catalan",
	"cs": "Czech",
	"cy": "Welsh",
	"da": "Danish",
	"de": "German",
	"el": "Greek",
	"en": "English",
	"eo": "Esperanto",
	"es": "Spanish",
	"et": "Estonian",
	"eu": "Basque",
	"fa": "Persian",
	"fi": "Finnish",
	"fo": "Faroese",
	"fr": "French",
	"ga": "Irish",
	"gl": "Galician",
	"gu": "Gujarati",
	"he": "Hebrew",
	"hi": "Hindi",
	"hr": "Croatian",
	"hu": "Hungarian",
	"hy": "Armenian",
	"id": "Indonesian",
	"is": "Icelandic",
	"it": "Italian",
	"ja": "Japanese",
	"ka": "Georgian",
	"kk": "Kazakh",
	"km": "Khmer",
	"kn": "Kannada",
	"ko": "Korean",
	"ku": "Kurdish",
	"lb": "Luxembourgish",
	"lo": "Lao",
	"lt": "Lithuanian",
	"lv": "Latvian",
	"mk": "Macedonian",
	"ml": "Malayalam",
	"mn": "Mongolian",
	"mr": "Marathi",
	"ms": "Malay",
	"mt": "Maltese",
	"my": "Burmese",
	"nb": "Norwegian",
	"ne": "Nepali",
	"nl": "Dutch",
	"nn": "Norwegian Nynorsk",
	"pa": "Punjabi",
	"pl": "Polish",
	"pt": "Portuguese",
	"ro": "Romanian",
	"ru": "Russian",
	"si": "Sinhala",
	"sk": "Slovak",
	"sl": "Slovenian",
	"sq": "Albanian",
	"sr": "Serbian",
	"sv": "Swedish",
	"sw": "Swahili",
	"ta": "Tamil",
	"te": "Telugu",
	"th": "Thai",
	"tl": "Tagalog",
	"tr": "Turkish",
	"uk": "Ukrainian",
	"ur": "Urdu",
	"uz": "Uzbek",
	"vi": "Vietnamese",
	"yi": "Yiddish",
	"zh": "Chinese",
	"zu": "Zulu",
}

// aliases maps legacy or macrolanguage codes onto an allowed TemplateLanguages key.
var aliases = map[string]string{
	"no": "nb", // Norwegian macrolanguage → Bokmål
	"iw": "he", // legacy Hebrew
	"in": "id", // legacy Indonesian
	"ji": "yi", // legacy Yiddish
	"fil": "tl", // Filipino → Tagalog
}

// Normalize returns a lowercased code if it is allowed, otherwise "".
func Normalize(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == "" {
		return ""
	}
	if mapped, ok := aliases[c]; ok {
		c = mapped
	}
	if _, ok := TemplateLanguages[c]; ok {
		return c
	}
	return ""
}

// Valid reports whether code is an allowed template language.
func Valid(code string) bool {
	return Normalize(code) != ""
}
