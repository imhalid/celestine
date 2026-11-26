package library

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The library's writer of a document as Markdown (#595), a port of MyGo's richtext Text.Markdown (MIT) onto the
// library's document: CommonMark with GitHub's extensions, headings, lists and task lists, quotations, fenced blocks
// of code, **bold**, *italic*, ~~strikethrough~~, `code`, links and pictures; underlines are HTML's <u>, which
// Markdown keeps. Body paragraphs are apart by a blank line, so that they stay paragraphs. An object is written as a
// picture: the app's own writing (markdown.go) gives this its objects as their text first.

// WriteMarkdown is the document as Markdown.
func (d Doc) WriteMarkdown() string {
	var b strings.Builder
	// numbers counts the items of the numbered lists open, by level.
	var numbers []int
	prevList := false
	styleOf := func(p Para) ParaStyle {
		if p.S == nil {
			return ParaStyle{}
		}
		return *p.S
	}
	for i := 0; i < len(d); i++ {
		p := d[i]
		st := styleOf(p)
		if i > 0 && !(st.List != ListNone && prevList) {
			b.WriteString("\n\n")
		} else if i > 0 {
			b.WriteByte('\n')
		}
		prevList = st.List != ListNone
		if st.List == ListNone {
			numbers = numbers[:0]
		}
		quote := ""
		if st.Quote {
			quote = strings.Repeat("> ", quoteDepth(st))
		}
		switch {
		case st.CodeBlock:
			fence := "```"
			for k := i; k < len(d) && styleOf(d[k]).CodeBlock; k++ {
				for strings.Contains(d[k].text(), fence) {
					fence += "`"
				}
			}
			b.WriteString(quote + fence + "\n")
			for ; i < len(d) && styleOf(d[i]).CodeBlock; i++ {
				b.WriteString(quote + d[i].text() + "\n")
			}
			i--
			b.WriteString(quote + fence)
			continue
		case st.Heading > 0:
			b.WriteString(quote + strings.Repeat("#", min(st.Heading, 6)) + " ")
		case st.List != ListNone:
			level := st.Indent
			for len(numbers) <= level {
				numbers = append(numbers, 0)
			}
			numbers = numbers[:level+1]
			b.WriteString(quote + strings.Repeat("    ", level))
			switch st.List {
			case ListNumbers:
				numbers[level]++
				b.WriteString(strconv.Itoa(numbers[level]) + ". ")
			case ListTasks:
				if st.Checked {
					b.WriteString("- [x] ")
				} else {
					b.WriteString("- [ ] ")
				}
				numbers[level] = 0
			default:
				b.WriteString("- ")
				numbers[level] = 0
			}
		default:
			b.WriteString(quote)
		}
		if st.Heading > 0 {
			// A heading's closing #s are not its text: escape them.
			var h strings.Builder
			writeMarkdownRuns(&h, p.R)
			s := h.String()
			if k := len(strings.TrimRight(s, "#")); k < len(s) {
				s = s[:k] + "\\" + s[k:]
			}
			b.WriteString(s)
			continue
		}
		writeMarkdownRuns(&b, p.R)
	}
	return b.String()
}

// mark is a Markdown mark around runs: a link (by its URL), bold, italic, strikethrough or underline, from the
// outside in. Code is a span of each run, fenced as its backticks need.
type mark struct {
	kind byte // 'a', 'b', 'i', 's', 'u'
	url  string
}

func marksOf(r Run) []mark {
	var m []mark
	if r.L != "" {
		m = append(m, mark{kind: 'a', url: r.L})
	}
	if r.W >= 600 {
		m = append(m, mark{kind: 'b'})
	}
	if r.I {
		m = append(m, mark{kind: 'i'})
	}
	if r.X {
		m = append(m, mark{kind: 's'})
	}
	if r.U {
		m = append(m, mark{kind: 'u'})
	}
	return m
}

// open is the mark's opening, as HTML when html: what Markdown cannot open or close where it is, as bold "foo."
// before "bar".
func (m mark) open(html bool) string {
	switch m.kind {
	case 'a':
		return "["
	case 'b':
		if html {
			return "<strong>"
		}
		return "**"
	case 'i':
		if html {
			return "<em>"
		}
		return "*"
	case 's':
		if html {
			return "<del>"
		}
		return "~~"
	case 'u':
		return "<u>"
	}
	return ""
}

func (m mark) close(html bool) string {
	switch m.kind {
	case 'a':
		return "](" + markdownURL(m.url) + ")"
	case 'u':
		return "</u>"
	}
	if html {
		return "</" + m.open(true)[1:]
	}
	return m.open(false)
}

// writeMarkdownRuns writes runs with their marks, opening and closing them as they change, whitespace outside them
// so that they flank words as CommonMark wants. Where a delimiter cannot open or close, its mark is written as HTML,
// which Markdown keeps: the runs written are read back to check, and those still read otherwise are written in HTML
// whole. What would start a block at the start of the paragraph is escaped.
func writeMarkdownRuns(b *strings.Builder, runs []Run) {
	s, delims := renderRuns(runs, nil)
	if len(delims) > 0 && !readsAs(s, runs) {
		html := map[delimKey]bool{}
		for _, d := range delims {
			if !d.flanks(s) {
				html[d.key] = true
			}
		}
		if len(html) > 0 {
			s2, _ := renderRuns(runs, html)
			if readsAs(s2, runs) {
				b.WriteString(s2)
				return
			}
		}
		for _, d := range delims {
			html[d.key] = true
		}
		s, _ = renderRuns(runs, html)
	}
	b.WriteString(s)
}

// delimKey names a mark written in Markdown: the run it opens at and its kind.
type delimKey struct {
	run  int
	kind byte
}

// delim is where a mark's delimiters are in what renderRuns wrote.
type delim struct {
	key                 delimKey
	char                byte
	open, close, length int
}

// flanks reports whether the delimiters can open and close where they are, as CommonMark's rules of flanking say.
func (d delim) flanks(s string) bool {
	return canFlank(s, d.open, d.length, d.char, true) && canFlank(s, d.close, d.length, d.char, false)
}

func canFlank(s string, at, n int, c byte, left bool) bool {
	from, to := at, at+n
	for from > 0 && s[from-1] == c {
		from--
	}
	for to < len(s) && s[to] == c {
		to++
	}
	before, after := ' ', ' '
	if from > 0 {
		before, _ = utf8.DecodeLastRuneInString(s[:from])
	}
	if to < len(s) {
		after, _ = utf8.DecodeRuneInString(s[to:])
	}
	if !left {
		before, after = after, before
	}
	return !unicode.IsSpace(after) && (!isMarkdownPunct(after) || unicode.IsSpace(before) || isMarkdownPunct(before))
}

func isMarkdownPunct(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }

// renderRuns writes runs, the marks of html in HTML, and says where the delimiters of the others are.
func renderRuns(runs []Run, html map[delimKey]bool) (string, []delim) {
	var b strings.Builder
	type opened struct {
		mark
		key delimKey
		at  int
	}
	var open []opened
	var delims []delim
	pending := ""
	first := true
	closeTo := func(n int) {
		for len(open) > n {
			o := open[len(open)-1]
			h := html[o.key]
			if c := o.close(h); !h && (o.kind == 'b' || o.kind == 'i' || o.kind == 's') {
				delims = append(delims, delim{key: o.key, char: c[0], open: o.at, close: b.Len(), length: len(c)})
			}
			b.WriteString(o.close(h))
			open = open[:len(open)-1]
		}
	}
	for i, r := range runs {
		if o := r.O; o != nil {
			closeTo(0)
			b.WriteString(pending)
			pending = ""
			b.WriteString("![" + escapeMarkdown(o.Alt) + "](" + markdownURL(o.Ref) + ")")
			first = false
			continue
		}
		core := strings.TrimFunc(r.T, unicode.IsSpace)
		lead, trail := "", ""
		switch {
		case r.C && r.T != "":
			// A code span keeps its spaces, at its ends too, and one of spaces alone.
			core = r.T
		case core == "":
			pending += r.T
			continue
		default:
			lead = r.T[:strings.Index(r.T, core)]
			trail = r.T[len(lead)+len(core):]
		}
		want := marksOf(r)
		k := 0
		for k < len(open) && k < len(want) && open[k].mark == want[k] {
			k++
		}
		closeTo(k)
		if first {
			// Spaces at the start would make a block of code.
			lead = strings.NewReplacer(" ", "&#32;", "\t", "&#9;").Replace(pending + lead)
			pending = ""
		}
		b.WriteString(pending + lead)
		for _, m := range want[k:] {
			key := delimKey{i, m.kind}
			open = append(open, opened{m, key, b.Len()})
			b.WriteString(m.open(html[key]))
		}
		if r.C {
			b.WriteString(codeSpan(core))
		} else {
			s := escapeMarkdown(core)
			if first {
				s = escapeBlockStart(s)
			}
			if strings.HasSuffix(s, "!") {
				// Not a picture with a link after it.
				s = s[:len(s)-1] + "\\!"
			}
			b.WriteString(s)
		}
		first = false
		pending = trail
	}
	closeTo(0)
	b.WriteString(pending)
	return b.String(), delims
}

// readsAs reports whether s reads back as runs, as far as Markdown keeps them: their text, and the marks of all but
// whitespace.
func readsAs(s string, runs []Run) bool {
	d := ReadMarkdown(s)
	if len(d) > 1 {
		return false
	}
	var got []Run
	if len(d) == 1 {
		got = d[0].R
	}
	return sameMarkRuns(got, runs)
}

// runMarks are the marks of a character that Markdown keeps.
type runMarks struct {
	bold, italic, strike, under, code bool
	link, atName, atURL               string
}

// markScanner walks the runes of runs and the marks Markdown keeps for each, skipping trailing spaces and tabs
// outside code.
type markScanner struct {
	runs []Run
	ri   int
	bi   int
	rem  int
	cur  runMarks
}

func newMarkScanner(runs []Run) markScanner {
	rem := 0
	for _, r := range runs {
		rem += len(r.T)
	}
	for i := len(runs) - 1; i >= 0; i-- {
		text := runs[i].T
		if text == "" {
			continue
		}
		if runs[i].C {
			break
		}
		j := len(text)
		for j > 0 && (text[j-1] == ' ' || text[j-1] == '\t') {
			j--
			rem--
		}
		if j > 0 {
			break
		}
	}
	return markScanner{runs: runs, rem: rem}
}

func (s *markScanner) next() (rune, runMarks, bool) {
	for s.rem > 0 && s.ri < len(s.runs) {
		r := &s.runs[s.ri]
		if s.bi >= len(r.T) {
			s.ri++
			s.bi = 0
			continue
		}
		if s.bi == 0 {
			s.cur = runMarks{bold: r.W >= 600, italic: r.I, strike: r.X, under: r.U, code: r.C, link: r.L}
			if r.O != nil {
				s.cur.atName, s.cur.atURL = r.O.Alt, r.O.Ref
			}
		}
		c, size := utf8.DecodeRuneInString(r.T[s.bi:])
		s.bi += size
		s.rem -= size
		m := s.cur
		if unicode.IsSpace(c) && !m.code {
			m = runMarks{}
		}
		return c, m, true
	}
	return 0, runMarks{}, false
}

func sameMarkRuns(a, b []Run) bool {
	sa, sb := newMarkScanner(a), newMarkScanner(b)
	for {
		ca, ma, oka := sa.next()
		cb, mb, okb := sb.next()
		if oka != okb {
			return false
		}
		if !oka {
			return true
		}
		if ca != cb || ma != mb {
			return false
		}
	}
}

// codeSpan is s as a code span, fenced by more backticks than any run of them in it.
func codeSpan(s string) string {
	longest, n := 0, 0
	for _, r := range s {
		if r == '`' {
			n++
			longest = max(longest, n)
		} else {
			n = 0
		}
	}
	fence := strings.Repeat("`", longest+1)
	spaced := strings.HasPrefix(s, " ") && strings.HasSuffix(s, " ") && strings.Trim(s, " ") != ""
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") || spaced {
		s = " " + s + " "
	}
	return fence + s + fence
}

// escapeMarkdown escapes what Markdown would read as marks.
func escapeMarkdown(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch r {
		case '\\', '*', '_', '`', '[', ']', '<', '~':
			b.WriteByte('\\')
		case '&':
			// Not the start of an entity, as &amp; or &#38;.
			if i+1 < len(s) && (s[i+1] == '#' || unicode.IsLetter(rune(s[i+1]))) {
				b.WriteByte('\\')
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

// escapeBlockStart escapes the start of a body paragraph that Markdown would read as a heading, a list, a quotation
// or a rule.
func escapeBlockStart(s string) string {
	switch {
	case strings.HasPrefix(s, "#"), strings.HasPrefix(s, ">"), strings.HasPrefix(s, "-"), strings.HasPrefix(s, "+"), strings.HasPrefix(s, "="):
		return "\\" + s
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s) && (s[i] == '.' || s[i] == ')') {
		return s[:i] + "\\" + s[i:]
	}
	return s
}

// markdownURL is a URL as the destination of a link.
func markdownURL(u string) string {
	if strings.ContainsAny(u, " ()<>") {
		return "<" + strings.NewReplacer("<", "%3C", ">", "%3E").Replace(u) + ">"
	}
	return u
}
