package library

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/gif" // the pictures an HTML block can hold in a data: URI
	_ "image/jpeg"
	_ "image/png"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ReadHTML is the document of HTML s, as Markdown's HTML blocks bring it to the reader (ReadMarkdown): paragraphs,
// headings, lists, task lists, quotations, blocks of code, alignment, bold, italic, underline, strikethrough, code,
// links and pictures; fonts, sizes and colors are left out. It is a port of MyGo's richtext.ParseHTML (MIT) onto the
// library's document. A picture in a data: URI is one with no reference, which the page file does not keep.
func ReadHTML(s string) Doc {
	root, err := xhtml.Parse(strings.NewReader(s))
	if err != nil {
		var d Doc
		for _, l := range strings.Split(s, "\n") {
			d = append(d, Para{R: []Run{{T: l}}})
		}
		return d
	}
	b := htmlBuilder{space: true}
	b.node(root, marks{})
	b.flush()
	paras := b.paras
	// Leading and trailing empty paragraphs are the document's margins.
	for len(paras) > 1 && len(paras[len(paras)-1].R) == 0 {
		paras = paras[:len(paras)-1]
	}
	return paras
}

// The alignments of a paragraph, as the page file numbers them (the editor's).
const (
	alignStart  = 0
	alignCenter = 1
	alignEnd    = 2
)

// htmlBuilder makes the paragraphs of HTML.
type htmlBuilder struct {
	paras   []Para
	cur     Para
	started bool
	space   bool // after whitespace, which collapses into one space
	br      bool // after a <br>, until the next flush: a <br> last in a block starts no line
	lists   []int
	style   ParaStyle
	pre     int
}

// flush ends the paragraph being built.
func (b *htmlBuilder) flush() {
	b.br = false
	if !b.started {
		return
	}
	p := b.cur
	// Trailing whitespace of a paragraph is not shown.
	if n := len(p.R); n > 0 && b.pre == 0 {
		p.R[n-1].T = strings.TrimRight(p.R[n-1].T, " ")
		if p.R[n-1].T == "" && p.R[n-1].O == nil {
			p.R = p.R[:n-1]
		}
	}
	if p.S != nil {
		st := normalized(*p.S)
		p.S = &st
		if st == (ParaStyle{}) {
			p.S = nil
		}
	}
	b.paras = append(b.paras, p)
	b.cur, b.started, b.space = Para{}, false, true
}

// begin starts a paragraph of the style, unless one is being built.
func (b *htmlBuilder) begin() {
	if !b.started {
		st := b.style
		b.cur = Para{S: &st}
		b.started = true
	}
}

// text adds text in marks a.
func (b *htmlBuilder) text(s string, a marks) {
	if b.pre > 0 {
		for i, line := range strings.Split(s, "\n") {
			if i > 0 {
				b.begin()
				b.flush()
			}
			if line != "" {
				b.begin()
				b.cur.R = append(b.cur.R, a.run(line))
			}
		}
		return
	}
	var out strings.Builder
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' || r == '\f' {
			if !b.space {
				out.WriteByte(' ')
				b.space = true
			}
			continue
		}
		if r == ' ' {
			r = ' '
		}
		out.WriteRune(r)
		b.space = false
	}
	if out.Len() == 0 {
		return
	}
	b.begin()
	t := out.String()
	if len(b.cur.R) == 0 {
		t = strings.TrimLeft(t, " ")
		if t == "" {
			return
		}
	}
	b.cur.R = append(b.cur.R, a.run(t))
}

// node adds n and what it holds, in marks a.
func (b *htmlBuilder) node(n *xhtml.Node, a marks) {
	switch n.Type {
	case xhtml.TextNode:
		b.text(n.Data, a)
		return
	case xhtml.ElementNode:
	default:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.node(c, a)
		}
		return
	}
	saved := b.style
	block := false
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Head, atom.Title, atom.Template, atom.Noscript:
		return
	case atom.Br:
		b.begin()
		b.flush()
		b.br = true
		return
	case atom.Img:
		src, alt := attr(n, "src"), attr(n, "alt")
		if data, ok := strings.CutPrefix(src, "data:"); ok {
			if _, enc, ok := strings.Cut(data, ";base64,"); ok {
				if raw, err := base64.StdEncoding.DecodeString(enc); err == nil && smallImage(raw) {
					src = "" // a picture the editor shows from its data, which the page file does not keep (EncodeBody)
				}
			}
		}
		b.begin()
		r := Run{T: ObjectChar, W: a.w, I: a.i, U: a.u, X: a.x, O: &Obj{K: "image", Alt: alt, Ref: src}}
		b.cur.R = append(b.cur.R, r)
		b.space = false
		return
	case atom.Input:
		if strings.EqualFold(attr(n, "type"), "checkbox") {
			b.begin()
			b.cur.S.List = ListTasks
			_, checked := hasAttr(n, "checked")
			b.cur.S.Checked = checked
			b.space = true
		}
		return
	case atom.B, atom.Strong:
		a.w = 700
	case atom.I, atom.Em, atom.Cite, atom.Dfn:
		a.i = true
	case atom.U, atom.Ins:
		a.u = true
	case atom.S, atom.Strike, atom.Del:
		a.x = true
	case atom.Code, atom.Kbd, atom.Samp, atom.Tt:
		if b.pre == 0 {
			a.c = true
		}
	case atom.A:
		a.l = safeLink(attr(n, "href"))
	case atom.Ul, atom.Ol:
		b.flush()
		kind := ListBullets
		if n.DataAtom == atom.Ol {
			kind = ListNumbers
		}
		b.lists = append(b.lists, kind)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.node(c, a)
		}
		b.lists = b.lists[:len(b.lists)-1]
		b.flush()
		b.style = saved
		return
	case atom.Li:
		b.flush()
		kind := ListBullets
		if k := len(b.lists); k > 0 {
			kind = b.lists[k-1]
		}
		b.style = ParaStyle{List: kind, Indent: max(len(b.lists)-1, 0), Align: saved.Align, Quote: saved.Quote, QuoteDepth: saved.QuoteDepth}
		block = true
	case atom.P, atom.Div, atom.Section, atom.Article, atom.Header, atom.Footer, atom.Main, atom.Aside, atom.Nav,
		atom.Figure, atom.Figcaption, atom.Tr, atom.Dt, atom.Dd, atom.Address, atom.Center:
		b.flush()
		block = true
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		b.flush()
		b.style = ParaStyle{Heading: int(n.Data[1] - '0'), Align: saved.Align, Quote: saved.Quote, QuoteDepth: saved.QuoteDepth}
		block = true
	case atom.Blockquote:
		b.flush()
		if b.style.Quote {
			b.style.QuoteDepth = quoteDepth(b.style) + 1
		}
		b.style.Quote = true
		block = true
	case atom.Pre:
		b.flush()
		b.style = ParaStyle{CodeBlock: true, Indent: saved.Indent, Quote: saved.Quote, QuoteDepth: saved.QuoteDepth}
		b.pre++
		defer func() { b.pre-- }()
		block = true
	case atom.Td, atom.Th:
		b.text(" ", a)
	}
	if style := attr(n, "style"); style != "" {
		a = applyCSS(style, a, &b.style, block)
	}
	if block {
		switch strings.ToLower(attr(n, "align")) {
		case "center":
			b.style.Align = alignCenter
		case "right":
			b.style.Align = alignEnd
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.node(c, a)
	}
	if block {
		if n.DataAtom == atom.P && !b.started && !b.br && b.pre == 0 {
			// An empty paragraph shows as one; <p><br></p> as one too.
			b.begin()
		}
		b.flush()
		b.style = saved
	}
}

// applyCSS applies the declarations of a style attribute to a run's marks and, for a block, its paragraph's style:
// weights, italics, lines and alignment.
func applyCSS(style string, a marks, ps *ParaStyle, block bool) marks {
	for _, decl := range strings.Split(style, ";") {
		name, value, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(strings.ToLower(name))
		value = strings.TrimSpace(strings.ToLower(value))
		value = strings.TrimSuffix(strings.TrimSpace(strings.TrimSuffix(value, "!important")), ";")
		switch name {
		case "font-weight":
			switch value {
			case "bold", "bolder":
				a.w = 700
			case "normal", "lighter":
				a.w = 0
			default:
				if w, err := strconv.Atoi(value); err == nil {
					if w >= 600 {
						a.w = 700
					} else {
						a.w = 0
					}
				}
			}
		case "font-style":
			a.i = value == "italic" || value == "oblique"
		case "text-decoration", "text-decoration-line":
			a.u = a.u || strings.Contains(value, "underline")
			a.x = a.x || strings.Contains(value, "line-through")
			if value == "none" {
				a.u, a.x = false, false
			}
		case "text-align":
			if block {
				switch value {
				case "center":
					ps.Align = alignCenter
				case "right", "end":
					ps.Align = alignEnd
				case "left", "start", "justify":
					ps.Align = alignStart
				}
			}
		}
	}
	return a
}

// maxImagePixels is how many pixels a picture may have to be decoded.
const maxImagePixels = 1 << 25

// smallImage reports whether the picture of data is small enough to decode.
func smallImage(data []byte) bool {
	c, _, err := image.DecodeConfig(bytes.NewReader(data))
	return err == nil && c.Width > 0 && c.Height > 0 && int64(c.Width)*int64(c.Height) <= maxImagePixels
}

func attr(n *xhtml.Node, key string) string {
	v, _ := hasAttr(n, key)
	return v
}

func hasAttr(n *xhtml.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val, true
		}
	}
	return "", false
}
