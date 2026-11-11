package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
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
	if a.page == nil || a.noPage() {
		return
	}
	ui.Row(c.Key("page-buttons")).Gap(2).Children(func() {
		if !phone && a.barButton(c, iconPanel, "Yan panel (⇧⌘\\)") { // a phone has no side panel
			a.togglePanel()
		}
		if _, ok := historyOf(a.page); ok && a.barButton(c, iconLapse, map[bool]string{false: locales.CommonTimeLapse(), true: locales.CommonBackText()}[a.lapse.on]) {
			a.toggleLapse()
		}
		if a.barButton(c, iconMore, "Sayfa menüsü") {
			ww, wh := c.Size()
			a.showContext(ww-24, wh-34, a.pageMenu(a.page, false)) // opens upward, from the bar
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

// currentMarkdown is page p's Markdown as it is in the editor now, its
// front matter left out.
func (a *app) currentMarkdown(p *Page) string {
	if p == a.page && a.doc != nil {
		a.flush()
	}
	body, _ := p.Text()
	return body
}

// duplicatePage makes a copy of p, its sub-pages and pictures with it, right
// after it, and opens it.
func (a *app) duplicatePage(p *Page) {
	if p == a.page {
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
	cp := &Page{Title: p.Title + " (kopya)", Created: a.clock().Truncate(1e9), Dir: dir, Coll: p.Coll, Parent: p.Parent, Props: map[string]string{}}
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
			Title:       "Dışa aktar",
			DefaultPath: safeFile(title) + ".md",
			Filters:     []mygo.FileFilter{{Name: "Markdown", Extensions: []string{"md"}}},
		})
		if err != nil || path == "" {
			return
		}
		a.do(func() error {
			if err := a.writeExport(p, md, path); err != nil {
				a.say(toastErr, locales.CommonDidntWork(err.Error()))
			} else {
				a.say(toastOK, locales.CommonSavedTo(path))
			}
			return nil
		})
	}()
}

// writeExport writes md to path and the pictures of p beside it.
func (a *app) writeExport(p *Page, md, path string) error {
	if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
		return err
	}
	entries, _ := p.FS().ReadDir(p.Dir)
	for _, e := range entries {
		if e.IsDir() || e.Name() == PageFileName || !strings.Contains(md, e.Name()) {
			continue
		}
		data, err := p.FS().ReadFile(filepath.Join(p.Dir, e.Name()))
		if err != nil {
			return err
		}
		to := filepath.Join(filepath.Dir(path), e.Name())
		if _, err := os.Stat(to); err == nil { // a file of that name is there already: it is not written over (#217)
			continue
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
		return "sayfa"
	}
	return s
}

// barButton is a small icon button for the status bar, as tall as its text.
func (a *app) barButton(c *ui.Context, glyph *ui.SVG, label string) bool {
	label = T(label) // its tip, in the window's language
	t := c.Theme()
	b := ui.Box(c).Width(22).Height(18).Radius(rad(5)).AlignItems(ui.Center).Justify(ui.Center).Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	ryugo.Tooltip(c, b, label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() { ui.Icon(c, glyph).Width(15).Height(15).TextColor(t.TextMuted) })
	return b.Clicked()
}
