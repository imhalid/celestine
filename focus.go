package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Ways of writing without being looked at by the rest of the page: the line or the paragraph the caret
// is in stays, the others are veiled; the typewriter keeps the caret's line in the middle of the window.
// And the small ones that go with them: how far a Tab goes, how soon the caret dozes off, the caret
// without its effects, and the way back to the page as it was before the last save.

// focusMode is "" (none), "line" or "paragraph".
func (a *app) focusMode() string {
	switch a.settings.Focus {
	case "line", "paragraph":
		return a.settings.Focus
	}
	return ""
}

// tabText is what a Tab writes: a tab, or n spaces.
func (a *app) tabText() string {
	if n := a.settings.TabSpaces; n >= 1 && n <= 8 {
		return strings.Repeat(" ", n)
	}
	return "\t"
}

// caretSleep is how long the caret is at rest before it dozes: five seconds when not set.
func (a *app) caretSleep() time.Duration {
	if s := a.settings.SleepSecs; s >= 1 && s <= 600 {
		return time.Duration(s * float64(time.Second))
	}
	return 5 * time.Second
}

// drawFocusVeil covers what is not the line or the paragraph of the caret, over the text.
func (a *app) drawFocusVeil(p *ui.Painter, r ui.Rect) {
	mode := a.focusMode()
	if mode == "" || !a.focused || a.doc == nil {
		return
	}
	cb, ok := locate(a.doc, a.boxes, r.W, a.font, a.theme)
	if !ok {
		return
	}
	veil := withAlpha(a.pageBg, 0.72)
	top, bottom := r.Y+cb.top, r.Y+cb.top // the paragraph's extent
	if mode == "line" {
		top, bottom = r.Y+cb.full.Y, r.Y+cb.full.Y+cb.full.H
	} else {
		for _, b := range a.boxes {
			if b.Start == cb.para {
				top, bottom = r.Y+b.Top, r.Y+b.Top+b.Height
			}
		}
	}
	vis := p.Visible()
	if top > vis.Y {
		p.Fill(ui.Rect{X: r.X - 400, Y: vis.Y, W: r.W + 800, H: top - vis.Y}, veil, 0)
	}
	if end := vis.Y + vis.H; bottom < end {
		p.Fill(ui.Rect{X: r.X - 400, Y: bottom, W: r.W + 800, H: end - bottom}, veil, 0)
	}
}

// revertToBeforeSave puts the page back to what it was before its last save.
func (a *app) revertToBeforeSave() {
	p := a.page
	if p == nil || a.noPage() {
		a.notice = "Açık bir sayfa yok."
		return
	}
	h, ok := historyOf(p)
	if !ok {
		a.notice = "Bu kitaplık sürüm tutmuyor."
		return
	}
	a.flush()
	vers := h.Versions(p.File())
	if len(vers) == 0 {
		a.notice = "Bu sayfanın önceki bir sürümü yok."
		return
	}
	data, ok := h.VersionData(p.File(), vers[0].ID)
	if !ok {
		return
	}
	h.SnapshotNow(p.File())
	_, body := SplitPage(data)
	if err := p.Save(body); err != nil {
		a.saveErr = err.Error()
		return
	}
	a.takeOutside(p)
	a.linksStale()
	a.notice = Tf("“%s” son kayıttan önceki hâline döndü", p.Title)
}
