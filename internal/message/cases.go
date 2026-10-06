package message

// The cases subject-case and type-case judge, as @commitlint/ensure judges
// them through es-toolkit's case functions: the text with its quoted parts
// left out is in a case when the case function leaves it as it is. Upper and
// lower case are JavaScript's (toUpperCase, toLowerCase), whose few mappings
// to more than one character Go's unicode lacks are written out, and the
// first character a function changes is JavaScript's first UTF-16 code unit,
// which an astral character's surrogate leaves unchanged. es-toolkit's
// deburr decomposes a letter (NFD) before taking its marks off; with no
// normalization tables in Go's standard library, the letters of Latin-1 and
// Latin Extended-A are decomposed by table, so a subject with another
// precomposed letter may be named in a case commitlint does not name. The
// verdict never differs: every case subject-case refuses is sentence-case
// too, which needs no table.

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/donvargax/itos/v6/internal/value"
)

// refusedCases are the cases config-conventional's subject-case refuses, in
// the order it lists them.
var refusedCases = []string{"sentence-case", "start-case", "pascal-case", "upper-case"}

// quoted are the parts of a text ensureCase leaves out: they may hold proper
// names.
var quoted = regexp.MustCompile("`" + line + "*?`|\"" + line + "*?\"|'" + line + "*?'")

// subjectCases are the refused cases a subject is in, when it starts with a
// letter that has case; none otherwise.
func subjectCases(subject string) []string {
	first, _ := utf8.DecodeRuneInString(subject)
	// [\p{Ll}\p{Lu}\p{Lt}] under /iu, which also takes U+0345: it folds to ι.
	if subject == "" || !unicode.In(first, unicode.Ll, unicode.Lu, unicode.Lt) && first != 0x345 {
		return nil
	}
	var in []string
	for _, c := range refusedCases {
		if inCase(subject, c) {
			in = append(in, c)
		}
	}
	return in
}

// inCase is ensureCase: whether a text, its quoted parts out and trimmed, is
// in the case; a text the case function empties or starts with a digit is.
func inCase(raw, target string) bool {
	input := value.Trim(quoted.ReplaceAllString(raw, ""))
	var transformed string
	switch target {
	case "sentence-case":
		transformed = upperFirst(input)
	case "start-case":
		transformed = startCase(input)
	case "pascal-case":
		transformed = upperFirst(camelCase(input))
	case "upper-case":
		transformed = jsUpper(input)
	case "lower-case":
		transformed = jsLower(input)
	}
	if transformed == "" || transformed[0] >= '0' && transformed[0] <= '9' {
		return true
	}
	return transformed == input
}

// upperSpecial are the characters JavaScript's toUpperCase maps to more than
// one (SpecialCasing.txt's unconditional mappings), which Go's ToUpper
// leaves or maps to one.
var upperSpecial = map[rune]string{
	'ß': "SS", 'ŉ': "ʼN", 'ǰ': "J̌", 'ΐ': "Ϊ́", 'ΰ': "Ϋ́", 'և': "ԵՒ",
	'ẖ': "H̱", 'ẗ': "T̈", 'ẘ': "W̊", 'ẙ': "Y̊", 'ẚ': "Aʾ",
	'ﬀ': "FF", 'ﬁ': "FI", 'ﬂ': "FL", 'ﬃ': "FFI", 'ﬄ': "FFL", 'ﬅ': "ST", 'ﬆ': "ST",
	'ﬓ': "ՄՆ", 'ﬔ': "ՄԵ", 'ﬕ': "ՄԻ", 'ﬖ': "ՎՆ", 'ﬗ': "ՄԽ",
}

// jsUpper is toUpperCase.
func jsUpper(text string) string {
	var b strings.Builder
	for _, r := range text {
		if s, ok := upperSpecial[r]; ok {
			b.WriteString(s)
		} else {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

// jsLower is toLowerCase: İ is i and a combining dot.
func jsLower(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r == 'İ' {
			b.WriteString("i̇")
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// firstUnit is a text's first UTF-16 code unit through change, and the
// rest: an astral first character is a surrogate, which no case changes.
func firstUnit(text string, change func(string) string) (string, string) {
	r, size := utf8.DecodeRuneInString(text)
	if r > 0xffff {
		return text[:size], text[size:]
	}
	return change(text[:size]), text[size:]
}

// upperFirst is es-toolkit's upperFirst.
func upperFirst(text string) string {
	if text == "" {
		return ""
	}
	first, rest := firstUnit(text, jsUpper)
	return first + rest
}

// capitalize is es-toolkit's capitalize: the first unit upper, the rest
// lower.
func capitalize(word string) string {
	first, rest := firstUnit(word, jsUpper)
	return first + jsLower(rest)
}

// startCase is es-toolkit's startCase: each word with an upper first letter
// and the rest lower, unless it is all upper, joined by spaces.
func startCase(text string) string {
	words := words(value.Trim(forCase(text)))
	for i, w := range words {
		if w != jsUpper(w) {
			words[i] = capitalize(w)
		}
	}
	return strings.Join(words, " ")
}

// camelCase is es-toolkit's camelCase.
func camelCase(text string) string {
	words := words(forCase(text))
	if len(words) == 0 {
		return ""
	}
	out := jsLower(words[0])
	for _, w := range words[1:] {
		out += capitalize(w)
	}
	return out
}

// forCase is the text deburred and without its apostrophes, as es-toolkit
// reads it before it splits it into words.
func forCase(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch {
		case r == '\'' || r == '’':
		case r >= 0x300 && r <= 0x36f, r >= 0x20d0 && r <= 0x20ff, r >= 0xfe20 && r <= 0xfe2f:
		default:
			if s, ok := deburred[r]; ok {
				b.WriteString(s)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// deburred are the letters deburr changes: es-toolkit's own map, and the
// Latin-1 and Latin Extended-A letters NFD decomposes, without their marks.
var deburred = func() map[rune]string {
	m := map[rune]string{
		'Æ': "Ae", 'Ð': "D", 'Ø': "O", 'Þ': "Th", 'ß': "ss", 'æ': "ae", 'ð': "d", 'ø': "o",
		'þ': "th", 'Đ': "D", 'đ': "d", 'Ħ': "H", 'ħ': "h", 'ı': "i", 'Ĳ': "IJ", 'ĳ': "ij",
		'ĸ': "k", 'Ŀ': "L", 'ŀ': "l", 'Ł': "L", 'ł': "l", 'ŉ': "'n", 'Ŋ': "N", 'ŋ': "n",
		'Œ': "Oe", 'œ': "oe", 'Ŧ': "T", 'ŧ': "t", 'ſ': "s",
	}
	for base, letters := range map[string]string{
		"A": "ÀÁÂÃÄÅĀĂĄ", "a": "àáâãäåāăą", "C": "ÇĆĈĊČ", "c": "çćĉċč", "D": "Ď", "d": "ď",
		"E": "ÈÉÊËĒĔĖĘĚ", "e": "èéêëēĕėęě", "G": "ĜĞĠĢ", "g": "ĝğġģ", "H": "Ĥ", "h": "ĥ",
		"I": "ÌÍÎÏĨĪĬĮİ", "i": "ìíîïĩīĭį", "J": "Ĵ", "j": "ĵ", "K": "Ķ", "k": "ķ",
		"L": "ĹĻĽ", "l": "ĺļľ", "N": "ÑŃŅŇ", "n": "ñńņň", "O": "ÒÓÔÕÖŌŎŐ", "o": "òóôõöōŏő",
		"R": "ŔŖŘ", "r": "ŕŗř", "S": "ŚŜŞŠ", "s": "śŝşš", "T": "ŢŤ", "t": "ţť",
		"U": "ÙÚÛÜŨŪŬŮŰŲ", "u": "ùúûüũūŭůűų", "W": "Ŵ", "w": "ŵ", "Y": "ÝŶŸ", "y": "ýÿŷ",
		"Z": "ŹŻŽ", "z": "źżž",
	} {
		for _, r := range letters {
			m[r] = base
		}
	}
	return m
}()

// The character classes of es-toolkit's word pattern.
func isUpper(r rune) bool { return unicode.Is(unicode.Lu, r) }
func isLower(r rune) bool { return unicode.Is(unicode.Ll, r) }
func isMisc(r rune) bool  { return unicode.In(r, unicode.Lm, unicode.Lo) }
func isDigit(r rune) bool { return r >= '0' && r <= '9' }
func isWordChar(r rune) bool {
	return isDigit(r) || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// isBreak is [\p{Z}\p{P}] and the Latin-1 characters that are no letter or
// digit.
func isBreak(r rune) bool {
	return unicode.In(r, unicode.Z, unicode.P) || r <= 0x2f || r >= 0x3a && r <= 0x40 ||
		r >= 0x5b && r <= 0x60 || r >= 0x7b && r <= 0xbf || r == 0xd7 || r == 0xf7
}

// pictographs approximate \p{Extended_Pictographic} and
// \p{Emoji_Presentation}, which Go's unicode lacks: each is a word.
var pictographs = [][2]rune{
	{0xa9, 0xa9}, {0xae, 0xae}, {0x203c, 0x203c}, {0x2049, 0x2049}, {0x2122, 0x2122},
	{0x2139, 0x2139}, {0x2194, 0x2199}, {0x21a9, 0x21aa}, {0x231a, 0x231b}, {0x2328, 0x2328},
	{0x23cf, 0x23cf}, {0x23e9, 0x23f3}, {0x23f8, 0x23fa}, {0x24c2, 0x24c2}, {0x25aa, 0x25ab},
	{0x25b6, 0x25b6}, {0x25c0, 0x25c0}, {0x25fb, 0x25fe}, {0x2600, 0x27bf}, {0x2934, 0x2935},
	{0x2b05, 0x2b07}, {0x2b1b, 0x2b1c}, {0x2b50, 0x2b50}, {0x2b55, 0x2b55}, {0x3030, 0x3030},
	{0x303d, 0x303d}, {0x3297, 0x3297}, {0x3299, 0x3299}, {0x1f000, 0x1faff}, {0x1fc00, 0x1fffd},
}

func isPictograph(r rune) bool {
	for _, span := range pictographs {
		if r >= span[0] && r <= span[1] {
			return true
		}
	}
	return false
}

// words is es-toolkit's words: its pattern's alternatives tried in order at
// each place, leftmost first, with the backtracking each needs.
func words(text string) []string {
	rs := []rune(text)
	var found []string
	for at := 0; at < len(rs); {
		if end := wordAt(rs, at); end > at {
			found = append(found, string(rs[at:end]))
			at = end
		} else {
			at++
		}
	}
	return found
}

// wordAt is where the word starting at the index ends, the index itself when
// none starts there.
func wordAt(rs []rune, at int) int {
	n := len(rs)
	is := func(i int, class func(rune) bool) bool { return i < n && class(rs[i]) }
	// A unit of (?:[\p{Lm}\p{Lo}]\p{M}*): where it ends, or the index.
	misc := func(i int) int {
		if !is(i, isMisc) {
			return i
		}
		i++
		for is(i, func(r rune) bool { return unicode.Is(unicode.M, r) }) {
			i++
		}
		return i
	}
	lowerOrMisc := func(i int) int {
		if is(i, isLower) {
			return i + 1
		}
		return misc(i)
	}
	// \p{Lu}?\p{Ll}+(?=break|\p{Lu}|$)
	for _, upper := range []int{1, 0} {
		if upper == 1 && !is(at, isUpper) {
			continue
		}
		start := at + upper
		end := start
		for is(end, isLower) {
			end++
		}
		for e := end; e > start; e-- {
			if e == n || isBreak(rs[e]) || isUpper(rs[e]) {
				return e
			}
		}
	}
	// (?:\p{Lu}|misc)+(?=break|\p{Lu}(?:\p{Ll}|misc)|$)
	var ends []int
	for i := at; ; {
		next := i
		if is(i, isUpper) {
			next = i + 1
		} else {
			next = misc(i)
		}
		if next == i {
			break
		}
		ends = append(ends, next)
		i = next
	}
	for k := len(ends) - 1; k >= 0; k-- {
		e := ends[k]
		if e == n || isBreak(rs[e]) || isUpper(rs[e]) && lowerOrMisc(e+1) > e+1 {
			return e
		}
	}
	// \p{Lu}?(?:\p{Ll}|misc)+
	for _, upper := range []int{1, 0} {
		if upper == 1 && !is(at, isUpper) {
			continue
		}
		end := at + upper
		for next := lowerOrMisc(end); next > end; next = lowerOrMisc(end) {
			end = next
		}
		if end > at+upper {
			return end
		}
	}
	// \p{Lu}+
	if is(at, isUpper) {
		end := at
		for is(end, isUpper) {
			end++
		}
		return end
	}
	// \d*(?:1ST|2ND|3RD|(?![123])\dTH)(?=\b|[a-z_]), then the same in lower
	// case before (?=\b|[A-Z_])
	digits := at
	for is(digits, isDigit) {
		digits++
	}
	boundary := func(i int) bool {
		return (i > 0 && isWordChar(rs[i-1])) != (i < n && isWordChar(rs[i]))
	}
	for _, ordinal := range []struct {
		suffixes [4]string
		after    func(rune) bool
	}{
		{[4]string{"1ST", "2ND", "3RD", "TH"}, func(r rune) bool { return r >= 'a' && r <= 'z' || r == '_' }},
		{[4]string{"1st", "2nd", "3rd", "th"}, func(r rune) bool { return r >= 'A' && r <= 'Z' || r == '_' }},
	} {
		for d := digits; d >= at; d-- {
			end := -1
			for _, suffix := range ordinal.suffixes[:3] {
				if hasRunes(rs, d, suffix) {
					end = d + 3
				}
			}
			if is(d, isDigit) && !strings.ContainsRune("123", rs[d]) && hasRunes(rs, d+1, ordinal.suffixes[3]) {
				end = d + 3
			}
			if end >= 0 && (boundary(end) || is(end, ordinal.after)) {
				return end
			}
		}
	}
	// \d+
	if digits > at {
		return digits
	}
	if is(at, isPictograph) {
		return at + 1
	}
	return at
}

// hasRunes is whether the runes from the index spell the text.
func hasRunes(rs []rune, at int, text string) bool {
	for _, r := range text {
		if at >= len(rs) || rs[at] != r {
			return false
		}
		at++
	}
	return true
}
