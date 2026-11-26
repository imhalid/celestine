package library

import (
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/imhalid/celestine/internal/textnorm"
)

// A page's Markdown, read and written without losing what Markdown's codec does not know (#595). The library is
// plain files other tools write too (Obsidian, GitHub, Bear), and a page edited here is written whole: what the codec
// drops or changes - a table, HTML, math, a footnote, an Obsidian callout, a code block's language, [[links]] - must
// come back as it was.
//
// Reading, those blocks are taken out of the Markdown before the codec reads it (ReadMarkdown), a token paragraph in
// their place, and put back after as paragraphs of code: a "raw" block, named by its first paragraph's ID. Writing,
// every block of code, raw or not, and every divider is written here; the rest by the codec (WriteMarkdown), its
// needless escapes taken back. Besides, Markdown has no word for a task of a kind ("[*]", "[!]"…), a task in the
// running text, or a list of the app's own look; each is written as Markdown other readers show as it is meant:
//
//	Buy [ ] milk [x]                 a task in the running text, an object: a box not escaped, in the running text
//	1. [ ] Call                      a list of tasks with a number before each box (task=number)
//	<!--celestine:list mark=%E2%AD%90-->
//	- First                          the look of the list under it, as its item's ID says it, in a comment no
//	                                 reader shows, on the line right over the list's first item, as far in
//
// While the codec reads or writes the text around them, each is a character of Unicode's private use in the text,
// as a held block is a line of them: an escape is what tells a box from text, and the codec drops it. This was the
// app's (markdown.go, listmd.go), on the editor's text; it is the library's, on its document.

// The IDs a paragraph's style can have, which say what Markdown has no style for.
const (
	HrID         = "celestine:hr"         // a divider: an empty paragraph
	CodeIDPrefix = "celestine:code:"      // a code block's first paragraph: its language after it
	RawIDPrefix  = "celestine:raw:"       // a raw block's first paragraph: its kind after it
	TaskIDPrefix = "celestine:task:"      // a task of a kind: its kind after it
	ListIDPrefix = "celestine:list:"      // an item that says how its list looks: its spec after it
	emptyFill    = "\uE010"               // what fills a task with no words while the codec reads it (#584)
	blockMark    = "\uE000CELESTINEBLOCK" // a held block's token line starts so
)

// TaskKindMarks are the kinds of task and the character each has in its Markdown box, as Obsidian has them: a task
// done is "[x]" whatever its kind.
var TaskKindMarks = []struct{ ID, MD string }{
	{"star", "*"}, {"important", "!"}, {"question", "?"}, {"progress", "/"}, {"blocked", "w"}, {"billable", "$"},
	{"error", "e"}, {"canceled", "-"},
}

// taskKindOfMD is the kind whose box has character c.
func taskKindOfMD(c string) (string, bool) {
	for _, k := range TaskKindMarks {
		if k.MD == c {
			return k.ID, true
		}
	}
	return "", false
}

// taskMDOf is the character of kind id's box.
func taskMDOf(id string) (string, bool) {
	for _, k := range TaskKindMarks {
		if k.ID == id {
			return k.MD, true
		}
	}
	return "", false
}

// heldBlock is a block written here.
type heldBlock struct {
	kind string // "hr", "code", or a raw block's kind: "table", "html", "math", "footnote", "callout"
	lang string // a code block's language
	text string
}

// blockToken is the line a held block stands as while the codec reads or writes the rest. Its marks are of Unicode's
// private use: a line typed as text never is one (#130).
func blockToken(i int) string { return blockMark + strconv.Itoa(i) + "\uE001" }

var tokenLine = regexp.MustCompile("^\uE000CELESTINEBLOCK(\\d+)\uE001$")

var (
	hrLine     = regexp.MustCompile(`^ {0,3}(?:(?:- *){3,}|(?:\* *){3,}|(?:_ *){3,})$`)
	DelimRow   = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$`) // a table's delimiter row
	htmlStart  = regexp.MustCompile(`^ {0,3}<(?:!--|/?[A-Za-z][A-Za-z0-9-]*(?:[\s/>]|$))`)
	inlineHTML = regexp.MustCompile(`<(?:/?[A-Za-z][A-Za-z0-9-]*(?:\s[^<>]*)?/?|!--.*?--)>`)
	footDef    = regexp.MustCompile(`^ {0,3}\[\^[^\]]+\]:`)
	// Callout is the first line of an Obsidian callout.
	Callout = regexp.MustCompile(`^ {0,3}>\s*\[!`)
	// emptyTaskLine is a task's line with nothing after its box: GFM reads it as no task, though the codec writes an
	// empty task so (#584). It is given emptyFill after its box, taken away once read.
	emptyTaskLine = regexp.MustCompile(`^\s*(?:>\s*)*(?:[-*+]|\d+[.)])\s+\[[^\]]\]\s*$`)
	// emptyItemLine is a list's mark with nothing after it; one under an item is filled too.
	emptyItemLine = regexp.MustCompile(`^\s*(?:>\s*)*(?:[-*+]|\d+[.)])\s*$`)
	// ListItemLine is a line that starts an item of a list, and TaskBoxLine one with a box: what comes before the
	// box, and what is in it.
	ListItemLine = regexp.MustCompile(`^\s*(?:>\s*)*(?:[-*+]|\d+[.)])\s`)
	TaskBoxLine  = regexp.MustCompile(`^(\s*(?:>\s*)*(?:[-*+]|\d+[.)])\s+)\[(.)\](\s|$)`)
)

// ---- reading ----

// PageFromMarkdown reads a page's Markdown into its document: [[links]] stay text (the app makes chips of them), a
// task in the running text is a "check" object, and what the codec would lose is held as paragraphs of code.
func PageFromMarkdown(md string) Doc {
	md = strings.ToValidUTF8(md, "\uFFFD") // read as it will be written: a byte that is not UTF-8 is U+FFFD both times (#586)
	// one form at the door: a file written on Windows has CRLF lines, and one from some editors a BOM (#131)
	md = strings.ReplaceAll(strings.TrimPrefix(md, "\uFEFF"), "\r\n", "\n")
	lines := strings.Split(md, "\n")
	var out []string
	var held []heldBlock
	afterHeld := false // the last line given to the codec is a held block's: what follows starts a block
	hold := func(h heldBlock) {
		out = append(out, "", blockToken(len(held)), "")
		held = append(held, h)
		afterHeld = true
	}
	blank := func(i int) bool { return i < 0 || strings.TrimSpace(lines[i]) == "" }
	var looks []string
	look, tokens := -1, false // the look of the line after a list's comment
	filled := false           // a task with no words was given emptyFill
	refX := RefDefX.MatchString(md)
	plain := strings.ContainsAny(md, listRunes) // its characters are text already: read as before, without tokens
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
			if kind, n := RawBlockOf(lines, i, blank(i-1), !plain); n > 0 {
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
			under := len(out) > 0 && ListItemLine.MatchString(out[len(out)-1])
			if emptyTaskLine.MatchString(l) || under && emptyItemLine.MatchString(l) {
				l = strings.TrimRight(l, " \t") + " " + emptyFill
				filled = true
			}
		}
		out = append(out, l)
		afterHeld = false
		i++
	}
	kinds := taskKindsIn(out) // the boxes the codec has no kind for: "[*]", "[!]"…
	d := withTaskKinds(ReadMarkdown(strings.Join(out, "\n")), kinds)
	if filled {
		d = withoutRune(d, emptyFill)
	}
	if tokens {
		d = listTokensIn(d, looks)
	}
	if len(held) == 0 {
		return d
	}
	var res Doc
	for _, p := range d {
		m := tokenLine.FindStringSubmatch(p.text())
		k := -1
		if m != nil {
			k, _ = strconv.Atoi(m[1])
		}
		if m == nil || k >= len(held) {
			res = append(res, p)
			continue
		}
		h := held[k]
		if h.kind == "hr" {
			res = append(res, Para{S: &ParaStyle{ID: HrID}})
			continue
		}
		id := CodeIDPrefix + h.lang
		if h.kind != "code" {
			id = RawIDPrefix + h.kind
		}
		lines := strings.Split(h.text, "\n")
		for j, line := range lines {
			if j > 0 && j == len(lines)-1 && line == "" { // the line after a block that ends its text with one: no code
				res = append(res, Para{})
				continue
			}
			st := ParaStyle{CodeBlock: true}
			if j == 0 {
				st.ID = id
			}
			q := Para{S: &st}
			if line != "" {
				q.R = []Run{{T: line}}
			}
			res = append(res, q)
		}
	}
	return res
}

// RawBlockOf reports whether a block the codec would lose starts at line i - a table, HTML, math, an Obsidian
// callout, or a paragraph with HTML in it (only where a paragraph starts) - and its lines. looks says that a list's
// look is no HTML of the paragraph it is in.
func RawBlockOf(lines []string, i int, paraStart, looks bool) (string, int) {
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
	case strings.Contains(l, "|") && i+1 < len(lines) && strings.Contains(lines[i+1], "-") && DelimRow.MatchString(lines[i+1]):
		// the delimiter row is the table's whatever it holds, a "-" without a pipe too (#589)
		return "table", until(i+2, func(s string) bool { return strings.Contains(s, "|") })
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
	case Callout.MatchString(l):
		return "callout", until(i, func(s string) bool { return strings.HasPrefix(strings.TrimSpace(s), ">") })
	// a footnote's note ([^1]: …) is a paragraph of the page, written in as any other (#308)
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

// ownTags are the tags the codec writes and reads itself: underline, and bold, italic or strikethrough that Markdown
// cannot flank. A line with them is the editor's text, not HTML to keep as it is (#124).
var ownTags = map[string]bool{"u": true, "ins": true, "s": true, "strike": true, "del": true, "b": true, "strong": true, "i": true, "em": true, "br": true}

// foreignHTML says whether line s has a tag of HTML the editor does not read: not one of ownTags, and not one
// escaped with a backslash, as the codec writes a "<" that was typed.
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

// protectInline keeps footnote references as text, which the codec reads as "[1]" and writes so.
func protectInline(l string) string {
	if !strings.Contains(l, "[^") && !strings.Contains(l, "[[") {
		return l
	}
	return OutsideCode(l, func(s string) string {
		var sb strings.Builder
		for i := 0; i < len(s); i++ {
			// one not escaped already: "\\[^" read and written again would grow a backslash at each round (#130)
			if strings.HasPrefix(s[i:], "[^") && !EscapedAt(s, i) {
				sb.WriteByte('\\')
			}
			sb.WriteByte(s[i])
		}
		return protectLinks(sb.String())
	})
}

// EscapedAt says whether the byte at i of s is escaped: an odd run of backslashes before it.
func EscapedAt(s string, i int) bool {
	n := 0
	for k := i - 1; k >= 0 && s[k] == '\\'; k-- {
		n++
	}
	return n%2 == 1
}

// wikiInLine is a [[link]] as it is typed, its inside kept whole.
var wikiInLine = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)

// protectLinks escapes the marks inside a [[link]] that the codec would read as emphasis: [[_private]] is the page
// "_private", not "private" in italics (#130).
func protectLinks(s string) string {
	if !strings.Contains(s, "[[") {
		return s
	}
	return wikiInLine.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[2 : len(m)-2]
		var sb strings.Builder
		for i := 0; i < len(inner); i++ {
			if strings.IndexByte("_*~", inner[i]) >= 0 && !EscapedAt(inner, i) {
				sb.WriteByte('\\')
			}
			sb.WriteByte(inner[i])
		}
		return "[[" + sb.String() + "]]"
	})
}

// OutsideCode applies fn to the parts of line l outside inline code.
func OutsideCode(l string, fn func(string) string) string {
	parts := strings.Split(l, "`")
	for i := range parts {
		if i%2 == 0 {
			parts[i] = fn(parts[i])
		}
	}
	return strings.Join(parts, "`")
}

// withoutRune is d without any r in its text.
func withoutRune(d Doc, r string) Doc {
	for i := range d {
		rs := d[i].R[:0:0]
		for _, run := range d[i].R {
			if run.O == nil && strings.Contains(run.T, r) {
				run.T = strings.ReplaceAll(run.T, r, "")
				if run.T == "" {
					continue
				}
			}
			rs = append(rs, run)
		}
		d[i].R = mergeRuns(rs)
	}
	return d
}

// mergeRuns joins the runs of the same marks side by side, as the editor's text has them.
func mergeRuns(rs []Run) []Run {
	out := rs[:0:0]
	for _, r := range rs {
		if n := len(out); n > 0 && r.O == nil && out[n-1].O == nil && sameMarks(out[n-1], r) {
			out[n-1].T += r.T
			continue
		}
		out = append(out, r)
	}
	return out
}

// taskKindsIn takes the kinds out of the boxes of md's lines, which the codec reads as plain ones, and gives them
// back in the order of the tasks: "" for a plain one or a done one.
func taskKindsIn(lines []string) []string {
	var kinds []string
	inCode, prevBlank, prevList := false, true, false
	for i, l := range lines {
		// an indented block of code (four spaces after a blank line, not under a list item) is code: a box in it is
		// text, not to be rewritten, or the count of tasks differs and every task loses its kind (#131)
		if strings.TrimSpace(l) == "" {
			prevBlank = true
			continue
		}
		indent := len(strings.ReplaceAll(l, "\t", "    ")) - len(strings.TrimLeft(strings.ReplaceAll(l, "\t", "    "), " "))
		switch {
		case inCode && indent >= 4:
			prevBlank = false
			continue
		case !inCode && indent >= 4 && prevBlank && !prevList:
			inCode, prevBlank = true, false
			continue
		}
		inCode = false
		prevList = ListItemLine.MatchString(l) || prevList && indent >= 2
		prevBlank = false
		m := TaskBoxLine.FindStringSubmatchIndex(l)
		if m == nil {
			continue
		}
		c := l[m[4]:m[5]]
		switch c {
		case " ", "x", "X":
			kinds = append(kinds, "")
		default:
			id, ok := taskKindOfMD(c)
			if !ok {
				continue // not a box the codec takes: no task
			}
			kinds = append(kinds, id)
			lines[i] = l[:m[4]] + " " + l[m[5]:]
		}
	}
	return kinds
}

// withTaskKinds gives d's tasks, in order, the kinds read from their Markdown: only when there are as many.
func withTaskKinds(d Doc, kinds []string) Doc {
	has := false
	for _, k := range kinds {
		has = has || k != ""
	}
	if !has {
		return d
	}
	var tasks []int
	for i, p := range d {
		if p.S != nil && p.S.List == ListTasks {
			tasks = append(tasks, i)
		}
	}
	if len(tasks) != len(kinds) {
		return d
	}
	for i, at := range tasks {
		if kinds[i] == "" {
			continue
		}
		st := *d[at].S
		st.ID = TaskIDPrefix + kinds[i]
		d[at].S = &st
	}
	return d
}

// ---- tasks in the running text, and lists' looks ----

const (
	tokCheck    = '\uE002' // a task in the running text, not done
	tokChecked  = '\uE003' // and one done
	tokLook     = '\uE004' // a list's look: tokLook, its number among the page's looks, tokEnd
	tokNumbered = '\uE006' // an item of a numbered list of tasks: tokNumbered, its number in the list, tokEnd
	tokEnd      = '\uE005'
	// listTokens are the characters that start a token of lists in the text, and listRunes all of its characters: a
	// text that has one already, as an icon font's letter, is read and written as before, without tokens.
	listTokens = "\uE002\uE003\uE004\uE006"
	listRunes  = "\uE002\uE003\uE004\uE005\uE006"
)

var (
	// ListMarkLine is the comment over a list: what is before it (a quote's marks, the list's indent) and the look.
	ListMarkLine = regexp.MustCompile(`^([> ]*)<!--celestine:list ([^\s<>]*)-->[ \t]*$`)
	// listLead is what is before a list item's marker: a quote's marks and the indent.
	listLead = regexp.MustCompile(`^[> ]*`)
	// bulletMarker is a list item's bullet, as the codec writes it, with what is before it.
	bulletMarker = regexp.MustCompile(`^([> ]*)[-*+] `)
	// numberedTaskLine is an item of a numbered list with a box.
	numberedTaskLine = regexp.MustCompile(`^\s*(?:>\s*)*\d+[.)]\s+\[.\](?:\s|$)`)
	// RefDefX is a link's reference "[x]: …": a page that has one has its "[x]" as a link, not a task.
	RefDefX = regexp.MustCompile(`(?mi)^ {0,3}\[x\]:`)
	// safeLook is a look the comment can say as the ID has it: nothing that ends the comment, no spaces.
	safeLook = regexp.MustCompile(`^[^\s<>]*$`)
)

// InlineBoxes are where in line l, of Markdown, a box "[ ]" or "[x]" is a task in the running text: not escaped,
// outside code, not a list item's own box, not in a link's text or a [[link]], after a space, "(" or an emphasis's
// mark or at the start, and before a space, a stop, ")" or an emphasis's mark or at the end. refX says the page
// has a reference "[x]:", so that "[x]" is its link. The reader reads only these as tasks, and the writer writes a
// task bare only where this finds it, so the two agree.
func InlineBoxes(l string, refX bool) []int {
	if !strings.Contains(l, "[ ]") && !strings.Contains(l, "[x]") {
		return nil
	}
	own := -1 // the item's box, a task line's
	if m := TaskBoxLine.FindStringSubmatchIndex(l); m != nil {
		own = m[4] - 1
	}
	var out []int
	code, depth := false, 0
	for i := 0; i < len(l); i++ {
		switch c := l[i]; {
		case c == '`' && (code || !EscapedAt(l, i)): // an escaped one opens no code; in code no backslash escapes
			code = !code
		case code:
		case c == '[' && !EscapedAt(l, i):
			if i != own && depth == 0 && boxAt(l, i, refX) {
				out = append(out, i)
				i += 2
				continue
			}
			depth++
		case c == ']' && !EscapedAt(l, i) && depth > 0:
			depth--
		}
	}
	return out
}

// boxAt says whether l has, at i, a box with room around it to be a task (InlineBoxes).
func boxAt(l string, i int, refX bool) bool {
	box := l[i:min(i+3, len(l))]
	if box != "[ ]" && (box != "[x]" || refX) {
		return false
	}
	if i > 0 && !strings.ContainsRune(" \t(*_~", rune(l[i-1])) {
		return false
	}
	return i+3 == len(l) || strings.ContainsRune(" \t.,;!?)*_~", rune(l[i+3]))
}

// inlineChecksIn puts a token in the place of each box of line l that is a task in the running text.
func inlineChecksIn(l string, refX bool) string {
	at := InlineBoxes(l, refX)
	for k := len(at) - 1; k >= 0; k-- {
		tok := tokCheck
		if l[at[k]+1] == 'x' {
			tok = tokChecked
		}
		l = l[:at[k]] + string(tok) + l[at[k]+3:]
	}
	return l
}

// listMarkAt is the look the comment on line i says of the list under it; false when line i is not one, or what
// is under it is not an item of a list read as one, at the same place.
func listMarkAt(lines []string, i int) (string, bool) {
	m := ListMarkLine.FindStringSubmatch(lines[i])
	if m == nil || i+1 >= len(lines) {
		return "", false
	}
	next := lines[i+1]
	if !ListItemLine.MatchString(next) || listLead.FindString(next) != m[1] || hrLine.MatchString(next) {
		return "", false
	}
	if kind, _ := RawBlockOf(lines, i+1, false, true); kind != "" {
		return "", false
	}
	// as far in as a block of code, after an empty line, it is code (a list's comment is under its item)
	if len(m[1])-len(strings.TrimRight(m[1], " ")) >= 4 && (i == 0 || strings.TrimSpace(lines[i-1]) == "") {
		return "", false
	}
	return m[2], true
}

// withItemTokens puts toks in item line l's text, at its start: after its marker, and after its box if it has one.
func withItemTokens(l, toks string) string {
	if toks == "" {
		return l
	}
	if m := TaskBoxLine.FindStringSubmatchIndex(l); m != nil {
		if m[6] == m[7] { // the box ends the line: a space after it, or it is no task's
			return l + " " + toks
		}
		return l[:m[1]] + toks + l[m[1]:]
	}
	if m := ListItemLine.FindStringIndex(l); m != nil {
		return l[:m[1]] + toks + l[m[1]:]
	}
	return l
}

// listToken is a token of kind tok and number n.
func listToken(tok rune, n int) string { return string(tok) + strconv.Itoa(n) + string(tokEnd) }

// checkMarkdown is a task in the running text's box.
func checkMarkdown(done bool) string {
	if done {
		return "[x]"
	}
	return "[ ]"
}

// listTokensIn turns the tokens read from Markdown in d into what they say: the tasks in the running text into
// objects (their boxes again, in code), and a list's look, or its numbered tasks, into the ID of its first item
// without one. looks are the comments' looks, by the numbers in their tokens.
func listTokensIn(d Doc, looks []string) Doc {
	lookAt := map[int]string{} // a look, by the index of the paragraph it is over
	numbered := map[int]bool{} // the items of a numbered list of tasks, by index
	for i := range d {
		para := &d[i]
		code := para.S != nil && para.S.CodeBlock
		var runs []Run
		for _, r := range para.R {
			if r.O != nil || !strings.ContainsAny(r.T, listTokens) {
				runs = append(runs, r)
				continue
			}
			var sb strings.Builder
			flush := func() {
				if sb.Len() > 0 {
					t := r
					t.T = sb.String()
					runs = append(runs, t)
					sb.Reset()
				}
			}
			s := r.T
			for j := 0; j < len(s); {
				c, n := utf8.DecodeRuneInString(s[j:])
				switch c {
				case tokCheck, tokChecked:
					if code || r.C { // in code, the box it was
						sb.WriteString(checkMarkdown(c == tokChecked))
						break
					}
					flush()
					o := Obj{K: "check"}
					if c == tokChecked {
						o.Target = "x"
					}
					t := r
					t.T, t.O, t.L, t.C = ObjectChar, &o, r.L, r.C
					runs = append(runs, t)
				case tokLook, tokNumbered:
					end := strings.IndexRune(s[j+n:], tokEnd)
					if end < 0 {
						sb.WriteRune(c)
						break
					}
					k, err := strconv.Atoi(s[j+n : j+n+end])
					switch {
					case err != nil || code:
					case c == tokLook && k >= 0 && k < len(looks):
						if _, ok := lookAt[i]; !ok {
							lookAt[i] = looks[k]
						}
					case c == tokNumbered:
						numbered[i] = true
					}
					n += end + utf8.RuneLen(tokEnd)
				default:
					sb.WriteRune(c)
				}
				j += n
			}
			flush()
		}
		para.R = mergeRuns(runs)
	}
	if len(lookAt) == 0 && len(numbered) == 0 {
		return d
	}
	lm := listsOf(d)
	ids := map[*listGroup]string{}
	var groups []*listGroup
	say := func(p int, id string) {
		it := lm.all[p]
		if it == nil {
			return
		}
		if _, ok := ids[it.group]; !ok {
			ids[it.group] = id
			groups = append(groups, it.group)
		}
	}
	for _, p := range sortedKeys(lookAt) {
		if id := ListIDPrefix + lookAt[p]; func() bool { _, ok := ParseListSpec(id); return ok }() {
			say(p, id)
		}
	}
	for _, p := range sortedKeys(numbered) {
		if it := lm.all[p]; it != nil && it.group.kind == ListTasks {
			say(p, ListSpec{Task: "number"}.ID())
		}
	}
	for _, g := range groups {
		for _, at := range g.paras {
			if d[at].S.ID == "" {
				st := *d[at].S
				st.ID = ids[g]
				d[at].S = &st
				break
			}
		}
	}
	return d
}

// sortedKeys are the keys of m, least first.
func sortedKeys[V any](m map[int]V) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// ListSpec is how a list looks (the app draws it).
type ListSpec struct {
	Mark string // a bulleted list's mark: an emoji, or "icon:" and an icon's name
	Num  string // a numbered list's numbers, or a list of tasks' with numbers: "roman" or "ROMAN"
	Task string // a list of tasks': "bullet" or "number" before each box
}

// ParseListSpec reads the spec of a paragraph's ID; false when the ID says none.
func ParseListSpec(id string) (ListSpec, bool) {
	rest, ok := strings.CutPrefix(id, ListIDPrefix)
	if !ok {
		return ListSpec{}, false
	}
	v, err := url.ParseQuery(rest)
	if err != nil {
		return ListSpec{}, false
	}
	return ListSpec{Mark: v.Get("mark"), Num: v.Get("num"), Task: v.Get("task")}, true
}

// ID is the spec as an item's ID: "" for a list of the usual look.
func (s ListSpec) ID() string {
	v := url.Values{}
	if s.Mark != "" {
		v.Set("mark", s.Mark)
	}
	if s.Num != "" {
		v.Set("num", s.Num)
	}
	if s.Task != "" {
		v.Set("task", s.Task)
	}
	if len(v) == 0 {
		return ""
	}
	return ListIDPrefix + v.Encode()
}

// listGroup is a list: the items of one level next to each other, of one kind, the deeper ones between them
// skipped, as the editor numbers them.
type listGroup struct {
	kind  int
	spec  ListSpec
	has   bool  // an item says the spec
	paras []int // the items' paragraphs, by index
}

// listItem is a paragraph's list and its place among the items of its level (from 1).
type listItem struct {
	n     int
	group *listGroup
}

// docLists is the lists of a document: every item's.
type docLists struct{ all map[int]*listItem }

// listsOf reads the lists of d: each item's list and the list's spec.
func listsOf(d Doc) docLists {
	m := docLists{all: map[int]*listItem{}}
	open := map[int]*listGroup{} // the list open at each level
	for i, para := range d {
		st := ParaStyle{}
		if para.S != nil {
			st = *para.S
		}
		if st.List == ListNone {
			clear(open)
			continue
		}
		for lvl := range open {
			if lvl > st.Indent {
				delete(open, lvl)
			}
		}
		g := open[st.Indent]
		if g == nil || g.kind != st.List {
			g = &listGroup{kind: st.List}
			open[st.Indent] = g
		}
		if s, ok := ParseListSpec(st.ID); ok && !g.has {
			g.spec, g.has = s, true
		}
		g.paras = append(g.paras, i)
		m.all[i] = &listItem{n: len(g.paras), group: g}
	}
	return m
}

// ---- writing ----

// PageMarkdown writes a page's document as its Markdown: its "check" objects as tasks in the running text, any other
// object as a picture (the app gives the others as their text first), and what the codec would lose as it was held.
func (d Doc) PageMarkdown() string {
	d, looks, marked := listTokensOut(slices.Clone(d))
	type span struct {
		s, e int // paragraphs, [s, e)
		h    heldBlock
	}
	var spans []span
	styleOf := func(p Para) ParaStyle {
		if p.S == nil {
			return ParaStyle{}
		}
		return *p.S
	}
	for i := 0; i < len(d); {
		st := styleOf(d[i])
		switch {
		case st.ID == HrID && d[i].text() == "":
			spans = append(spans, span{i, i + 1, heldBlock{kind: "hr"}})
			i++
		case st.CodeBlock && !st.Quote && st.Indent == 0 && st.List == ListNone:
			h := heldBlock{kind: "code"}
			if k, ok := strings.CutPrefix(st.ID, RawIDPrefix); ok {
				h.kind = k
			} else if lang, ok := strings.CutPrefix(st.ID, CodeIDPrefix); ok {
				h.lang = lang
			}
			var texts []string
			j := i
			for ; j < len(d); j++ {
				st2 := styleOf(d[j])
				if !st2.CodeBlock || st2.Quote || st2.Indent != 0 || st2.List != ListNone {
					break
				}
				if j > i && (strings.HasPrefix(st2.ID, CodeIDPrefix) || strings.HasPrefix(st2.ID, RawIDPrefix)) {
					break // the next block
				}
				texts = append(texts, d[j].text())
			}
			h.text = strings.Join(texts, "\n")
			spans = append(spans, span{i, j, h})
			i = j
		default:
			i++
		}
	}
	if len(spans) > 0 {
		var res Doc
		at := 0
		for k, sp := range spans {
			res = append(res, d[at:sp.s]...)
			res = append(res, Para{R: []Run{{T: blockToken(k)}}})
			at = sp.e
		}
		d = append(res, d[at:]...)
	}
	lines := strings.Split(d.WriteMarkdown(), "\n")
	markTaskKinds(d, lines)
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
		// the codec's own fences (code in a list or a quote): left as they are
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
		refX := RefDefX.MatchString(strings.Join(lines, "\n"))
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

// markTaskKinds writes the kinds of d's tasks in their boxes in md's lines, as the codec wrote them plain.
func markTaskKinds(d Doc, lines []string) {
	var kinds []string
	has := false
	for _, para := range d {
		if para.S == nil || para.S.List != ListTasks {
			continue
		}
		st := *para.S
		md, ok := "", false
		if id, isKind := strings.CutPrefix(st.ID, TaskIDPrefix); isKind {
			md, ok = taskMDOf(id)
		}
		has = has || ok
		if ok && !st.Checked {
			kinds = append(kinds, md)
		} else {
			kinds = append(kinds, "")
		}
	}
	if !has { // no task of a kind: nothing to write
		return
	}
	var at []int
	for i, l := range lines {
		if m := TaskBoxLine.FindStringSubmatchIndex(l); m != nil && (l[m[4]:m[5]] == " " || l[m[4]:m[5]] == "x") {
			at = append(at, i)
		}
	}
	if len(at) != len(kinds) {
		return
	}
	for i, li := range at {
		if kinds[i] != "" {
			m := TaskBoxLine.FindStringSubmatchIndex(lines[li])
			lines[li] = lines[li][:m[4]] + kinds[i] + lines[li][m[5]:]
		}
	}
}

// listTokensOut writes d's tasks in the running text as tokens, for the codec to write; and at the end of an item, a
// token for its list's look (on its first item) and its number (on each item of a numbered list of tasks). looks are
// what the comments say, by the numbers in their tokens; false when d has no tokens. A text that has their characters
// already writes its tasks in the running text as their boxes, as text.
func listTokensOut(d Doc) (Doc, []string, bool) {
	has, hasLook := false, false
	for _, para := range d {
		hasLook = hasLook || para.S != nil && strings.HasPrefix(para.S.ID, ListIDPrefix)
		for _, r := range para.R {
			if r.O == nil && strings.ContainsAny(r.T, listRunes) {
				return checksAsText(d), nil, false
			}
			if r.O != nil && r.O.K == "check" {
				has = true
			}
		}
	}
	if !has && !hasLook {
		return d, nil, false
	}
	if has {
		for i := range d {
			runs := make([]Run, 0, len(d[i].R))
			for _, r := range d[i].R {
				if r.O != nil && r.O.K == "check" {
					tok := tokCheck
					if r.O.Target == "x" {
						tok = tokChecked
					}
					r.O, r.T = nil, string(tok)
				}
				runs = append(runs, r)
			}
			d[i].R = mergeRuns(runs)
		}
	}
	if !hasLook {
		return d, nil, true
	}
	tail := map[int]string{} // the tokens at the end of a paragraph, by its index
	var looks []string
	lm := listsOf(d)
	seen := map[*listGroup]bool{}
	for _, p := range sortedKeys(lm.all) {
		g := lm.all[p].group
		if seen[g] || !g.has {
			continue
		}
		seen[g] = true
		raw := ""
		for _, q := range g.paras {
			if id := d[q].S.ID; strings.HasPrefix(id, ListIDPrefix) {
				if _, ok := ParseListSpec(id); ok {
					raw = strings.TrimPrefix(id, ListIDPrefix)
					break
				}
			}
		}
		if !safeLook.MatchString(raw) || strings.Contains(raw, "--") || strings.HasSuffix(raw, "-") { // said again, safe
			raw = strings.ReplaceAll(strings.TrimPrefix(g.spec.ID(), ListIDPrefix), "-", "%2D")
		}
		numbered := g.kind == ListTasks && g.spec.Task == "number"
		if !(numbered && ListIDPrefix+raw == (ListSpec{Task: "number"}).ID()) && !d[g.paras[0]].S.CodeBlock {
			tail[g.paras[0]] += listToken(tokLook, len(looks))
			looks = append(looks, raw)
		}
		if numbered {
			for k, q := range g.paras {
				if !d[q].S.CodeBlock {
					tail[q] += listToken(tokNumbered, k+1)
				}
			}
		}
	}
	for i, s := range tail {
		d[i].R = mergeRuns(append(slices.Clone(d[i].R), Run{T: s}))
	}
	return d, looks, true
}

// checksAsText is d with its tasks in the running text as the text of their boxes.
func checksAsText(d Doc) Doc {
	for i := range d {
		runs := make([]Run, 0, len(d[i].R))
		for _, r := range d[i].R {
			if r.O != nil && r.O.K == "check" {
				r.T, r.O = checkMarkdown(r.O.Target == "x"), nil
			}
			runs = append(runs, r)
		}
		d[i].R = mergeRuns(runs)
	}
	return d
}

// listMarksOut writes the tokens of lists' looks and numbered tasks in lines, the codec's Markdown: a look as the
// comment over its list's first item, and a numbered task's number in place of its bullet. The tokens are at the
// end of an item's text, which can run over lines: the item's line is the nearest over them that starts an item.
func listMarksOut(lines []string, looks []string) []string {
	out := make([]string, 0, len(lines)+len(looks))
	for _, l := range lines {
		if !strings.ContainsRune(l, tokLook) && !strings.ContainsRune(l, tokNumbered) {
			out = append(out, l)
			continue
		}
		look, num := -1, ""
		var sb strings.Builder
		for j := 0; j < len(l); {
			c, n := utf8.DecodeRuneInString(l[j:])
			if c == tokLook || c == tokNumbered {
				if end := strings.IndexRune(l[j+n:], tokEnd); end >= 0 {
					v := l[j+n : j+n+end]
					if c == tokLook {
						look, _ = strconv.Atoi(v)
					} else {
						num = v
					}
					j += n + end + utf8.RuneLen(tokEnd)
					continue
				}
			}
			sb.WriteString(l[j : j+n])
			j += n
		}
		out = append(out, sb.String())
		at := len(out) - 1
		for at > 0 && !ListItemLine.MatchString(out[at]) {
			at--
		}
		if !ListItemLine.MatchString(out[at]) {
			continue
		}
		if m := bulletMarker.FindStringSubmatchIndex(out[at]); m != nil && num != "" {
			out[at] = out[at][:m[3]] + num + "." + out[at][m[3]+1:]
		}
		if look >= 0 && look < len(looks) {
			mark := listLead.FindString(out[at]) + textnorm.ListMarkStart + looks[look] + "-->"
			out = append(out[:at], append([]string{mark}, out[at:]...)...)
		}
	}
	return out
}

// checksOut writes the tasks in the running text of line l, of Markdown: a bare box where it is read back as a
// task (InlineBoxes), else escaped, as text. refX says the page has a reference "[x]:".
func checksOut(l string, refX bool) string {
	for {
		i := strings.IndexAny(l, "\uE002\uE003")
		if i < 0 {
			return l
		}
		c, n := utf8.DecodeRuneInString(l[i:])
		bare, esc := "[ ]", `\[ \]`
		if c == tokChecked {
			bare, esc = "[x]", `\[x\]`
		}
		cand := l[:i] + bare + l[i+n:]
		if slices.Contains(InlineBoxes(cand, refX), i) || InCode(l, i) { // or in code, not escaped
			l = cand
		} else {
			l = l[:i] + esc + l[i+n:]
		}
	}
}

// InCode says whether the byte at i of l is in inline code, as InlineBoxes reads it: a backtick opens and closes
// code, but for one escaped outside it, which the codec writes for a backtick of the text (#589).
func InCode(l string, i int) bool {
	code := false
	for k := 0; k < i; k++ {
		if l[k] == '`' && (code || !EscapedAt(l, k)) {
			code = !code
		}
	}
	return code
}

// checksBare writes the tasks in the running text of s bare: in code, where nothing is escaped.
func checksBare(s string) string {
	if !strings.ContainsAny(s, "\uE002\uE003") {
		return s
	}
	return strings.NewReplacer(string(tokCheck), "[ ]", string(tokChecked), "[x]").Replace(s)
}

var (
	escapedLink = regexp.MustCompile(`\\\[\\\[(.+?)\\\]\\\]`)
	footRef     = regexp.MustCompile(`\\\[\^([^\]\\\s]+)\\\]`)
	// linkEscape is an escape the codec writes inside a [[link]]'s text
	linkEscape = regexp.MustCompile(`\\([\\_*~#!\[\]<>` + "`" + `])`)
)

// unescape takes back the escapes the codec writes where Markdown needs none, outside inline code: "!" not before
// "[", "_" inside a word, "#" starting a line without a space after it (no heading), and the brackets of [[links]]
// and footnote references.
func unescape(l string) string {
	if !strings.Contains(l, `\`) {
		return l
	}
	lead := len(l) - len(strings.TrimLeft(l, " >-*+0123456789.)[]x"))
	head, rest := l[:lead], l[lead:]
	if strings.HasPrefix(rest, `\#`) && len(rest) > 2 && rest[2] != ' ' && rest[2] != '#' {
		rest = rest[1:]
	}
	rest = OutsideCode(rest, func(s string) string {
		s = escapedLink.ReplaceAllStringFunc(s, func(m string) string { // its inside as it is typed, not escaped
			inner := escapedLink.FindStringSubmatch(m)[1]
			if strings.ContainsAny(inner, string(tokCheck)+string(tokChecked)) {
				// brackets around a task, which no link holds, stay text, and the task a task (#589)
				return m
			}
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
				case next == '_' && i > 0 && i+2 < len(r) && IsWord(r[i-1]) && IsWord(r[i+2]):
					continue
				}
			}
			sb.WriteRune(r[i])
		}
		return sb.String()
	})
	return head + rest
}

// IsWord is a letter or a digit.
func IsWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// ---- the tasks in the running text, as the task view reads them ----

// TaskSpot is a task in the running text as a page's Markdown has it: its line, where its box starts in the line
// (-1 when the line is read in another form, its box elsewhere), and whether it is done.
type TaskSpot struct {
	Line, Col int
	Done      bool
}

// InlineTaskSpots are the tasks in the running text of a page's Markdown md, in their order: the boxes
// PageFromMarkdown makes objects of. It reads the lines as that does, passing over what it holds.
func InlineTaskSpots(md string) []TaskSpot {
	if (!strings.Contains(md, "[ ]") && !strings.Contains(md, "[x]")) || strings.ContainsAny(md, listRunes) {
		return nil // no box, or a text read as before, without tokens: none is a task
	}
	md = strings.ReplaceAll(strings.TrimPrefix(md, "\uFEFF"), "\r\n", "\n")
	lines := strings.Split(md, "\n")
	refX := RefDefX.MatchString(md)
	blank := func(i int) bool { return i < 0 || strings.TrimSpace(lines[i]) == "" }
	var out []TaskSpot
	afterHeld := false
	for i := 0; i < len(lines); {
		l := lines[i]
		t := strings.TrimSpace(l)
		indent := len(l) - len(strings.TrimLeft(l, " "))
		if indent <= 3 && (strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")) {
			n := 0
			for n < len(t) && t[n] == t[0] {
				n++
			}
			fence, lang := t[:n], strings.TrimSpace(t[n:])
			if !(fence[0] == '`' && strings.Contains(lang, "`")) {
				j := i + 1
				for ; j < len(lines); j++ {
					if c := strings.TrimSpace(lines[j]); strings.HasPrefix(c, fence) && strings.Trim(c, fence[:1]) == "" {
						break
					}
				}
				i, afterHeld = j+1, true
				continue
			}
		}
		if _, ok := listMarkAt(lines, i); ok {
			i++
			continue
		}
		if indent <= 3 {
			if _, n := RawBlockOf(lines, i, blank(i-1), true); n > 0 {
				i, afterHeld = i+n, true
				continue
			}
			if hrLine.MatchString(l) && (blank(i-1) || afterHeld || !strings.Contains(l, "-")) {
				i, afterHeld = i+1, true
				continue
			}
		}
		read := protectInline(l)
		boxes, own := InlineBoxes(read, refX), InlineBoxes(l, refX)
		for k, b := range boxes {
			col := -1
			if len(own) == len(boxes) {
				col = own[k]
			}
			out = append(out, TaskSpot{i, col, read[b+1] == 'x'})
		}
		afterHeld = false
		i++
	}
	return out
}
