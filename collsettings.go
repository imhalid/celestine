package main

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// A collection's settings, as Thymer has them: a dialog (not a sheet along
// the window's edge: the window's controls are there) with tabs - General, Properties, Views, Pages, Code - working on a
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

	code   string // the Code tab's JSON
	closed bool   // what closing does is done: the panel only slides away
}

// configFields are the fields of a collection's collection.json, without
// the folder and pages.
func (c *Collection) configJSON() []byte {
	data, _ := json.MarshalIndent(c, "", "  ")
	return data
}

// setConfig gives c the configuration in data, every field of it.
func (c *Collection) setConfig(data []byte) error {
	var fresh Collection
	if err := json.Unmarshal(data, &fresh); err != nil {
		return err
	}
	c.Name, c.Icon, c.Color, c.Description, c.PageNoun, c.Template = fresh.Name, fresh.Icon, fresh.Color, fresh.Description, fresh.PageNoun, fresh.Template
	c.SubPages, c.Views, c.Properties, c.ActiveView = fresh.SubPages, fresh.Views, fresh.Properties, fresh.ActiveView
	c.JournalFormat = fresh.JournalFormat // c.Pinned and c.Order are the sidebar's, not the code tab's
	// c.Journal stays as it is: the code tab does not take it away
	if c.Name == "" {
		c.Name = "Adsız koleksiyon"
	}
	c.normalize()
	return nil
}

func (a *app) openCollSettings(c *Collection, tab int) {
	cs := &a.cs
	*cs = collSettings{open: true, coll: c, tab: tab, orig: c.configJSON()}
	cs.draft.setConfig(cs.orig)
	cs.draft.Dir = c.Dir
	cs.code = string(cs.orig)
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
	cs.coll.setConfig(cs.draft.configJSON())
	if err := cs.coll.save(); err != nil {
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
	if !cs.open && !cs.closed {
		a.endCollSettings() // closed by Vazgeç, Esc or the scrim: as it was
	}
	d := &cs.draft
	ryugo.Dialog(c.Key("coll-settings"), &cs.open, ryugo.DialogOptions{Title: d.Name + " / Ayarlar", Width: 560, Fixed: true, Footer: func() {
		if !cs.coll.Journal { // beside the buttons that close it, on the other side
			if ryugo.Button(c.Key("cs-trash"), "Koleksiyonu çöpe taşı", ryugo.ButtonOptions{Color: ryugo.From(c).Colors.Up, Variant: ryugo.Soft, Size: ryugo.Sm, Icon: iconTrash}).Clicked() {
				coll := cs.coll
				cs.open, cs.saved = false, true
				a.trashCollection(coll)
			}
		}
		ui.Spacer(c)
		if ryugo.Button(c.Key("cs-cancel"), "Vazgeç", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			cs.open = false
		}
		if ryugo.Button(c.Key("cs-save"), "Kaydet", ryugo.ButtonOptions{Size: ryugo.Sm, Color: ryugo.From(c).Colors.Accent, Variant: ryugo.Solid}).Clicked() {
			a.saveCollSettings()
		}
	}}, func() {
		cl := ryugo.From(c).Colors
		was := cs.tab
		ryugo.Tabs(c.Key("cs-tabs"), &cs.tab, []ryugo.Tab{{Label: "Genel"}, {Label: "Özellikler"}, {Label: "Görünümler"}, {Label: "Sayfalar"}, {Label: "Kod"}}, ryugo.TabsOptions{Label: "Koleksiyon ayarları"})
		if cs.tab == 4 && was != 4 {
			cs.code = string(d.configJSON())
		}
		if was == 4 && cs.tab != 4 {
			a.applyCode()
		}
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
			case 4:
				ui.Text(c, "Koleksiyonun collection.json dosyası. Değiştirince başka sekmeye geçmek ya da Kaydet uygular.").FontSize(ryugo.FsSm).TextColor(cl.Muted)
				ryugo.Textarea(c.Key("cs-code"), &cs.code, ryugo.TextareaOptions{Label: "collection.json", Rows: 14, MaxRows: 24, Size: ryugo.Sm}).Font(ryugo.FontMono)
				if ryugo.Button(c.Key("cs-code-apply"), "Uygula", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.applyCode()
				}
			}
			if cs.err != "" {
				ui.Text(c, cs.err).FontSize(ryugo.FsSm).TextColor(cl.Up)
			}
		})
	})
}

// applyCode takes the Code tab's JSON into the draft.
func (a *app) applyCode() {
	cs := &a.cs
	if err := cs.draft.setConfig([]byte(cs.code)); err != nil {
		cs.err = "JSON okunamadı: " + err.Error()
		return
	}
	cs.err = ""
}

// ---- General ----

func (a *app) csGeneral(c *ui.Context) {
	cs := &a.cs
	d := &cs.draft
	t := ryugo.From(c)
	cl := t.Colors
	ryugo.Field(c, ryugo.FieldOptions{Label: "Ad"}, func() {
		ryugo.Input(c.Key("cs-name"), &d.Name, ryugo.InputOptions{Label: "Koleksiyonun adı"})
	})
	ryugo.Field(c, ryugo.FieldOptions{Label: "Açıklama", Help: "Koleksiyonun sayfasında adının altında görünür."}, func() {
		ryugo.Input(c.Key("cs-desc"), &d.Description, ryugo.InputOptions{Label: "Açıklama", Placeholder: "Bu koleksiyon ne için?"})
	})
	ryugo.Field(c, ryugo.FieldOptions{Label: "İkon"}, func() {
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for _, ic := range collIcons {
				b := ui.Box(c.Key("cs-icon:"+ic.Name)).Size(34, 34).Radius(8).AlignItems(ui.Center).Justify(ui.Center).
					Label("İkon: " + map[bool]string{true: "yığın", false: ic.Name}[ic.Name == ""]).Role(ui.RoleButton)
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
	ryugo.Field(c, ryugo.FieldOptions{Label: "Renk"}, func() {
		ui.Row(c).Gap(8).Children(func() {
			for _, name := range collColors {
				col, ok := colorOf(t, name)
				b := ui.Box(c.Key("cs-color:"+name)).Size(24, 24).Radius(12).Label("Renk: " + map[bool]string{true: "yok", false: name}[name == ""]).Role(ui.RoleButton)
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
		ryugo.Field(c, ryugo.FieldOptions{Label: "Günlerin başlığı", Help: "Günlerin sayfaları bu biçimle adlandırılır; başlık sayfada yazılmaz, yalnız buradan değişir. YYYY yıl, MM ay, D gün, MMMM ay adı, dddd gün adı."}, func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Row(c).Wrap().Gap(6).Children(func() {
					for _, f := range journalFormats {
						f := f
						on := formatOrDefault(d.JournalFormat) == f
						b := ui.Row(c.Key("cs-fmt:"+f)).Padding(5, 10).Radius(8).Label("Biçim: " + f).Role(ui.RoleButton).Cursor(ui.CursorPointer)
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
				ryugo.Input(c.Key("cs-journal-format"), &d.JournalFormat, ryugo.InputOptions{Label: "Başlık biçimi", Placeholder: defaultFormat})
				ui.Text(c, "Bugün: "+formatDay(d.JournalFormat, a.clock())).FontSize(ryugo.FsSm).TextColor(cl.Muted)
			})
		})
		ui.Text(c, "Günlük her kitaplıkta durur: günlerin sayfaları burada tutulur. Çöpe taşınamaz.").FontSize(ryugo.FsSm).TextColor(cl.Muted)
		return
	}
}

// ---- Properties ----

// builtins are the properties every page has.
var builtins = []struct{ name, kind string }{{"Başlık", "Metin"}, {"Oluşturulma", "Tarih"}, {"Değiştirilme", "Tarih"}, {"Üst sayfa", "Sayfa"}}

func (a *app) csProperties(c *ui.Context) {
	cs := &a.cs
	d := &cs.draft
	cl := ryugo.From(c).Colors
	// the built-in ones, as the ones added are, but passive: they are there
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		for _, b := range builtins {
			b := b
			ui.Row(c.Key("cs-builtin:"+b.name)).FillWidth().Height(48).Padding(0, 12, 0, 10).Gap(10).Radius(12).AlignItems(ui.Center).
				Background(cl.Card).Border(1, cl.Hairline).Opacity(0.55).Label("Yerleşik özellik: " + b.name).Children(func() {
				ui.Icon(c, iconLock).Width(15).Height(15).TextColor(cl.Muted)
				ui.Text(c, b.name).FontSize(ryugo.FsMd).FontWeight(600).TextColor(cl.Text).MaxLines(1).Shrink(0)
				ui.Text(c, "· "+b.kind).FontSize(ryugo.FsSm).TextColor(cl.Muted).MaxLines(1)
			})
		}
	})
	if len(d.Properties) > 0 {
		a.sortList(c, "cs-props", &cs.propList, sortOpts{
			n:        len(d.Properties),
			id:       func(i int) string { return d.Properties[i].ID },
			label:    func(i int) string { return "Özellik: " + d.Properties[i].Name },
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
func tagField(c *ui.Context, key string, tags *[]string, placeholder string) {
	before := slices.Clone(*tags)
	ryugo.TagInput(c.Key(key), tags, ryugo.TagInputOptions{Placeholder: placeholder, Size: ryugo.Sm, Label: "Seçenek ekle"})
	if !slices.Equal(before, *tags) {
		*tags = cleanOptions(*tags)
	}
}

// csPropEditor changes a property of the draft.
func (a *app) csPropEditor(c *ui.Context, p *Property) {
	cs := &a.cs
	ui.Column(c.Key("cs-prop-edit:" + p.ID)).FillWidth().Gap(10).Padding(12).Children(func() {
		ryugo.Field(c, ryugo.FieldOptions{Label: "Ad"}, func() {
			ryugo.Input(c.Key("cs-prop-name"), &p.Name, ryugo.InputOptions{Label: "Özelliğin adı", Size: ryugo.Sm})
		})
		if p.Type == PropChoice {
			ryugo.Field(c, ryugo.FieldOptions{Label: "Seçenekler", Help: "Sırası panonun sütunları olur."}, func() {
				tagField(c, "cs-prop-opts", &p.Options, "Seçenek yaz, ↵")
			})
		}
		if ryugo.Button(c.Key("cs-prop-del"), "Özelliği sil", ryugo.ButtonOptions{Color: ryugo.From(c).Colors.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
			id := p.ID
			cs.draft.Properties = slices.DeleteFunc(cs.draft.Properties, func(q Property) bool { return q.ID == id })
			cs.draft.normalize()
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
			if ryugo.Button(c.Key("cs-add-open"), "Özellik ekle", ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconPlus}).Clicked() {
				cs.adding = true
			}
		})
		return
	}
	ui.Column(c).Gap(10).Padding(12).Radius(12).Border(1, cl.Hairline).Children(func() {
		ui.Text(c, "Özellik ekle").FontSize(ryugo.FsMd).FontWeight(600).TextColor(cl.Text)
		ryugo.Input(c.Key("cs-new-name"), &cs.newName, ryugo.InputOptions{Label: "Yeni özelliğin adı", Placeholder: "Ad yaz; başka koleksiyonlardakiler önerilir", Size: ryugo.Sm})
		// the properties of the other collections, as the name is typed
		if sugg := a.sharedProps(cs.newName); len(sugg) > 0 {
			ui.Column(c).Gap(2).Children(func() {
				for _, sp := range sugg {
					b := ui.Row(c.Key("cs-sugg:"+sp.from+":"+sp.p.ID)).Height(32).Gap(8).Padding(0, 10).Radius(8).AlignItems(ui.Center).
						Label("Öneri: " + sp.p.Name).Role(ui.RoleButton)
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
							p.ID = slug(p.Name) + "-2"
						}
						p.Options = slices.Clone(p.Options)
						d.Properties = append(d.Properties, p)
						d.normalize()
						cs.newName, cs.adding = "", false
					}
				}
			})
		}
		var names []string
		for _, t := range PropTypes {
			names = append(names, t.Name)
		}
		ryugo.Segmented(c.Key("cs-new-type"), &cs.newType, names, ryugo.SegmentedOptions{Label: "Yeni özelliğin türü", Fill: true, Size: ryugo.Sm})
		if PropTypes[cs.newType].Type == PropChoice {
			tagField(c, "cs-new-opts", &cs.newTags, "Seçenek yaz, ↵: Yapılacak, Sürüyor, Bitti")
		}
		ui.Row(c).Gap(8).Children(func() { // as wide as their labels
			if ryugo.Button(c.Key("cs-new-add"), "Ekle", ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconPlus, Disabled: strings.TrimSpace(cs.newName) == ""}).Clicked() {
				a.addDraftProp()
			}
			if ryugo.Button(c.Key("cs-new-cancel"), "Vazgeç", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
				cs.adding, cs.newName, cs.newTags = false, "", nil
			}
		})
	})
}

func (a *app) addDraftProp() {
	cs := &a.cs
	d := &cs.draft
	if _, err := d.addProperty(cs.newName, PropTypes[cs.newType].Type, cs.newTags); err != nil {
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

var viewTypes = []struct{ Type, Name string }{{"list", "Liste"}, {"table", "Tablo"}, {"board", "Pano"}, {"calendar", "Takvim"}}

func (a *app) csViews(c *ui.Context) {
	cs := &a.cs
	d := &cs.draft
	cl := ryugo.From(c).Colors
	ui.Text(c, "Sayfaları liste, tablo ya da pano olarak göster; neyin görüneceğini her görünümde seç. Sürükleyerek sırala.").FontSize(ryugo.FsSm).TextColor(cl.Muted)
	a.sortList(c, "cs-views", &cs.viewList, sortOpts{
		n:        len(d.Views),
		id:       func(i int) string { return d.Views[i].ID },
		label:    func(i int) string { return "Görünüm: " + d.Views[i].Label },
		selected: cs.view,
		content: func(i int) {
			v := d.Views[i]
			ui.Text(c, v.Label).FontSize(ryugo.FsMd).FontWeight(600).TextColor(cl.Text).MaxLines(1).Shrink(0)
			for _, vt := range viewTypes {
				if vt.Type == v.Type {
					ui.Text(c, "· "+vt.Name).FontSize(ryugo.FsSm).TextColor(cl.Muted).MaxLines(1).Shrink(0)
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
		ui.Text(c, "Pano için bir Seçim özelliği gerekir.").FontSize(ryugo.FsSm).TextColor(cl.Muted)
	}
}

// csAddView is the menu adding a view to the draft.
func (a *app) csAddView(c *ui.Context, d *Collection, hasChoice bool) {
	cs := &a.cs
	ryugo.MenuButton(c.Key("cs-add-view"), "Görünüm ekle", ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconPlus}, func(m *ryugo.MenuList) {
		for _, vt := range viewTypes {
			if vt.Type == "board" && !hasChoice {
				continue
			}
			if m.Item(vt.Name, ryugo.ItemOptions{}) {
				id := vt.Type
				for n := 2; slices.ContainsFunc(d.Views, func(v View) bool { return v.ID == id }); n++ {
					id = vt.Type + "-" + itoa(n)
				}
				v := View{ID: id, Label: vt.Name, Type: vt.Type, Sort: "title"}
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
		ryugo.Field(c, ryugo.FieldOptions{Label: "Ad"}, func() {
			ryugo.Input(c.Key("cs-view-name"), &v.Label, ryugo.InputOptions{Label: "Görünümün adı", Size: ryugo.Sm})
		})
		switch v.Type {
		case "table":
			ryugo.Field(c, ryugo.FieldOptions{Label: "Sütunlar", Help: "Başlık hep ilk sütun."}, func() {
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
		case "calendar":
			ryugo.Field(c, ryugo.FieldOptions{Label: "Hangi güne konur", Help: "Sayfalar takvimde bu tarihin gününde görünür."}, func() {
				opts := []ryugo.Option[string]{{Value: "created", Label: "Oluşturulma günü"}, {Value: "modified", Label: "Değiştirilme günü"}}
				for _, p := range d.Properties {
					if p.Type == PropDate {
						opts = append(opts, ryugo.Option[string]{Value: "prop:" + p.ID, Label: p.Name})
					}
				}
				f := v.DateField
				if f == "" {
					f = "created"
				}
				ryugo.Select(c.Key("cs-datefield"), &f, opts, ryugo.SelectOptions{Label: "Tarih", Size: ryugo.Sm})
				v.DateField = f
			})
		case "board":
			var opts []ryugo.Option[string]
			for _, p := range d.Properties {
				if p.Type == PropChoice {
					opts = append(opts, ryugo.Option[string]{Value: p.ID, Label: p.Name})
				}
			}
			ryugo.Field(c, ryugo.FieldOptions{Label: "Sütunlar şuna göre"}, func() {
				ryugo.Select(c.Key("cs-group"), &v.GroupBy, opts, ryugo.SelectOptions{Label: "Gruplanan özellik", Size: ryugo.Sm})
			})
			fallthrough
		default: // list and board: the properties shown on a row or card
			ryugo.Field(c, ryugo.FieldOptions{Label: map[bool]string{true: "Kartlarda", false: "Satırlarda"}[v.Type == "board"]}, func() {
				ui.Column(c).Gap(6).Children(func() {
					show := !v.NoExcerpt
					ryugo.Checkbox(c.Key("cs-excerpt"), &show, "Metnin başı", ryugo.CheckOptions{})
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
		ui.Row(c).Children(func() { // as wide as its label
			if len(d.Views) > 1 && v.Type != "calendar" && ryugo.Button(c.Key("cs-view-del"), "Görünümü sil", ryugo.ButtonOptions{Color: ryugo.From(c).Colors.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
				id := v.ID
				d.Views = slices.DeleteFunc(d.Views, func(x View) bool { return x.ID == id })
				cs.view = ""
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
	return append(out, colChoice{"parent", "Üst sayfa"}, colChoice{"created", "Oluşturulma"}, colChoice{"modified", "Değiştirilme"})
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
	ryugo.Switch(c.Key("cs-subpages"), &d.SubPages, ryugo.SwitchOptions{Label: "Alt sayfalar", Description: "Bir sayfanın altına sayfa eklenebilir."})
	ryugo.Field(c, ryugo.FieldOptions{Label: "Bir sayfaya ne denir?", Help: "“Yeni sayfa” yerine “Yeni Tarif” gibi."}, func() {
		ryugo.Input(c.Key("cs-noun"), &d.PageNoun, ryugo.InputOptions{Label: "Sayfanın adı", Placeholder: "sayfa"})
	})
	ryugo.Field(c, ryugo.FieldOptions{Label: "Yeni sayfa şablonu", Help: "Yeni bir sayfa bu Markdown ile başlar."}, func() {
		ryugo.Textarea(c.Key("cs-template"), &d.Template, ryugo.TextareaOptions{Label: "Şablon", Placeholder: "## Notlar\n\n- [ ] ", Rows: 6, MaxRows: 14, Size: ryugo.Sm})
	})
}

func itoa(n int) string {
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if s == "" {
		return "0"
	}
	return s
}
