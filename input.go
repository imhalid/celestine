package main

import (
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// changed is called after every edit: the caret comes into view, the
// document is saved a moment later, and the caret hears of it.
func (e *editor) changed(wrote int) {
	e.follow, e.hasX = true, false
	e.editAt = e.now()
	e.saveErr = ""
	if wrote > 0 {
		e.wrote += wrote
	} else {
		e.edited = true
	}
}

func (e *editor) moved() { e.follow = true; e.doc.breakRun() }

// ---- the system's text input (typing, input methods, the Edit menu) ----
// MyGo's text input sees one block at a time: the one the caret is in.

func (e *editor) focusBlock() *Block { return e.doc.Blocks[e.doc.caret().B] }

func (e *editor) TextForRange(r ui.TextInputRange) (string, ui.TextInputRange) {
	b := e.focusBlock()
	r.Start, r.End = max(0, min(r.Start, b.len())), max(0, min(r.End, b.len()))
	return b.slice(r.Start, r.End), r
}

func (e *editor) Selection() ui.TextInputSelection {
	s := e.doc.Sel
	if s.Anchor.B != s.Focus.B {
		return ui.TextInputSelection{Range: ui.TextInputRange{Start: s.Focus.O, End: s.Focus.O}}
	}
	a, z := s.ordered()
	return ui.TextInputSelection{Range: ui.TextInputRange{Start: a.O, End: z.O}, Reversed: s.Focus.before(s.Anchor)}
}

func (e *editor) MarkedRange() (ui.TextInputRange, bool) {
	if e.marked != nil {
		return *e.marked, true
	}
	return ui.TextInputRange{}, false
}

func (e *editor) ReplaceText(r *ui.TextInputRange, text string) {
	if r == nil && e.marked == nil {
		e.doc.typeText(text)
		e.changed(utf8.RuneCountInString(text))
		return
	}
	target := r
	if target == nil {
		target = e.marked
	}
	p := e.doc.caret()
	b := e.focusBlock()
	e.doc.record("type")
	b.remove(target.Start, target.End)
	b.insert(target.Start, text, nil)
	e.marked = nil
	e.doc.setCaret(Pos{p.B, target.Start + ui.UTF16Len(text)})
	if b.Kind != Code && !e.doc.blockRule(p.B) {
		e.doc.inlineRule(p.B)
	}
	e.changed(utf8.RuneCountInString(text))
}

func (e *editor) SetMarkedText(r *ui.TextInputRange, text string, selected ui.TextInputRange) {
	p := e.doc.caret()
	b := e.focusBlock()
	target := ui.TextInputRange{Start: p.O, End: p.O}
	switch {
	case r != nil:
		target = *r
	case e.marked != nil:
		target = *e.marked
	case !e.doc.Sel.collapsed():
		e.doc.deleteSelection()
		p = e.doc.caret()
		target = ui.TextInputRange{Start: p.O, End: p.O}
	}
	e.doc.record("type")
	b.remove(target.Start, target.End)
	b.insert(target.Start, text, nil)
	mark := ui.TextInputRange{Start: target.Start, End: target.Start + ui.UTF16Len(text)}
	e.marked = &mark
	e.doc.Sel = Selection{Pos{p.B, mark.Start + selected.Start}, Pos{p.B, mark.Start + selected.End}}
	e.changed(0)
}

func (e *editor) UnmarkText() { e.marked = nil }

func (e *editor) BoundsForRange(r ui.TextInputRange) (ui.Rect, ui.TextInputRange, bool) {
	if len(e.boxes) == 0 {
		return ui.Rect{}, r, false
	}
	c, _ := e.caretRect(Pos{e.doc.caret().B, r.Start})
	c.Y -= e.scrollY
	return c, r, true
}

func (e *editor) IndexForPoint(pt ui.Point) (int, bool) {
	if len(e.boxes) == 0 {
		return 0, false
	}
	p := e.hit(pt.X, pt.Y+e.scrollY)
	if p.B != e.doc.caret().B {
		return 0, false
	}
	return p.O, true
}

// ---- keys and the pointer ----

func (e *editor) input(ev ui.InputEvent) bool {
	d := e.doc
	shift := ev.Mods&ui.Shift != 0
	cmd := ev.Mods&ui.Cmd != 0
	alt := ev.Mods&ui.Alt != 0
	switch ev.Kind {
	case ui.InputScroll:
		e.scrollY += ev.DY
		return true
	case ui.InputPointerDown:
		if ev.Button != 0 || len(e.boxes) == 0 {
			return false
		}
		if i, ok := e.checkboxAt(ev.X, ev.Y+e.scrollY); ok {
			d.toggleChecked(i)
			e.changed(0)
			return true
		}
		p := e.hit(ev.X, ev.Y+e.scrollY)
		switch {
		case ev.Clicks == 2:
			a, z := wordAt(d.Blocks[p.B].Text, p.O)
			d.Sel = Selection{Pos{p.B, a}, Pos{p.B, z}}
		case ev.Clicks >= 3:
			d.Sel = Selection{Pos{p.B, 0}, Pos{p.B, d.Blocks[p.B].len()}}
		case shift:
			d.Sel.Focus = p
		default:
			d.setCaret(p)
		}
		e.dragging, e.hasX = true, false
		e.moved()
		return true
	case ui.InputPointerMove:
		if e.dragging {
			d.Sel.Focus = e.hit(ev.X, ev.Y+e.scrollY)
			e.follow = true
			return true
		}
	case ui.InputPointerUp:
		e.dragging = false
		return true
	case ui.InputCommand:
		return e.command(ev.Text)
	case ui.InputKeyDown:
		return e.key(ev.Key, shift, cmd, alt, ev.Mods)
	}
	return false
}

// command answers the Edit menu and the system's editing commands.
func (e *editor) command(name string) bool {
	d := e.doc
	switch name {
	case "copy", "cut":
		if d.Sel.collapsed() {
			return true
		}
		mygo.Clipboard.WriteText(toMarkdown(d.selectedBlocks()))
		if name == "cut" {
			d.record("cut")
			d.deleteSelection()
			e.changed(0)
		}
	case "paste":
		d.paste(mygo.Clipboard.ReadText())
		e.changed(0)
	case "selectAll":
		e.selectAll()
	case "undo":
		if d.Undo() {
			e.changed(0)
		}
	case "redo":
		if d.Redo() {
			e.changed(0)
		}
	case "delete":
		d.record("delete")
		d.deleteSelection()
		e.changed(0)
	default:
		return false
	}
	return true
}

func (e *editor) selectAll() {
	d := e.doc
	last := len(d.Blocks) - 1
	d.Sel = Selection{Pos{0, 0}, Pos{last, d.Blocks[last].len()}}
	e.moved()
}

func (e *editor) key(k ui.Key, shift, cmd, alt bool, mods ui.Modifiers) bool {
	d := e.doc
	u := unitOf(mods)
	switch {
	case cmd && k == ui.KeyB:
		d.toggleMark(Bold)
	case cmd && k == ui.KeyI:
		d.toggleMark(Italic)
	case cmd && k == ui.KeyE:
		d.toggleMark(InlineCode)
	case cmd && shift && k == ui.KeyX:
		d.toggleMark(Strike)
	case cmd && !shift && k == ui.KeyZ:
		if !d.Undo() {
			return true
		}
	case cmd && (shift && k == ui.KeyZ || k == ui.KeyY):
		if !d.Redo() {
			return true
		}
	case cmd && k == ui.KeyA:
		e.selectAll()
		return true
	case cmd && k == ui.KeyS:
		e.save()
		return true
	case cmd && shift && k == ui.KeyM:
		e.caretKind = (e.caretKind + 1) % 2
		return true
	case cmd && alt && k >= ui.Key1 && k <= ui.Key3:
		d.setKind(Heading1 + Kind(k-ui.Key1))
	case cmd && alt && k == ui.Key0:
		d.setKind(Paragraph)
	case cmd && k == ui.KeyEnter:
		d.toggleChecked(d.caret().B)
	case k == ui.KeyEnter:
		d.enter(shift)
	case k == ui.KeyBackspace:
		if cmd && d.Sel.collapsed() { // to the line's start
			p := d.caret()
			d.Sel.Anchor = Pos{p.B, e.lineEdge(p, -1)}
			d.record("delete")
			d.deleteSelection()
		} else {
			d.backspace(u == byWord)
		}
	case k == ui.KeyDelete:
		d.forwardDelete(u == byWord)
	case k == ui.KeyTab:
		if shift {
			d.indent(-1)
		} else {
			d.indent(1)
		}
	case k == ui.KeyEscape:
		d.setCaret(d.caret())
		e.moved()
		return true
	case k == ui.KeyLeft || k == ui.KeyRight:
		dir := 1
		if k == ui.KeyLeft {
			dir = -1
		}
		e.horizontal(dir, u, shift)
		return true
	case k == ui.KeyUp || k == ui.KeyDown:
		dir := 1
		if k == ui.KeyUp {
			dir = -1
		}
		if cmd {
			to := Pos{0, 0}
			if dir > 0 {
				last := len(d.Blocks) - 1
				to = Pos{last, d.Blocks[last].len()}
			}
			e.moveTo(to, shift)
			return true
		}
		e.vertical(dir, shift)
		return true
	case k == ui.KeyHome || k == ui.KeyEnd:
		dir := 1
		if k == ui.KeyHome {
			dir = -1
		}
		p := d.caret()
		e.moveTo(Pos{p.B, e.lineEdge(p, dir)}, shift)
		return true
	default:
		return false
	}
	e.changed(0)
	return true
}

func (e *editor) moveTo(p Pos, extend bool) {
	d := e.doc
	if extend {
		d.Sel.Focus = p
	} else {
		d.setCaret(p)
	}
	e.moved()
}

// horizontal moves a letter, a word or to the line's edge, crossing into the
// neighbouring block at either end; a selection collapses to its edge first.
func (e *editor) horizontal(dir int, u unit, extend bool) {
	d := e.doc
	e.hasX = false
	if !d.Sel.collapsed() && !extend && u == byChar {
		a, z := d.Sel.ordered()
		if dir < 0 {
			e.moveTo(a, false)
		} else {
			e.moveTo(z, false)
		}
		return
	}
	e.fresh()
	p := d.caret()
	b := d.Blocks[p.B]
	var to Pos
	switch {
	case u == byLine:
		to = Pos{p.B, e.lineEdge(p, dir)}
	case dir < 0 && p.O == 0:
		if p.B == 0 {
			return
		}
		to = Pos{p.B - 1, d.Blocks[p.B-1].len()}
	case dir > 0 && p.O == b.len():
		if p.B+1 >= len(d.Blocks) {
			return
		}
		to = Pos{p.B + 1, 0}
	case u == byWord:
		to = Pos{p.B, wordEdge(b.Text, p.O, dir)}
	default:
		bx := e.boxes[p.B]
		if dir < 0 {
			to = Pos{p.B, bx.layout.PreviousBoundary(p.O)}
		} else {
			to = Pos{p.B, bx.layout.NextBoundary(p.O)}
		}
	}
	e.moveTo(d.clampPos(to), extend)
}

// vertical moves a visual line up or down, keeping the column it started at,
// and on into the block above or below.
func (e *editor) vertical(dir int, extend bool) {
	c, _ := e.caretRect(e.doc.caret())
	if !e.hasX {
		e.desiredX, e.hasX = c.X, true
	}
	y := c.Y - 4
	if dir > 0 {
		y = c.Y + c.H + 4
	}
	// Across a gap between blocks, go on to the next one's first or last line.
	p := e.hit(e.desiredX, y)
	if p == e.doc.caret() {
		if dir < 0 && p.B > 0 {
			bx := e.boxes[p.B-1]
			p = e.hit(e.desiredX, bx.ty+bx.h-styleOf(e.doc.Blocks[p.B-1].Kind).bottom-6)
		} else if dir > 0 && p.B+1 < len(e.boxes) {
			p = e.hit(e.desiredX, e.boxes[p.B+1].ty+2)
		} else if dir < 0 {
			p.O = 0
		} else {
			p.O = e.doc.Blocks[p.B].len()
		}
	}
	keep := e.desiredX
	e.moveTo(p, extend)
	e.desiredX, e.hasX = keep, true
}

// lineEdge is the start or end of the visual line p is on.
func (e *editor) lineEdge(p Pos, dir int) int {
	e.fresh()
	bx := e.boxes[p.B]
	b := e.doc.Blocks[p.B]
	if bx.layout == nil || b.Text == "" {
		return 0
	}
	c := bx.layout.Caret(p.O)
	mid := c.Y + c.H/2
	for _, l := range bx.layout.Lines() {
		if mid < l.Rect.Y || mid > l.Rect.Y+l.Rect.H {
			continue
		}
		if dir < 0 {
			return l.Start
		}
		end := min(l.End, b.len())
		if end > l.Start && end <= b.len() {
			if s := b.slice(end-1, end); s == "\n" || (s == " " && end < b.len()) {
				end--
			}
		}
		return end
	}
	return p.O
}

// checkboxAt is the task whose checkbox is under (x, y), in document coordinates.
func (e *editor) checkboxAt(x, y float32) (int, bool) {
	for i, bx := range e.boxes {
		b := e.doc.Blocks[i]
		if b.Kind != Todo || bx.layout == nil {
			continue
		}
		l := bx.layout.Lines()[0]
		mid := bx.ty + l.Baseline - l.Ascent*0.36
		gx := bx.tx - gutterW
		if x >= gx-2 && x <= gx+22 && y >= mid-12 && y <= mid+12 {
			return i, true
		}
	}
	return 0, false
}

// ---- the file ----

func (e *editor) save() {
	if e.path == "" {
		return
	}
	if err := os.WriteFile(e.path, []byte(toMarkdown(e.doc.Blocks)), 0o644); err != nil {
		e.saveErr = err.Error()
		return
	}
	e.doc.Dirty, e.saveErr = false, ""
}

// autosave writes the file a moment after the last edit.
func (e *editor) autosave(p *ui.Painter) {
	if !e.doc.Dirty || e.path == "" || e.saveErr != "" {
		return
	}
	if wait := 700*time.Millisecond - e.now().Sub(e.editAt); wait > 0 {
		p.After(wait)
		return
	}
	e.save()
}

func loadFile(path string) (*Doc, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	blocks := parseMarkdown(strings.TrimRight(string(src), "\n"))
	return newDoc(blocks...), nil
}
