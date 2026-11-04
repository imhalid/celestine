package main

import (
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

// The document is a list of blocks. A block has a kind (paragraph, heading,
// list item, task…), a nesting depth and its text, which holds no Markdown
// marks at all: bold, italic, code and strike are ranges laid over the text.
// Markdown is only made when the document is written to a file, and only
// read when one is opened (markdown.go).
//
// Offsets are in UTF-16 code units, as MyGo's text layout and text input use.

type Kind uint8

const (
	Paragraph Kind = iota
	Heading1
	Heading2
	Heading3
	Bullet
	Numbered
	Todo
	Quote
	Code
	Divider
)

// isList reports the kinds that sit in a list: they carry a marker in the
// gutter and keep their kind when Enter splits them.
func (k Kind) isList() bool { return k == Bullet || k == Numbered || k == Todo }

func (k Kind) isHeading() bool { return k >= Heading1 && k <= Heading3 }

type MarkKind uint8

const (
	Bold MarkKind = iota
	Italic
	InlineCode
	Strike
)

// Mark is a run of formatting over [S, E) of a block's text.
type Mark struct {
	K    MarkKind
	S, E int
}

type Block struct {
	ID      int
	Ver     int // changes with every edit, unique over the whole run (the layout cache keys on it)
	Kind    Kind
	Depth   int
	Text    string
	Marks   []Mark
	Checked bool   // Todo
	Lang    string // Code
}

var nextID, nextVer = 1, 1

func newBlock(k Kind, text string) *Block {
	b := &Block{ID: nextID, Kind: k, Text: text}
	nextID++
	b.touch()
	return b
}

func (b *Block) touch() { b.Ver = nextVer; nextVer++ }

func (b *Block) len() int { return ui.UTF16Len(b.Text) }

func (b *Block) clone() *Block {
	c := *b
	c.Marks = append([]Mark(nil), b.Marks...)
	return &c
}

// byteAt is the byte index of UTF-16 offset o in the block's text.
func (b *Block) byteAt(o int) int { return ui.UTF16ByteOffset(b.Text, o) }

func (b *Block) slice(a, z int) string { return b.Text[b.byteAt(a):b.byteAt(z)] }

// hasMark reports whether all of [a, z) carries k; for an empty range,
// whether typing at a would (see activeMarks).
func (b *Block) hasMark(k MarkKind, a, z int) bool {
	if a == z {
		for _, m := range b.activeMarks(a) {
			if m == k {
				return true
			}
		}
		return false
	}
	covered := a
	for _, m := range b.sortedMarks(k) {
		if m.S <= covered && m.E > covered {
			covered = m.E
		}
		if covered >= z {
			return true
		}
	}
	return false
}

func (b *Block) sortedMarks(k MarkKind) []Mark {
	var out []Mark
	for _, m := range b.Marks {
		if m.K == k {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].S < out[j].S })
	return out
}

// activeMarks are the marks text typed at o takes on: those of the letter
// before it, as in every word processor. Inline code does not stretch at its
// end (typing after `code` leaves the code).
func (b *Block) activeMarks(o int) []MarkKind {
	var out []MarkKind
	for _, m := range b.Marks {
		if (m.S < o && o < m.E) || (o == m.E && o > m.S && m.K != InlineCode) {
			out = append(out, m.K)
		}
	}
	return out
}

// normalize drops empty marks and joins overlapping or touching ones of a kind.
func (b *Block) normalize() {
	n := b.len()
	var out []Mark
	for k := Bold; k <= Strike; k++ {
		var cur *Mark
		for _, m := range b.sortedMarks(k) {
			m.S, m.E = max(0, m.S), min(n, m.E)
			if m.E <= m.S {
				continue
			}
			if cur != nil && m.S <= cur.E {
				cur.E = max(cur.E, m.E)
				continue
			}
			out = append(out, m)
			cur = &out[len(out)-1]
		}
	}
	b.Marks = out
}

// insert puts s at o, formatted with marks (nil: as activeMarks(o) says).
func (b *Block) insert(o int, s string, marks []MarkKind) {
	if s == "" {
		return
	}
	if marks == nil {
		marks = b.activeMarks(o)
	}
	n := ui.UTF16Len(s)
	at := b.byteAt(o)
	b.Text = b.Text[:at] + s + b.Text[at:]
	for i := range b.Marks {
		m := &b.Marks[i]
		if m.S >= o {
			m.S += n
		}
		if m.E > o {
			m.E += n
		}
	}
	for _, k := range marks {
		b.Marks = append(b.Marks, Mark{K: k, S: o, E: o + n})
	}
	// The marks given are all the new text has: a run of another kind it was
	// typed into is cut around it (bold turned off in the middle of a bold word).
	for k := Bold; k <= Strike; k++ {
		if !containsKind(marks, k) {
			b.cut(k, o, o+n)
		}
	}
	b.normalize()
	b.touch()
}

// remove deletes [a, z).
func (b *Block) remove(a, z int) {
	if z <= a {
		return
	}
	b.Text = b.Text[:b.byteAt(a)] + b.Text[b.byteAt(z):]
	shift := func(p int) int {
		switch {
		case p <= a:
			return p
		case p >= z:
			return p - (z - a)
		}
		return a
	}
	for i := range b.Marks {
		b.Marks[i].S, b.Marks[i].E = shift(b.Marks[i].S), shift(b.Marks[i].E)
	}
	b.normalize()
	b.touch()
}

// toggle sets k over [a, z) when it is not all there, else takes it off.
func (b *Block) toggle(k MarkKind, a, z int, on bool) {
	if z <= a {
		return
	}
	var out []Mark
	for _, m := range b.Marks {
		if m.K != k || m.E <= a || m.S >= z {
			out = append(out, m)
			continue
		}
		if m.S < a {
			out = append(out, Mark{K: k, S: m.S, E: a})
		}
		if m.E > z {
			out = append(out, Mark{K: k, S: z, E: m.E})
		}
	}
	if on {
		out = append(out, Mark{K: k, S: a, E: z})
	}
	b.Marks = out
	b.normalize()
	b.touch()
}

// split cuts the block at o: it keeps the text before, and returns a new
// block with the text after and its marks.
func (b *Block) split(o int) *Block {
	nb := newBlock(b.Kind, b.Text[b.byteAt(o):])
	nb.Depth = b.Depth
	for _, m := range b.Marks {
		if m.E > o {
			nb.Marks = append(nb.Marks, Mark{K: m.K, S: max(0, m.S-o), E: m.E - o})
		}
	}
	nb.normalize()
	b.remove(o, b.len())
	return nb
}

// join appends other's text and marks to b.
func (b *Block) join(other *Block) {
	n := b.len()
	b.Text += other.Text
	for _, m := range other.Marks {
		b.Marks = append(b.Marks, Mark{K: m.K, S: m.S + n, E: m.E + n})
	}
	b.normalize()
	b.touch()
}

// Pos is a place in the document: a block and an offset in its text.
type Pos struct{ B, O int }

func (p Pos) before(q Pos) bool { return p.B < q.B || (p.B == q.B && p.O < q.O) }

// Selection runs from Anchor, where it began, to Focus, where the caret is.
type Selection struct{ Anchor, Focus Pos }

func (s Selection) collapsed() bool { return s.Anchor == s.Focus }

// ordered returns the selection's start and end.
func (s Selection) ordered() (Pos, Pos) {
	if s.Focus.before(s.Anchor) {
		return s.Focus, s.Anchor
	}
	return s.Anchor, s.Focus
}

// Doc is the document and its editing history.
type Doc struct {
	Blocks []*Block
	Sel    Selection
	Stored []MarkKind // marks ⌘B and friends set at a collapsed caret, for the next letters
	stored bool       // Stored is in force (it may be empty: "no bold any more")

	undo, redo []snapshot
	lastKind   string // the last edit, to fold a run of typing into one undo step
	Dirty      bool
}

type snapshot struct {
	blocks []*Block
	sel    Selection
}

func newDoc(blocks ...*Block) *Doc {
	if len(blocks) == 0 {
		blocks = []*Block{newBlock(Paragraph, "")}
	}
	return &Doc{Blocks: blocks}
}

func (d *Doc) snap() snapshot {
	s := snapshot{sel: d.Sel, blocks: make([]*Block, len(d.Blocks))}
	for i, b := range d.Blocks {
		s.blocks[i] = b.clone()
	}
	return s
}

// record keeps the state before an edit of kind for undo. Typing in a row is
// one step ("type"); anything else, an input rule included, is a step of its own.
func (d *Doc) record(kind string) {
	d.Dirty = true
	d.redo = nil
	if kind == "type" && d.lastKind == "type" {
		return
	}
	d.lastKind = kind
	d.undo = append(d.undo, d.snap())
	if len(d.undo) > 500 {
		d.undo = d.undo[1:]
	}
}

// breakRun ends a run of typing, so the next letters start a new undo step.
func (d *Doc) breakRun() { d.lastKind = "" }

func (d *Doc) restore(s snapshot) {
	d.Blocks, d.Sel = s.blocks, s.sel
	for _, b := range d.Blocks {
		b.touch()
	}
	d.stored, d.lastKind, d.Dirty = false, "", true
}

func (d *Doc) Undo() bool {
	if len(d.undo) == 0 {
		return false
	}
	d.redo = append(d.redo, d.snap())
	d.restore(d.undo[len(d.undo)-1])
	d.undo = d.undo[:len(d.undo)-1]
	return true
}

func (d *Doc) Redo() bool {
	if len(d.redo) == 0 {
		return false
	}
	d.undo = append(d.undo, d.snap())
	d.restore(d.redo[len(d.redo)-1])
	d.redo = d.redo[:len(d.redo)-1]
	return true
}

func (d *Doc) block(i int) *Block { return d.Blocks[i] }

func (d *Doc) caret() Pos { return d.Sel.Focus }

func (d *Doc) setCaret(p Pos) { d.Sel = Selection{p, p}; d.stored = false }

func (d *Doc) clampPos(p Pos) Pos {
	p.B = max(0, min(len(d.Blocks)-1, p.B))
	p.O = max(0, min(d.Blocks[p.B].len(), p.O))
	return p
}

func (d *Doc) insertBlocks(at int, bs ...*Block) {
	d.Blocks = append(d.Blocks[:at], append(bs, d.Blocks[at:]...)...)
}

func (d *Doc) removeBlocks(a, z int) {
	d.Blocks = append(d.Blocks[:a], d.Blocks[z:]...)
	if len(d.Blocks) == 0 {
		d.Blocks = []*Block{newBlock(Paragraph, "")}
	}
}

// deleteSelection removes what is selected; across blocks, the first block
// keeps its kind and takes on what is left of the last one.
func (d *Doc) deleteSelection() bool {
	if d.Sel.collapsed() {
		return false
	}
	a, z := d.Sel.ordered()
	first := d.Blocks[a.B]
	if a.B == z.B {
		first.remove(a.O, z.O)
	} else {
		last := d.Blocks[z.B]
		first.remove(a.O, first.len())
		tail := last.clone()
		tail.remove(0, z.O)
		if first.Kind == Divider {
			first.Kind = Paragraph
		}
		first.join(tail)
		d.removeBlocks(a.B+1, z.B+1)
	}
	d.setCaret(a)
	return true
}

// selectedText is the selection as plain text, a line per block.
func (d *Doc) selectedText() string {
	a, z := d.Sel.ordered()
	var lines []string
	for i := a.B; i <= z.B; i++ {
		b := d.Blocks[i]
		s, e := 0, b.len()
		if i == a.B {
			s = a.O
		}
		if i == z.B {
			e = z.O
		}
		lines = append(lines, b.slice(s, e))
	}
	return strings.Join(lines, "\n")
}

// selectedBlocks is the selection as blocks of their own, cut at its ends.
func (d *Doc) selectedBlocks() []*Block {
	a, z := d.Sel.ordered()
	var out []*Block
	for i := a.B; i <= z.B; i++ {
		b := d.Blocks[i].clone()
		if i == z.B {
			b.remove(z.O, b.len())
		}
		if i == a.B {
			b.remove(0, a.O)
		}
		out = append(out, b)
	}
	return out
}

// numberOf is the number a Numbered block shows: its place among the
// numbered blocks at its depth since the run began.
func (d *Doc) numberOf(i int) int {
	n, depth := 1, d.Blocks[i].Depth
	for j := i - 1; j >= 0; j-- {
		b := d.Blocks[j]
		if b.Depth > depth && b.Kind.isList() {
			continue // a nested item does not break the run
		}
		if b.Kind != Numbered || b.Depth != depth {
			break
		}
		n++
	}
	return n
}

func containsKind(ks []MarkKind, k MarkKind) bool {
	for _, x := range ks {
		if x == k {
			return true
		}
	}
	return false
}

// cut takes mark k off [a, z), splitting a run that covers it.
func (b *Block) cut(k MarkKind, a, z int) {
	var out []Mark
	for _, m := range b.Marks {
		if m.K != k || m.E <= a || m.S >= z {
			out = append(out, m)
			continue
		}
		if m.S < a {
			out = append(out, Mark{K: k, S: m.S, E: a})
		}
		if m.E > z {
			out = append(out, Mark{K: k, S: z, E: m.E})
		}
	}
	b.Marks = out
}
