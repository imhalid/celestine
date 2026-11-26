package textnorm

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ListMarkStart starts the comment over a list that encodes its visual style in Markdown.
const ListMarkStart = "<!--celestine:list "

var (
	mdLink = regexp.MustCompile(`!?\[([^\[\]]*)\]\([^)]*\)`)
	mdWiki = regexp.MustCompile(`\[\[([^\[\]|]*\|)?([^\[\]]*)\]\]`)
)

// InlineTaskEnd is the end of a task in the running text, in Markdown: a comment no reader shows (#610).
const InlineTaskEnd = "<!--/task-->"

// Plain renders Markdown as visible plain words on a single line: link labels are kept, line
// markers (headings, quotes, list markers, task boxes) and fenced code markers are stripped,
// and emphasis/table bars become spaces.
func Plain(md string) string {
	s := strings.ReplaceAll(md, InlineTaskEnd, "")
	if strings.Contains(s, "](") {
		s = mdLink.ReplaceAllString(s, "$1")
	}
	if strings.Contains(s, "[[") {
		s = mdWiki.ReplaceAllString(s, "$2")
	}
	var b strings.Builder
	b.Grow(len(s))
	gap := false
	for len(s) > 0 {
		line := s
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			line, s = s[:i], s[i+1:]
		} else {
			s = ""
		}
		line = strings.TrimLeft(line, " \t\r\f")
		if strings.HasPrefix(line, "```") {
			gap = true
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(line, "> "), ListMarkStart) {
			continue
		}
		line = CutBox(CutLineMark(line))
		for i := 0; i < len(line); i++ {
			switch c := line[i]; c {
			case ' ', '\t', '\r', '\f', '\v', '*', '_', '~', '`', '|':
				gap = true
			default:
				if gap && b.Len() > 0 {
					b.WriteByte(' ')
				}
				gap = false
				b.WriteByte(c)
			}
		}
		gap = true
	}
	return b.String()
}

// CutLineMark trims leading indentation and one Markdown block prefix (headings, quotes, bullet or
// ordered list markers) followed by whitespace.
func CutLineMark(line string) string {
	n := 0
	switch {
	case strings.HasPrefix(line, "#"):
		for n < len(line) && n < 6 && line[n] == '#' {
			n++
		}
	case strings.HasPrefix(line, ">"), strings.HasPrefix(line, "-"), strings.HasPrefix(line, "*"), strings.HasPrefix(line, "+"):
		n = 1
	default:
		for n < len(line) && line[n] >= '0' && line[n] <= '9' {
			n++
		}
		if n == 0 || n >= len(line) || (line[n] != '.' && line[n] != ')') {
			return line
		}
		n++
	}
	if n >= len(line) || (line[n] != ' ' && line[n] != '\t') {
		return line
	}
	return strings.TrimLeft(line[n:], " \t")
}

// CutBox strips a leading Markdown task checkbox (`[ ]`, `[x]`, `[X]`) and trailing space.
func CutBox(line string) string {
	if len(line) >= 3 && line[0] == '[' && line[2] == ']' && (line[1] == ' ' || line[1] == 'x' || line[1] == 'X') {
		return strings.TrimLeft(line[3:], " \t")
	}
	return line
}

// Fold normalizes s for case- and diacritic-insensitive matching (lowercase, Turkish letters mapped
// to their ASCII equivalents).
func Fold(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return strings.ToLower(s)
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if c, ok := TurkishASCII(r); ok {
			b.WriteByte(c)
			continue
		}
		r = unicode.ToLower(r)
		if c, ok := TurkishASCII(r); ok {
			b.WriteByte(c)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// TurkishASCII maps Turkish-specific letters to their plain ASCII equivalents.
func TurkishASCII(r rune) (byte, bool) {
	switch r {
	case 'ç', 'Ç':
		return 'c', true
	case 'ğ', 'Ğ':
		return 'g', true
	case 'ı', 'İ':
		return 'i', true
	case 'ö', 'Ö':
		return 'o', true
	case 'ş', 'Ş':
		return 's', true
	case 'ü', 'Ü':
		return 'u', true
	}
	return 0, false
}

// Match scores how well q matches s (-1 when q does not match s).
func Match(q, s string) int {
	q = Fold(strings.TrimSpace(q))
	if q == "" {
		return 0
	}
	return MatchFolded(q, strings.Fields(q), []rune(strings.ReplaceAll(q, " ", "")), Fold(s))
}

// MatchFolded scores a pre-folded query against a pre-folded target string s.
func MatchFolded(q string, words []string, qr []rune, s string) int {
	if q == "" {
		return 0
	}
	score := 0
	all := true
	for _, w := range words {
		i := strings.Index(s, w)
		if i < 0 {
			all = false
			break
		}
		score += 100 - min(i, 60)
		if i == 0 || s[i-1] == ' ' || s[i-1] == '-' || s[i-1] == ':' {
			score += 40
		}
	}
	if all {
		if strings.HasPrefix(s, q) {
			score += 80
		}
		return score + 200
	}
	j, gaps, last := 0, 0, -1
	i := -1
	for _, r := range s {
		i++
		if j < len(qr) && r == qr[j] {
			if last >= 0 {
				gaps += i - last - 1
			}
			last = i
			j++
		}
	}
	if j < len(qr) {
		return -1
	}
	return max(1, 100-gaps*4)
}
