package main

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
)

// Reading and writing a page's Markdown without losing what richtext does
// not know. The library is plain files other tools write too (Obsidian,
// GitHub, Bear), and a page edited here is written whole: what richtext
// drops or changes - a table, HTML, math, a footnote, an Obsidian callout,
// a code block's language, [[links]] - must come back as it was.
//
// Reading, those blocks are taken out of the Markdown before richtext reads
// it, a token paragraph in their place, and put back after as paragraphs
// of code (the editor shows them in the monospaced font, its marks and
// all): a "raw" block, named by its first paragraph's ID. Writing, every
// block of code, raw or not, and every divider is written by us; the rest
// by richtext, its needless escapes taken back.

// heldBlock is a block we write ourselves.
type heldBlock struct {
	kind string // "hr", "code", or a raw block's kind: "table", "html", "math", "footnote", "callout"
	lang string // a code block's language
	text string
}

const (
	codeIDPrefix = "celestine:code:" // a code block's first paragraph: its language after it
	rawIDPrefix  = "celestine:raw:"  // a raw block's first paragraph: its kind after it
)

// rawNames are what a raw block is called over it, in the editor.
var rawNames = map[string]func() string{"table": locales.BlocksRawTable, "html": func() string { return "HTML" }, "math": locales.BlocksRawMath,
	"footnote": locales.BlocksRawFootnote, "callout": locales.BlocksRawCallout}

// blockToken is the line a held block stands as while richtext reads or writes the rest. Its marks are of Unicode's
// private use: a line typed as text never is one (#130).
func blockToken(i int) string { return "\uE000CELESTINEBLOCK" + strconv.Itoa(i) + "\uE001" }

var tokenLine = regexp.MustCompile("^\uE000CELESTINEBLOCK(\\d+)\uE001$")

var (
	hrLine     = regexp.MustCompile(`^ {0,3}(?:(?:- *){3,}|(?:\* *){3,}|(?:_ *){3,})$`)
	delimRow   = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$`)
	htmlStart  = regexp.MustCompile(`^ {0,3}<(?:!--|/?[A-Za-z][A-Za-z0-9-]*(?:[\s/>]|$))`)
	inlineHTML = regexp.MustCompile(`<(?:/?[A-Za-z][A-Za-z0-9-]*(?:\s[^<>]*)?/?|!--.*?--)>`)
	footDef    = regexp.MustCompile(`^ {0,3}\[\^[^\]]+\]:`)
	callout    = regexp.MustCompile(`^ {0,3}>\s*\[!`)
)

// parseBody reads a page's Markdown into the editor's text.
func parseBody(md string) richtext.Text {
	// one form at the door: a file written on Windows has CRLF lines, and one from some editors a BOM (#131)
	md = strings.ReplaceAll(strings.TrimPrefix(md, "\uFEFF"), "\r\n", "\n")
	lines := strings.Split(md, "\n")
	var out []string
	var held []heldBlock
	afterHeld := false // the last line given to richtext is a held block's: what follows starts a block
	hold := func(h heldBlock) {
		out = append(out, "", blockToken(len(held)), "")
		held = append(held, h)
		afterHeld = true
	}
	blank := func(i int) bool { return i < 0 || strings.TrimSpace(lines[i]) == "" }
	// lists' looks and tasks in the running text (listmd.go): tokens in the lines, what they say after richtext
	var looks []string
	look, tokens := -1, false // the look of the line after a list's comment
	refX := refDefX.MatchString(md)
	plain := strings.ContainsAny(md, listRunes) // its characters are text already: read as before
	for i := 0; i < len(lines); {
		l := lines[i]
		t := strings.TrimSpace(l)
		indent := len(l) - len(strings.TrimLeft(l, " "))
		// a fenced block of code, its language kept
		if indent <= 3 && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")) {
			n := 0
			for n < len(t) && t[n] == t[0] {
				n++
			}
			fence, lang := t[:n], strings.TrimSpace(t[n:])
			if !(fence[0] == '`' && strings.Contains(lang, "`")) {
				j := i + 1
				var body []string
				for ; j < len(lines); j++ {
					if c := strings.TrimSpace(lines[j]); strings.HasPrefix(c, fence) && strings.Trim(c, fence[:1]) == "" {
						break
					}
					body = append(body, lines[j])
				}
				hold(heldBlock{kind: "code", lang: lang, text: strings.Join(body, "\n")})
				i = j + 1
				continue
			}
		}
		if m, ok := listMarkAt(lines, i); ok && !plain { // the line under it says it
			look = len(looks)
			looks = append(looks, m)
			i++
			continue
		}
		if indent <= 3 {
			if kind, n := rawBlockOf(lines, i, blank(i-1), !plain); n > 0 {
				hold(heldBlock{kind: kind, text: strings.Join(lines[i:i+n], "\n")})
				i += n
				continue
			}
			// "---" right under a line of text underlines it as a heading
			if hrLine.MatchString(l) && (blank(i-1) || afterHeld || !strings.Contains(l, "-")) {
				hold(heldBlock{kind: "hr"})
				i++
				continue
			}
		}
		if footDef.MatchString(l) && len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "") // a footnote's note is a paragraph of its own, under another note too (#308)
		}
		l = protectInline(l)
		if !plain {
			l = inlineChecksIn(l, refX)
			toks := ""
			if look >= 0 {
				toks, look = listToken(tokLook, look), -1
			}
			if numberedTaskLine.MatchString(l) {
				toks += listToken(tokNumbered, 0)
			}
			l = withItemTokens(l, toks)
			tokens = tokens || strings.ContainsAny(l, listTokens)
		}
		out = append(out, l)
		afterHeld = false
		i++
	}
	kinds := taskKindsIn(out) // the boxes richtext has no kind for: "[*]", "[!]"… (taskkinds.go)
	t := withTaskKinds(richtext.ParseMarkdown(strings.Join(out, "\n")), kinds)
	if tokens {
		t = listTokensIn(t, looks)
	}
	if len(held) == 0 {
		return t
	}
	for _, at := range starts(t) {
		s, e := t.ParagraphRange(at)
		m := tokenLine.FindStringSubmatch(t.Slice(s, e).String())
		if m == nil {
			continue
		}
		k, _ := strconv.Atoi(m[1])
		if k >= len(held) {
			continue
		}
		h := held[k]
		if h.kind == "hr" {
			t = t.ReplaceString(s, e, "", richtext.Attributes{})
			t = t.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{ID: hrID} })
			continue
		}
		t = t.ReplaceString(s, e, h.text, richtext.Attributes{})
		end := s + utf8.RuneCountInString(h.text)
		t = t.FormatParagraphs(s, end, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{CodeBlock: true} })
		id := codeIDPrefix + h.lang
		if h.kind != "code" {
			id = rawIDPrefix + h.kind
		}
		t = t.FormatParagraphs(s, s, func(p *richtext.ParagraphStyle) { p.ID = id })
	}
	return t
}

// rawBlock reports whether a block richtext would lose starts at line i -
// a table, HTML, math, a footnote's definition, an Obsidian callout, or a
// paragraph with HTML in it (only where a paragraph starts) - and its lines.
func rawBlock(lines []string, i int, paraStart bool) (string, int) {
	return rawBlockOf(lines, i, paraStart, true)
}

// rawBlockOf is rawBlock; looks says that a list's look (listmd.go) is no HTML of the paragraph it is in.
func rawBlockOf(lines []string, i int, paraStart, looks bool) (string, int) {
	l := lines[i]
	until := func(j int, in func(string) bool) int {
		for j < len(lines) && strings.TrimSpace(lines[j]) != "" && in(lines[j]) {
			j++
		}
		return j - i
	}
	any := func(string) bool { return true }
	t := strings.TrimSpace(l)
	switch {
	case strings.Contains(l, "|") && i+1 < len(lines) && strings.Contains(lines[i+1], "-") && delimRow.MatchString(lines[i+1]):
		return "table", until(i, func(s string) bool { return strings.Contains(s, "|") })
	case strings.HasPrefix(t, "$$"):
		if len(t) > 2 && strings.HasSuffix(t, "$$") && t != "$$" {
			return "math", 1
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.HasSuffix(strings.TrimSpace(lines[j]), "$$") {
				return "math", j - i + 1
			}
		}
		return "", 0
	case callout.MatchString(l):
		return "callout", until(i, func(s string) bool { return strings.HasPrefix(strings.TrimSpace(s), ">") })
	// a footnote's note ([^1]: …) is a paragraph of the page, written in as any other (footnote.go, #308)
	case htmlStart.MatchString(l) && foreignHTML(l):
		return "html", until(i, any)
	case paraStart:
		n := until(i, any)
		for k, s := range lines[i : i+n] {
			if _, look := listMarkAt(lines, i+k); look && looks { // a list's look, over an item of it
				continue
			}
			if foreignHTML(withoutCode(s)) {
				return "html", n
			}
		}
	}
	return "", 0
}

// ownTags are the tags richtext writes and reads itself: underline, and bold, italic or strikethrough that Markdown
// cannot flank. A line with them is the editor's text, not HTML to keep as it is (#124).
var ownTags = map[string]bool{"u": true, "ins": true, "s": true, "strike": true, "del": true, "b": true, "strong": true, "i": true, "em": true, "br": true}

// foreignHTML says whether line s has a tag of HTML that the editor does not read: not one of ownTags, and not one
// escaped with a backslash, as richtext writes a "<" that was typed.
func foreignHTML(s string) bool {
	for _, m := range inlineHTML.FindAllStringIndex(s, -1) {
		slashes := 0
		for k := m[0] - 1; k >= 0 && s[k] == '\\'; k-- {
			slashes++
		}
		if slashes%2 == 1 {
			continue
		}
		tag := strings.TrimPrefix(s[m[0]+1:m[1]-1], "/")
		if end := strings.IndexAny(tag, " \t/"); end >= 0 {
			tag = tag[:end]
		}
		if !ownTags[strings.ToLower(tag)] {
			return true
		}
	}
	return false
}

// withoutCode is line s without its spans of inline code.
func withoutCode(s string) string {
	parts := strings.Split(s, "`")
	var sb strings.Builder
	for i, p := range parts {
		if i%2 == 0 {
			sb.WriteString(p)
		}
	}
	return sb.String()
}

// protectInline keeps footnote references as text, which richtext reads
// as "[1]" and writes so.
func protectInline(l string) string {
	if !strings.Contains(l, "[^") && !strings.Contains(l, "[[") {
		return l
	}
	return outsideCode(l, func(s string) string {
		var sb strings.Builder
		for i := 0; i < len(s); i++ {
			// one not escaped already: "\\[^" read and written again would grow a backslash at each round (#130)
			if strings.HasPrefix(s[i:], "[^") && !escapedAt(s, i) {
				sb.WriteByte('\\')
			}
			sb.WriteByte(s[i])
		}
		return protectLinks(sb.String())
	})
}

// escapedAt says whether the byte at i of s is escaped: an odd run of backslashes before it.
func escapedAt(s string, i int) bool {
	n := 0
	for k := i - 1; k >= 0 && s[k] == '\\'; k-- {
		n++
	}
	return n%2 == 1
}

// wikiInLine is a [[link]] as it is typed, its inside kept whole.
var wikiInLine = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)

// protectLinks escapes the marks inside a [[link]] that richtext would read as emphasis: [[_private]] is the page
// "_private", not "private" in italics (#130). The chips find the link as it was typed.
func protectLinks(s string) string {
	if !strings.Contains(s, "[[") {
		return s
	}
	return wikiInLine.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[2 : len(m)-2]
		var sb strings.Builder
		for i := 0; i < len(inner); i++ {
			if strings.IndexByte("_*~", inner[i]) >= 0 && !escapedAt(inner, i) {
				sb.WriteByte('\\')
			}
			sb.WriteByte(inner[i])
		}
		return "[[" + sb.String() + "]]"
	})
}

// outsideCode applies fn to the parts of line l outside inline code.
func outsideCode(l string, fn func(string) string) string {
	parts := strings.Split(l, "`")
	for i := range parts {
		if i%2 == 0 {
			parts[i] = fn(parts[i])
		}
	}
	return strings.Join(parts, "`")
}

// bodyMarkdown writes the editor's text as the page's Markdown.
func bodyMarkdown(t richtext.Text) string {
	// chips are their [[links]] (chips.go); tasks in the running text and lists' looks are tokens (listmd.go)
	t, looks, marked := listTokensOut(unfold(t))
	type span struct {
		s, e int
		h    heldBlock
	}
	var spans []span
	// the paragraphs read once, in order: a paragraph's range, style and text asked of the text one by one cost a
	// walk of it each, seconds for a page of thirty thousand lines (#504)
	type para struct {
		s, e int
		st   richtext.ParagraphStyle
		text string
	}
	var ps []para
	for at, p := range t.Paragraphs() {
		text := paraText(p)
		ps = append(ps, para{at, at + utf8.RuneCountInString(text), p.Style, text})
	}
	for i := 0; i < len(ps); {
		s, e, st := ps[i].s, ps[i].e, ps[i].st
		switch {
		case st.ID == hrID && s == e:
			spans = append(spans, span{s, e, heldBlock{kind: "hr"}})
			i++
		case st.CodeBlock && !st.Quote && st.Indent == 0 && st.List == richtext.NoList:
			h := heldBlock{kind: "code"}
			if k, ok := strings.CutPrefix(st.ID, rawIDPrefix); ok {
				h.kind = k
			} else if lang, ok := strings.CutPrefix(st.ID, codeIDPrefix); ok {
				h.lang = lang
			}
			var texts []string
			end, j := e, i
			for ; j < len(ps); j++ {
				st2 := ps[j].st
				if !st2.CodeBlock || st2.Quote || st2.Indent != 0 || st2.List != richtext.NoList {
					break
				}
				if j > i && (strings.HasPrefix(st2.ID, codeIDPrefix) || strings.HasPrefix(st2.ID, rawIDPrefix)) {
					break // the next block
				}
				texts = append(texts, ps[j].text)
				end = ps[j].e
			}
			h.text = strings.Join(texts, "\n")
			spans = append(spans, span{s, end, h})
			i = j
		default:
			i++
		}
	}
	for k := len(spans) - 1; k >= 0; k-- {
		sp := spans[k]
		t = t.ReplaceString(sp.s, sp.e, blockToken(k), richtext.Attributes{})
		t = t.FormatParagraphs(sp.s, sp.s, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
	}
	lines := strings.Split(t.Markdown(), "\n")
	markTaskKinds(t, lines)
	if marked {
		lines = listMarksOut(lines, looks)
	}
	var text []bool // the lines of text, not of code: where a task in the running text can be written bare
	if marked {
		text = make([]bool, len(lines))
	}
	fence := ""
	for i, l := range lines {
		if m := tokenLine.FindStringSubmatch(l); m != nil {
			if k, _ := strconv.Atoi(m[1]); k < len(spans) {
				lines[i] = writeHeld(spans[k].h)
				continue
			}
		}
		// richtext's own fences (code in a list or a quote): left as they are
		if t := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), ">")); strings.HasPrefix(t, "```") {
			if fence == "" {
				fence = "```"
			} else {
				fence = ""
			}
			continue
		}
		if fence == "" {
			lines[i] = unescape(l)
			if text != nil {
				text[i] = true
			}
		}
	}
	if marked {
		refX := refDefX.MatchString(strings.Join(lines, "\n"))
		for i, l := range lines {
			if text[i] {
				lines[i] = checksOut(l, refX)
			} else {
				lines[i] = checksBare(l)
			}
		}
	}
	// footnotes' notes one under the other, as they are written: no empty line between two (#308)
	for i := len(lines) - 2; i > 0; i-- {
		if lines[i] == "" && footDef.MatchString(lines[i-1]) && footDef.MatchString(lines[i+1]) {
			lines = append(lines[:i], lines[i+1:]...)
		}
	}
	return strings.Join(lines, "\n")
}

// writeHeld is a held block's Markdown.
func writeHeld(h heldBlock) string {
	switch h.kind {
	case "hr":
		return "---"
	case "code":
		fence := "```"
		for strings.Contains(h.text, fence) {
			fence += "`"
		}
		return fence + h.lang + "\n" + h.text + "\n" + fence
	}
	return h.text // raw: as it was, as it was edited
}

var (
	wikiLink = regexp.MustCompile(`\\\[\\\[(.+?)\\\]\\\]`)
	footRef  = regexp.MustCompile(`\\\[\^([^\]\\\s]+)\\\]`)
	// linkEscape is an escape richtext writes inside a [[link]]'s text
	linkEscape = regexp.MustCompile(`\\([\\_*~#!\[\]<>` + "`" + `])`)
)

// unescape takes back the escapes richtext writes where Markdown needs
// none, outside inline code: "!" not before "[", "_" inside a word, "#"
// starting a line without a space after it (no heading), and the brackets
// of [[links]] and footnote references.
func unescape(l string) string {
	if !strings.Contains(l, `\`) {
		return l
	}
	lead := len(l) - len(strings.TrimLeft(l, " >-*+0123456789.)[]x"))
	head, rest := l[:lead], l[lead:]
	if strings.HasPrefix(rest, `\#`) && len(rest) > 2 && rest[2] != ' ' && rest[2] != '#' {
		rest = rest[1:]
	}
	rest = outsideCode(rest, func(s string) string {
		s = wikiLink.ReplaceAllStringFunc(s, func(m string) string { // its inside as it is typed, not escaped
			inner := wikiLink.FindStringSubmatch(m)[1]
			return "[[" + linkEscape.ReplaceAllString(inner, "$1") + "]]"
		})
		s = footRef.ReplaceAllString(s, "[^$1]")
		r := []rune(s)
		var sb strings.Builder
		for i := 0; i < len(r); i++ {
			if r[i] == '\\' && i+1 < len(r) {
				next := r[i+1]
				if next == '\\' { // an escaped backslash is a pair: what follows it is not escaped by it (#130)
					sb.WriteString(`\\`)
					i++
					continue
				}
				switch {
				case next == '!' && (i+2 >= len(r) || (r[i+2] != '[' && !(r[i+2] == '\\' && i+3 < len(r) && r[i+3] == '['))):
					continue
				case next == '_' && i > 0 && i+2 < len(r) && isWord(r[i-1]) && isWord(r[i+2]):
					continue
				}
			}
			sb.WriteRune(r[i])
		}
		return sb.String()
	})
	return head + rest
}

func isWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// starts is where t's paragraphs start, last first: to change them from
// the end, the places of those before staying put.
func starts(t richtext.Text) []int {
	var out []int
	for at := range t.Paragraphs() {
		out = append(out, at)
	}
	slices.Reverse(out) // put each first, it was a copy of all the others: 46 ms at 10 000 paragraphs (#198)
	return out
}
