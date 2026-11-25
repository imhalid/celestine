package main

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The page's properties section, under its title, as Thymer's: a row per
// property of its collection, then of its own - its name, its value to change - and
// "+ Add property", which adds one to this page alone (#487); the collection's are added in its settings. A
// property's name opens it to rename it, change its options or, one of the page's own, delete it.

// propEdit is the property dialog: a new property, or one to change.
type propEdit struct {
	open    bool
	coll    *Collection
	page    *Page  // set for a property of this page alone
	id      string // "" for a new one
	name    string
	typ     int // index in PropTypes
	options string
	err     string
}

// A property's row is quiet, so as not to pull the eye from the page: its name small and muted in one
// column, its value as plain text in the next, with no field drawn around it until the pointer is on
// it or it is being changed. A choice opens a menu, a date a small calendar; every value is one
// height and starts at one place, so that several properties line up.
const (
	propNameW = 116
	propValW  = 300
	propRowH  = 28
	propFs    = 13.5
)

var iconChevronBack = icon(`<path d="M14.5 6.5 9 12l5.5 5.5"/>`)

func (a *app) propsSection(c *ui.Context, pn *paneState, p *Page) {
	coll := p.Coll
	ui.Column(c).Gap(1).Padding(0, 0, 14, 0).Children(func() {
		for _, pr := range p.Properties() {
			a.propRow(c, p, pr)
		}
		ui.Row(c).Gap(2).Padding(2, 0, 0, 0).Children(func() {
			if a.quietButton(c.Key("add-prop"), iconPlus, locales.CommonAddProperty()) {
				a.prop = propEdit{open: true, coll: coll, page: p}
			}
			if a.coverKind(p) == coverKindNone { // a cover is one click away, beside it
				if a.quietButton(c.Key("add-cover"), iconCover, locales.PropsAddCoverImage()) {
					a.pickCover(p)
				}
			}
		})
	})
}

// quietButton is a muted button of an icon and a few words, with no fill until the pointer is on it.
func (a *app) quietButton(c *ui.Context, glyph *ui.SVG, label string) bool {
	t := c.Theme()
	b := ui.Row(c).Height(26).Gap(5).Padding(0, 8, 0, 6).Radius(rad(6)).AlignItems(ui.Center).
		Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer).
		Transition(ui.ElementTransition{Duration: 120 * time.Millisecond, Colors: true})
	color := t.TextMuted.Alpha(0.7)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
		color = t.TextMuted
	} else {
		b.Background(t.SurfaceHover.Alpha(0))
	}
	b.Children(func() {
		ui.Icon(c, glyph).Width(14).Height(14).TextColor(color)
		ui.Text(c, label).FontSize(13).TextColor(color).NoWrap()
	})
	return b.Clicked()
}

// propCell is the place of a value: no fill, but when hot, the pointer on it or it being changed.
func propCell(c *ui.Context, e ui.Element, hot bool) ui.Element {
	t := c.Theme()
	e.Width(propValW).Height(propRowH).Padding(0, 8).Radius(rad(6)).AlignItems(ui.Center).Justify(ui.Start).Gap(6).
		Transition(ui.ElementTransition{Duration: 120 * time.Millisecond, Colors: true})
	if hot {
		e.Background(t.SurfaceHover)
	} else {
		e.Background(t.SurfaceHover.Alpha(0))
	}
	return e
}

// propEmpty is what a value without one shows: a faint "Boş".
func propEmpty(c *ui.Context) {
	ui.Text(c, locales.CommonEmpty()).FontSize(propFs).TextColor(c.Theme().TextMuted.Alpha(0.5)).NoWrap()
}

// propRow is a property of page p: its name, and its value to change.
func (a *app) propRow(c *ui.Context, p *Page, pr *Property) {
	t := c.Theme()
	key := p.Dir + "|" + pr.ID
	ui.Row(c.Key("prop-row:" + key)).Gap(8).AlignItems(ui.Center).Children(func() {
		name := ui.Row(c.Key("prop-name:"+pr.ID)).Width(propNameW).Shrink(0).Height(propRowH).Padding(0, 6).Radius(rad(6)).
			AlignItems(ui.Center).Label(locales.CollectionPropertyLabel(pr.Name)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
		if name.Hovered() {
			name.Background(t.SurfaceHover)
		}
		name.Children(func() {
			ui.Text(c, pr.Name).FontSize(13).TextColor(t.TextMuted).MaxLines(1)
		})
		if name.Clicked() {
			a.editProp(p, pr)
		}
		a.contextOn(name, func() []menuItem { return a.propRowMenu(p, pr) })
		switch pr.Type {
		case PropChoice:
			a.propChoice(c, p, pr, key)
		case PropDate:
			a.propDate(c, p, pr, key)
		case PropTags, PropAliases:
			a.propTags(c, p, pr, key)
		case PropPage:
			a.propPageLink(c, p, pr, key)
		case PropFile:
			a.propFile(c, p, pr, key)
		default:
			a.propText(c, p, pr, key)
		}
	})
}

// propChoice is a choice's value, a badge tinted with the option's color, which opens the menu of its options.
func (a *app) propChoice(c *ui.Context, p *Page, pr *Property, key string) {
	t := c.Theme()
	kc := c.Key("prop-val:" + key)
	tr := ui.ButtonBase(kc).Role(ui.RolePopUpButton).Label(locales.PropsValueLabel(pr.Name)).Cursor(ui.CursorPointer).FocusRing(false)
	a.contextOn(tr, func() []menuItem { return a.propRowMenu(p, pr) })
	open := ui.Local(tr, "open", func() bool { return false })
	if tr.Clicked() {
		*open = !*open
	}
	tr.Expanded(*open)
	hot := tr.Hovered() || tr.FocusVisible() || *open
	propCell(c, tr, hot)
	v := p.Value(pr)
	tone := columnTone(c, v) // the option's color, as its column's on the board
	tint := tone.Alpha(0.16)
	if hot { // on the cell's fill: a step stronger
		tint = tone.Alpha(0.26)
	}
	tr.Children(func() {
		if v == "" {
			propEmpty(c)
			return
		}
		ui.Row(c).Height(20).Padding(0, 9).Radius(ryugo.RadiusPill).AlignItems(ui.Center).Justify(ui.Center).
			Background(tint).Children(func() {
			ui.Text(c, v).FontSize(12.5).FontWeight(560).TextColor(t.Text).NoWrap()
		})
	})
	ryugo.Menu(kc, tr, open, func(m *ryugo.MenuList) {
		if m.Item(locales.CommonEmpty(), ryugo.ItemOptions{Checkable: true, Checked: v == ""}) && v != "" {
			a.setValue(p, pr.ID, "")
		}
		for _, o := range pr.Options {
			if m.Item(o, ryugo.ItemOptions{Checkable: true, Checked: v == o}) && v != o {
				a.setValue(p, pr.ID, o)
			}
		}
	})
}

// propDate is a date's value, which opens a small calendar to choose another, or to take it away.
func (a *app) propDate(c *ui.Context, p *Page, pr *Property, key string) {
	t := c.Theme()
	var d time.Time
	if v := p.Props[pr.ID]; v != "" {
		d, _ = time.Parse("2006-01-02", v)
	}
	kc := c.Key("prop-val:" + key)
	tr := ui.ButtonBase(kc).Role(ui.RolePopUpButton).Label(locales.PropsValueLabel(pr.Name)).Cursor(ui.CursorPointer).FocusRing(false)
	a.contextOn(tr, func() []menuItem { return a.propRowMenu(p, pr) })
	open := ui.Local(tr, "open", func() bool { return false })
	month := ui.Local(tr, "month", func() time.Time { return time.Time{} })
	if tr.Clicked() {
		*open = !*open
		if *month = d; d.IsZero() {
			*month = a.clock()
		}
	}
	tr.Expanded(*open)
	propCell(c, tr, tr.Hovered() || tr.FocusVisible() || *open)
	tr.Children(func() {
		if d.IsZero() {
			propEmpty(c)
			return
		}
		ui.Text(c, shortDate(d)).FontSize(propFs).TextColor(t.Text).NoWrap().FontFeatures("tnum")
	})
	ryugo.Popover(kc, tr, open, func() {
		a.miniCalendar(c, month, d, func(day time.Time) {
			a.setValue(p, pr.ID, day.Format("2006-01-02"))
			*open = false
		})
		if !d.IsZero() && a.quietButton(c.Key("prop-date-clear"), iconClose, locales.PropsRemoveDate()) {
			a.setValue(p, pr.ID, "")
			*open = false
		}
	})
}

// miniCalendar is a month's days to choose one from, with the buttons to the months around it.
func (a *app) miniCalendar(c *ui.Context, month *time.Time, chosen time.Time, pick func(time.Time)) {
	t := c.Theme()
	now := a.clock()
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.Local)
	ui.Column(c).Gap(2).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(2).Padding(0, 0, 4, 4).Children(func() {
			ui.Text(c, monthYear(first, false)).FontSize(13.5).FontWeight(600).TextColor(t.Text)
			ui.Spacer(c)
			if a.iconButton(c.Key("cal-prev"), iconChevronBack, locales.CommonPreviousMonth()) {
				*month = first.AddDate(0, -1, 0)
			}
			if a.iconButton(c.Key("cal-next"), iconChevron, locales.CommonNextMonth()) {
				*month = first.AddDate(0, 1, 0)
			}
		})
		const cell = 30
		ui.Row(c).Children(func() {
			for _, w := range weekdayHeads(false) {
				ui.Box(c).Width(cell).Height(22).Center().Children(func() {
					ui.Text(c, w).FontSize(11).TextColor(t.TextMuted.Alpha(0.7))
				})
			}
		})
		start := weekStart(first)
		for w := range 6 {
			ui.Row(c.Key(w)).Children(func() {
				for i := range 7 {
					day := start.AddDate(0, 0, w*7+i)
					b := ui.Box(c.Key(i)).Width(cell).Height(cell - 2).Center().Radius(rad(6)).
						Role(ui.RoleButton).Label(shortDate(day)).Cursor(ui.CursorPointer)
					color := t.Text
					switch {
					case sameDay(day, chosen):
						b.Background(t.Accent)
						color = ui.RGB(255, 255, 255)
					case b.Hovered():
						b.Background(t.SurfaceHover)
					}
					if day.Month() != first.Month() && !sameDay(day, chosen) {
						color = t.TextMuted.Alpha(0.45)
					} else if sameDay(day, now) && !sameDay(day, chosen) {
						color = t.Accent
					}
					b.Children(func() {
						ui.Text(c, strconv.Itoa(day.Day())).FontSize(12.5).TextColor(color).FontFeatures("tnum")
					})
					if b.Clicked() {
						pick(day)
					}
				}
			})
		}
	})
}

// propText is a value typed: text, a number or an address, written where it shows.
func (a *app) propText(c *ui.Context, p *Page, pr *Property, key string) {
	t := c.Theme()
	buf := a.propBuf[key]
	if buf == nil {
		buf = new(string)
		a.propBuf[key] = buf
	}
	if a.propEditing != key { // what the page has, unless it is being typed
		*buf = p.Props[pr.ID]
	}
	kc := c.Key("prop-val:" + key)
	cell := ui.Row(kc).Cursor(ui.CursorText)
	var in ui.Element
	cell.Children(func() {
		in = ui.TextInputBase(kc, buf).Grow(1).MinWidth(0).Placeholder(locales.CommonEmpty()).PlaceholderColor(t.TextMuted.Alpha(0.5)).
			TextColor(t.Text).FontSize(propFs).FocusRing(false).Label(locales.PropsValueLabel(pr.Name))
		a.fieldMenu(in, buf, fieldOpts{oneLine: true, changed: func() { a.commitValue(p, pr.ID, key, *buf) }, more: func() []menuItem { return a.propRowMenu(p, pr) }})
	})
	if pr.Type == PropNumber { // a number takes what a number is made of, and nothing else as it is typed (#487)
		*buf = numberOnly(*buf)
	}
	propCell(c, cell, cell.Hovered() || in.Focused())
	if a.propErr == key {
		cell.Border(1, ryugo.From(c).Colors.Up)
	}
	if cell.Clicked() {
		in.Focus()
	}
	switch {
	case in.Submitted():
		a.commitValue(p, pr.ID, key, *buf)
	case in.Focused():
		a.propEditing = key
	case a.propEditing == key: // the focus went elsewhere
		a.propEditing = ""
		a.commitValue(p, pr.ID, key, *buf)
	}
}

// numberOnly is s with only what a number is written with: digits, a sign before them and one decimal mark,
// a point or a comma.
func numberOnly(s string) string {
	var b strings.Builder
	mark := false
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case (r == '-' || r == '+') && i == 0:
		case (r == '.' || r == ',') && !mark:
			mark = true
		default:
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// commitValue writes a typed value; one that doesn't fit stays, marked.
func (a *app) commitValue(p *Page, id, key, v string) {
	a.propErr = ""
	if p == a.pane().page {
		a.flush()
	}
	if err := p.SetValue(id, v); err != nil {
		a.propErr = key
		a.say(toastErr, err.Error())
	}
}

func (a *app) setValue(p *Page, id, v string) {
	if p == a.pane().page {
		a.flush()
	}
	if err := p.SetValue(id, v); err != nil {
		a.fail(err)
	}
}

// propDialog adds a property, or changes or deletes one.
func (a *app) propDialog(c *ui.Context) {
	e := &a.prop
	title := locales.CommonAddProperty()
	if e.id != "" {
		title = locales.PropsEditProperty()
	}
	ryugo.Dialog(c.Key("prop-dialog"), &e.open, ryugo.DialogOptions{Title: title, Width: 440, Footer: func() {
		if e.id != "" && e.page != nil && ryugo.Button(c.Key("prop-delete"), locales.CommonDeleteProperty(), ryugo.ButtonOptions{Color: ryugo.From(c).Colors.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
			a.removePageProperty(e.page, e.id)
			e.open = false
		}
		ui.Spacer(c)
		label := locales.SlashGroupInsert()
		if e.id != "" {
			label = locales.CommonSave()
		}
		if ryugo.Button(c.Key("prop-save"), label, ryugo.ButtonOptions{Color: ryugo.From(c).Colors.Accent, Variant: ryugo.Solid, Size: ryugo.Sm}).Clicked() {
			a.saveProp()
		}
	}}, func() {
		cl := ryugo.From(c).Colors
		ui.Column(c).Gap(14).Children(func() {
			ryugo.Field(c, ryugo.FieldOptions{Label: locales.CommonName()}, func() {
				in := a.fieldMenu(ryugo.Input(c.Key("prop-name-in"), &e.name, ryugo.InputOptions{Label: locales.CommonThePropertysName(), Placeholder: locales.PropsStatusAuthorDue()}), &e.name, fieldOpts{oneLine: true})
				in.AutoFocus()
				if in.Submitted() {
					a.saveProp()
				}
			})
			if e.id == "" {
				ryugo.Field(c, ryugo.FieldOptions{Label: locales.CommonKind()}, func() {
					ryugo.Select(c.Key("prop-type"), &e.typ, propTypeOptions(), ryugo.SelectOptions{Label: locales.CommonKind()})
				})
			}
			if e.typ >= 0 && PropTypes[e.typ].Type == PropChoice {
				ryugo.Field(c, ryugo.FieldOptions{Label: locales.CommonChoices(), Help: locales.CollectionChoicesHelp()}, func() {
					tags := CleanOptions(strings.Split(e.options, ","))
					before := strings.Join(tags, ",")
					a.tagsField(c, "prop-options", &tags, locales.CommonTypeChoice(), locales.CommonAddChoice())
					if now := strings.Join(tags, ","); now != before {
						e.options = now
					}
				})
			}
			if e.id != "" && e.page == nil { // the collection's: every page of it has it, and its settings take it away
				ui.Text(c, locales.PropsCollectionsProperty(e.coll.Name)).FontSize(ryugo.FsSm).TextColor(cl.Muted)
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
	switch {
	case e.page != nil && e.id == "":
		_, err = e.page.AddProperty(e.name, PropTypes[max(e.typ, 0)].Type, opts)
	case e.page != nil:
		err = e.page.EditProperty(e.id, e.name, opts)
	case e.id == "":
		_, err = e.coll.AddProperty(e.name, PropTypes[max(e.typ, 0)].Type, opts)
	default:
		err = e.coll.EditProperty(e.id, e.name, opts)
	}
	if err != nil {
		e.err = err.Error()
		return
	}
	e.open, e.err = false, ""
}

// propRowMenu is what a property's row offers on a right click, after its text's lines: the property
// to change, its value to clear and, one of the page's own, the property to delete. The collection's are
// deleted in its settings, not from one of its pages (#487).
func (a *app) propRowMenu(p *Page, pr *Property) []menuItem {
	id := pr.ID
	out := []menuItem{{title: "props_editPropertyMenu", glyph: iconEdit, run: func() { a.editProp(p, pr) }}}
	if p.Props[id] != "" {
		out = append(out, menuItem{title: "props_clearValue", glyph: iconClose, run: func() { a.setValue(p, id, "") }})
	}
	if !p.IsOwn(id) {
		return out
	}
	return append(out, menuItem{title: "common_deleteProperty", group: "~", glyph: iconTrash, danger: true, run: func() {
		a.removePageProperty(p, id)
	}})
}

// editProp opens the dialog of property pr: the page's own, or its collection's.
func (a *app) editProp(p *Page, pr *Property) {
	a.prop = propEdit{open: true, coll: p.Coll, id: pr.ID, name: pr.Name, options: strings.Join(pr.Options, ", ")}
	if p.IsOwn(pr.ID) {
		a.prop.page = p
	}
	a.prop.typ = slices.IndexFunc(PropTypes, func(x struct{ Type, Name string }) bool { return x.Type == pr.Type })
}

// removePageProperty takes page p's own property id away, with a way back that brings its value too.
func (a *app) removePageProperty(p *Page, id string) {
	own, value := slices.Clone(p.Own), p.Props[id]
	if p == a.pane().page {
		a.flush()
	}
	if err := p.RemoveProperty(id); err != nil {
		a.fail(err)
		return
	}
	a.toastAction(locales.PropsPropertyDeleted(), locales.CommonUndo(), func() {
		p.Own = own
		if value != "" {
			p.Props[id] = value
		}
		if p == a.pane().page {
			a.flush()
		}
		body, err := p.Body()
		if err == nil {
			err = p.Save(body)
		}
		if err != nil {
			a.fail(err)
		}
	})
}
