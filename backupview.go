package main

import (
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// A backup to look at (#409): its file is put back from the gzip into a folder of its own, opened as a library, and
// shown in a window of its own, read only: its collections and pages on the left, the page chosen as plain text on
// the right. Nothing in it is written; the window's closing closes the library and removes the folder.

// backupView is a backup opened in its window.
type backupView struct {
	at    time.Time
	dir   string // the folder its file was put back in
	store *Store
	lib   *Library
	page  *Page  // the page shown
	text  string // its text
	theme int    // the app's light or dark, when it opened
	glass bool

	// What is shown of it (#496): the pages whose title or text has the words, of the collection chosen ("" all),
	// and those picked to be restored, alone when only asks
	app    *app
	query  string
	coll   string
	only   bool
	picked map[*Page]bool
	texts  map[*Page]string // each page's text, read once, for the search
	note   string           // what the last restore did
}

// previewBackup opens the backup b in a window of its own, read away from the window that asked.
func (a *app) previewBackup(b Backup) {
	theme, glassOn := a.settings.Theme, a.settings.Glass
	go func() {
		defer a.guard()
		v, err := openBackupView(b)
		if err != nil {
			a.do(func() error { a.say(toastErr, locales.CommonDidntWork(err.Error())); return nil })
			return
		}
		v.theme, v.glass, v.app = theme, glassOn, a
		opts := mygo.WindowOptions{Title: locales.BackupPreviewTitle(dateTime(b.At, false)), Width: 900, Height: 640,
			MinWidth: 520, MinHeight: 360, Content: ui.View(v.view)}
		if glassOn && runtime.GOOS == "darwin" {
			opts.Transparent = true
			opts.Vibrancy = mygo.VibrancyGlass
		}
		w := mygo.NewWindow(opts)
		w.OnClosed(v.close)
	}()
}

// openBackupView puts b's file back in a folder of its own and opens it, with its first page chosen.
func openBackupView(b Backup) (*backupView, error) {
	dir, err := os.MkdirTemp("", "celestine-backup-")
	if err != nil {
		return nil, err
	}
	file := filepath.Join(dir, "library.db")
	v := &backupView{at: b.At, dir: dir}
	if err := gunzipFile(b.Path, file); err != nil {
		v.close()
		return nil, err
	}
	if v.store, err = OpenStore(file); err != nil {
		v.close()
		return nil, err
	}
	if v.lib, err = OpenLibraryIn(v.store, StoreRoot); err != nil {
		v.close()
		return nil, err
	}
	for _, c := range v.lib.Collections {
		if all := c.All(); len(all) > 0 {
			v.show(all[0])
			break
		}
	}
	return v, nil
}

// show makes p the page shown.
func (v *backupView) show(p *Page) {
	v.page = p
	v.text, _ = p.Text()
}

// close closes the library and removes its folder.
func (v *backupView) close() {
	if v.store != nil {
		v.store.Close()
	}
	os.RemoveAll(v.dir)
}

// view is the window: the backup's pages, and the one chosen.
func (v *backupView) view(c *ui.Context) {
	ryugo.Root(c, ryugo.Options{Mode: []ryugo.Mode{ryugo.ModeInherit, ryugo.Light, ryugo.Dark}[v.theme%3]})
	cl := ryugo.From(c).Colors
	sideBg := cl.Well
	if v.glass && c.Vibrancy() {
		c.Root().Background(ui.Transparent)
		sideBg = cl.Well.Alpha(0.45)
	} else {
		c.Root().Background(cl.Card)
	}
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Padding(14, 18).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.BackupPreviewTitle(dateTime(v.at, true))).FontSize(14).FontWeight(650).TextColor(cl.Text)
			ui.Text(c, locales.BackupReadOnly()).FontSize(12.5).TextColor(cl.Muted).Grow(1).MaxLines(1)
			if n := len(v.picked); n > 0 && v.app != nil {
				if ryugo.Button(c.Key("bv-restore"), locales.BackupRestorePages(n), ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Accent, Variant: ryugo.Solid}).Clicked() {
					v.restorePicked()
				}
			}
		})
		// the bar of what is shown: square at its top, under the title (#496)
		ui.Row(c).FillWidth().Padding(8, 18).Gap(10).AlignItems(ui.Center).Background(sideBg).Children(func() {
			ryugo.Input(c.Key("bv-q"), &v.query, ryugo.InputOptions{Label: locales.BackupSearch(), Placeholder: locales.BackupSearch(), PrefixIcon: iconSearch, Clearable: true, Size: ryugo.Sm}).Width(220)
			opts := []ryugo.Option[string]{{Value: "", Label: locales.BackupAllCollections()}}
			for _, co := range v.lib.Collections {
				opts = append(opts, ryugo.Option[string]{Value: co.Dir, Label: co.Name})
			}
			ryugo.Select(c.Key("bv-coll"), &v.coll, opts, ryugo.SelectOptions{Label: locales.Collection(), Size: ryugo.Sm}).Width(170)
			ui.Spacer(c)
			ui.Text(c, locales.BackupSelectedOnly()).FontSize(12.5).TextColor(cl.Text2)
			ryugo.Switch(c.Key("bv-only"), &v.only, ryugo.SwitchOptions{}).Label(locales.BackupSelectedOnly())
		})
		if v.note != "" {
			ui.Text(c, v.note).FontSize(12.5).TextColor(cl.Muted).Padding(6, 18)
		}
		ui.Box(c).FillWidth().Height(1).Background(cl.Hairline)
		ui.Row(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			ui.Scroll(c.Key("bv-pages")).Width(270).FillHeight().Radius(0).Background(sideBg).Children(func() {
				ui.Column(c).FillWidth().Padding(10, 8).Gap(2).Children(func() { v.pages(c, cl) })
			})
			ui.Scroll(c.Key("bv-page")).Grow(1).FillHeight().Children(func() {
				ui.Column(c).FillWidth().Padding(24, 32).Gap(14).Children(func() {
					if v.page == nil {
						ui.Text(c, locales.BackupNoPages()).FontSize(13).TextColor(cl.Muted)
						return
					}
					ui.Text(c, v.page.Title).FontSize(22).FontWeight(700).TextColor(cl.Text).Wrap()
					ui.Text(c, v.text).FontSize(14).LineHeight(1.6).TextColor(cl.Text2).Wrap().Selectable()
				})
			})
		})
	})
}

// shows reports whether page p is shown: of the collection chosen, picked when only those are, with the words
// asked for in its title or text.
func (v *backupView) shows(p *Page) bool {
	if v.coll != "" && p.Coll.Dir != v.coll || v.only && !v.picked[p] {
		return false
	}
	q := foldText(strings.TrimSpace(v.query))
	if q == "" || strings.Contains(foldText(p.Title), q) {
		return true
	}
	if v.texts == nil {
		v.texts = map[*Page]string{}
	}
	t, ok := v.texts[p]
	if !ok {
		t, _ = p.Text()
		t = foldText(t)
		v.texts[p] = t
	}
	return strings.Contains(t, q)
}

// restorePicked copies the pages picked into the library, each into the collection of its name there, made when there
// is none: their text, values and properties of their own, not their files. The backup is not changed.
func (v *backupView) restorePicked() {
	type copied struct {
		coll, title, body string
		props             map[string]string
		own               []Property
	}
	var out []copied
	for _, coll := range v.lib.Collections {
		for _, p := range coll.All() {
			if !v.picked[p] {
				continue
			}
			body, err := p.Body()
			if err != nil {
				continue
			}
			out = append(out, copied{coll.Name, p.Title, body, maps.Clone(p.Props), slices.Clone(p.Own)})
		}
	}
	a := v.app
	n := 0
	err := a.do(func() error {
		for _, cp := range out {
			var coll *Collection
			for _, c := range a.lib.Collections {
				if c.Name == cp.coll && !c.Journal && !c.Dynamic() {
					coll = c
				}
			}
			if coll == nil {
				c, err := a.lib.NewCollection(cp.coll)
				if err != nil {
					return err
				}
				coll = c
			}
			p, err := coll.NewPage(nil, cp.title)
			if err != nil {
				return err
			}
			p.Props, p.Own = cp.props, cp.own
			if err := p.Save(cp.body); err != nil {
				return err
			}
			n++
		}
		PagesChanged()
		a.say(toastOK, locales.BackupRestoredPages(n))
		return nil
	})
	if err != nil {
		v.note = locales.CommonDidntWork(err.Error())
		return
	}
	v.note, v.picked, v.only = locales.BackupRestoredPages(n), nil, false
}

// pages lists the backup's collections, each with its pages shown, a click showing one, a box picking it.
func (v *backupView) pages(c *ui.Context, cl ryugo.Colors) {
	for _, coll := range v.lib.Collections {
		var shown []*Page
		for _, p := range coll.All() {
			if v.shows(p) {
				shown = append(shown, p)
			}
		}
		if len(shown) == 0 {
			continue
		}
		ui.Text(c, coll.Name).FontSize(11.5).FontWeight(650).TextColor(cl.Muted).Padding(10, 8, 4, 8).MaxLines(1)
		for _, p := range shown {
			on := p == v.page
			title := p.Title
			if title == "" {
				title = locales.CommonUntitled()
			}
			depth := float32(0)
			for q := p.Parent; q != nil; q = q.Parent {
				depth++
			}
			e := ui.Row(c.Key("bv:"+p.Dir)).FillWidth().Gap(8).AlignItems(ui.Center).Padding(6, 8, 6, 8+12*depth).Radius(rad(7)).Role(ui.RoleButton).
				Label(title).Cursor(ui.CursorPointer)
			if on {
				e.Background(ryugo.Mix(cl.Card, cl.Accent, 0.18))
			} else if e.Hovered() {
				e.Background(cl.Fill)
			}
			e.Children(func() {
				pick := v.picked[p]
				ryugo.Checkbox(c.Key("pick"), &pick, "", ryugo.CheckOptions{}).Label(locales.BackupPickPage(title))
				if pick != v.picked[p] {
					if v.picked == nil {
						v.picked = map[*Page]bool{}
					}
					if pick {
						v.picked[p] = true
					} else {
						delete(v.picked, p)
					}
				}
				ui.Text(c, title).FontSize(13).TextColor(map[bool]ui.Color{true: cl.Text, false: cl.Text2}[on]).MaxLines(1)
			})
			if e.Clicked() {
				v.show(p)
			}
		}
	}
}
