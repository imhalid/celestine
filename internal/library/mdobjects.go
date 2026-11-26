package library

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/imhalid/celestine/internal/table"
)

// The objects of a page in its Markdown (#595): a link to a page or a day ([[Page]], [[2026-11-26]]), a page's card
// (![[Page]] on a line of its own), a footnote's reference ([^1]), a table (a fenced block of celestine-table, or a
// Markdown table other tools write), a query (a fenced block of query), and a block of a kind this version does not
// know (a comment no reader shows, holding it as the file has it). ObjectsIn makes them objects of the document read
// from Markdown; ObjectsAsText writes them back as their text. This was the app's (chips.go, embed.go, footnote.go,
// tables.go, queryblock.go, blockmark.go); the app now only dresses the objects to be drawn.

// The languages of the fenced blocks a table and a query are written as.
const (
	TableLang = table.Lang
	QueryLang = "query"
)

// blockMarkStart starts the comment that holds a block of a kind this version does not know.
const blockMarkStart = "<!--celestine:block "

var (
	blockMarkLine = regexp.MustCompile(`^<!--celestine:block ([A-Za-z0-9+/]+)-->$`)
	wikiInText    = regexp.MustCompile(`\[\[([^\[\]\n]+?)\]\]`)
	embedLine     = regexp.MustCompile(`^!\[\[([^\[\]|\n]+)\]\]$`)
	footRefInText = regexp.MustCompile(`\[\^([^\]\s]+)\]`)
	isDay         = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// ImportMarkdown reads Markdown that comes in from outside (a file imported or dropped, what an agent writes, a
// template) into a page's document with its objects.
func ImportMarkdown(md string) Doc { return ObjectsIn(PageFromMarkdown(md)) }

// ExportMarkdown writes a page's document, its objects as their text, as its Markdown.
func (d Doc) ExportMarkdown() string { return ObjectsAsText(d).PageMarkdown() }

// ObjectsIn turns what Markdown says of the objects in d into objects, in the order the app made them: the blocks
// of newer kinds, tables, queries, cards, links, footnotes' references.
func ObjectsIn(d Doc) Doc {
	return footnotesIn(linksIn(embedsIn(queriesIn(tablesIn(blockMarksIn(d))))))
}

// style is p's style, the zero one for none.
func (p Para) style() ParaStyle {
	if p.S == nil {
		return ParaStyle{}
	}
	return *p.S
}

// runeLen is how many characters run r has, an object one.
func runeLen(r Run) int {
	if r.O != nil {
		return 1
	}
	return utf8.RuneCountInString(r.T)
}

// marksAt is the marks of the character at i of p (no text, no object), as the editor's text has them there.
func (p Para) marksAt(i int) Run {
	at := 0
	for _, r := range p.R {
		n := runeLen(r)
		if i < at+n {
			r.T, r.O = "", nil
			return r
		}
		at += n
	}
	if n := len(p.R); n > 0 { // at the end: the last run's
		r := p.R[n-1]
		r.T, r.O = "", nil
		return r
	}
	return Run{}
}

// splice is p's runs with the characters [from, to) in place of repl.
func (p Para) splice(from, to int, repl ...Run) []Run {
	var out []Run
	at := 0
	put := false
	for _, r := range p.R {
		n := runeLen(r)
		s, e := at, at+n
		at = e
		if e <= from || s >= to {
			if s >= to && !put {
				out, put = append(out, repl...), true
			}
			out = append(out, r)
			continue
		}
		if r.O == nil { // the part of the run outside the range stays
			rs := []rune(r.T)
			if s < from {
				head := r
				head.T = string(rs[:from-s])
				out = append(out, head)
			}
			if !put {
				out, put = append(out, repl...), true
			}
			if e > to {
				tail := r
				tail.T = string(rs[to-s:])
				out = append(out, tail)
			}
			continue
		}
		if !put {
			out, put = append(out, repl...), true
		}
	}
	if !put {
		out = append(out, repl...)
	}
	return mergeRuns(out)
}

// runeAt is the character offset of byte offset b of s.
func runeAt(s string, b int) int { return utf8.RuneCountInString(s[:b]) }

// inCode reports whether the character at i of p is code, or p a block of code.
func (p Para) inCode(i int) bool { return p.style().CodeBlock || p.marksAt(i).C }

// objectsByRegexp turns each match of re in each paragraph of d, from the last, into the object make says, in the
// marks of its first character; make says false to leave a match as it is.
func objectsByRegexp(d Doc, re *regexp.Regexp, mk func(p Para, m []int, s string, from int) (*Obj, bool)) Doc {
	for pi := range d {
		p := d[pi]
		s := p.text()
		if !re.MatchString(s) {
			continue
		}
		locs := re.FindAllStringSubmatchIndex(s, -1)
		for k := len(locs) - 1; k >= 0; k-- {
			m := locs[k]
			from, to := runeAt(s, m[0]), runeAt(s, m[1])
			o, ok := mk(p, m, s, from)
			if !ok {
				continue
			}
			r := p.marksAt(from)
			r.T, r.O = ObjectChar, o
			p.R = p.splice(from, to, r)
		}
		d[pi] = p
	}
	return d
}

// linksIn turns the [[links]] of d, outside code, into objects of pages and days.
func linksIn(d Doc) Doc {
	return objectsByRegexp(d, wikiInText, func(p Para, m []int, s string, from int) (*Obj, bool) {
		if p.inCode(from) {
			return nil, false
		}
		return LinkOf(s[m[2]:m[3]]), true
	})
}

// LinkOf is the object of a link's inside: "Dune", "Dune|the book", "2026-11-05".
func LinkOf(inside string) *Obj {
	target, alias, _ := strings.Cut(inside, "|")
	target = strings.TrimSpace(target)
	if isDay.MatchString(target) && alias == "" {
		return &Obj{K: "date", Target: target}
	}
	return &Obj{K: "page", Target: target, Alias: strings.TrimSpace(alias)}
}

// embedsIn turns the lines ![[Page]] of d that stand alone into cards.
func embedsIn(d Doc) Doc {
	return objectsByRegexp(d, embedLine, func(p Para, m []int, s string, from int) (*Obj, bool) {
		if p.style().CodeBlock {
			return nil, false
		}
		return &Obj{K: "embed", Target: strings.TrimSpace(s[m[2]:m[3]])}, true
	})
}

// footnotesIn turns the references of d into objects, outside code, and not the one that starts a note.
func footnotesIn(d Doc) Doc {
	return objectsByRegexp(d, footRefInText, func(p Para, m []int, s string, from int) (*Obj, bool) {
		if p.inCode(from) || from == 0 && strings.HasPrefix(s[m[1]:], ":") {
			return nil, false
		}
		return &Obj{K: "footnote", Target: s[m[2]:m[3]]}, true
	})
}

// blocksOf turns the blocks of code of d that start with an ID wanted says yes to, their lines with no ID after
// them, into one paragraph holding the object make makes of their lines; nil leaves them.
func blocksOf(d Doc, wanted func(id string) bool, mk func(id string, lines []string) *Obj) Doc {
	var out Doc
	for i := 0; i < len(d); i++ {
		st := d[i].style()
		if !st.CodeBlock || !wanted(st.ID) {
			out = append(out, d[i])
			continue
		}
		lines := []string{d[i].text()}
		j := i + 1
		for ; j < len(d); j++ {
			if st2 := d[j].style(); !st2.CodeBlock || st2.ID != "" {
				break
			}
			lines = append(lines, d[j].text())
		}
		o := mk(st.ID, lines)
		if o == nil {
			out = append(out, d[i])
			continue
		}
		out = append(out, Para{R: []Run{{T: ObjectChar, O: o}}})
		i = j - 1
	}
	return out
}

// tablesIn turns the tables of d into objects: the fenced blocks the app writes, and the Markdown tables other tools
// write (made tables of their own, written as such from now on).
func tablesIn(d Doc) Doc {
	return blocksOf(d, func(id string) bool { return id == RawIDPrefix+"table" || id == CodeIDPrefix+TableLang },
		func(id string, lines []string) *Obj {
			var m *table.Model
			if id == RawIDPrefix+"table" {
				m = table.FromMarkdown(lines)
			} else if tm, ok := table.Parse(strings.Join(lines, "\n")); ok {
				m = tm
			}
			if m == nil {
				return nil
			}
			return &Obj{K: "table", Cols: m.Cols, Rows: m.Rows, Q: m.Filter}
		})
}

// queriesIn turns the fenced blocks of query of d into query blocks.
func queriesIn(d Doc) Doc {
	return blocksOf(d, func(id string) bool { return id == CodeIDPrefix+QueryLang }, func(_ string, lines []string) *Obj {
		return &Obj{K: "query", Q: strings.TrimSpace(strings.Join(strings.Fields(strings.Join(lines, " ")), " "))}
	})
}

// blockMarksIn turns the comments of d that hold a block of a newer kind back into the block.
func blockMarksIn(d Doc) Doc {
	for i, p := range d {
		s := p.text()
		if !strings.Contains(s, blockMarkStart) {
			continue
		}
		m := blockMarkLine.FindStringSubmatch(strings.TrimSpace(s))
		if m == nil {
			continue
		}
		raw, err := base64.RawStdEncoding.DecodeString(m[1])
		if err != nil || len(raw) == 0 {
			continue
		}
		var meta struct {
			K string `json:"k"`
		}
		if json.Unmarshal(raw, &meta) != nil || meta.K == "" || KnownObjKinds[meta.K] {
			continue
		}
		d[i] = Para{R: []Run{{T: ObjectChar, O: &Obj{K: UnknownKind, Raw: raw}}}}
	}
	return d
}

// ---- written ----

// ObjectsAsText writes d's objects as the text the file has for them: a link and a card as their [[link]], a
// footnote's reference as [^1], a file as its name linking to it, a table and a query as their fenced blocks, and a
// block of a newer kind as its comment. A task in the running text and a picture stay objects: PageMarkdown writes
// them.
func ObjectsAsText(d Doc) Doc {
	var out Doc
	for _, p := range d {
		out = append(out, objectsAsText(p)...)
	}
	return out
}

// objectsAsText is p's objects as their text, from the last, as paragraphs: a block splits p.
func objectsAsText(p Para) []Para {
	k, at := -1, 0
	for i, r := range p.R {
		if r.O != nil && r.O.K != "check" && r.O.K != "image" {
			k = i
		}
	}
	if k < 0 {
		return []Para{p}
	}
	for _, r := range p.R[:k] {
		at += runeLen(r)
	}
	o := p.R[k].O
	plain := p.R[k]
	plain.T, plain.O = "", nil
	text := func(s string, marks Run) Para {
		marks.T = s
		q := p
		q.R = p.splice(at, at+1, marks)
		return q
	}
	switch o.K {
	case "table":
		return blockAsText(p, k, at, (&table.Model{Cols: o.Cols, Rows: o.Rows, Filter: o.Q}).Text(), ParaStyle{CodeBlock: true, ID: CodeIDPrefix + TableLang}, plain)
	case "query":
		return blockAsText(p, k, at, o.Q, ParaStyle{CodeBlock: true, ID: CodeIDPrefix + QueryLang}, plain)
	case "file": // its name, a link to the file beside the page
		link := plain
		link.L = o.Target
		return objectsAsText(text(o.Target, link))
	}
	if !o.Known() { // a mark no reader shows, that brings it back
		return blockAsText(p, k, at, blockMarkStart+base64.RawStdEncoding.EncodeToString(o.Raw)+"-->", ParaStyle{CodeBlock: true, ID: RawIDPrefix + "html"}, plain)
	}
	return objectsAsText(text(ObjMarkdown(o), plain))
}

// ObjMarkdown is an object's Markdown in the running text: [[Page]], [[Page|alias]], ![[Page]], [^1], a box.
func ObjMarkdown(o *Obj) string {
	switch o.K {
	case "embed":
		return "![[" + o.Target + "]]"
	case "footnote":
		return "[^" + o.Target + "]"
	case "check":
		return checkMarkdown(o.Target == "x")
	case "file":
		ref := o.Target
		if strings.ContainsAny(ref, " ()") {
			ref = "<" + ref + ">"
		}
		return "[" + o.Target + "](" + ref + ")"
	}
	if o.Alias != "" {
		return "[[" + o.Target + "|" + o.Alias + "]]"
	}
	return "[[" + o.Target + "]]"
}

// blockAsText puts txt where the object of run k (character at) of p is, as paragraphs of style st (its first's ID,
// the others' none) in the marks plain: what was before it stays a paragraph of p's style, what was after it one of
// p's style without its ID, as the editor splits a paragraph.
func blockAsText(p Para, k, at int, txt string, st ParaStyle, plain Run) []Para {
	var out []Para
	before := Para{S: p.S, R: mergeRuns(append([]Run(nil), p.R[:k]...))}
	after := Para{R: mergeRuns(append([]Run(nil), p.R[k+1:]...))}
	if p.S != nil {
		as := *p.S
		as.ID = ""
		if as != (ParaStyle{}) {
			after.S = &as
		}
	}
	if at > 0 {
		out = append(out, objectsAsText(before)...)
	}
	for j, line := range strings.Split(txt, "\n") {
		s := st
		if j > 0 {
			s.ID = ""
		}
		q := Para{S: &s}
		if line != "" {
			r := plain
			r.T = line
			q.R = []Run{r}
		}
		out = append(out, q)
	}
	if len(after.R) > 0 {
		out = append(out, after)
	}
	return out
}
