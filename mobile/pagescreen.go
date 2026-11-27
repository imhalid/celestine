package main

import (
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// A page's screen: its collection, its title and its text, the whole screen to write in and nothing else; the
// Markdown a line starts with makes it a heading, a list or a checklist ("# ", "- ", "[] "), and Done at the top puts
// the keyboard away. What is typed is saved a moment after the typing stops,
// as the screen is left, and as the app goes to the background. A past day of the journal, and a page a newer
// Celestine wrote, are read only.

// editorState is the page being written.
type editorState struct {
	id       string             // the page's ID; "" for none
	doc      *richtext.Document // its text
	title    string             // its title as typed
	readOnly string             // why it cannot be changed; "" when it can
	dirty    bool               // typed since it was saved
	saveAt   time.Time          // when it is saved, the typing having stopped
	writing  bool               // its text has the keyboard: the bar of formats shows
	opened   time.Time          // when its screen came, its text shown once it has landed
	letters  []letter           // its letters as they hop into place, where the editor shows them
	probed   bool               // the letters were found
	hopped   time.Time          // when they started to hop
}

// A letter is one of a page's letters as it hops into place (#652): its rune, where the editor shows it (its line's
// top and height, in DIPs from the editor's top left), and its run's look.
type letter struct {
	r       rune
	x, y, h float32
	size    float32
	weight  int
	italic  bool
	color   ui.Color
}

// The letters' hop: each takes hopFor, the next starting hopStep after it, all of them within hopSpread; at most
// maxLetters hop, those the editor shows first.
const (
	hopFor     = 420 * time.Millisecond
	hopStep    = 9 * time.Millisecond
	hopSpread  = 460 * time.Millisecond
	maxLetters = 700
)

// landing is how long a page's screen takes to come before its text is shown: MyGo's rich text editor is costly to
// draw while it moves, which made a page stutter as it opened (#650); at rest it draws fast.
const landing = 380 * time.Millisecond

// saveQuiet is how long after the last key the page is saved.
const saveQuiet = 800 * time.Millisecond

// stateOf is page p to be written, read from its file.
func (a *app) stateOf(p *library.Page) editorState {
	ed := editorState{id: p.ID, title: p.Title}
	if a.fresh == p.ID {
		ed.title = ""
	}
	body, err := p.Body()
	switch {
	case err != nil:
		ed.readOnly = err.Error()
	case p.Unreadable != "" || library.BodyReadable(body) != nil:
		ed.readOnly = locales.PageUnreadable()
	case p.Locked != "":
		ed.readOnly = locales.PageLocked()
	}
	text := richtext.Text{}
	if d, err := library.DecodeBody(body); err == nil {
		text = textOf(d)
	}
	ed.doc = richtext.NewDocument(text)
	return ed
}

// pagePath is the router's path of page p's screen.
func pagePath(p *library.Page) string { return "/p/" + url.PathEscape(p.ID) }

// letterHop has a page's letters hop into place as it is shown (#652). MyGo's editor draws its text whole, so the
// letters are drawn over it for the moment: the frame after the editor first shows (hidden), each visible letter's
// place is read from it, the caret put at each in turn; then they hop one after another in reading order, lightly and
// where they stand, the editor hidden under them, and it is shown in their place as they settle.
func (a *app) letterHop(c *ui.Context, ed *editorState) {
	if ed.opened.IsZero() || (ed.probed && ed.letters == nil) {
		return
	}
	now := c.Now()
	cover := func(alpha float32) { // the editor drawn as ever under the page's ground, so that it lays its text out
		ui.Box(c.Key("cover")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(pal.bg.Alpha(alpha)).
			HitTest(func(x, y float32, r ui.Rect) bool { return false })
	}
	if !ed.probed {
		c.AnimationFrame()
		if !ed.probeLetters() {
			if now.Sub(ed.opened) > landing+300*time.Millisecond { // it never told: the text as it is
				ed.probed, ed.letters = true, nil
				return
			}
			cover(1)
			return
		}
		ed.hopped = now
		if ed.letters == nil {
			return
		}
	}
	t := now.Sub(ed.hopped)
	n := len(ed.letters)
	step := min(hopStep, hopSpread/time.Duration(max(n, 1)))
	total := step*time.Duration(n) + hopFor
	if t >= total {
		ed.letters = nil
		return
	}
	c.AnimationFrame()
	// The editor comes in under the letters as the last of them settle, its marks (a task's box, a list's dot)
	// with it.
	cover(1 - min(max(float32(t-(total-140*time.Millisecond))/float32(140*time.Millisecond), 0), 1))
	ui.Box(c.Key("letters")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		HitTest(func(x, y float32, r ui.Rect) bool { return false }).Children(func() {
		for i, l := range ed.letters {
			p := float32(t-step*time.Duration(i)) / float32(hopFor)
			if p <= 0 {
				continue
			}
			p = min(p, 1)
			// It comes up from a little below, past its place, and settles: a light hop where it stands.
			off := 10 * float32(math.Exp(float64(-4*p))) * float32(math.Cos(float64(2.8*math.Pi*p)))
			lt := ui.Text(c.Key(i), string(l.r)).Absolute().Left(l.x).Top(l.y + off).FontSize(l.size).
				FontWeight(l.weight).LineHeight(l.h / l.size).TextColor(l.color.Alpha(min(p*4, 1)))
			if l.italic {
				lt.Italic()
			}
		}
	})
}

// probeLetters reads where the editor shows each letter of the paragraphs it shows, putting the caret at each and
// back where it was; false while the editor has not laid the text out yet.
func (ed *editorState) probeLetters() bool {
	doc := ed.doc
	if _, ok := doc.CaretRect(); !ok {
		return false
	}
	text := doc.Text()
	anchor, caret := doc.Selection()
	defer doc.SetSelection(anchor, caret)
	var out []letter
	for _, box := range doc.VisibleParagraphs() {
		start, end := text.ParagraphRange(box.Index)
		style := text.ParagraphStyleAt(start)
		size := float32(18)
		weight := 400
		if style.Heading > 0 && style.Heading <= 6 {
			th := textTheme()
			size, weight = 18*th.Headings[style.Heading-1].Size, th.Headings[style.Heading-1].Weight
			if size == 0 { // the theme's own: the default sizes
				size = 18 * []float32{1.8, 1.45, 1.2, 1.1, 1, 0.9}[style.Heading-1]
			}
			if weight == 0 {
				weight = 700
			}
		}
		i := start
		for _, r := range text.Slice(start, end).String() {
			if len(out) >= maxLetters {
				ed.probed, ed.letters = true, out
				return true
			}
			at := text.AttributesAt(i)
			doc.SetSelection(i, i)
			rect, ok := doc.CaretRect()
			i++
			if !ok || r == ' ' || r == '\t' || r == '\uFFFC' {
				continue
			}
			l := letter{r: r, x: rect.X, y: rect.Y, h: rect.H, size: size, weight: weight, italic: at.Italic, color: pal.text}
			if at.Size > 0 {
				l.size = at.Size
			}
			if at.Weight > 0 {
				l.weight = at.Weight
			}
			if at.Color.A > 0 {
				l.color = at.Color
			} else if at.Link != "" {
				l.color = pal.accent
			}
			out = append(out, l)
		}
	}
	ed.probed, ed.letters = true, out
	if len(out) == 0 {
		ed.letters = nil
	}
	return true
}

// flush saves what was typed in the page being written.
func (a *app) flush() {
	if a.ed.id == "" || !a.ed.dirty || a.ed.readOnly != "" {
		return
	}
	a.ed.dirty = false
	p := a.lib.PageByID(a.ed.id)
	if p == nil {
		return
	}
	if title := strings.TrimSpace(a.ed.title); title != "" && title != p.Title && !a.journalDay(p) {
		if err := p.SetTitle(title); err != nil {
			a.say(locales.PageSaveFailed(err.Error()))
		}
	}
	if err := p.Save(library.EncodeBody(docOf(a.ed.doc.Text()))); err != nil {
		a.say(locales.PageSaveFailed(err.Error()))
		return
	}
	a.syncSoon()
}

// leaveEditor saves the page that was being written once the router shows another screen. A screen left still draws
// while it slides away, from its file again.
func (a *app) leaveEditor() {
	if a.ed.id != "" && a.router.Path() != "/p/"+url.PathEscape(a.ed.id) {
		a.flush()
		a.ed = editorState{}
	}
}

// typed marks the page changed at now, to be saved once the typing stops.
func (a *app) typed(now time.Time) {
	a.ed.dirty, a.ed.saveAt = true, now.Add(saveQuiet)
	time.AfterFunc(saveQuiet+50*time.Millisecond, func() {
		if a.win != nil {
			a.win.Update(func() {}) // a frame, which saves if the typing stopped
		}
	})
}

func (a *app) pageScreen(c *ui.Context, p *library.Page) {
	ed, live := &a.ed, true
	switch {
	case a.ed.id == p.ID:
	case a.router.Path() == pagePath(p): // the screen shown: the page being written
		a.flush()
		a.ed = a.stateOf(p)
		a.ed.opened = c.Now()
	default: // a screen sliding away, or under the one shown: drawn as its file has it
		tmp := a.stateOf(p)
		ed, live = &tmp, false
	}
	if a.ed.dirty && !c.Now().Before(a.ed.saveAt) {
		a.flush()
	}
	day := a.journalDay(p)
	screen := ui.Column(c.Key("page")).Fill().Background(pal.bg)
	if !ed.writing { // its block in a list grows into it (#649); not while the keyboard is up, so that it resizes at once
		screen.Transition(pageFlight(p.ID))
	}
	screen.Children(func() {
		a.topBar(c, true, func() {
			if !live {
				return
			}
			if ed.writing { // Done puts the keyboard away: it takes the focus from the text
				done := ui.Row(c.Key("done")).Height(46).Padding(0, 20).Radius(23).AlignItems(ui.Center).
					Background(pal.accent).Role(ui.RoleButton).Label(locales.PageDone()).Focusable()
				pressable(done)
				done.Children(func() { ui.Text(c, locales.PageDone()).FontSize(16).FontWeight(700).TextColor(pal.accentText) })
				if done.Clicked() {
					done.Focus()
					a.flush()
				}
				return
			}
			if roundButton(c, "more", iconMore, locales.PageMore()) {
				a.moreOpen = true
			}
		})
		ui.Column(c).Grow(1).Basis(0).FillWidth().Padding(0, side+4).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 0, 6, 0).Children(func() {
				if p.Coll != nil {
					collChip(c, p.Coll)
				}
				if ed.readOnly != "" {
					ui.Icon(c, iconLock).Size(16, 16).TextColor(pal.faint)
				}
			})
			if day || ed.readOnly != "" || !live {
				ui.Text(c, titleOf(p)).FontSize(32).FontWeight(800).LineHeight(1.1).Wrap().Padding(4, 0, 14, 0)
			} else {
				in := ui.TextInput(c.Key("title"), &ed.title).Placeholder(locales.PageTitle()).FontSize(32).FontWeight(800).
					Background(pal.bg).Border(0, pal.bg).Padding(4, 0, 14, 0).Label(locales.PageTitle())
				if a.fresh == p.ID {
					in.AutoFocus()
					a.fresh = ""
				}
				if in.Changed() {
					a.typed(c.Now())
				}
			}
			if ed.readOnly != "" {
				ui.Text(c, ed.readOnly).FontSize(14).LineHeight(1.4).TextColor(pal.muted).Wrap().Padding(12, 16).Radius(16).
					Background(pal.raised).Margin(0, 0, 14, 0)
			}
			// Its text once it has landed, and not as it goes: in the meantime its room, empty.
			if !live || c.Now().Sub(ed.opened) < landing {
				if live {
					c.AnimationFrame()
				}
				ui.Box(c.Key("body-room")).Grow(1).Basis(0).FillWidth()
				return
			}
			var body ui.Element
			ui.Box(c.Key("body-box")).Grow(1).Basis(0).FillWidth().Children(func() {
				body = richtext.EditorBase(c.Key("body:"+p.ID), ed.doc, richtext.EditorOptions{
					Label: titleOf(p), Placeholder: locales.PagePlaceholder(), ReadOnly: ed.readOnly != "" || !live,
					Font: ui.Font{Family: fontFamily, Size: 18, LineHeight: 1.55}, Theme: textTheme(),
				}).Fill()
				a.letterHop(c, ed)
			})
			if body.Changed() && live {
				a.typed(c.Now())
			}
			ed.writing = live && body.Focused() && ed.readOnly == ""
		})
	})
}
