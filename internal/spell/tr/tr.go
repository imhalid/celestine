// Package tr is Turkish casing and letter helpers, as tinyfix-tr's src/turkish.ts has them (#623): Go's own
// ToLower and ToUpper get I/ı and İ/i wrong for Turkish. Offsets are in runes, which for Turkish text are the
// UTF-16 units the TypeScript engine counts.
package tr

import (
	"strings"
	"unicode"
)

// Letters are the letters of a Turkish word, as a regular expression's character class body.
const Letters = "A-Za-zÇĞİÖŞÜçğıöşüÂÎÛâîû"

// Alphabet is the Turkish alphabet, lower case.
const Alphabet = "abcçdefgğhıijklmnoöprsştuüvyz"

// Lower is text in Turkish lower case: I is ı, İ is i, and a typographic apostrophe is a plain one.
func Lower(text string) string {
	text = strings.ReplaceAll(text, "I", "ı")
	text = strings.ReplaceAll(text, "İ", "i")
	return strings.ReplaceAll(jsLower(text), "’", "'")
}

// jsLower is JavaScript's String.prototype.toLowerCase for the text Lower gives it.
func jsLower(text string) string {
	return strings.Map(func(r rune) rune {
		if r == 'İ' { // replaced before; as JavaScript, "i̇" would be two runes
			return 'i'
		}
		return unicode.ToLower(r)
	}, text)
}

// Upper is text in Turkish upper case: i is İ, ı is I.
func Upper(text string) string {
	text = strings.ReplaceAll(text, "i", "İ")
	text = strings.ReplaceAll(text, "ı", "I")
	return strings.Map(func(r rune) rune {
		if r == 'ß' { // JavaScript makes it "SS", which changes the length; kept as it is
			return r
		}
		return unicode.ToUpper(r)
	}, text)
}

// IsLetter reports whether r is a letter of a Turkish word.
func IsLetter(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		return true
	}
	return strings.ContainsRune("ÇĞİÖŞÜçğıöşüÂÎÛâîû", r)
}

// IsUpperInitial reports whether word starts with a capital letter.
func IsUpperInitial(word string) bool {
	if word == "" {
		return false
	}
	first := string([]rune(word)[0])
	return first != Lower(first)
}

// IsAllUpper reports whether word, longer than one letter, is in capitals.
func IsAllUpper(word string) bool {
	return len([]rune(word)) > 1 && word == Upper(word) && word != Lower(word)
}

// MatchCase gives replacement the typed word's casing: "Cok" becomes "Çok", "COK" becomes "ÇOK".
func MatchCase(typed, replacement string) string {
	if IsAllUpper(typed) {
		return Upper(replacement)
	}
	if IsUpperInitial(typed) {
		r := []rune(replacement)
		if len(r) == 0 {
			return replacement
		}
		return Upper(string(r[:1])) + string(r[1:])
	}
	return replacement
}

// toggle are the letters that lose their marks when typed on a keyboard without Turkish characters, and back.
var toggle = map[rune]rune{'c': 'ç', 'g': 'ğ', 'i': 'ı', 'o': 'ö', 's': 'ş', 'u': 'ü', 'ı': 'i', 'ç': 'c', 'ğ': 'g', 'ö': 'o', 'ş': 's', 'ü': 'u'}

// MarkVariants are the words that differ from a lowercase word only in Turkish marks: "cok" gives "çok", "cök",
// "çök". Only ASCII letters are toggled to Turkish ones, except a leading "ı", which is how a typed capital "I"
// lowercases ("Istanbul" lowercases to "ıstanbul" and should reach "istanbul"). At most limit-1 of them, in the order
// of the bit masks over the positions.
func MarkVariants(word string, limit int) []string {
	rs := []rune(word)
	var positions []int
	for i, r := range rs {
		if strings.ContainsRune("cgiosu", r) || r == 'ı' && i == 0 {
			positions = append(positions, i)
		}
	}
	count := limit
	if len(positions) < 31 && 1<<len(positions) < limit {
		count = 1 << len(positions)
	}
	out := make([]string, 0, max(0, count-1))
	chars := make([]rune, len(rs))
	for mask := 1; mask < count; mask++ {
		copy(chars, rs)
		for bit, pos := range positions {
			if bit < 31 && mask&(1<<bit) != 0 {
				chars[pos] = toggle[chars[pos]]
			}
		}
		out = append(out, string(chars))
	}
	return out
}

// HasTurkishMarks reports whether text has a Turkish letter with a mark.
func HasTurkishMarks(text string) bool { return strings.ContainsAny(text, "çğıöşüÇĞİÖŞÜ") }

// unmarked are the plain forms of the Turkish letters.
var unmarked = map[rune]rune{'ç': 'c', 'ğ': 'g', 'ı': 'i', 'ö': 'o', 'ş': 's', 'ü': 'u', 'Ç': 'C', 'Ğ': 'G', 'İ': 'I', 'Ö': 'O', 'Ş': 'S', 'Ü': 'U'}

// StripMarksIn replaces Turkish letters with their plain forms inside the [start, end) ranges of runes; the length
// stays the same.
func StripMarksIn(text string, ranges [][2]int) string {
	if len(ranges) == 0 {
		return text
	}
	rs := []rune(text)
	for _, g := range ranges {
		for i := max(0, g[0]); i < min(g[1], len(rs)); i++ {
			if u, ok := unmarked[rs[i]]; ok {
				rs[i] = u
			}
		}
	}
	return string(rs)
}

// keyboardRows are the Turkish Q keyboard's rows, to judge how likely a slip of the fingers is.
var keyboardRows = [][]rune{[]rune("qwertyuıopğü"), []rune("asdfghjklşi"), []rune("zxcvbnmöç")}

// near are the keys next to each key.
var near = func() map[rune]map[rune]bool {
	out := map[rune]map[rune]bool{}
	for ri, row := range keyboardRows {
		for ki, key := range row {
			set := map[rune]bool{}
			for _, rc := range [][2]int{{ri, ki - 1}, {ri, ki + 1}, {ri - 1, ki}, {ri - 1, ki + 1}, {ri + 1, ki}, {ri + 1, ki - 1}} {
				if rc[0] >= 0 && rc[0] < len(keyboardRows) && rc[1] >= 0 && rc[1] < len(keyboardRows[rc[0]]) {
					set[keyboardRows[rc[0]][rc[1]]] = true
				}
			}
			out[key] = set
		}
	}
	return out
}()

// AreNearKeys reports whether two keys are next to each other on a Turkish Q keyboard.
func AreNearKeys(first, second rune) bool { return near[first][second] }
