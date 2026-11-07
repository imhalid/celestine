package main

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The page's properties section, under its title, as Thymer's: a row per
// property of its collection - its name, its value to change - and
// "+ Özellik ekle", which adds one to every page of the collection. A
// property's name opens it to rename it, change its options or delete it.

// propEdit is the property dialog: a new property, or one to change.
type propEdit struct {
	open    bool
	coll    *Collection
	id      string // "" for a new one
	name    string
	typ     int // index in PropTypes
	options string
	err     string
}

// A property's row is a grid: the names in one column, every value in the next, of one width and one
// height whatever its kind, and the button to clear it in a slot of its own at the end, so that
// a page with several properties lines up instead of ragged widths and heights.
const (
	propNameW = 132
	propValW  = 260
	propSlotW = 28
	propRowH  = 34
	// Ryugo's input is as wide as it is asked, and its padding more: measured 22
	propInputPad = 22
)

// propSlot is the room at the end of a row: the clear button of a value that has one, else the same room.
func (a *app) propSlot(c *ui.Context, key string, on bool, label string, clear func()) {
	ui.Box(c.Key("prop-slot:" + key)).Width(propSlotW).Height(propSlotW).Center().Children(func() {
		if on && a.iconButton(c, iconClose, label) {
			clear()
		}
	})
}

func (a *app) propsSection(c *ui.Context, p *Page) {
	coll := p.Coll
	ui.Column(c).Gap(4).Padding(0, 0, 12, 0).Children(func() {
		for i := range coll.Properties {
			a.propRow(c, p, &coll.Properties[i])
		}
		ui.Row(c).Gap(6).Children(func() {
			if ryugo.Button(c.Key("add-prop"), "Özellik ekle", ryugo.ButtonOptions{Icon: iconPlus, Size: ryugo.Sm}).Clicked() {
				a.prop = propEdit{open: true, coll: coll}
			}
			if a.coverKind(p) == coverKindNone { // a cover is one click away, beside it
				if ryugo.Button(c.Key("add-cover"), "Kapak resmi ekle", ryugo.ButtonOptions{Icon: iconCover, Size: ryugo.Sm}).Clicked() {
					a.pickCover(p)
				}
			}
		})
	})
}

// propRow is a property of page p: its name, and its value to change.
func (a *app) propRow(c *ui.Context, p *Page, pr *Property) {
	t := c.Theme()
	key := p.Dir + "|" + pr.ID
	ui.Row(c.Key("prop-row:" + key)).Gap(12).AlignItems(ui.Center).Height(propRowH).Children(func() {
		name := ui.Row(c.Key("prop-name:"+pr.ID)).Width(propNameW).Shrink(0).Gap(6).Padding(4, 6).Radius(rad(6)).AlignItems(ui.Center).
			Label("Özellik: " + pr.Name).Role(ui.RoleButton)
		if name.Hovered() {
			name.Background(t.SurfaceHover)
		}
		name.Children(func() {
			ui.Text(c, pr.Name).FontSize(13).TextColor(t.TextMuted).MaxLines(1)
		})
		if name.Clicked() {
			a.prop = propEdit{open: true, coll: p.Coll, id: pr.ID, name: pr.Name, options: strings.Join(pr.Options, ", ")}
			a.prop.typ = slices.IndexFunc(PropTypes, func(x struct{ Type, Name string }) bool { return x.Type == pr.Type })
		}
		if pr.Type == PropChoice {
			v := p.Value(pr)
			opts := []ryugo.Option[string]{{Value: "", Label: "—"}}
			for _, o := range pr.Options {
				opts = append(opts, ryugo.Option[string]{Value: o, Label: o})
			}
			ryugo.Select(c.Key("prop-val:"+key), &v, opts, ryugo.SelectOptions{Label: pr.Name, Size: ryugo.Sm}).Width(propValW)
			if v != p.Value(pr) {
				a.setValue(p, pr.ID, v)
			}
			a.propSlot(c, key, false, "", nil)
			return
		}
		switch pr.Type {
		case PropDate: // a calendar to pick from, and a cross to take it away
			var d time.Time
			if v := p.Props[pr.ID]; v != "" {
				d, _ = time.Parse("2006-01-02", v)
			}
			was := d
			ryugo.DatePicker(c.Key("prop-val:"+key), &d, ryugo.DatePickerOptions{Label: pr.Name, Placeholder: T("Tarih seç"), Size: ryugo.Sm,
				Format: func(t time.Time) string { return shortDate(t) }}).Width(propValW)
			if !d.Equal(was) && !d.IsZero() {
				a.setValue(p, pr.ID, d.Format("2006-01-02"))
			}
			a.propSlot(c, key, !was.IsZero(), "Temizle: "+pr.Name, func() { a.setValue(p, pr.ID, "") })
			return
		case PropNumber: // a number input with its buttons; none until one is added
			raw := p.Props[pr.ID]
			if raw == "" {
				if ryugo.Button(c.Key("prop-num-add:"+key), T("Sayı ekle"), ryugo.ButtonOptions{Icon: iconPlus, Size: ryugo.Sm, Variant: ryugo.Soft}).Width(propValW).Clicked() {
					a.setValue(p, pr.ID, "0")
				}
				a.propSlot(c, key, false, "", nil)
				return
			}
			f, _ := strconv.ParseFloat(strings.TrimSpace(raw), 64)
			was := f
			ryugo.NumberInput(c.Key("prop-val:"+key), &f, ryugo.NumberInputOptions{Label: pr.Name, Size: ryugo.Sm, Step: 1}).Width(propValW)
			if f != was {
				a.setValue(p, pr.ID, strconv.FormatFloat(f, 'f', -1, 64))
			}
			a.propSlot(c, key, true, "Temizle: "+pr.Name, func() { a.setValue(p, pr.ID, "") })
			return
		}
		buf := a.propBuf[key]
		if buf == nil {
			buf = new(string)
			a.propBuf[key] = buf
		}
		if a.propEditing != key { // what the page has, unless it is being typed
			*buf = p.Props[pr.ID]
		}
		place := map[string]string{PropText: "Boş", PropNumber: "Sayı", PropDate: "2026-11-04", PropURL: "https://"}[pr.Type]
		in := ryugo.Input(c.Key("prop-val:"+key), buf, ryugo.InputOptions{Label: pr.Name + " değeri", Placeholder: place, Size: ryugo.Sm, Invalid: a.propErr == key}).Width(propValW - propInputPad)
		switch {
		case in.Submitted():
			a.commitValue(p, pr.ID, key, *buf)
		case in.Focused():
			a.propEditing = key
		case a.propEditing == key: // the focus went elsewhere
			a.propEditing = ""
			a.commitValue(p, pr.ID, key, *buf)
		}
		a.propSlot(c, key, false, "", nil)
	})
}

// commitValue writes a typed value; one that doesn't fit stays, marked.
func (a *app) commitValue(p *Page, id, key, v string) {
	a.propErr = ""
	if p == a.page {
		a.flush()
	}
	if err := p.SetValue(id, v); err != nil {
		a.propErr, a.notice = key, err.Error()
	}
}

func (a *app) setValue(p *Page, id, v string) {
	if p == a.page {
		a.flush()
	}
	if err := p.SetValue(id, v); err != nil {
		a.saveErr = err.Error()
	}
}

// propDialog adds a property, or changes or deletes one.
func (a *app) propDialog(c *ui.Context) {
	e := &a.prop
	title := "Özellik ekle"
	if e.id != "" {
		title = "Özelliği düzenle"
	}
	ryugo.Dialog(c.Key("prop-dialog"), &e.open, ryugo.DialogOptions{Title: title, Width: 440, Footer: func() {
		if e.id != "" && ryugo.Button(c.Key("prop-delete"), T("Özelliği sil"), ryugo.ButtonOptions{Color: ryugo.From(c).Colors.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
			if err := e.coll.RemoveProperty(e.id); err != nil {
				e.err = err.Error()
				return
			}
			e.open = false
		}
		ui.Spacer(c)
		label := "Ekle"
		if e.id != "" {
			label = "Kaydet"
		}
		if ryugo.Button(c.Key("prop-save"), label, ryugo.ButtonOptions{Color: ryugo.From(c).Colors.Accent, Variant: ryugo.Solid, Size: ryugo.Sm}).Clicked() {
			a.saveProp()
		}
	}}, func() {
		cl := ryugo.From(c).Colors
		ui.Column(c).Gap(14).Children(func() {
			ryugo.Field(c, ryugo.FieldOptions{Label: "Ad"}, func() {
				in := ryugo.Input(c.Key("prop-name-in"), &e.name, ryugo.InputOptions{Label: T("Özelliğin adı"), Placeholder: T("Durum, Yazar, Bitiş…")})
				in.AutoFocus()
				if in.Submitted() {
					a.saveProp()
				}
			})
			if e.id == "" {
				ryugo.Field(c, ryugo.FieldOptions{Label: T("Tür")}, func() {
					var names []string
					for _, t := range PropTypes {
						names = append(names, t.Name)
					}
					ryugo.Segmented(c.Key("prop-type"), &e.typ, names, ryugo.SegmentedOptions{Label: T("Tür"), Fill: true})
				})
			}
			if e.typ >= 0 && PropTypes[e.typ].Type == PropChoice {
				ryugo.Field(c, ryugo.FieldOptions{Label: T("Seçenekler"), Help: "Sırası panonun sütunları olur."}, func() {
					tags := cleanOptions(strings.Split(e.options, ","))
					before := strings.Join(tags, ",")
					ryugo.TagInput(c.Key("prop-options"), &tags, ryugo.TagInputOptions{Placeholder: T("Seçenek yaz, ↵"), Label: T("Seçenek ekle")})
					if now := strings.Join(tags, ","); now != before {
						e.options = now
					}
				})
			}
			if e.err != "" {
				ui.Text(c, e.err).FontSize(ryugo.FsSm).TextColor(cl.Up)
			}
		})
	})
}

func (a *app) saveProp() {
	e := &a.prop
	opts := strings.Split(e.options, ",")
	var err error
	if e.id == "" {
		_, err = e.coll.AddProperty(e.name, PropTypes[max(e.typ, 0)].Type, opts)
	} else {
		err = e.coll.EditProperty(e.id, e.name, opts)
	}
	if err != nil {
		e.err = err.Error()
		return
	}
	e.open, e.err = false, ""
}
