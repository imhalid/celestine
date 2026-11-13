package main

import (
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// A collection's settings, as Thymer has them: a dialog (not a sheet along
// the window's edge: the window's controls are there) with tabs - General, Properties, Views, Pages - working on a
// draft of the collection. Kaydet keeps it, Vazgeç (or closing) goes back to
// how it was.

type collSettings struct {
	open  bool
	coll  *Collection
	draft Collection
	orig  []byte // the collection as it was, to go back to
	saved bool
	tab   int
	err   string

	propList, viewList sortState
	prop               string // the property being changed
	view               string // the view being changed

	newName string
	newTags []string // the options of the property being added
	adding  bool     // the form for a new property is open
	newType int

	closed  bool // what closing does is done: the panel only slides away
	discard bool // closing was asked about, and its changes are let go
}

// changed says whether the draft differs from the collection as it was.
func (cs *collSettings) changed() bool {
	return string(cs.draft.ConfigJSON()) != string(cs.orig)
}

func (a *app) openCollSettings(c *Collection, tab int) {
	cs := &a.cs
	*cs = collSettings{open: true, coll: c, tab: tab, orig: c.ConfigJSON()}
	cs.draft.SetConfig(cs.orig)
	cs.draft.Dir = c.Dir
}

// endCollSettings closes the panel: kept if saved, else as it was. The
// panel is still built as it slides away.
func (a *app) endCollSettings() {
	cs := &a.cs
	cs.closed = true
}

func (a *app) saveCollSettings() {
	cs := &a.cs
	oldFormat := cs.coll.JournalFormat
	cs.coll.SetConfig(cs.draft.ConfigJSON())
	if err := cs.coll.Save(); err != nil {
		cs.err = err.Error()
		return
	}
	if a.coll == cs.coll {
		a.collName = cs.coll.Name
	}
	if cs.coll.Journal && formatOrDefault(cs.coll.JournalFormat) != formatOrDefault(oldFormat) {
		a.retitleJournal(cs.coll) // every day by the new format
	}
	cs.saved, cs.open = true, false
}

// collSettingsView is the panel, while open.
func (a *app) collSettingsView(c *ui.Context) {
	cs := &a.cs
	if cs.coll == nil {
		return
	}
	if !cs.open && !cs.closed { // closed by Vazgeç, Esc or the scrim: as it was, once asked when it changed (#217)
		if !cs.saved && !cs.discard && cs.changed() {
			cs.open = true
			a.ask(locales.CollectionDiscardChanges(), locales.CollectionDiscardChangesNote(cs.coll.Name), locales.CollectionDiscard(), func() {
				a.cs.open, a.cs.discard = false, true
			})
		} else {
			a.endCollSettings()
		}
	}
	d := &cs.draft
	ryugo.Dialog(c.Key("coll-settings"), &cs.open, ryugo.DialogOptions{Title: d.Name + " / " + locales.CommonSettings(), Width: 560, Fixed: true, Footer: func() {
		if !cs.coll.Journal { // beside the buttons that close it, on the other side
			if ryugo.Button(c.Key("cs-trash"), locales.CollectionMoveCollectionTrash(), ryugo.ButtonOptions{Color: ryugo.From(c).Colors.Up, Variant: ryugo.Soft, Size: ryugo.Sm, Icon: iconTrash}).Clicked() {
				coll := cs.coll
				cs.open, cs.saved = false, true
				a.trashCollection(coll)
			}
		}
		ui.Spacer(c)
		if ryugo.Button(c.Key("cs-cancel"), locales.CommonCancel(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			cs.open = false
		}
		if ryugo.Button(c.Key("cs-save"), locales.CommonSave(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: ryugo.From(c).Colors.Accent, Variant: ryugo.Solid}).Clicked() {
			a.saveCollSettings()
		}
	}}, func() {
		cl := ryugo.From(c).Colors
		ryugo.Tabs(c.Key("cs-tabs"), &cs.tab, []ryugo.Tab{{Label: locales.CollectionGeneral()}, {Label: locales.CollectionProperties()}, {Label: locales.CollectionViews()}, {Label: locales.CollectionPagesTab()}}, ryugo.TabsOptions{Label: locales.CollectionSettings()})
		ui.Column(c).Gap(16).Padding(16, 0, 8, 0).Children(func() {
			switch cs.tab {
			case 0:
				a.csGeneral(c)
			case 1:
				a.csProperties(c)
			case 2:
				a.csViews(c)
			case 3:
				a.csPages(c)
			}
			if cs.err != "" {
				ui.Text(c, cs.err).FontSize(ryugo.FsSm).TextColor(cl.Up)
			}
		})
	})
}

// ---- General ----

func (a *app) csGeneral(c *ui.Context) {
	cs := &a.cs
	d := &cs.draft
	t := ryugo.From(c)
	cl := t.Colors
	ryugo.Field(c, ryugo.FieldOptions{Label: locales.CommonName()}, func() {
		a.fieldMenu(ryugo.Input(c.Key("cs-name"), &d.Name, ryugo.InputOptions{Label: locales.CollectionTheCollectionsName()}), &d.Name, fieldOpts{oneLine: true})
	})
	if d.Dynamic() {
		ryugo.Field(c, ryugo.FieldOptions{Label: locales.CommonQuery(), Help: locales.CollectionThisCollectionShowsPages()}, func() {
			a.fieldMenu(ryugo.Input(c.Key("cs-query"), &d.Query, ryugo.InputOptions{Label: locales.CommonQuery(), Placeholder: "#fikir koleksiyon:Notlar"}), &d.Query, fieldOpts{oneLine: true})
		})
	}
	ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionDescription(), Help: locales.CollectionDescriptionHelp()}, func() {
		a.fieldMenu(ryugo.Input(c.Key("cs-desc"), &d.Description, ryugo.InputOptions{Label: locales.CollectionDescription(), Placeholder: locales.CollectionWhatThisCollectionFor()}), &d.Description, fieldOpts{oneLine: true})
	})
	ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionIcon()}, func() {
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for _, ic := range collIcons {
				b := ui.Box(c.Key("cs-icon:"+ic.Name)).Size(34, 34).Radius(rad(8)).AlignItems(ui.Center).Justify(ui.Center).
					Label(locales.CollectionIconLabel(map[bool]string{true: locales.CollectionIconStack(), false: ic.Name}[ic.Name == ""])).Role(ui.RoleButton)
				switch {
				case d.Icon == ic.Name:
					b.Background(cl.Accent.Alpha(0.16)).Border(1.5, cl.Accent)
				case b.Hovered():
					b.Background(cl.Fill)
				}
				tint := cl.Text2
				if col, ok := colorOf(t, d.Color); ok {
					tint = col
				}
				b.Children(func() { ui.Icon(c, ic.Glyph).Width(18).Height(18).TextColor(tint) })
				if b.Clicked() {
					d.Icon = ic.Name
				}
			}
		})
	})
	ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionColor()}, func() {
		ui.Row(c).Gap(8).Children(func() {
			for _, name := range collColors {
				col, ok := colorOf(t, name)
				b := ui.Box(c.Key("cs-color:"+name)).Size(24, 24).Radius(12).Label(locales.CollectionColorLabel(map[bool]string{true: locales.CommonNone(), false: name}[name == ""])).Role(ui.RoleButton)
				if ok {
					b.Background(col)
				} else {
					b.Border(1.5, cl.Hairline).Background(cl.Card)
				}
				if d.Color == name {
					b.Border(2.5, cl.Text)
				}
				if b.Clicked() {
					d.Color = name
				}
			}
		})
	})
	if cs.coll.Journal { // how the days are titled: here and nowhere else
		ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionTheDaysTitle(), Help: locales.CollectionDayTitleHelp()}, func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Row(c).Wrap().Gap(6).Children(func() {
					for _, f := range journalFormats {
						f := f
						on := formatOrDefault(d.JournalFormat) == f
						b := ui.Row(c.Key("cs-fmt:"+f)).Padding(5, 10).Radius(rad(8)).Label(locales.CollectionFormatLabel(f)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
						switch {
						case on:
							b.Background(cl.Accent.Alpha(0.16)).Border(1.5, cl.Accent)
						case b.Hovered():
							b.Background(cl.Fill)
						default:
							b.Border(1, cl.Hairline)
						}
						b.Children(func() { ui.Text(c, formatDay(f, a.clock())).FontSize(13) })
						if b.Clicked() {
							d.JournalFormat = f
						}
					}
				})
				a.fieldMenu(ryugo.Input(c.Key("cs-journal-format"), &d.JournalFormat, ryugo.InputOptions{Label: locales.CollectionTitleFormat(), Placeholder: defaultFormat}), &d.JournalFormat, fieldOpts{oneLine: true})
				ui.Text(c, locales.CollectionTodayIs(formatDay(d.JournalFormat, a.clock()))).FontSize(ryugo.FsSm).TextColor(cl.Muted)
			})
		})
		return
	}
}

// ---- Properties ----

// builtins are the properties every page has.
var builtins = []struct {
	key        string
	name, kind func() string
}{
	{"title", locales.CollectionPropTitle, locales.CollectionKindText}, {"created", locales.CollectionPropCreated, locales.CollectionKindDate},
	{"modified", locales.CollectionPropModified, locales.CollectionKindDate}, {"parent", locales.CollectionPropParent, locales.CollectionKindPage},
}

func (a *app) csProperties(c *ui.Context) {
	cs := &a.cs
	d := &cs.draft
	cl := ryugo.From(c).Colors
	// the built-in ones, as the ones added are, but passive: they are there
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		for _, b := range builtins {
			b := b
			ui.Row(c.Key("cs-builtin:"+b.key)).FillWidth().Height(48).Padding(0, 12, 0, 10).Gap(10).Radius(rad(12)).AlignItems(ui.Center).
				Background(cl.Card).Border(1, cl.Hairline).Opacity(0.55).Label(locales.CollectionBuiltInLabel(b.name())).Children(func() {
				ui.Icon(c, iconLock).Width(15).Height(15).TextColor(cl.Muted)
				ui.Text(c, b.name()).FontSize(ryugo.FsMd).FontWeight(600).TextColor(cl.Text).MaxLines(1).Shrink(0)
				ui.Text(c, locales.CollectionBuiltInKind(b.kind())).FontSize(ryugo.FsSm).TextColor(cl.Muted).MaxLines(1)
			})
		}
	})
	if len(d.Properties) > 0 {
		a.sortList(c, "cs-props", &cs.propList, sortOpts{
			n:        len(d.Properties),
			id:       func(i int) string { return d.Properties[i].ID },
			label:    func(i int) string { return locales.CollectionPropertyLabel(d.Properties[i].Name) },
			selected: cs.prop,
			content: func(i int) {
				p := d.Properties[i]
				ui.Text(c, p.Name).FontSize(ryugo.FsMd).FontWeight(600).TextColor(cl.Text).MaxLines(1).Shrink(0)
				ui.Text(c, "· "+PropTypeName(p.Type)).FontSize(ryugo.FsSm).TextColor(cl.Muted).MaxLines(1).Shrink(0)
				if p.Type == PropChoice {
					ui.Text(c, strings.Join(p.Options, ", ")).FontSize(ryugo.FsSm).TextColor(cl.Muted).MaxLines(1).Shrink(1).MinWidth(0)
				}
			},
			toggle: func(i int) {
				cs.prop = map[bool]string{true: "", false: d.Properties[i].ID}[cs.prop == d.Properties[i].ID]
			},
			editor: func(i int) { a.csPropEditor(c, &d.Properties[i]) },
			move:   func(from, to int) { d.Properties = moveItem(d.Properties, from, to) },
		})
	}
	a.csAddProperty(c)
}

var iconLock = icon(`<rect x="5.5" y="10.5" width="13" height="9" rx="2.5"/><path d="M8.5 10.5V8a3.5 3.5 0 0 1 7 0v2.5"/>`)

// tagField is a field of tags for the options of a choice: each with its cross,
// added with ↵ or a comma.
func (a *app) tagField(c *ui.Context, key string, tags *[]string, placeholder string) {
	before := slices.Clone(*tags)
	a.tagsField(c, key, tags, placeholder, locales.CommonAddChoice())
	if !slices.Equal(before, *tags) {
		*tags = CleanOptions(*tags)
	}
}

// csPropEditor changes a property of the draft.
func (a *app) csPropEditor(c *ui.Context, p *Property) {
	cs := &a.cs
	ui.Column(c.Key("cs-prop-edit:" + p.ID)).FillWidth().Gap(10).Padding(12).Children(func() {
		ryugo.Field(c, ryugo.FieldOptions{Label: locales.CommonName()}, func() {
			a.fieldMenu(ryugo.Input(c.Key("cs-prop-name"), &p.Name, ryugo.InputOptions{Label: locales.CommonThePropertysName(), Size: ryugo.Sm}), &p.Name, fieldOpts{oneLine: true})
		})
		if p.Type == PropChoice {
			ryugo.Field(c, ryugo.FieldOptions{Label: locales.CommonChoices(), Help: locales.CollectionChoicesHelp()}, func() {
				a.tagField(c, "cs-prop-opts", &p.Options, "common_typeChoice")
			})
		}
		if ryugo.Button(c.Key("cs-prop-del"), locales.CommonDeleteProperty(), ryugo.ButtonOptions{Color: ryugo.From(c).Colors.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
			id := p.ID
			cs.draft.Properties = slices.DeleteFunc(cs.draft.Properties, func(q Property) bool { return q.ID == id })
			cs.draft.Normalize()
			cs.prop = ""
		}
	})
}

// csAddProperty adds a property to the draft: a new one, or one of another
// collection's, offered as its name is typed.
func (a *app) csAddProperty(c *ui.Context) {
	cs := &a.cs
	d := &cs.draft
	cl := ryugo.From(c).Colors
	if !cs.adding { // a button at the foot; the form opens when it is pressed
		ui.Row(c).Children(func() {
			if ryugo.Button(c.Key("cs-add-open"), locales.CommonAddProperty(), ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconPlus}).Clicked() {
				cs.adding = true
			}
		})
		return
	}
	ui.Column(c).Gap(10).Padding(12).Radius(rad(12)).Border(1, cl.Hairline).Children(func() {
		ui.Text(c, locales.CommonAddProperty()).FontSize(ryugo.FsMd).FontWeight(600).TextColor(cl.Text)
		a.fieldMenu(ryugo.Input(c.Key("cs-new-name"), &cs.newName, ryugo.InputOptions{Label: locales.CollectionTheNewPropertysName(), Placeholder: locales.CollectionTypeNameOtherCollections(), Size: ryugo.Sm}), &cs.newName, fieldOpts{oneLine: true})
		// the properties of the other collections, as the name is typed
		if sugg := a.sharedProps(cs.newName); len(sugg) > 0 {
			ui.Column(c).Gap(2).Children(func() {
				for _, sp := range sugg {
					b := ui.Row(c.Key("cs-sugg:"+sp.from+":"+sp.p.ID)).Height(32).Gap(8).Padding(0, 10).Radius(rad(8)).AlignItems(ui.Center).
						Label(locales.CollectionSuggestionLabel(sp.p.Name)).Role(ui.RoleButton)
					if b.Hovered() {
						b.Background(cl.Fill)
					}
					b.Children(func() {
						ui.Icon(c, iconPlus).Width(14).Height(14).TextColor(cl.Muted)
						ui.Text(c, sp.p.Name).FontSize(ryugo.FsSm).FontWeight(600).TextColor(cl.Text)
						ui.Text(c, "· "+PropTypeName(sp.p.Type)+" · "+sp.from).FontSize(ryugo.FsSm).TextColor(cl.Muted)
					})
					if b.Clicked() {
						p := sp.p
						if d.Prop(p.ID) != nil {
							p.ID = Slug(p.Name) + "-2"
						}
						p.Options = slices.Clone(p.Options)
						d.Properties = append(d.Properties, p)
						d.Normalize()
						cs.newName, cs.adding = "", false
					}
				}
			})
		}
		ryugo.Select(c.Key("cs-new-type"), &cs.newType, propTypeOptions(), ryugo.SelectOptions{Label: locales.CollectionTheNewPropertysType(), Size: ryugo.Sm})
		if PropTypes[cs.newType].Type == PropChoice {
			a.tagField(c, "cs-new-opts", &cs.newTags, locales.CollectionChoicesPlaceholder())
		}
		ui.Row(c).Gap(8).Children(func() { // as wide as their labels
			if ryugo.Button(c.Key("cs-new-add"), locales.CommonAdd(), ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconPlus, Disabled: strings.TrimSpace(cs.newName) == ""}).Clicked() {
				a.addDraftProp()
			}
			if ryugo.Button(c.Key("cs-new-cancel"), locales.CommonCancel(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
				cs.adding, cs.newName, cs.newTags = false, "", nil
			}
		})
	})
}

func (a *app) addDraftProp() {
	cs := &a.cs
	d := &cs.draft
	if _, err := d.AddPropertyDraft(cs.newName, PropTypes[cs.newType].Type, cs.newTags); err != nil {
		cs.err = err.Error()
		return
	}
	cs.err, cs.newName, cs.newTags, cs.adding = "", "", nil, false
}

// sharedProp is a property of another collection.
type sharedProp struct {
	p    Property
	from string
}

// sharedProps are the properties of the other collections whose names
// match name, not yet in the draft, a name once.
func (a *app) sharedProps(name string) []sharedProp {
	cs := &a.cs
	if strings.TrimSpace(name) == "" {
		return nil
	}
	var out []sharedProp
	seen := map[string]bool{}
	for _, c := range a.lib.Collections {
		if c == cs.coll {
			continue
		}
		for _, p := range c.Properties {
			k := fold(p.Name)
			if seen[k] || cs.draft.PropNamed(p.Name) != nil || match(name, p.Name) < 0 {
				continue
			}
			seen[k] = true
			out = append(out, sharedProp{p, c.Name})
		}
	}
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

// ---- Views ----

// viewTypes are the kinds of view, each with its name in the window's language and that name's message id.
var viewTypes = []struct {
	Type string
	Name func() string
	id   string
}{
	{"list", locales.ViewsTypeList, "views_typeList"}, {"table", locales.ViewsTypeTable, "views_typeTable"}, {"board", locales.ViewsTypeBoard, "views_typeBoard"},
	{"calendar", locales.ViewsTypeCalendar, "views_typeCalendar"}, {"gallery", locales.ViewsTypeGallery, "views_typeGallery"},
}

// viewLabel is what a view is called in the window: the name it was given, or, while that is still its type's
// name in one of the window's languages ("Liste", which collections made in Turkish keep, or "List"), its type's
// name in the window's language, so that an English window does not show a Turkish "Liste".
func viewLabel(v View) string {
	for _, vt := range viewTypes {
		if vt.Type != v.Type {
			continue
		}
		if v.Label == "" {
			return vt.Name()
		}
		for _, l := range languages {
			if locales.Catalog.Format(l.code, vt.id, nil) == v.Label {
				return vt.Name()
			}
		}
	}
	return v.Label
}

func (a *app) csViews(c *ui.Context) {
	cs := &a.cs
	d := &cs.draft
	cl := ryugo.From(c).Colors
	ui.Text(c, locales.CollectionShowPagesAsList()).FontSize(ryugo.FsSm).TextColor(cl.Muted)
	a.sortList(c, "cs-views", &cs.viewList, sortOpts{
		n:        len(d.Views),
		id:       func(i int) string { return d.Views[i].ID },
		label:    func(i int) string { return msg("view: " + viewLabel(d.Views[i])) },
		selected: cs.view,
		content: func(i int) {
			v := d.Views[i]
			ui.Text(c, viewLabel(v)).FontSize(ryugo.FsMd).FontWeight(600).TextColor(cl.Text).MaxLines(1).Shrink(0)
			for _, vt := range viewTypes {
				if vt.Type == v.Type {
					ui.Text(c, "· "+vt.Name()).FontSize(ryugo.FsSm).TextColor(cl.Muted).MaxLines(1).Shrink(0)
				}
			}
		},
		toggle: func(i int) { cs.view = map[bool]string{true: "", false: d.Views[i].ID}[cs.view == d.Views[i].ID] },
		editor: func(i int) { a.csViewEditor(c, &d.Views[i]) },
		move:   func(from, to int) { d.Views = moveItem(d.Views, from, to) },
	})
	hasChoice := slices.ContainsFunc(d.Properties, func(p Property) bool { return p.Type == PropChoice })
	ui.Row(c).Children(func() { a.csAddView(c, d, hasChoice) }) // as wide as its label
	if !hasChoice {
		ui.Text(c, locales.CollectionABoardNeedsSelect()).FontSize(ryugo.FsSm).TextColor(cl.Muted)
	}
}

// csAddView is the menu adding a view to the draft.
func (a *app) csAddView(c *ui.Context, d *Collection, hasChoice bool) {
	cs := &a.cs
	ryugo.MenuButton(c.Key("cs-add-view"), locales.CollectionAddView(), ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconPlus}, func(m *ryugo.MenuList) {
		for _, vt := range viewTypes {
			if vt.Type == "board" && !hasChoice {
				continue
			}
			if m.Item(vt.Name(), ryugo.ItemOptions{}) {
				id := vt.Type
				for n := 2; slices.ContainsFunc(d.Views, func(v View) bool { return v.ID == id }); n++ {
					id = vt.Type + "-" + itoa(n)
				}
				v := View{ID: id, Label: vt.Name(), Type: vt.Type, Sort: "title"}
				if vt.Type == "board" {
					v.Sort = "rank"
					for _, p := range d.Properties {
						if p.Type == PropChoice {
							v.GroupBy = p.ID
							break
						}
					}
				}
				d.Views = append(d.Views, v)
				cs.view = id
			}
		}
	})
}

// csViewEditor changes a view of the draft: its name and what it shows.
func (a *app) csViewEditor(c *ui.Context, v *View) {
	cs := &a.cs
	d := &cs.draft
	ui.Column(c.Key("cs-view-edit:" + v.ID)).FillWidth().Gap(12).Padding(12).Children(func() {
		ryugo.Field(c, ryugo.FieldOptions{Label: locales.CommonName()}, func() {
			a.fieldMenu(ryugo.Input(c.Key("cs-view-name"), &v.Label, ryugo.InputOptions{Label: locales.CollectionTheViewsName(), Size: ryugo.Sm}), &v.Label, fieldOpts{oneLine: true})
		})
		switch v.Type {
		case "table":
			ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionColumns(), Help: locales.CollectionColumnsHelp()}, func() {
				ui.Column(c).Gap(6).Children(func() {
					for _, col := range allColumns(d) {
						on := slices.Contains(viewColumns(d, v), col.key)
						ryugo.Checkbox(c.Key("cs-col:"+col.key), &on, col.label, ryugo.CheckOptions{})
						if on != slices.Contains(viewColumns(d, v), col.key) {
							v.Columns = toggleKey(viewColumns(d, v), col.key, on, allKeys(d))
						}
					}
				})
			})
			a.csViewSort(c, d, v)
			groups := []ryugo.Option[string]{{Value: "", Label: locales.CollectionNoGroups()}}
			for _, p := range d.Properties {
				if p.Type == PropChoice {
					groups = append(groups, ryugo.Option[string]{Value: p.ID, Label: p.Name})
				}
			}
			ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionGroupedBy()}, func() {
				ryugo.Select(c.Key("cs-table-group"), &v.GroupBy, groups, ryugo.SelectOptions{Label: locales.CollectionGroupedBy(), Size: ryugo.Sm})
			})
			a.csTableCalcs(c, d, v)
		case "calendar":
			ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionWhichDayGoes(), Help: locales.CollectionCalendarDayHelp()}, func() {
				opts := []ryugo.Option[string]{{Value: "created", Label: locales.CollectionDayMade()}, {Value: "modified", Label: locales.CollectionDayChanged()}}
				for _, p := range d.Properties {
					if p.Type == PropDate {
						opts = append(opts, ryugo.Option[string]{Value: "prop:" + p.ID, Label: p.Name})
					}
				}
				f := v.DateField
				if f == "" {
					f = "created"
				}
				ryugo.Select(c.Key("cs-datefield"), &f, opts, ryugo.SelectOptions{Label: locales.CollectionDateField(), Size: ryugo.Sm})
				v.DateField = f
			})
		case "list", "gallery":
			a.csViewSort(c, d, v)
			if v.Type == "gallery" {
				a.csViewFields(c, d, v)
				break
			}
			opts := []ryugo.Option[string]{{Value: "", Label: locales.CollectionWhenTheyChanged()}}
			for _, p := range d.Properties {
				if p.Type == PropChoice {
					opts = append(opts, ryugo.Option[string]{Value: p.ID, Label: p.Name})
				}
			}
			ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionGroupedBy()}, func() {
				ryugo.Select(c.Key("cs-list-group"), &v.GroupBy, opts, ryugo.SelectOptions{Label: locales.CollectionGroupedBy(), Size: ryugo.Sm})
			})
			a.csViewFields(c, d, v)
		case "board":
			var opts []ryugo.Option[string]
			for _, p := range d.Properties {
				if p.Type == PropChoice {
					opts = append(opts, ryugo.Option[string]{Value: p.ID, Label: p.Name})
				}
			}
			ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionColumnsBy()}, func() {
				ryugo.Select(c.Key("cs-group"), &v.GroupBy, opts, ryugo.SelectOptions{Label: locales.CollectionGroupedBy2(), Size: ryugo.Sm})
			})
			ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionCardSize()}, func() {
				sizes := []ryugo.Option[string]{{Value: "small", Label: locales.CollectionSmall()}, {Value: "", Label: locales.CollectionMedium()}, {Value: "large", Label: locales.CollectionLarge()}}
				ryugo.Select(c.Key("cs-cardsize"), &v.CardSize, sizes, ryugo.SelectOptions{Label: locales.CollectionCardSize(), Size: ryugo.Sm})
			})
			a.csViewFields(c, d, v)
		default: // board and gallery: the properties shown on a card
			a.csViewFields(c, d, v)
		}
		a.csFilters(c, d, v)
		ui.Row(c).Children(func() { // as wide as its label
			if len(d.Views) > 1 && v.Type != "calendar" && ryugo.Button(c.Key("cs-view-del"), locales.CollectionDeleteView(), ryugo.ButtonOptions{Color: ryugo.From(c).Colors.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
				id := v.ID
				d.Views = slices.DeleteFunc(d.Views, func(x View) bool { return x.ID == id })
				cs.view = ""
			}
		})
	})
}

// csTableCalcs are what the foot of a table says under its columns: for the title, how many pages; for a number,
// its sum, mean, least or most, or how many have one.
func (a *app) csTableCalcs(c *ui.Context, d *Collection, v *View) {
	type target struct{ key, name, typ string }
	targets := []target{{"title", locales.CollectionTitle(), "title"}}
	for _, p := range d.Properties {
		if slices.Contains(viewColumns(d, v), "prop:"+p.ID) {
			targets = append(targets, target{"prop:" + p.ID, p.Name, p.Type})
		}
	}
	ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionAtFoot(), Help: locales.CollectionWhatLastRowSays()}, func() {
		ui.Column(c).Gap(6).FillWidth().Children(func() {
			for _, tg := range targets {
				opts := []ryugo.Option[string]{{Value: "", Label: locales.CollectionNothing()}, {Value: "count", Label: footName("count")}}
				if tg.typ == PropNumber {
					for _, k := range []string{"sum", "avg", "min", "max"} {
						opts = append(opts, ryugo.Option[string]{Value: k, Label: footName(k)})
					}
				}
				calc := v.Calcs[tg.key]
				ui.Row(c.Key("cs-calc:" + tg.key)).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, tg.name).FontSize(13).Width(140).MaxLines(1)
					ryugo.Select(c.Key("cs-calc-sel:"+tg.key), &calc, opts, ryugo.SelectOptions{Label: tg.name, Size: ryugo.Sm}).Width(160)
				})
				if calc != v.Calcs[tg.key] {
					if v.Calcs == nil {
						v.Calcs = map[string]string{}
					}
					if calc == "" {
						delete(v.Calcs, tg.key)
					} else {
						v.Calcs[tg.key] = calc
					}
				}
			}
		})
	})
}

// csViewSort is how a list or a gallery orders its pages: by title, when made or changed, the collection's own order
// (dragged in a list), or a property; and which way.
func (a *app) csViewSort(c *ui.Context, d *Collection, v *View) {
	opts := []ryugo.Option[string]{{Value: "title", Label: locales.CollectionTitle()}, {Value: "created", Label: locales.CommonCreated()}, {Value: "modified", Label: locales.CommonModified()}, {Value: "manual", Label: locales.CollectionYourOwnOrder()}}
	for _, p := range d.Properties {
		opts = append(opts, ryugo.Option[string]{Value: "prop:" + p.ID, Label: p.Name})
	}
	if v.Sort == "" || v.Sort == "rank" {
		v.Sort = "title"
	}
	ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionSortedBy(), Help: map[bool]string{true: locales.CollectionDragRowsListInto(), false: ""}[v.Sort == "manual"]}, func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ryugo.Select(c.Key("cs-sort"), &v.Sort, opts, ryugo.SelectOptions{Label: locales.CollectionSortedBy(), Size: ryugo.Sm}).Width(200)
			if v.Sort != "manual" {
				ryugo.Checkbox(c.Key("cs-desc"), &v.Desc, locales.CollectionReversed(), ryugo.CheckOptions{})
			} else {
				v.Desc = false // the order is the one dragged
			}
		})
	})
}

// editViewFilters opens view v's settings, where its filters are.
func (a *app) editViewFilters(coll *Collection, v *View) {
	a.openCollSettings(coll, 2)
	a.cs.view = v.ID
}

// csFilters are view v's filters: each a property, a comparison and a value (none for "is empty"), removed by its
// cross; a new one is on the title, to be changed.
func (a *app) csFilters(c *ui.Context, d *Collection, v *View) {
	cl := ryugo.From(c).Colors
	ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionFilters(), Help: locales.CollectionTheViewShowsOnly()}, func() {
		ui.Column(c).Gap(6).FillWidth().Children(func() {
			props := []ryugo.Option[string]{{Value: "title", Label: locales.CollectionTitle()}}
			for _, p := range d.Properties {
				props = append(props, ryugo.Option[string]{Value: p.ID, Label: p.Name})
			}
			remove := -1
			for i := range v.Filters {
				f := &v.Filters[i]
				typ, pr := "title", d.Prop(f.Prop)
				if pr != nil {
					typ = pr.Type
				}
				ops := FilterOps(typ)
				if !slices.Contains(ops, f.Op) {
					f.Op = ops[0]
				}
				var opOpts []ryugo.Option[string]
				for _, o := range ops {
					opOpts = append(opOpts, ryugo.Option[string]{Value: o, Label: filterOpName(o, typ)})
				}
				k := strconv.Itoa(i)
				ui.Row(c.Key("cs-filter:" + k)).Gap(6).AlignItems(ui.Center).FillWidth().Children(func() {
					was := f.Prop
					ryugo.Select(c.Key("cs-fprop:"+k), &f.Prop, props, ryugo.SelectOptions{Label: locales.CollectionProperty(), Size: ryugo.Sm}).Width(150)
					if f.Prop != was {
						f.Value = ""
					}
					ryugo.Select(c.Key("cs-fop:"+k), &f.Op, opOpts, ryugo.SelectOptions{Label: locales.CollectionComparison(), Size: ryugo.Sm}).Width(140)
					switch {
					case f.Op == "empty" || f.Op == "notEmpty":
						ui.Spacer(c)
					case pr != nil && pr.Type == PropChoice:
						var vals []ryugo.Option[string]
						for _, o := range pr.Options {
							vals = append(vals, ryugo.Option[string]{Value: o, Label: o})
						}
						ryugo.Select(c.Key("cs-fval:"+k), &f.Value, vals, ryugo.SelectOptions{Label: locales.CollectionValue(), Size: ryugo.Sm}).Grow(1)
					default:
						ph := ""
						if typ == PropDate {
							ph = "2026-11-04"
						}
						a.fieldMenu(ryugo.Input(c.Key("cs-fval:"+k), &f.Value, ryugo.InputOptions{Label: locales.CollectionValue(), Placeholder: ph, Size: ryugo.Sm}), &f.Value, fieldOpts{oneLine: true}).Grow(1)
					}
					if ryugo.Button(c.Key("cs-fdel:"+k), locales.CollectionRemoveFilter(), ryugo.ButtonOptions{Icon: iconClose, IconOnly: true, Variant: ryugo.Soft, Size: ryugo.Sm, Color: cl.Muted}).Clicked() {
						remove = i
					}
				})
			}
			if remove >= 0 {
				v.Filters = slices.Delete(v.Filters, remove, remove+1)
			}
			ui.Row(c).Children(func() {
				if ryugo.Button(c.Key("cs-fadd"), locales.CollectionAddFilter(), ryugo.ButtonOptions{Icon: iconPlus, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
					v.Filters = append(v.Filters, Filter{Prop: "title", Op: "contains"})
				}
			})
		})
	})
}

// filterOpName is a comparison as the filter's menu says it, for a property of type typ.
func filterOpName(op, typ string) string {
	switch op {
	case "is":
		return locales.CollectionIs()
	case "isNot":
		return locales.CollectionIsNot()
	case "contains":
		return locales.CollectionContains()
	case "empty":
		return locales.CollectionIsEmpty()
	case "notEmpty":
		return locales.CollectionIsNotEmpty()
	case "lt":
		if typ == PropDate {
			return locales.CollectionIsBefore()
		}
		return locales.CollectionIsLessThan()
	case "gt":
		if typ == PropDate {
			return locales.CollectionIsAfter()
		}
		return locales.CollectionIsMoreThan()
	}
	return op
}

// csViewFields is what a list's rows or a board's or gallery's cards show: the start of the text, and which
// properties.
func (a *app) csViewFields(c *ui.Context, d *Collection, v *View) {
	ryugo.Field(c, ryugo.FieldOptions{Label: map[bool]string{true: locales.CollectionOnCards(), false: locales.CollectionOnRows()}[v.Type != "list"]}, func() {
		ui.Column(c).Gap(6).Children(func() {
			show := !v.NoExcerpt
			ryugo.Checkbox(c.Key("cs-excerpt"), &show, locales.CollectionExcerpt(), ryugo.CheckOptions{})
			v.NoExcerpt = !show
			for _, p := range d.Properties {
				if v.Type == "board" && p.ID == v.GroupBy {
					continue
				}
				on := slices.Contains(v.Fields, p.ID)
				ryugo.Checkbox(c.Key("cs-field:"+p.ID), &on, p.Name, ryugo.CheckOptions{})
				if on != slices.Contains(v.Fields, p.ID) {
					if on {
						v.Fields = append(v.Fields, p.ID)
					} else {
						v.Fields = slices.DeleteFunc(v.Fields, func(f string) bool { return f == p.ID })
					}
				}
			}
		})
	})
}

// column is a column a table can show.
type colChoice struct{ key, label string }

// allColumns are the columns a table of c can show, besides the title.
func allColumns(c *Collection) []colChoice {
	var out []colChoice
	for _, p := range c.Properties {
		out = append(out, colChoice{"prop:" + p.ID, p.Name})
	}
	return append(out, colChoice{"parent", locales.CollectionPropParent()}, colChoice{"created", locales.CollectionPropCreated()}, colChoice{"modified", locales.CollectionPropModified()})
}

func allKeys(c *Collection) []string {
	var out []string
	for _, col := range allColumns(c) {
		out = append(out, col.key)
	}
	return out
}

// viewColumns are the columns view v of c shows: its own, or all.
func viewColumns(c *Collection, v *View) []string {
	if len(v.Columns) == 0 {
		return allKeys(c)
	}
	return v.Columns
}

// toggleKey puts key in or out of keys, in the order of all.
func toggleKey(keys []string, key string, on bool, all []string) []string {
	var out []string
	for _, k := range all {
		if k == key {
			if on {
				out = append(out, k)
			}
			continue
		}
		if slices.Contains(keys, k) {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		return []string{"none"} // none but the title: not "all"
	}
	return out
}

// ---- Pages ----

func (a *app) csPages(c *ui.Context) {
	d := &a.cs.draft
	ryugo.Switch(c.Key("cs-subpages"), &d.SubPages, ryugo.SwitchOptions{Label: locales.CollectionSubPages(), Description: locales.CollectionAPageCanHave()})
	ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionPageNounTitle(), Help: locales.CollectionPageNounHelp()}, func() {
		a.fieldMenu(ryugo.Input(c.Key("cs-noun"), &d.PageNoun, ryugo.InputOptions{Label: locales.CollectionThePagesName(), Placeholder: locales.CommonPages()}), &d.PageNoun, fieldOpts{oneLine: true})
	})
	ryugo.Field(c, ryugo.FieldOptions{Label: locales.CollectionNewPageTemplate(), Help: locales.CollectionTemplateHelp()}, func() {
		a.fieldMenu(ryugo.Textarea(c.Key("cs-template"), &d.Template, ryugo.TextareaOptions{Label: locales.CollectionTemplate(), Placeholder: locales.CollectionTemplatePlaceholder(), Rows: 6, MaxRows: 14, Size: ryugo.Sm}), &d.Template, fieldOpts{})
	})
	a.csTemplates(c)
}

// itoa writes n in decimal, as strconv.Itoa does: a key made of it is the number's own, a negative one too (the page
// history keys the copy of the page as it was with -1, #377).
func itoa(n int) string { return strconv.Itoa(n) }
