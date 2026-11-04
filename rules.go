package main

import (
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Editing: what typing, Enter, Backspace, Tab and the format keys do to the
// blocks. The Markdown a user types is read here and turned into structure at
// once - "# " makes a heading, "**word**" bold - and its marks are taken out
// of the text, so they are never seen. Each rule is its own undo step: ⌘Z
// right after one gives back the marks as typed.

// typeText puts s at the caret (over the selection), then applies the rules.
func (d *Doc) typeText(s string) {
	if s == "" {
		return
	}
	if strings.ContainsAny(s, "\n") {
		d.paste(s)
		return
	}
	d.record("type")
	d.deleteSelection()
	p := d.caret()
	b := d.Blocks[p.B]
	if b.Kind == Divider { // typing on a divider writes in a new paragraph after it
		nb := newBlock(Paragraph, "")
		d.insertBlocks(p.B+1, nb)
		p = Pos{p.B + 1, 0}
		b = nb
	}
	var marks []MarkKind
	if d.stored {
		marks = append([]MarkKind{}, d.Stored...)
	}
	b.insert(p.O, s, marks)
	stored, keep := d.Stored, d.stored
	d.setCaret(Pos{p.B, p.O + ui.UTF16Len(s)})
	d.Stored, d.stored = stored, keep
	if b.Kind != Code {
		if !d.blockRule(p.B) {
			d.inlineRule(p.B)
		}
	}
}

// Block rules: what a paragraph's start turns it into when a space follows.
var blockRules = []struct {
	prefix *regexp.Regexp
	apply  func(b *Block, m []string)
}{
	{regexp.MustCompile(`^(#{1,3}) $`), func(b *Block, m []string) { b.Kind = Heading1 + Kind(len(m[1])-1) }},
	{regexp.MustCompile(`^[-*+] $`), func(b *Block, m []string) { b.Kind = Bullet }},
	{regexp.MustCompile(`^\d{1,3}[.)] $`), func(b *Block, m []string) { b.Kind = Numbered }},
	{regexp.MustCompile(`^\[( |x|X)?\] $`), func(b *Block, m []string) { b.Kind = Todo; b.Checked = m[1] == "x" || m[1] == "X" }},
	{regexp.MustCompile(`^> $`), func(b *Block, m []string) { b.Kind = Quote }},
}

// blockRule turns the block's kind from what was typed at its start.
func (d *Doc) blockRule(i int) bool {
	b := d.Blocks[i]
	o := d.caret().O
	head := b.slice(0, o)
	switch {
	case b.Kind == Paragraph && head == "```":
		d.record("rule")
		b.remove(0, o)
		b.Kind, b.Marks = Code, nil
		d.setCaret(Pos{i, 0})
		return true
	case b.Kind == Paragraph && b.Text == "---":
		d.record("rule")
		b.remove(0, b.len())
		b.Kind = Divider
		nb := newBlock(Paragraph, "")
		d.insertBlocks(i+1, nb)
		d.setCaret(Pos{i + 1, 0})
		return true
	}
	if !strings.HasSuffix(head, " ") || (b.Kind != Paragraph && !(b.Kind.isList() && head == "[] ")) {
		return false
	}
	for _, r := range blockRules {
		if m := r.prefix.FindStringSubmatch(head); m != nil {
			d.record("rule")
			b.remove(0, o)
			r.apply(b, m)
			b.touch()
			d.setCaret(Pos{i, 0})
			return true
		}
	}
	return false
}

// Inline rules, tried at the caret right after a letter of a closing mark.
var inlineRules = []struct {
	delim string
	kind  MarkKind
}{
	{"**", Bold}, {"__", Bold}, {"~~", Strike}, {"`", InlineCode}, {"*", Italic}, {"_", Italic},
}

// inlineRule finds a closed pair of marks ending at the caret, takes the
// marks out and formats what was between them.
func (d *Doc) inlineRule(i int) bool {
	b := d.Blocks[i]
	o := d.caret().O
	text := b.slice(0, o)
	for _, r := range inlineRules {
		if !strings.HasSuffix(text, r.delim) {
			continue
		}
		body := text[:len(text)-len(r.delim)]
		if len(r.delim) == 1 && strings.HasSuffix(body, r.delim) {
			continue // the first half of a double mark ("**"), not a closing one
		}
		open := strings.LastIndex(body, r.delim)
		if open < 0 {
			continue
		}
		inner := body[open+len(r.delim):]
		if inner == "" || strings.TrimSpace(inner) != inner {
			continue // "* *", "** x**": not emphasis
		}
		if len(r.delim) == 1 && open > 0 && strings.HasSuffix(body[:open], r.delim) {
			continue // "**x*": wait for the second star
		}
		if r.delim == "_" || r.delim == "__" {
			if open > 0 && isWordByte(body[open-1]) {
				continue // snake_case_names stay as they are
			}
		}
		so := ui.UTF16Len(body[:open])
		if b.hasMark(InlineCode, so, so+1) {
			continue // nothing is formatted inside code
		}
		d.record("rule")
		n := ui.UTF16Len(r.delim)
		innerLen := ui.UTF16Len(inner)
		b.remove(so+n+innerLen, so+n+innerLen+n) // the closing mark
		b.remove(so, so+n)                       // the opening one
		b.toggle(r.kind, so, so+innerLen, true)
		d.setCaret(Pos{i, so + innerLen})
		// What is typed next is plain again, as it is after a closing mark.
		d.Stored, d.stored = without(b.activeMarks(so+innerLen), r.kind), true
		return true
	}
	return false
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func without(ks []MarkKind, k MarkKind) []MarkKind {
	out := []MarkKind{}
	for _, x := range ks {
		if x != k {
			out = append(out, x)
		}
	}
	return out
}

// enter splits the block at the caret. An empty list item or quote stops
// being one (outdenting first when nested); in code, Enter is a new line, and
// on an empty last line it leaves the code. shift is a line break in the block.
func (d *Doc) enter(shift bool) {
	d.record("enter")
	d.deleteSelection()
	p := d.caret()
	b := d.Blocks[p.B]
	if shift && b.Kind != Divider {
		b.insert(p.O, "\n", nil)
		d.setCaret(Pos{p.B, p.O + 1})
		return
	}
	switch {
	case b.Kind == Code:
		if p.O == b.len() && (b.Text == "" || strings.HasSuffix(b.Text, "\n")) {
			if b.Text != "" {
				b.remove(b.len()-1, b.len())
			}
			nb := newBlock(Paragraph, "")
			d.insertBlocks(p.B+1, nb)
			d.setCaret(Pos{p.B + 1, 0})
			return
		}
		b.insert(p.O, "\n", []MarkKind{})
		d.setCaret(Pos{p.B, p.O + 1})
		return
	case b.Text == "" && (b.Kind.isList() || b.Kind == Quote):
		if b.Depth > 0 {
			b.Depth--
		} else {
			b.Kind = Paragraph
		}
		b.touch()
		return
	case b.Kind == Divider:
		d.insertBlocks(p.B+1, newBlock(Paragraph, ""))
		d.setCaret(Pos{p.B + 1, 0})
		return
	}
	if p.O == 0 && b.Text != "" { // at the start: a fresh block above, the caret stays with the text
		nb := newBlock(b.Kind, "")
		if b.Kind.isHeading() {
			nb.Kind = Paragraph
		}
		nb.Depth = b.Depth
		d.insertBlocks(p.B, nb)
		d.setCaret(Pos{p.B + 1, 0})
		return
	}
	nb := b.split(p.O)
	switch {
	case b.Kind.isHeading(), b.Kind == Quote && nb.Text == "":
		nb.Kind = Paragraph
	case b.Kind == Todo:
		nb.Checked = false
	}
	d.insertBlocks(p.B+1, nb)
	d.setCaret(Pos{p.B + 1, 0})
}

// backspace deletes back: the selection, a letter (a word with word), or at
// a block's start its kind, then its depth, then the line between it and the
// block above.
func (d *Doc) backspace(word bool) {
	if !d.Sel.collapsed() {
		d.record("delete")
		d.deleteSelection()
		return
	}
	p := d.caret()
	b := d.Blocks[p.B]
	if p.O > 0 {
		d.record("type")
		to := prevGrapheme(b.Text, p.O)
		if word {
			to = wordEdge(b.Text, p.O, -1)
		}
		b.remove(to, p.O)
		d.setCaret(Pos{p.B, to})
		return
	}
	d.record("delete")
	switch {
	case b.Kind != Paragraph && b.Kind != Divider:
		b.Kind, b.Checked = Paragraph, false
		b.touch()
	case b.Depth > 0:
		b.Depth--
		b.touch()
	case p.B == 0:
	default:
		prev := d.Blocks[p.B-1]
		if prev.Kind == Divider {
			d.removeBlocks(p.B-1, p.B)
			d.setCaret(Pos{p.B - 1, 0})
			return
		}
		at := prev.len()
		if b.Kind == Divider {
			d.removeBlocks(p.B, p.B+1)
		} else {
			prev.join(b)
			d.removeBlocks(p.B, p.B+1)
		}
		d.setCaret(Pos{p.B - 1, at})
	}
}

// forwardDelete deletes ahead: a letter or word, or at a block's end, the line
// between it and the next block.
func (d *Doc) forwardDelete(word bool) {
	if !d.Sel.collapsed() {
		d.record("delete")
		d.deleteSelection()
		return
	}
	p := d.caret()
	b := d.Blocks[p.B]
	if p.O < b.len() {
		d.record("type")
		to := nextGrapheme(b.Text, p.O)
		if word {
			to = wordEdge(b.Text, p.O, 1)
		}
		b.remove(p.O, to)
		return
	}
	if p.B+1 >= len(d.Blocks) {
		return
	}
	d.record("delete")
	next := d.Blocks[p.B+1]
	if next.Kind != Divider {
		b.join(next)
	}
	d.removeBlocks(p.B+1, p.B+2)
}

// indent moves the selected blocks a level in (or out), never more than one
// level deeper than the block above.
func (d *Doc) indent(by int) {
	d.record("indent")
	a, z := d.Sel.ordered()
	for i := a.B; i <= z.B; i++ {
		b := d.Blocks[i]
		limit := 0
		if i > 0 {
			limit = d.Blocks[i-1].Depth + 1
		}
		b.Depth = max(0, min(limit, min(6, b.Depth+by)))
		b.touch()
	}
}

// toggleMark formats the selection with k, or takes k off when it is all
// there; at a collapsed caret it sets what the next letters will be.
func (d *Doc) toggleMark(k MarkKind) {
	a, z := d.Sel.ordered()
	if d.Sel.collapsed() {
		cur := d.Blocks[a.B].activeMarks(a.O)
		if d.stored {
			cur = d.Stored
		}
		has := false
		for _, x := range cur {
			has = has || x == k
		}
		if has {
			d.Stored = without(cur, k)
		} else {
			d.Stored = append(append([]MarkKind{}, cur...), k)
		}
		d.stored = true
		return
	}
	d.record("format")
	all := true
	for i := a.B; i <= z.B; i++ {
		s, e := 0, d.Blocks[i].len()
		if i == a.B {
			s = a.O
		}
		if i == z.B {
			e = z.O
		}
		if e > s && !d.Blocks[i].hasMark(k, s, e) {
			all = false
		}
	}
	for i := a.B; i <= z.B; i++ {
		s, e := 0, d.Blocks[i].len()
		if i == a.B {
			s = a.O
		}
		if i == z.B {
			e = z.O
		}
		d.Blocks[i].toggle(k, s, e, !all)
	}
}

// setKind turns the selected blocks into k (back to paragraphs when they all are k).
func (d *Doc) setKind(k Kind) {
	d.record("kind")
	a, z := d.Sel.ordered()
	all := true
	for i := a.B; i <= z.B; i++ {
		all = all && d.Blocks[i].Kind == k
	}
	for i := a.B; i <= z.B; i++ {
		b := d.Blocks[i]
		if all {
			b.Kind = Paragraph
		} else {
			b.Kind = k
		}
		b.touch()
	}
}

// toggleChecked ticks or unticks a task.
func (d *Doc) toggleChecked(i int) {
	b := d.Blocks[i]
	if b.Kind != Todo {
		return
	}
	d.record("check")
	b.Checked = !b.Checked
	b.touch()
}

// paste puts text at the caret: one line goes in as typed, several are read
// as Markdown into blocks of their own.
func (d *Doc) paste(s string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.Contains(s, "\n") {
		d.typeText(s)
		d.breakRun()
		return
	}
	d.record("paste")
	d.deleteSelection()
	p := d.caret()
	cur := d.Blocks[p.B]
	if cur.Kind == Code { // into code, as it is
		cur.insert(p.O, s, []MarkKind{})
		d.setCaret(Pos{p.B, p.O + ui.UTF16Len(s)})
		return
	}
	parsed := parseMarkdown(s)
	if len(parsed) == 0 {
		return
	}
	tail := cur.split(p.O)
	first := parsed[0]
	at := p.B + 1
	if cur.Text == "" { // an empty block takes on the first one's kind
		cur.Kind, cur.Checked, cur.Lang = first.Kind, first.Checked, first.Lang
	}
	cur.join(first)
	rest := parsed[1:]
	d.insertBlocks(at, rest...)
	last := cur
	if len(rest) > 0 {
		last = rest[len(rest)-1]
	}
	end := Pos{at - 1 + len(rest), last.len()}
	if tail.Text != "" {
		last.join(tail)
	}
	d.setCaret(end)
}

// Text navigation helpers, in UTF-16 offsets.

func prevGrapheme(s string, o int) int {
	l := ui.ShapeText(s, ui.Font{Size: 12}, 0)
	return l.PreviousBoundary(o)
}

func nextGrapheme(s string, o int) int {
	l := ui.ShapeText(s, ui.Font{Size: 12}, 0)
	return l.NextBoundary(o)
}
