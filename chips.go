package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// Chips: a link to a page and a date, in the text, as one object each -
// richtext's attachments, drawn here. In the file they are Obsidian's
// links: [[Dune]], [[Dune|the book]], and a date [[2026-11-05]] (as the
// day's journal page will be called). A click on a page's chip opens it;
// a link to a page that is not there makes it.

// chip is what an attachment of ours is.
type chip struct {
	kind   string // "page" or "date"
	target string // the page's title, or the day as 2006-01-02
	alias  string // what a page's chip says instead of its title
	// a fold's: what it hides, and how many lines (fold.go)
	hidden *richtext.Text
	lines  int
}

// markdown is the chip in the file.
func (ch *chip) markdown() string {
	if ch.kind == "embed" {
		return "![[" + ch.target + "]]"
	}
	if ch.alias != "" {
		return "[[" + ch.target + "|" + ch.alias + "]]"
	}
	return "[[" + ch.target + "]]"
}

// label is what the chip says.
func (ch *chip) label() string {
	if ch.kind == "fold" {
		return "⋯ " + strconv.Itoa(ch.lines) + " satır"
	}
	if ch.kind == "date" {
		if d, err := time.Parse("2006-01-02", ch.target); err == nil {
			return shortDate(d)
		}
	}
	if ch.alias != "" {
		return ch.alias
	}
	return ch.target
}

const (
	chipSize = 14  // its text
	chipH    = 21  // its height
	chipPad  = 9   // room on each side of its text
	chipIcon = 14  // a date's calendar
	chipEm   = .57 // a letter's width, in ems, to size it before it is drawn
)

var isDay = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// chipOf reads a link's inside: "Dune", "Dune|the book", "2026-11-05".
func chipOf(inside string) *chip {
	target, alias, _ := strings.Cut(inside, "|")
	target = strings.TrimSpace(target)
	if isDay.MatchString(target) && alias == "" {
		return &chip{kind: "date", target: target}
	}
	return &chip{kind: "page", target: target, alias: strings.TrimSpace(alias)}
}

// attachment is the chip as richtext's attachment; how it is drawn
// depends on the app (its pages, its colors), so the app gives it.
func (a *app) attachment(ch *chip) *richtext.Attachment {
	w := float32(utf8.RuneCountInString(ch.label()))*chipSize*chipEm + 2*chipPad
	if ch.kind == "date" {
		w += chipIcon + 4
	}
	return &richtext.Attachment{Width: w, Height: chipH, Name: ch.label(), Data: ch, Draw: func(p *ui.Painter, r ui.Rect) { a.drawChip(p, r, ch) }}
}

var wikiInText = regexp.MustCompile(`\[\[([^\[\]\n]+?)\]\]`)

// withChips turns the [[links]] in t, outside code, into chips.
func (a *app) withChips(t richtext.Text) richtext.Text {
	s := t.String()
	if !strings.Contains(s, "[[") {
		return t
	}
	locs := wikiInText.FindAllStringSubmatchIndex(s, -1)
	for i := len(locs) - 1; i >= 0; i-- {
		m := locs[i]
		start := utf8.RuneCountInString(s[:m[0]])
		end := start + utf8.RuneCountInString(s[m[0]:m[1]])
		if t.ParagraphStyleAt(start).CodeBlock || t.AttributesAt(start).Code {
			continue
		}
		attrs := t.AttributesAt(start)
		attrs.Attachment = a.attachment(chipOf(s[m[2]:m[3]]))
		t = t.ReplaceString(start, end, string(richtext.ObjectReplacement), attrs)
	}
	return t
}

// parse reads a page's Markdown into the editor's text, its links as chips.
func (a *app) parse(md string) richtext.Text { return a.withChips(a.withEmbeds(parseBody(md))) }

// withoutChips turns the chips in t back into the text of their links.
func withoutChips(t richtext.Text) richtext.Text {
	type at struct {
		i  int
		ch *chip
		a  richtext.Attributes
	}
	var found []at
	for i, r := range t.Runs(0, t.Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok {
				plain := r.Attributes
				plain.Attachment = nil
				found = append(found, at{i, ch, plain})
			}
		}
	}
	for k := len(found) - 1; k >= 0; k-- {
		f := found[k]
		if f.ch.kind == "fold" { // what was hidden comes back, under its line
			_, pe := t.ParagraphRange(f.i)
			t = t.Replace(pe+1, pe+1, withoutChips(*f.ch.hidden))
			t = t.Replace(f.i, f.i+1, richtext.Text{})
			continue
		}
		t = t.ReplaceString(f.i, f.i+1, f.ch.markdown(), f.a)
	}
	return t
}

// chipHit is a chip drawn this frame, where it is in the window.
type chipHit struct {
	r  ui.Rect
	ch *chip
}

// drawChip draws a chip: a page's in the accent, a link to a page that
// is not there faint and dashed; a date with a calendar.
func (a *app) drawChip(p *ui.Painter, r ui.Rect, ch *chip) {
	t := ryugo.Painting(p)
	cl := t.Colors
	r = ui.Rect{X: r.X + 1, Y: r.Y, W: r.W - 2, H: r.H}
	a.chipHits = append(a.chipHits, chipHit{r, ch})
	text := cl.Accent
	hover := a.hoverCh == ch
	switch {
	case ch.kind == "fold":
		p.Fill(r, cl.Fill, r.H/2)
		if hover {
			p.Fill(r, cl.Text.Alpha(0.08), r.H/2)
		}
		text = cl.Muted
	case ch.kind == "date":
		p.Fill(r, cl.Fill, r.H/2)
		if hover {
			p.Fill(r, cl.Text.Alpha(0.08), r.H/2)
		}
		text = cl.Text2
	case a.pageTitled(ch.target) != nil:
		p.Fill(r, cl.Accent.Alpha(0.12), r.H/2)
		if hover {
			p.Fill(r, cl.Accent.Alpha(0.16), r.H/2)
		}
	default:
		p.Stroke(r, cl.Muted.Alpha(0.6), r.H/2, 1)
		if hover {
			p.Fill(r, cl.Muted.Alpha(0.14), r.H/2)
		}
		text = cl.Muted
	}
	x := r.X + chipPad - 1
	if ch.kind == "date" {
		ic := ui.Rect{X: x, Y: r.Y + (r.H-chipIcon)/2, W: chipIcon, H: chipIcon}
		p.Stroke(ui.Rect{X: ic.X + 1, Y: ic.Y + 2, W: ic.W - 2, H: ic.H - 3}, text, 2, 1.2)
		p.Fill(ui.Rect{X: ic.X + 1, Y: ic.Y + 5, W: ic.W - 2, H: 1.2}, text, 0)
		x += chipIcon + 4
	}
	span := ui.Span{Text: ch.label(), Size: chipSize, Weight: 520, Color: text}
	_, h := p.MeasureText(r.W, span)
	p.RichText(x, r.Y+(r.H-h)/2, r.X+r.W-x, span)
}

// pageTitled is the page titled title (any case), in the open page's
// collection first, or nil.
func (a *app) pageTitled(title string) *Page {
	var found *Page
	for _, c := range a.lib.Collections {
		for _, p := range c.All() {
			if strings.EqualFold(p.Title, title) {
				if a.page != nil && p.Coll == a.page.Coll {
					return p
				}
				if found == nil {
					found = p
				}
			}
		}
	}
	return found
}

// chipAt is the chip drawn at x, y in the window, or nil.
func (a *app) chipAt(x, y float32) *chipHit {
	for i := range a.chipSeen {
		h := &a.chipSeen[i]
		if x >= h.r.X && x <= h.r.X+h.r.W && y >= h.r.Y && y <= h.r.Y+h.r.H {
			return h
		}
	}
	return nil
}

// clickChip opens the page of the chip at x, y in the window, making it
// first if it is not there, and reports whether there was one; a date's
// opens a calendar to change it.
func (a *app) clickChip(x, y float32) bool {
	if h := a.chipAt(x, y); h != nil {
		if h.ch.kind == "fold" {
			a.unfold(h.ch)
			return true
		}
		if h.ch.kind == "embed" { // the page it shows
			if p := a.pageTitled(h.ch.target); p != nil {
				a.open(p)
			}
			return true
		}
		if h.ch.kind == "date" {
			d, err := time.Parse("2006-01-02", h.ch.target)
			if err != nil {
				d = a.clock()
			}
			a.dateEdit = dateEdit{open: true, ch: h.ch, at: h.r, cal: d}
			return true
		}
		p := a.pageTitled(h.ch.target)
		if p == nil && a.page != nil {
			np, err := a.page.Coll.NewPage(nil, h.ch.target)
			if err != nil {
				a.saveErr = err.Error()
				return true
			}
			p = np
		}
		if p != nil {
			a.open(p)
		}
		return true
	}
	return false
}

// dateEdit is the calendar a date chip opens, under the chip.
type dateEdit struct {
	open bool
	ch   *chip
	at   ui.Rect
	cal  time.Time
}

// chipPos is where the chip is in the text, or -1.
func (a *app) chipPos(ch *chip) int {
	for i, r := range a.doc.Text().Runs(0, a.doc.Text().Len()) {
		if att := r.Attributes.Attachment; att != nil {
			// the text keeps its own copies of what an attachment carries: by value
			if other, ok := att.Data.(*chip); ok && *other == *ch {
				return i
			}
		}
	}
	return -1
}

// dateEditView is the calendar of a date chip, over the window: a day
// chosen changes the chip; a click elsewhere or Esc closes it.
func (a *app) dateEditView(c *ui.Context) {
	d := &a.dateEdit
	if !d.open || a.doc == nil {
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		d.open = false
		return
	}
	t := ryugo.From(c)
	cl := t.Colors
	ww, wh := c.Size()
	left := max(8, min(d.at.X, ww-260-8))
	top := d.at.Y + d.at.H + 6
	if top+300 > wh {
		top = max(8, d.at.Y-6-300)
	}
	ui.Overlay(c, func() {
		if ui.Box(c.Key("date-away")).Absolute().Left(0).Top(0).Size(ww, wh).Clicked() {
			d.open = false
			return
		}
		panel := ui.Column(c.Key("date-edit")).Absolute().Left(left).Top(top).Width(260).Padding(8).
			Radius(t.R(ryugo.RadiusControl)).Background(cl.Card).Border(1, cl.Hairline).Label("Tarih")
		t.Shadows.ShadowPop.Apply(panel)
		panel.Children(func() {
			if ryugo.Button(c.Key("date-day"), "Günün sayfasını aç", ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconPage}).Clicked() {
				d.open = false
				a.openDay(d.cal)
				return
			}
			if cal := ui.Calendar(c, &d.cal); cal.Changed() {
				if i := a.chipPos(d.ch); i >= 0 {
					nc := &chip{kind: "date", target: d.cal.Format("2006-01-02")}
					attrs := a.doc.Text().AttributesAt(i)
					attrs.Attachment = a.attachment(nc)
					a.doc.Edit(func(ed *richtext.Edit) {
						ed.ReplaceString(i, i+1, string(richtext.ObjectReplacement), attrs)
					})
				}
				d.open = false
			}
		})
	})
}

var wikiAtEnd = regexp.MustCompile(`\[\[([^\[\]\n]+?)\]\]$`)

// chipShortcut turns a [[link]] into a chip as its last bracket is typed,
// outside code; it reports whether it did, the caret after the chip.
func (a *app) chipShortcut(text richtext.Text, caret int, typed bool) bool {
	if !typed || caret < 5 || text.Slice(caret-1, caret).String() != "]" {
		return false
	}
	ps, _ := text.ParagraphRange(caret - 1)
	if text.ParagraphStyleAt(ps).CodeBlock || text.AttributesAt(caret-1).Code {
		return false
	}
	line := text.Slice(ps, caret).String()
	m := wikiAtEnd.FindStringSubmatchIndex(line)
	if m == nil {
		return false
	}
	start := ps + utf8.RuneCountInString(line[:m[0]])
	attrs := text.AttributesAt(start)
	attrs.Attachment = a.attachment(chipOf(line[m[2]:m[3]]))
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.ReplaceString(start, caret, string(richtext.ObjectReplacement), attrs)
		ed.Select(start+1, start+1)
	})
	a.doc.SetTypingAttributes(richtext.Attributes{})
	return true
}

var wikiOpen = regexp.MustCompile(`\[\[[^\[\]\n]*\]?$`)

// overtype takes the "]" typed over the "]" the "[[" closed itself with: the
// caret goes past it, as in any editor that closes brackets; it reports
// whether it did.
func (a *app) overtype(text richtext.Text, caret int, typed bool) bool {
	if !typed || caret < 3 || caret >= text.Len() || text.Slice(caret-1, caret+1).String() != "]]" {
		return false
	}
	ps, _ := text.ParagraphRange(caret - 1)
	if !wikiOpen.MatchString(text.Slice(ps, caret-1).String()) {
		return false
	}
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(caret, caret+1, richtext.Text{})
		ed.Select(caret, caret)
	})
	return true
}

// insertChip puts a chip in place of the text from start to the caret,
// with a space after it.
func (a *app) insertChip(start int, ch *chip) {
	_, caret := a.doc.Selection()
	// a "[[" the menu closed itself with "]]": they go with what was typed
	if t := a.doc.Text(); a.slash.sign == "[[" && caret+2 <= t.Len() && t.Slice(caret, caret+2).String() == "]]" {
		caret += 2
	}
	attrs := a.doc.Text().AttributesAt(start)
	attrs.Attachment = nil
	plain := attrs
	attrs.Attachment = a.attachment(ch)
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.ReplaceString(start, caret, string(richtext.ObjectReplacement), attrs)
		ed.ReplaceString(start+1, start+1, " ", plain)
		ed.Select(start+2, start+2)
	})
	a.doc.SetTypingAttributes(plain)
}

// parseDay reads a day written 2026-11-05.
func parseDay(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }
