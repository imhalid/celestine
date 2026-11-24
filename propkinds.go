package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The values of the kinds of property that are more than a line of text: tags, a chip each, with a field that adds
// more; a page, its title, chosen from the library's pages by what is typed; a file, its name, a file copied into the
// page's folder, opened with the system's program for it.

var iconAttach = icon(`<path d="m20 11.5-7.8 7.8a5 5 0 0 1-7.1-7.1L13 4.4a3.4 3.4 0 0 1 4.8 4.8l-7.9 7.9a1.7 1.7 0 0 1-2.4-2.4l7.3-7.3"/>`)

// propTags is a tags or aliases value: its words as chips, each with its cross, and a field that adds what is typed.
func (a *app) propTags(c *ui.Context, p *Page, pr *Property, key string) {
	tags := SplitTags(p.Props[pr.ID])
	before := strings.Join(tags, ", ")
	add := locales.PropsAddTag()
	if pr.Type == PropAliases {
		add = locales.PropsAddName()
	}
	ui.Box(c.Key("prop-val:" + key)).Grow(1).MinWidth(0).Children(func() {
		a.tagsField(c, "prop-tags:"+key, &tags, add, locales.PropsValueLabel(pr.Name))
	})
	if now := strings.Join(tags, ", "); now != before {
		a.setValue(p, pr.ID, now)
	}
}

// propPageLink is a page value: the title of the page it names, which opens a box to open that page, take the link
// away, or choose another by typing part of its title.
func (a *app) propPageLink(c *ui.Context, p *Page, pr *Property, key string) {
	t := c.Theme()
	target := a.lib.PageAt(p.Props[pr.ID])
	kc := c.Key("prop-val:" + key)
	tr := ui.ButtonBase(kc).Role(ui.RolePopUpButton).Label(locales.PropsValueLabel(pr.Name)).Cursor(ui.CursorPointer).FocusRing(false)
	a.contextOn(tr, func() []menuItem { return a.propRowMenu(p, pr) })
	open := ui.Local(tr, "open", func() bool { return false })
	query := ui.Local(tr, "query", func() string { return "" })
	if tr.Clicked() {
		*open, *query = !*open, ""
	}
	tr.Expanded(*open)
	propCell(c, tr, tr.Hovered() || tr.FocusVisible() || *open)
	tr.Children(func() {
		switch {
		case target != nil:
			ui.Row(c).Gap(6).AlignItems(ui.Center).MinWidth(0).Children(func() {
				ui.Icon(c, iconPage).Width(13).Height(13).TextColor(t.TextMuted)
				ui.Text(c, target.Title).FontSize(propFs).TextColor(t.Text).MaxLines(1)
			})
		case p.Props[pr.ID] != "": // it names a page that is gone
			ui.Text(c, p.Props[pr.ID]).FontSize(propFs).TextColor(t.TextMuted).MaxLines(1).Strikethrough()
		default:
			propEmpty(c)
		}
	})
	ryugo.Popover(kc, tr, open, func() {
		ui.Column(c).Gap(4).Width(280).Children(func() {
			if target != nil {
				if a.quietButton(c.Key("prop-page-open"), iconPage, locales.PropsOpenPage(target.Title)) {
					*open = false
					a.open(target)
				}
			}
			if p.Props[pr.ID] != "" && a.quietButton(c.Key("prop-page-clear"), iconClose, locales.PropsRemoveLink()) {
				a.setValue(p, pr.ID, "")
				*open = false
			}
			in := a.fieldMenu(ryugo.Input(c.Key("prop-page-q"), query, ryugo.InputOptions{Label: locales.PropsFindPage(), Placeholder: locales.PropsFindPage2(), PrefixIcon: iconSearch, Size: ryugo.Sm}), query, fieldOpts{oneLine: true})
			in.AutoFocus()
			hits := pagesTitled(a.lib, *query, p, 8)
			for _, h := range hits {
				if a.quietButton(c.Key("prop-page-hit:"+h.Dir), iconPage, h.Title+"  ·  "+h.Coll.Name) || in.Submitted() && h == hits[0] {
					a.setValue(p, pr.ID, h.Path())
					*open = false
					break
				}
			}
		})
	})
}

// pagesTitled are up to limit pages of lib whose titles have q in them (all, by title, when q is empty), not but.
func pagesTitled(lib *Library, q string, but *Page, limit int) []*Page {
	q = strings.ToLower(strings.TrimSpace(q))
	var out []*Page
	for _, c := range lib.Collections {
		for _, p := range c.All() {
			if p != but && strings.Contains(strings.ToLower(p.Title), q) {
				out = append(out, p)
				if len(out) == limit {
					return out
				}
			}
		}
	}
	return out
}

// propFile is a file value: its name, which opens a box to open the file, show it in its folder, choose another,
// or take it away (its file with it).
func (a *app) propFile(c *ui.Context, p *Page, pr *Property, key string) {
	t := c.Theme()
	name := p.Props[pr.ID]
	kc := c.Key("prop-val:" + key)
	tr := ui.ButtonBase(kc).Role(ui.RolePopUpButton).Label(locales.PropsValueLabel(pr.Name)).Cursor(ui.CursorPointer).FocusRing(false)
	a.contextOn(tr, func() []menuItem { return a.propRowMenu(p, pr) })
	open := ui.Local(tr, "open", func() bool { return false })
	if tr.Clicked() {
		if name == "" { // nothing to show: straight to choosing one
			a.pickPropFile(p, pr)
		} else {
			*open = !*open
		}
	}
	tr.Expanded(*open)
	propCell(c, tr, tr.Hovered() || tr.FocusVisible() || *open)
	tr.Children(func() {
		if name == "" {
			propEmpty(c)
			return
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).MinWidth(0).Children(func() {
			ui.Icon(c, iconAttach).Width(13).Height(13).TextColor(t.TextMuted)
			ui.Text(c, name).FontSize(propFs).TextColor(t.Text).MaxLines(1)
		})
	})
	ryugo.Popover(kc, tr, open, func() {
		ui.Column(c).Gap(4).Width(240).Children(func() {
			if safeAttachName(name) {
				if a.quietButton(c.Key("prop-file-open"), iconAttach, locales.PropsOpenFile()) {
					*open = false
					a.openAttachedFile(p, name, false)
				}
				if a.quietButton(c.Key("prop-file-show"), iconLibrary, locales.PropsShowFolder()) {
					*open = false
					a.openAttachedFile(p, name, true)
				}
			}
			if a.quietButton(c.Key("prop-file-pick"), iconPlus, locales.PropsChooseAnother()) {
				*open = false
				a.pickPropFile(p, pr)
			}
			if a.quietButton(c.Key("prop-file-clear"), iconClose, locales.PropsRemoveFile()) {
				*open = false
				a.setPropFile(p, pr, "")
			}
		})
	})
}

// pickPropFile asks for a file in a dialog and makes it page p's value of pr.
func (a *app) pickPropFile(p *Page, pr *Property) {
	go func() {
		defer a.guard()
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: locales.CommonChooseFile()})
		if err != nil || len(paths) == 0 {
			return
		}
		data, err := os.ReadFile(paths[0]) // read away from the window: a file may be big (#201)
		a.do(func() error {
			if err != nil {
				a.say(toastErr, err.Error())
				return nil
			}
			a.setPropData(p, pr, filepath.Base(paths[0]), data)
			return nil
		})
	}()
}

// setPropFile copies the file at src into page p's folder (its name kept, numbered when it is taken) and makes it
// p's value of pr, the file it had going; an empty src takes the value and its file away.
func (a *app) setPropFile(p *Page, pr *Property, src string) {
	if src == "" {
		a.setPropData(p, pr, "", nil)
		return
	}
	data, err := os.ReadFile(src)
	if err != nil {
		a.say(toastErr, err.Error())
		return
	}
	a.setPropData(p, pr, filepath.Base(src), data)
}

// setPropData is setPropFile with the file read: data, named base ("" takes the value away).
func (a *app) setPropData(p *Page, pr *Property, base string, data []byte) {
	old := p.Props[pr.ID]
	name := ""
	if base != "" {
		if !validAttachFileName(base) {
			a.fail(errors.New(locales.CommonInvalidAttachmentName(base)))
			return
		}
		name = freeName(p, base)
		if err := p.FS().WriteFile(filepath.Join(p.Dir, name), data); err != nil {
			a.fail(err)
			return
		}
	}
	if err := p.SetValue(pr.ID, name); err != nil {
		a.fail(err)
		return
	}
	if old != "" && old != name && safeAttachName(old) {
		p.FS().Remove(filepath.Join(p.Dir, old))
	}
}

// freeName is name, or name with a number before its extension, that no file of p's folder has.
func freeName(p *Page, name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		try := name
		if i > 1 {
			try = fmt.Sprintf("%s (%d)%s", base, i, ext)
		}
		if _, err := p.FS().ReadFile(filepath.Join(p.Dir, try)); err != nil {
			return try
		}
	}
}

// propLabel is a value of pr as one line of text: a page's title, tags with commas, the rest as they are.
func (a *app) propLabel(p *Page, pr *Property) string {
	v := p.Value(pr)
	if pr.Type == PropPage && v != "" {
		if target := a.lib.PageAt(v); target != nil {
			return target.Title
		}
	}
	return v
}

// propTypeOptions are the kinds of property as a menu's options, by their index in PropTypes.
func propTypeOptions() []ryugo.Option[int] {
	var out []ryugo.Option[int]
	for i, t := range PropTypes {
		out = append(out, ryugo.Option[int]{Value: i, Label: PropTypeName(t.Type)})
	}
	return out
}
