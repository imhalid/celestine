package library

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// The library's reader of Markdown into a document (#595): CommonMark with GitHub's extensions and footnotes, read by
// MyGo's parser (a package of its own that imports nothing of MyGo's UI) and made into paragraphs and runs as the
// editor's own reader made them (MyGo's richtext.ParseMarkdown, MIT, of which this is a port onto the library's
// document): headings, lists and task lists, quotations, blocks of code, bold, italic, strikethrough, code, links,
// images, and the HTML tags of these marks and of underlines. A list item's paragraphs after its first are indented
// under it; a table's rows are paragraphs of their cells apart by tabs, the header's bold. The app's own reading
// (markdown.go) holds what this would lose, and gives it the rest.

// maxIndent is the deepest a paragraph is indented, as the editor has it.
const maxIndent = 8

// ReadMarkdown is the document of Markdown s.
func ReadMarkdown(s string) Doc {
	// Invalid UTF-8 becomes U+FFFD: the halves of a broken sequence, apart in the source, would meet in the text as
	// one rune, which the runs count as two.
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	d := parser.New(parser.GFM | parser.ExtFootnotes)
	d.Set(s)
	m := mdReader{d: d}
	m.blocks(0, ParaStyle{}, -1)
	return m.b.doc()
}

// mdReader makes the blocks and inlines of a parsed document into a Doc.
type mdReader struct {
	d *parser.Document
	b docBuilder
	// html counts the inline HTML tags open, by the mark they turn on: <u>, <s>, <b> and <i> and their synonyms.
	html [4]int
}

// marks are a run's marks, as Markdown has them.
type marks struct {
	w       int
	i, u, x bool
	c       bool
	l       string
}

// run is a run of text s in marks m.
func (m marks) run(s string) Run { return Run{T: s, W: m.w, I: m.i, U: m.u, X: m.x, C: m.c, L: m.l} }

// docBuilder builds the paragraphs of a Doc.
type docBuilder struct {
	paras []Para
	cur   Para
	style ParaStyle
	open  bool
	// label is a footnote's prefix, put before the first paragraph of its definition.
	label string
}

// normalized is st with its levels in range, as the editor keeps them.
func normalized(st ParaStyle) ParaStyle {
	st.Heading = max(0, min(st.Heading, 6))
	st.Indent = max(0, min(st.Indent, maxIndent))
	if st.List > ListTasks || st.List < 0 {
		st.List = ListNone
	}
	if st.List != ListTasks {
		st.Checked = false
	}
	return st
}

// start opens a paragraph of style st.
func (b *docBuilder) start(st ParaStyle) {
	if b.open {
		b.end()
	}
	b.open, b.style, b.cur = true, st, Para{}
	if b.label != "" {
		b.addLine(b.label, marks{})
		b.label = ""
	}
}

// end closes the open paragraph.
func (b *docBuilder) end() {
	st := normalized(b.style)
	p := b.cur
	if st != (ParaStyle{}) {
		p.S = &st
	}
	b.paras = append(b.paras, p)
	b.open = false
}

// add puts s in marks a in the open paragraph: a newline in s starts another paragraph of the same style, without
// its ID.
func (b *docBuilder) add(s string, a marks) {
	for {
		line, rest, br := strings.Cut(s, "\n")
		b.addLine(line, a)
		if !br {
			return
		}
		b.end()
		st := b.style
		st.ID = ""
		b.start(st)
		s = rest
	}
}

// addLine puts a line (no newline in it) in marks a in the open paragraph, one run with the run before it when they
// have the same marks.
func (b *docBuilder) addLine(s string, a marks) {
	if s == "" {
		return
	}
	r := a.run(s)
	if n := len(b.cur.R); n > 0 && sameMarks(b.cur.R[n-1], r) {
		b.cur.R[n-1].T += s
		return
	}
	b.cur.R = append(b.cur.R, r)
}

// addObj puts object o in the open paragraph, in the marks of the text it is in but for a link and code, as the
// editor has it (the page file keeps no marks of an object: EncodeBody).
func (b *docBuilder) addObj(o *Obj, a marks) {
	a.l, a.c = "", false
	r := a.run(ObjectChar)
	r.O = o
	b.cur.R = append(b.cur.R, r)
}

func (b *docBuilder) doc() Doc {
	if b.open {
		b.end()
	}
	return b.paras
}

// blocks adds the children of block i in style st; depth is the level of the list they are in, -1 outside lists.
func (m *mdReader) blocks(i int, st ParaStyle, depth int) {
	for k := range m.d.Children(i) {
		m.block(k, st, depth)
	}
}

// quoteDepth is how many quotations deep a paragraph of style st is.
func quoteDepth(st ParaStyle) int {
	if !st.Quote {
		return 0
	}
	return max(1, st.QuoteDepth)
}

func (m *mdReader) block(i int, st ParaStyle, depth int) {
	b := m.d.Block(i)
	switch b.Kind {
	case parser.Paragraph:
		m.inlinePara(st, m.d.Inlines(i))
	case parser.Heading:
		st.Heading = b.Level
		m.inlinePara(st, m.d.Inlines(i))
	case parser.CodeBlock:
		st.CodeBlock, st.Heading = true, 0
		n := m.d.LineCount(i)
		for k := range n {
			line, _ := m.d.Line(i, k)
			m.b.start(st)
			m.b.add(line, marks{})
		}
		if n == 0 {
			m.b.start(st)
		}
	case parser.HTMLBlock:
		ps := ReadHTML(m.d.Lines(i))
		if len(ps) == 0 { // nothing shown (a comment): an empty paragraph in its place, as the editor's text has
			ps = Doc{{}}
		}
		for _, p := range ps {
			ps := ParaStyle{}
			if p.S != nil {
				ps = *p.S
			}
			d := quoteDepth(ps) + quoteDepth(st)
			ps.Quote = d > 0
			ps.QuoteDepth = 0
			if d > 1 {
				ps.QuoteDepth = d
			}
			ps.Indent += st.Indent
			m.b.start(ps)
			for _, r := range p.R {
				if r.O != nil {
					m.b.addObj(r.O, marks{w: r.W, i: r.I, u: r.U, x: r.X})
					continue
				}
				m.b.add(r.T, marks{w: r.W, i: r.I, u: r.U, x: r.X, c: r.C, l: r.L})
			}
		}
	case parser.Quote:
		if st.Quote {
			st.QuoteDepth = quoteDepth(st) + 1
		}
		st.Quote = true
		m.blocks(i, st, depth)
	case parser.List:
		kind := ListBullets
		if b.Ordered {
			kind = ListNumbers
		}
		for it := range m.d.Children(i) {
			ist := ParaStyle{List: kind, Indent: depth + 1, Quote: st.Quote, QuoteDepth: st.QuoteDepth}
			if task := m.d.Block(it).Task; task != parser.NoTask {
				ist.List, ist.Checked = ListTasks, task == parser.Checked
			}
			m.item(it, ist, depth+1)
		}
	case parser.Table:
		for row := range m.d.Children(i) {
			m.b.start(st)
			var a marks
			if m.d.Block(row).Header {
				a.w = 700
			}
			first := true
			for cell := range m.d.Children(row) {
				if !first {
					m.b.add("\t", a)
				}
				first = false
				m.html = [4]int{}
				in := m.d.Inlines(cell)
				m.inlines(in, 0, len(in), a, m.b.add)
			}
		}
	case parser.FootnoteDefinition:
		// The label goes before the first paragraph, after the labels of the definitions it is in.
		prev := m.b.label
		m.b.label = prev + "[" + b.Label() + "] "
		m.blocks(i, st, depth)
		if m.b.label != "" {
			// No paragraph took it: the next one takes the outer labels.
			m.b.label = prev
		}
	}
}

// item adds a list item: its first paragraph in the item's style ist, and what follows indented under it.
func (m *mdReader) item(i int, ist ParaStyle, depth int) {
	under := ParaStyle{Indent: depth + 1, Quote: ist.Quote, QuoteDepth: ist.QuoteDepth}
	first := true
	for k := range m.d.Children(i) {
		b := m.d.Block(k)
		if first {
			first = false
			if b.Kind == parser.Paragraph || b.Kind == parser.Heading {
				m.block(k, ist, depth)
				continue
			}
			// An item starting with code or a list has an empty line of its own, as it shows.
			m.b.start(ist)
		}
		m.block(k, under, depth)
	}
	if first {
		m.b.start(ist)
	}
}

// inlinePara adds a paragraph of inlines in style st; a hard line break starts another, indented under a list item
// rather than an item of its own.
func (m *mdReader) inlinePara(st ParaStyle, in []parser.Inline) {
	m.b.start(st)
	m.html = [4]int{}
	m.inlines(in, 0, len(in), marks{}, func(s string, a marks) {
		for {
			line, rest, br := strings.Cut(s, "\n")
			m.b.addLine(line, a)
			if !br {
				return
			}
			m.b.end()
			if st.List != ListNone {
				st = ParaStyle{Indent: st.Indent + 1, Quote: st.Quote, QuoteDepth: st.QuoteDepth}
			}
			m.b.start(st)
			s = rest
		}
	})
}

func (m *mdReader) inlines(in []parser.Inline, from, to int, a marks, emit func(string, marks)) {
	add := func(s string, a marks) {
		if m.html[0] > 0 {
			a.u = true
		}
		if m.html[1] > 0 {
			a.x = true
		}
		if m.html[2] > 0 {
			a.w = 700
		}
		if m.html[3] > 0 {
			a.i = true
		}
		emit(s, a)
	}
	for k := from; k < to; k = in[k].Next {
		n := &in[k]
		x := a
		switch n.Kind {
		case parser.Text:
			add(n.Text, a)
		case parser.SoftBreak:
			add(" ", a)
		case parser.LineBreak:
			add("\n", a)
		case parser.Code:
			x.c = true
			add(n.Text, x)
		case parser.Emphasis:
			x.i = true
			m.inlines(in, k+1, n.Next, x, emit)
		case parser.Strong:
			x.w = 700
			m.inlines(in, k+1, n.Next, x, emit)
		case parser.Strikethrough:
			x.x = true
			m.inlines(in, k+1, n.Next, x, emit)
		case parser.Link:
			x.l = safeLink(n.Dest)
			m.inlines(in, k+1, n.Next, x, emit)
		case parser.Image:
			m.b.addObj(&Obj{K: "image", Alt: parser.PlainText(in[k+1 : n.Next]), Ref: safeLink(n.Dest)}, a)
		case parser.HTML:
			m.tag(n.Text, a, emit)
		case parser.FootnoteRef:
			add("["+n.Text+"]", a)
		}
	}
}

// tag applies an inline HTML tag: the opening and closing tags of underlines, strikethrough, bold and italic, and
// line breaks. Other tags are left out, as a browser shows none.
func (m *mdReader) tag(s string, a marks, emit func(string, marks)) {
	s = strings.ToLower(strings.TrimSpace(s))
	if !strings.HasPrefix(s, "<") || !strings.HasSuffix(s, ">") {
		return
	}
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "<"), ">"))
	closing := strings.HasPrefix(s, "/")
	s = strings.TrimPrefix(s, "/")
	s = strings.TrimSuffix(s, "/")
	name, _, _ := strings.Cut(strings.TrimSpace(s), " ")
	which := -1
	switch name {
	case "br":
		emit("\n", a)
		return
	case "u", "ins":
		which = 0
	case "s", "strike", "del":
		which = 1
	case "b", "strong":
		which = 2
	case "i", "em":
		which = 3
	}
	if which < 0 {
		return
	}
	if closing {
		m.html[which] = max(m.html[which]-1, 0)
	} else {
		m.html[which]++
	}
}

// safeLink is a link's URL, or "" for one that runs a script where a browser opens it: javascript:, vbscript: and
// data:, written as browsers read them, with spaces before and tabs or newlines within.
func safeLink(s string) string {
	s = strings.TrimFunc(s, func(r rune) bool { return r <= ' ' })
	scheme := strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
	for _, bad := range []string{"javascript:", "vbscript:", "data:"} {
		if strings.HasPrefix(scheme, bad) {
			return ""
		}
	}
	return s
}
