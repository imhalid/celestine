package main

import (
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
}

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
	default: // a screen sliding away, or under the one shown: drawn as its file has it
		tmp := a.stateOf(p)
		ed, live = &tmp, false
	}
	if a.ed.dirty && !c.Now().Before(a.ed.saveAt) {
		a.flush()
	}
	day := a.journalDay(p)
	ui.Column(c).Fill().Children(func() {
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
					tint := collTint(p.Coll)
					ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(5, 12).Radius(14).Background(tint.Alpha(0.14)).Children(func() {
						ui.Box(c).Size(8, 8).Radius(4).Background(tint)
						ui.Text(c, p.Coll.Name).FontSize(13).FontWeight(700).TextColor(tint)
					})
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
			body := richtext.EditorBase(c.Key("body:"+p.ID), ed.doc, richtext.EditorOptions{
				Label: titleOf(p), Placeholder: locales.PagePlaceholder(), ReadOnly: ed.readOnly != "" || !live,
				Font: ui.Font{Family: fontFamily, Size: 18, LineHeight: 1.55}, Theme: textTheme(),
			}).Grow(1).Basis(0).FillWidth()
			if body.Changed() && live {
				a.typed(c.Now())
			}
			ed.writing = live && body.Focused() && ed.readOnly == ""
		})
	})
}
