package placeholders

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/topi314/campfire-event-manager/server/pokemon"
)

// Letter styles stored as Unicode mathematical alphanumeric characters.
// Keep this mapping aligned with frontend/utils/unicodeStyle.ts.
type letterStyle int

const (
	stylePlain letterStyle = iota
	styleBold
	styleItalic
	styleBoldItalic
	styleMono
)

const (
	markUnderline = '\u0332'
	markStrike    = '\u0336'
)

var styledKeyRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*(?:\[[0-9]+\])?(?::[a-zA-Z0-9]+(?:-[a-zA-Z0-9]+)*)?$`)

type textUnit struct {
	plain     rune
	style     letterStyle
	underline bool
	strike    bool
	extra     []rune
}

type styledToken struct {
	start     int
	end       int
	key       string
	style     letterStyle
	underline bool
	strike    bool
}

func isCombining(r rune) bool {
	return (r >= 0x0300 && r <= 0x036f) ||
		(r >= 0x1ab0 && r <= 0x1aff) ||
		(r >= 0x1dc0 && r <= 0x1dff) ||
		(r >= 0x20d0 && r <= 0x20ff) ||
		(r >= 0xfe20 && r <= 0xfe2f)
}

func foldRune(r rune) (plain rune, style letterStyle, ok bool) {
	switch {
	case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		return r, stylePlain, true
	case r >= 0x1d400 && r <= 0x1d419:
		return 'A' + (r - 0x1d400), styleBold, true
	case r >= 0x1d41a && r <= 0x1d433:
		return 'a' + (r - 0x1d41a), styleBold, true
	case r >= 0x1d7ce && r <= 0x1d7d7:
		return '0' + (r - 0x1d7ce), styleBold, true
	case r >= 0x1d434 && r <= 0x1d44d:
		return 'A' + (r - 0x1d434), styleItalic, true
	case r == 0x210e:
		return 'h', styleItalic, true
	case r >= 0x1d44e && r <= 0x1d454:
		return 'a' + (r - 0x1d44e), styleItalic, true
	case r >= 0x1d456 && r <= 0x1d467:
		return 'i' + (r - 0x1d456), styleItalic, true
	case r >= 0x1d468 && r <= 0x1d481:
		return 'A' + (r - 0x1d468), styleBoldItalic, true
	case r >= 0x1d482 && r <= 0x1d49b:
		return 'a' + (r - 0x1d482), styleBoldItalic, true
	case r >= 0x1d670 && r <= 0x1d689:
		return 'A' + (r - 0x1d670), styleMono, true
	case r >= 0x1d68a && r <= 0x1d6a3:
		return 'a' + (r - 0x1d68a), styleMono, true
	case r >= 0x1d7f6 && r <= 0x1d7ff:
		return '0' + (r - 0x1d7f6), styleMono, true
	default:
		return r, stylePlain, false
	}
}

func styleRune(plain rune, style letterStyle) rune {
	if style == stylePlain {
		return plain
	}
	switch {
	case plain >= '0' && plain <= '9':
		switch style {
		case styleBold, styleBoldItalic:
			return 0x1d7ce + (plain - '0')
		case styleMono:
			return 0x1d7f6 + (plain - '0')
		default:
			return plain
		}
	case plain >= 'A' && plain <= 'Z':
		offset := plain - 'A'
		switch style {
		case styleBold:
			return 0x1d400 + offset
		case styleItalic:
			return 0x1d434 + offset
		case styleBoldItalic:
			return 0x1d468 + offset
		case styleMono:
			return 0x1d670 + offset
		}
	case plain >= 'a' && plain <= 'z':
		offset := plain - 'a'
		switch style {
		case styleBold:
			return 0x1d41a + offset
		case styleItalic:
			if plain == 'h' {
				return 0x210e
			}
			if offset < 7 {
				return 0x1d44e + offset
			}
			return 0x1d456 + (offset - 8)
		case styleBoldItalic:
			return 0x1d482 + offset
		case styleMono:
			return 0x1d68a + offset
		}
	}
	return plain
}

func readCluster(text string, i int) (textUnit, int, bool) {
	if i >= len(text) {
		return textUnit{}, i, false
	}
	r, size := utf8.DecodeRuneInString(text[i:])
	if r == utf8.RuneError && size == 1 {
		return textUnit{}, i, false
	}
	if isCombining(r) {
		return textUnit{}, i, false
	}
	plain, style, folded := foldRune(r)
	unit := textUnit{plain: plain, style: stylePlain}
	if folded {
		unit.style = style
	}
	next := i + size
	for next < len(text) {
		mark, msize := utf8.DecodeRuneInString(text[next:])
		if !isCombining(mark) {
			break
		}
		switch mark {
		case markUnderline:
			unit.underline = true
		case markStrike:
			unit.strike = true
		default:
			unit.extra = append(unit.extra, mark)
		}
		next += msize
	}
	return unit, next, true
}

func isKeyRune(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
		r == '_' || r == ':' || r == '-' || r == '[' || r == ']'
}

func tokenFrom(text string, start int) (styledToken, bool) {
	first, next, ok := readCluster(text, start)
	if !ok || first.plain != '{' {
		return styledToken{}, false
	}
	second, next, ok := readCluster(text, next)
	if !ok || second.plain != '{' {
		return styledToken{}, false
	}
	var inner []textUnit
	i := next
	for i < len(text) {
		unit, n, ok := readCluster(text, i)
		if !ok || unit.plain == '\n' {
			return styledToken{}, false
		}
		if unit.plain == '}' {
			closeUnit, closeNext, ok := readCluster(text, n)
			if ok && closeUnit.plain == '}' {
				var b strings.Builder
				for _, part := range inner {
					b.WriteRune(part.plain)
				}
				key := strings.TrimSpace(b.String())
				if !styledKeyRE.MatchString(key) {
					return styledToken{}, false
				}
				tok := styledToken{start: start, end: closeNext, key: key}
				for _, part := range inner {
					if !isKeyRune(part.plain) {
						continue
					}
					if tok.style == stylePlain && part.style != stylePlain {
						tok.style = part.style
					}
					if part.underline {
						tok.underline = true
					}
					if part.strike {
						tok.strike = true
					}
				}
				return tok, true
			}
		}
		inner = append(inner, unit)
		i = n
	}
	return styledToken{}, false
}

func findStyledTokens(text string) []styledToken {
	var tokens []styledToken
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == '{' {
			if tok, ok := tokenFrom(text, i); ok {
				tokens = append(tokens, tok)
				i = tok.end
				continue
			}
		}
		if unit, next, ok := readCluster(text, i); ok {
			i = next
			_ = unit
			continue
		}
		i += size
	}
	return tokens
}

func applyLetterStyle(text string, style letterStyle, underline, strike bool) string {
	if text == "" || (style == stylePlain && !underline && !strike) {
		return text
	}
	var b strings.Builder
	for _, r := range text {
		out := r
		if style != stylePlain {
			if plain, _, ok := foldRune(r); ok {
				out = styleRune(plain, style)
			} else if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				out = styleRune(r, style)
			}
		}
		b.WriteRune(out)
		if underline {
			b.WriteRune(markUnderline)
		}
		if strike {
			b.WriteRune(markStrike)
		}
	}
	return b.String()
}

func lookupResolved(key string, values map[string]string) (string, bool) {
	if v, ok := Lookup(values, key); ok {
		return v, true
	}
	if v, ok := lookupPinnedLang(values, key); ok {
		return v, true
	}
	if v, ok := lookupUnscoped(values, key); ok {
		return v, true
	}
	if pokemon.IsEventPokemonBase(baseKey(key)) {
		return "", true
	}
	return "", false
}
