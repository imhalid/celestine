package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The page's "…" menu, at the top right of the window while a page is open:
// what is done with the page as a whole.

var iconMore = icon(`<circle cx="6" cy="12" r="1.3"/><circle cx="12" cy="12" r="1.3"/><circle cx="18" cy="12" r="1.3"/>`)
var iconExport = icon(`<path d="M12 4.5v10M8 8.5l4-4 4 4"/><path d="M5 14.5v3a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-3"/>`)

// pageButtons are the side panel's button and the page menu's "…", at the
// right end of the status bar while a page is open.
func (a *app) pageButtons(c *ui.Context) {
	pn := a.pane()
	if pn.page == nil || a.noPage() {
		return
	}
	ui.Row(c.Key("page-buttons")).Gap(2).Children(func() {
		if !phone && a.barButton(c, iconPanel, locales.CommonSidePanel()+" (⇧⌘\\)") { // a phone has no side panel
			a.togglePanel()
		}
		if _, ok := historyOf(pn.page); ok && a.barButton(c, iconLapse, map[bool]string{false: locales.CommonTimeLapse(), true: locales.CommonBackText()}[a.lapse.on]) {
			a.toggleLapse()
		}
		if a.barButton(c, iconMore, locales.PageMenu()) {
			ww, wh := c.Size()
			a.showContext(ww-24, wh-34, a.pageMenu(pn.page, false)) // opens upward, from the bar
		}
	})
}

// copyText puts text on the clipboard.
func (a *app) copyText(s string) {
	if a.clipText != nil {
		a.clipText(s)
		return
	}
	mygo.Clipboard.WriteText(s)
}

// copySecret puts a secret on the clipboard as a concealed item so clipboard
// managers and cross-device clipboards keep it out of history. It never
// falls back to un-concealed clipboard writes on failure.
func (a *app) copySecret(s string) bool {
	opts := mygo.ClipboardOptions{Concealed: true}
	if a.clipWrite != nil {
		if err := a.clipWrite(s, opts); err != nil {
			a.say(toastErr, locales.CommonDidntWork(err.Error()))
			return false
		}
		return true
	}
	if a.clipText != nil {
		a.clipText(s)
		return true
	}
	if err := mygo.Clipboard.Write(transfer.TextData(s), opts); err != nil {
		a.say(toastErr, locales.CommonDidntWork(err.Error()))
		return false
	}
	return true
}

var ryugoIconCopy = ryugo.Svg(16, 14, "M5.5 5.5v-2A1.5 1.5 0 0 1 7 2h5.5A1.5 1.5 0 0 1 14 3.5V9a1.5 1.5 0 0 1-1.5 1.5h-2M3.5 5.5H9A1.5 1.5 0 0 1 10.5 7v5.5A1.5 1.5 0 0 1 9 14H3.5A1.5 1.5 0 0 1 2 12.5V7a1.5 1.5 0 0 1 1.5-1.5z")

// drawCheckMark animates a self-drawing check stroke matching ryugo.CopyButton's checkmark.
func drawCheckMark(c *ui.Context, size, width float32, on bool, color ui.Color) ui.Element {
	m := ui.Box(c).Size(size, size).Shrink(0)
	anim := ui.Animation{Duration: 260 * time.Millisecond, Ease: ui.CubicBezier(0.65, 0, 0.35, 1)}
	if !on {
		anim = ui.Animation{Duration: 120 * time.Millisecond, Ease: ui.EaseOut}
	}
	drawn := m.AnimateTo("check", b2f(on), anim)
	if drawn <= 0.001 {
		return m
	}
	if !on {
		m.Opacity(drawn * drawn)
	}
	m.Draw(func(p *ui.Painter, r ui.Rect) {
		at := func(x, y float32) (float32, float32) { return r.X + x*r.W/16, r.Y + y*r.H/16 }
		var path ui.Path
		path.MoveTo(at(3.5, 8.5)).LineTo(at(6.5, 11.5)).LineTo(at(12.5, 4.5))
		p.StrokePath(path.Trim(0, drawn), width, color)
	})
	return m
}

// styleRaisedControl applies ryugo's raised control surface styling (gradient, border, shadow, hover/press/focus).
func styleRaisedControl(e ui.Element, t *ryugo.Tokens) {
	cl := t.Colors
	f := float32(1)
	switch {
	case e.Pressed():
		f = 0.94
		e.Translate(0, 1)
	case e.Hovered():
		f = t.ControlHover
	}
	stops := []ui.GradientStop{
		{Color: ryugo.Brightness(cl.ControlTop, f), At: 0},
		{Color: ryugo.Brightness(cl.ControlBottom, f), At: 0.72},
		{Color: ryugo.Brightness(ryugo.Mix(cl.ControlBottom, cl.ControlTop, 0.45), f), At: 1},
	}
	e.LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, Stops: stops}).
		Border(1, ryugo.Brightness(cl.ControlBorder, f)).
		FocusRing(false).
		Transition(ui.ElementTransition{Duration: 150 * time.Millisecond, Colors: true, Ease: ui.EaseOut})
	sh := t.Shadows.ControlShadow
	if e.FocusVisible() {
		sh = t.Shadows.FocusRing
	}
	sh.Apply(e)
}

// secretCopyButton renders a round copy button matching ryugo.CopyButton's look and check-draw
// animation while writing text as a concealed clipboard item.
func (a *app) secretCopyButton(c *ui.Context, key, text, label string) ui.Element {
	const copiedFor = 1500 * time.Millisecond
	if label == "" {
		label = locales.CommonCopy()
	}
	t := ryugo.From(c)
	m := t.Metrics
	cl := t.Colors
	e := ui.ButtonBase(c.Key(key)).Center().Size(m.BtnH, m.BtnH).Radius(t.Pill(m.BtnH)).Cursor(ui.CursorPointer)
	styleRaisedControl(e, t)
	at := ui.Local(e, "copied", func() time.Time { return time.Time{} })
	if e.Clicked() && a.copySecret(text) {
		*at = c.Now()
		c.Announce(locales.CommonCopied())
	}
	copied := !at.IsZero() && c.Now().Sub(*at) < copiedFor
	if copied {
		label = locales.CommonCopied()
		c.After(copiedFor - c.Now().Sub(*at))
	}
	e.Label(label)
	icon := e.AnimateTo("icon", b2f(!copied), ui.Animation{Duration: 160 * time.Millisecond, Ease: ui.EaseOut})
	e.TextColor(cl.Text).Children(func() {
		ui.Box(c).Size(16, 16).Children(func() {
			if icon > 0.01 {
				ui.Box(c).Absolute().Left(0).Top(0).Size(16, 16).Center().Opacity(icon).Scale(0.6+0.4*icon, 0.6+0.4*icon).Children(func() {
					ui.Icon(c, ryugoIconCopy)
				})
			}
			ui.Box(c).Absolute().Left(0).Top(0).Children(func() {
				drawCheckMark(c, 16, 2, copied, cl.Down)
			})
		})
	})
	return e
}

// currentMarkdown is page p's Markdown as it is in the editor now, its
// front matter left out.
func (a *app) currentMarkdown(p *Page) string {
	if pn := a.pane(); p == pn.page && pn.doc != nil {
		a.flush()
	}
	body, _ := p.Text()
	return body
}

// duplicatePage makes a copy of p, its sub-pages and pictures with it, right
// after it, and opens it.
func (a *app) duplicatePage(p *Page) {
	if p == a.pane().page {
		a.flush()
	}
	fsys := p.FS()
	container := filepath.Dir(p.Dir)
	dir, err := UniqueDir(fsys, container, filepath.Base(p.Dir)+"-kopya")
	if err == nil {
		err = CopyTree(fsys, p.Dir, fsys, dir)
	}
	if err != nil {
		a.fail(err)
		return
	}
	cp := &Page{Title: locales.PageCopyTitle(p.Title), Created: a.clock().Truncate(1e9), Dir: dir, Coll: p.Coll, Parent: p.Parent, Props: map[string]string{}}
	for k, v := range p.Props {
		cp.Props[k] = v
	}
	body, _ := cp.Body() // the copy's, as it came
	cp.Children = ReadPages(dir, p.Coll, cp)
	for _, ch := range cp.Children {
		ch.Parent = cp
	}
	siblings := &p.Coll.Pages
	if p.Parent != nil {
		siblings = &p.Parent.Children
	}
	at := 0
	for i, q := range *siblings {
		if q == p {
			at = i + 1
		}
	}
	*siblings = append((*siblings)[:at], append([]*Page{cp}, (*siblings)[at:]...)...)
	if err := cp.Save(body); err != nil {
		a.fail(err)
	}
	if err := Renumber(*siblings); err != nil {
		a.fail(err)
	}
	a.open(cp)
}

// exportPage writes the page as a Markdown file where a dialog says, its
// pictures beside it, which its lines name by their files.
func (a *app) exportPage(p *Page) {
	md := a.currentMarkdown(p)
	title := p.Title
	go func() {
		defer a.guard()
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Title:       locales.CommonExport(),
			DefaultPath: safeFile(title) + ".md",
			Filters:     []mygo.FileFilter{{Name: "Markdown", Extensions: []string{"md"}}},
		})
		if err != nil || path == "" {
			return
		}
		a.do(func() error {
			a.finishExport(p, md, path)
			return nil
		})
	}()
}

// finishExport writes the export or asks first if attachments in the target folder would be overwritten (#185).
func (a *app) finishExport(p *Page, md, path string) {
	write := func(overwrite bool) {
		if err := a.writeExportMode(p, md, path, overwrite); err != nil {
			a.say(toastErr, locales.CommonDidntWork(err.Error()))
		} else {
			a.say(toastOK, locales.CommonSavedTo(path))
		}
	}
	if conflicts := exportCollisions(p, md, path); len(conflicts) > 0 {
		a.askChoice(
			locales.PageExportOverwriteTitle(),
			locales.PageExportOverwriteBody(strings.Join(conflicts, ", ")),
			locales.PageExportSkipExisting(),
			func() { write(false) },
			locales.PageExportOverwriteAction(),
			func() { write(true) },
		)
		return
	}
	write(false)
}

// exportAttachmentNames returns the attachment filenames in p's folder that md references.
func exportAttachmentNames(p *Page, md string) []string {
	entries, _ := p.FS().ReadDir(p.Dir)
	var out []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == PageFileName || !strings.Contains(md, e.Name()) {
			continue
		}
		out = append(out, e.Name())
	}
	return out
}

// exportCollisions returns the filenames of p's attachments referenced by md that already exist next to path.
func exportCollisions(p *Page, md, path string) []string {
	dir := filepath.Dir(path)
	var out []string
	for _, name := range exportAttachmentNames(p, md) {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			out = append(out, name)
		}
	}
	return out
}

// writeExport writes md to path and the pictures of p beside it without overwriting existing attachments.
func (a *app) writeExport(p *Page, md, path string) error {
	return a.writeExportMode(p, md, path, false)
}

// writeExportMode writes md to path and p's referenced attachments beside it, overwriting collisions when overwrite is true.
func (a *app) writeExportMode(p *Page, md, path string, overwrite bool) error {
	if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
		return err
	}
	for _, name := range exportAttachmentNames(p, md) {
		data, err := p.FS().ReadFile(filepath.Join(p.Dir, name))
		if err != nil {
			return err
		}
		to := filepath.Join(filepath.Dir(path), name)
		if !overwrite {
			if _, err := os.Stat(to); err == nil {
				continue
			}
		}
		if err := os.WriteFile(to, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// safeFile is a title as a file name.
func safeFile(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(s))
	if s == "" {
		return "common_pages"
	}
	return s
}

// barButton is a small icon button for the status bar, as tall as its text.
func (a *app) barButton(c *ui.Context, glyph *ui.SVG, label string) bool {
	label = msg(label) // its tip, in the window's language
	t := c.Theme()
	b := ui.Box(c).Width(22).Height(18).Radius(rad(5)).AlignItems(ui.Center).Justify(ui.Center).Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	ryugo.Tooltip(c, b, label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, glyph).Width(15).Height(15).TextColor(t.TextMuted) })
	return b.Clicked()
}
