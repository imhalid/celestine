package main

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The collection's page: its name, its views (a list grouped by date, a
// table), a search over its pages and a button for a new one. What view is
// shown and how it is sorted is kept in the collection's collection.json.

const viewMax = 920

// openCollection shows collection c's page, saving the page that was open.
func (a *app) openCollection(c *Collection) {
	a.flush()
	a.coll, a.collName, a.search, a.trashOpen, a.tag, a.tasksOpen, a.searchOpen, a.graphOpen = c, c.Name, "", false, "", false, false, false
	a.calMonth = time.Time{} // this month, until the calendar goes elsewhere
	c.Open = true
	a.visit()
}

func (a *app) collectionView(c *ui.Context) {
	t := c.Theme()
	coll := a.coll
	v := coll.View()
	// the whole page scrolls - its icon and name, its toolbar and its view - over
	// the window's width, its bar at the edge, as a page of text does
	ui.Column(c).Fill().Children(func() { // as tall as the room: the scroll in it has a height to scroll in
		ui.Scroll(c.Key("coll-scroll:" + coll.Dir)).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, 32).Children(func() {
					ui.Column(c).FillWidth().MaxWidth(viewMax).Gap(10).Children(func() {
						// its icon and the capsule that opens its settings, as Thymer's
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, collGlyph(coll)).Width(20).Height(20).TextColor(collTint(c, coll))
							b := ui.Row(c.Key("coll-settings-btn")).Gap(6).Padding(3, 10).Radius(999).AlignItems(ui.Center).
								Background(ryugo.From(c).Colors.Fill).Label("Koleksiyon ayarları").Role(ui.RoleButton)
							if b.Hovered() {
								b.Background(ryugo.From(c).Colors.FillSelected)
							}
							b.Children(func() {
								ui.Text(c, "Koleksiyon").FontSize(12).FontWeight(560).TextColor(t.TextMuted)
								ui.Icon(c, iconSettings).Width(13).Height(13).TextColor(t.TextMuted)
							})
							if b.Clicked() {
								a.openCollSettings(coll, 0)
							}
						})
						c.Key("coll-name:" + coll.Dir)
						name := ui.TextInputBase(c, &a.collName).Label("Koleksiyon adı").FontSize(32).FontWeight(720).Padding(2, 0, 6, 0)
						if name.Changed() {
							n := strings.TrimSpace(a.collName)
							if n == "" {
								n = "Adsız koleksiyon"
							}
							if err := coll.Rename(n); err != nil {
								a.saveErr = err.Error()
							}
						}
						if coll.Dynamic() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Margin(-4, 0, 4, 0).Children(func() {
								ui.Icon(c, iconSearch).Width(14).Height(14).TextColor(t.TextMuted)
								ui.Text(c, coll.Query).FontSize(13.5).Font(ryugo.FontMono).TextColor(t.TextMuted)
							})
						}
						if coll.Description != "" {
							ui.Text(c, coll.Description).FontSize(14).TextColor(t.TextMuted).Margin(-4, 0, 4, 0)
						}
						// The toolbar: the views, then the search and a new page.
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							// The views: Ryugo's segmented control, its pill sliding to the one chosen.
							sel := 0
							labels := make([]string, len(coll.Views))
							for i := range coll.Views {
								labels[i] = coll.Views[i].Label
								if &coll.Views[i] == v {
									sel = i
								}
							}
							was := sel
							ryugo.Segmented(c.Key("views:"+coll.Dir), &sel, labels, ryugo.SegmentedOptions{Label: "Görünüm", Size: ryugo.Sm})
							if sel != was {
								if err := coll.SetView(coll.Views[sel].ID); err != nil {
									a.saveErr = err.Error()
								}
							}
							ui.Spacer(c)
							ryugo.Input(c.Key("search:"+coll.Dir), &a.search, ryugo.InputOptions{Label: "Ara", Placeholder: "Bu koleksiyonda ara…", PrefixIcon: iconSearch, Clearable: true, Size: ryugo.Sm}).Width(220)
							if coll.Journal { // a random day, and today's page
								if ryugo.Button(c.Key("random-day"), "Rastgele gün", ryugo.ButtonOptions{Icon: iconDice, Size: ryugo.Sm}).Clicked() {
									a.openRandomDay()
								}
							}
							label := "Yeni " + pageNoun(coll)
							if coll.Journal {
								label = "Bugünün sayfası"
							}
							if coll.Dynamic() {
								return // it has no pages to make
							}
							if ryugo.Button(c, label, ryugo.ButtonOptions{Icon: iconPlus, Color: ryugo.From(c).Colors.Accent, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
								a.newPage(coll, nil)
							}
							if !coll.Journal && len(coll.allTemplates()) > 0 { // pages from the other templates
								if ryugo.Button(c.Key("tpl-menu"), "Şablondan yeni sayfa", ryugo.ButtonOptions{Icon: iconChevronDown, IconOnly: true, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
									ww, _ := c.Size()
									a.showContext(ww-280, 190, a.templateMenu(coll, nil))
								}
							}
						})
					})
				})
				// the board is wider than the rest of the page: the head and toolbar stay in the column, what is under them has the room
				maxW := float32(viewMax)
				if v.Type == "board" {
					maxW = 4000
				}
				ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, 32).Children(func() {
					ui.Column(c).FillWidth().MaxWidth(maxW).Children(func() {
						pages := a.filtered(coll)
						sortPages(pages, v)
						func() {
							// A body per view: switching views drops the old one at once and
							// fades the new one in; within a view, rows come, go and move one by one.
							c.Key("body:" + coll.Dir + ":" + v.ID)
							ui.Column(c).Gap(2).Padding(4, 0, 40, 0).Transition(viewSwap).Children(func() {
								switch {
								case v.Type == "calendar": // a month shows even with nothing in it
									a.calendarView(c, coll, v, pages)
								case len(pages) == 0 && a.search != "":
									ui.Text(c, "“"+a.search+"” ile eşleşen sayfa yok.").TextColor(t.TextMuted).Padding(16, 4)
								case len(pages) == 0 && coll.Dynamic():
									ui.Text(c, "Sorguyla eşleşen sayfa yok.").TextColor(t.TextMuted).Padding(16, 4)
								case len(pages) == 0:
									ui.Text(c, "Bu koleksiyonda henüz sayfa yok.").TextColor(t.TextMuted).Padding(16, 4)
								case v.Type == "board":
									a.boardView(c, coll, v, pages)
								case v.Type == "table":
									a.tableView(c, v, pages)
								default:
									a.listView(c, v, pages)
								}
								if !coll.Journal && !coll.Dynamic() { // its one new page is today's, above
									a.newPageRow(c, coll)
								}
							})
						}()
					})
				})
			})
		})
	})
}

// viewSwap is a view coming in: it fades in, a little lower first.
var viewSwap = ui.ElementTransition{Duration: 220 * time.Millisecond, Enter: &ui.Motion{Opacity: 0, Y: 8}}

// rowEnter is a row of a view sliding to its new place as the sort changes,
// fading in when a search lets it in, its hover fading.
var rowEnter = ui.ElementTransition{
	Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond}},
	Position:  true, Colors: true,
	Enter: &ui.Motion{Opacity: 0, Y: 6},
	Exit:  &ui.Motion{Opacity: 0},
}

// filtered is the collection's pages, those matching the search when there is one.
func (a *app) filtered(coll *Collection) []*Page {
	all := coll.All()
	if coll.Dynamic() { // the pages the query finds, wherever they are
		all = all[:0:0]
		for _, h := range a.searchPages(coll.Query, 0, a.links.stale) {
			all = append(all, h.page)
		}
	}
	q := strings.ToLower(strings.TrimSpace(a.search))
	if q == "" {
		return all
	}
	var out []*Page
	for _, p := range all {
		if strings.Contains(strings.ToLower(p.Title), q) || strings.Contains(strings.ToLower(p.Excerpt()), q) {
			out = append(out, p)
		}
	}
	return out
}

// sortPages sorts by the view's field; a tie goes by title.
func sortPages(ps []*Page, v *View) {
	less := func(x, y *Page) int {
		switch v.Sort {
		case "created":
			return x.Created.Compare(y.Created)
		case "modified":
			return x.Modified.Compare(y.Modified)
		}
		if id, ok := strings.CutPrefix(v.Sort, "prop:"); ok {
			if pr := x.Coll.Prop(id); pr != nil {
				return compareValues(pr, x.Value(pr), y.Value(pr))
			}
		}
		return strings.Compare(strings.ToLower(x.Title), strings.ToLower(y.Title))
	}
	sort.SliceStable(ps, func(i, j int) bool {
		d := less(ps[i], ps[j])
		if d == 0 {
			return strings.ToLower(ps[i].Title) < strings.ToLower(ps[j].Title)
		}
		if v.Desc {
			return d > 0
		}
		return d < 0
	})
}

// compareValues orders two values of property pr: a choice by its
// options, a number as a number, the rest as text; no value last.
func compareValues(pr *Property, a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	switch pr.Type {
	case PropChoice:
		return slices.Index(pr.Options, a) - slices.Index(pr.Options, b)
	case PropNumber:
		x, _ := strconv.ParseFloat(strings.ReplaceAll(a, ",", "."), 64)
		y, _ := strconv.ParseFloat(strings.ReplaceAll(b, ",", "."), 64)
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// ---- the list ----

// listView shows the pages under headings of when they changed (or were made,
// as the view is sorted), each with the start of its text.
func (a *app) listView(c *ui.Context, v *View, pages []*Page) {
	t := c.Theme()
	now := a.clock()
	last := ""
	seen := map[string]int{} // a group coming again (an order not by date) gets a key of its own
	for _, p := range pages {
		if v.Sort != "title" {
			when := p.Modified
			if v.Sort == "created" {
				when = p.Created
			}
			if g := dateGroup(when, now); g != last {
				last = g
				seen[g]++
				c.Key(fmt.Sprintf("group:%s:%d", g, seen[g]))
				ui.Text(c, g).FontSize(12).FontWeight(650).TextColor(t.TextMuted).Padding(14, 8, 4, 8).Transition(rowEnter)
			}
		}
		c.Key("list:" + p.Dir)
		row := ui.Column(c).Padding(8, 10).Gap(3).Radius(8).Label("Aç: " + p.Title).Role(ui.RoleButton).Transition(rowEnter)
		a.contextOn(row, func() []menuItem { return a.pageMenu(p, false) })
		if row.Hovered() {
			row.Background(t.SurfaceHover)
		} else {
			row.Background(t.SurfaceHover.Alpha(0))
		}
		row.Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, p.Title).FontSize(15).FontWeight(620).MaxLines(1).Grow(1)
				fieldChips(c, p, v)
				when := p.Modified
				if v.Sort == "created" {
					when = p.Created
				}
				ui.Text(c, ago(when, now)).FontSize(12).TextColor(t.TextMuted)
			})
			if ex := p.Excerpt(); ex != "" && !v.NoExcerpt {
				ui.Text(c, ex).FontSize(13.5).TextColor(t.TextMuted).MaxLines(2)
			}
		})
		if row.Clicked() {
			a.open(p)
		}
	}
}

// fieldChips are the values of the properties view v shows on page p's
// row or card: a choice as a badge, the rest as muted text.
func fieldChips(c *ui.Context, p *Page, v *View) {
	t := c.Theme()
	for _, id := range v.Fields {
		pr := p.Coll.Prop(id)
		if pr == nil {
			continue
		}
		val := p.Value(pr)
		if val == "" {
			continue
		}
		if pr.Type == PropChoice {
			ryugo.Badge(c.Key("chip:"+id), val, choiceTone(pr, val))
		} else {
			ui.Text(c.Key("chip:"+id), val).FontSize(12.5).TextColor(t.TextMuted).MaxLines(1)
		}
	}
}

// ---- the table ----

type column struct {
	label, field string
	prop         *Property // the property it shows; a choice's value is a badge
	width        float32   // 0: the rest
	cell         func(p *Page, now time.Time) string
}

// tableColumns are the table's columns: the title, the collection's
// properties, then where it is and when.
func tableColumns(coll *Collection, v *View) []column {
	cols := []column{{label: "Başlık", field: "title", cell: func(p *Page, _ time.Time) string { return p.Title }}}
	byKey := map[string]column{
		"parent":   {label: "Üst sayfa", width: 150, cell: func(p *Page, _ time.Time) string { return parents(p) }},
		"created":  {label: "Oluşturulma", field: "created", width: 140, cell: func(p *Page, _ time.Time) string { return shortDate(p.Created) }},
		"modified": {label: "Değiştirilme", field: "modified", width: 130, cell: func(p *Page, now time.Time) string { return ago(p.Modified, now) }},
	}
	for i := range coll.Properties {
		pr := &coll.Properties[i]
		byKey["prop:"+pr.ID] = column{label: pr.Name, field: "prop:" + pr.ID, width: 140, prop: pr,
			cell: func(p *Page, _ time.Time) string { return p.Value(pr) }}
	}
	for _, k := range viewColumns(coll, v) { // the view's, in its order
		if col, ok := byKey[k]; ok {
			cols = append(cols, col)
		}
	}
	return cols
}

func (a *app) tableView(c *ui.Context, v *View, pages []*Page) {
	t := c.Theme()
	now := a.clock()
	columns := tableColumns(a.coll, v)
	cellBox := func(col column) ui.Element {
		b := ui.Row(c).Padding(0, 10).AlignItems(ui.Center).Height(36)
		if col.width > 0 {
			b.Width(col.width)
		} else {
			b.Grow(1)
		}
		return b
	}
	// The header: a column of a field sorts by it, again the other way.
	ui.Row(c).Children(func() {
		for _, col := range columns {
			c.Key("head:" + col.label)
			h := cellBox(col)
			if col.field != "" {
				h.Label("Sırala: " + col.label).Role(ui.RoleButton)
				if h.Hovered() {
					h.Background(t.SurfaceHover)
				}
			}
			h.Children(func() {
				mark := ""
				if col.field != "" && col.field == v.Sort {
					mark = map[bool]string{true: "  ↓", false: "  ↑"}[v.Desc]
				}
				ui.Text(c, col.label+mark).FontSize(12).FontWeight(650).TextColor(t.TextMuted)
			})
			if col.field != "" && h.Clicked() {
				if err := a.coll.SetSort(v, col.field); err != nil {
					a.saveErr = err.Error()
				}
			}
		}
	})
	ui.Box(c).Height(1).Background(t.Border)
	for _, p := range pages {
		c.Key("row:" + p.Dir)
		row := ui.Row(c).Radius(6).Label("Aç: " + p.Title).Role(ui.RoleButton).Transition(rowEnter)
		a.contextOn(row, func() []menuItem { return a.pageMenu(p, false) })
		if row.Hovered() {
			row.Background(t.SurfaceHover)
		} else {
			row.Background(t.SurfaceHover.Alpha(0))
		}
		row.Children(func() {
			for i, col := range columns {
				cellBox(col).Children(func() {
					if col.prop != nil && col.prop.Type == PropChoice {
						if v := p.Value(col.prop); v != "" {
							ryugo.Badge(c, v, choiceTone(col.prop, v))
						}
						return
					}
					tx := ui.Text(c, col.cell(p, now)).FontSize(13.5).MaxLines(1)
					if i == 0 {
						tx.FontWeight(560)
					} else {
						tx.TextColor(t.TextMuted)
					}
				})
			}
		})
		c.Key("rule:" + p.Dir)
		ui.Box(c).Height(1).Background(withAlpha(t.Border, 0.5)).Transition(rowEnter)
		if row.Clicked() {
			a.open(p)
		}
	}
}

// ---- the board ----

// choiceTone is the tone of an option of a choice of a few: the first
// gray, the last done, the ones between under way; all gray past five.
func choiceTone(pr *Property, v string) ryugo.Tone {
	i := slices.Index(pr.Options, v)
	switch n := len(pr.Options); {
	case n > 5 || i <= 0:
		return ryugo.ToneNeutral
	case i == n-1:
		return ryugo.ToneDown
	}
	return ryugo.ToneWarn
}

// boardView shows the pages in a column per status, as Ryugo's board: a
// card dragged to another column takes its status, and its place among
// the column's cards; a click opens it.
func (a *app) boardView(c *ui.Context, coll *Collection, v *View, pages []*Page) {
	t := c.Theme()
	pr := coll.Prop(v.GroupBy)
	if pr == nil {
		return
	}
	shown := map[*Page]bool{}
	for _, p := range pages {
		shown[p] = true
	}
	// The columns: each its own color, made from its name; the column of
	// pages with no value, when it is empty, is a strip; the cards are their titles, quiet; the column asked for
	// is wider and shows what its cards say, the others as they were; too many for the room
	// scroll sideways instead of narrowing.
	cl := ryugo.From(c).Colors
	focus := a.boardFocus
	ui.ScrollHorizontal(c).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Stretch).Padding(0, 0, 8, 0).Children(func() {
			for _, s := range append([]string{""}, pr.Options...) {
				var col []*Page
				for _, p := range coll.Column(pr.ID, s) {
					if shown[p] {
						col = append(col, p)
					}
				}
				title := s
				if s == "" {
					title = pr.Name + " yok"
				}
				s := s
				id := coll.Dir + "|" + v.ID + "|" + s
				tone := columnTone(c, s)
				opened := a.boardOpened[id]
				if slices.Contains(v.Collapsed, s) || (s == "" && len(col) == 0 && !opened) { // the empty "none" column is a strip; the stages are drop places
					a.boardStrip(c, coll, v, s, title, len(col), id)
					continue
				}
				wrap := ui.Column(c.Key("colwrap:" + s)).Gap(6).Shrink(0)
				want := float32(250)
				if focus == id {
					want = 360
				}
				wrap.Width(wrap.AnimateTo("w", want, boardSpring))
				wrap.Children(func() {
					// its head: a dot of its color, name, count, and a fold; the name is the focus
					head := ui.Row(c).Gap(8).Padding(0, 4, 0, 6).AlignItems(ui.Center)
					head.Children(func() {
						name := ui.Row(c.Key("colname:"+s)).Gap(7).AlignItems(ui.Center).Padding(3, 6).Radius(7).Grow(1).MinWidth(0).
							Label("Odakla: " + title).Role(ui.RoleButton).Cursor(ui.CursorPointer)
						if name.Hovered() {
							name.Background(cl.Fill)
						}
						name.Children(func() {
							ui.Box(c).Size(8, 8).Radius(3).Background(tone)
							ui.Text(c, title).FontSize(ryugo.FsSm).FontWeight(620).TextColor(cl.Text2).MaxLines(1).Shrink(1)
							ui.Text(c, strconv.Itoa(len(col))).FontSize(12).TextColor(cl.Muted)
						})
						if name.Clicked() {
							if focus == id {
								a.boardFocus = ""
							} else {
								a.boardFocus = id
							}
						}
						if a.iconButton(c, iconFold, "Daralt: "+title) {
							a.foldColumn(coll, v, s, true)
							a.boardOpened[id] = false
						}
					})
					ryugo.DropArea(c.Key("col:"+s), ryugo.DropAreaOptions{Fill: true}, func() {
						for _, p := range col {
							p := p
							card := ryugo.DragCard(c.Key("card:"+p.Dir), p, func() {
								ui.Box(c).FillWidth().Height(3).Radius(2).Background(tone.Alpha(0.75)) // its column's color
								ui.Text(c, p.Title).FontSize(14).FontWeight(600).MaxLines(2)
								if ex := p.Excerpt(); ex != "" && !v.NoExcerpt && focus == id {
									ui.Text(c, ex).FontSize(12.5).TextColor(t.TextMuted).MaxLines(map[bool]int{true: 3, false: 2}[focus == id])
								}
								if len(v.Fields) > 0 && focus == id {
									ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Children(func() { fieldChips(c, p, v) })
								}
							}).Label("Kart: " + p.Title)
							a.contextOn(card, func() []menuItem { return a.boardCardMenu(c, coll, pr, p) })
							if card.Clicked() {
								a.open(p)
							}
						}
					}, func(p *Page, index int) {
						if err := coll.MoveOnBoard(p, pr.ID, s, index); err != nil {
							a.saveErr = err.Error()
						}
					}).MinHeight(180) // an empty one has room to drop on
				})
			}
		})
	})
}

// boardCardMenu is what a card offers: its page's menu, and the groups it
// can be moved to - the one it is in marked - which put it last in the one
// chosen.
func (a *app) boardCardMenu(c *ui.Context, coll *Collection, pr *Property, p *Page) []menuItem {
	items := a.pageMenu(p, false)
	move := a.into("Grubu değiştir", iconBoard, func() []menuItem {
		var out []menuItem
		for _, s := range append([]string{""}, pr.Options...) {
			s := s
			title := s
			if s == "" {
				title = pr.Name + " yok"
			}
			tone := columnTone(c, s)
			it := menuItem{title: title, dot: &tone, run: func() {
				if p.Value(pr) == s {
					return
				}
				if err := coll.MoveOnBoard(p, pr.ID, s, len(coll.Column(pr.ID, s))); err != nil {
					a.saveErr = err.Error()
				}
			}}
			if p.Value(pr) == s {
				it.hint = "şimdi burada"
			}
			out = append(out, it)
		}
		return out
	})
	return append(items[:1:1], append([]menuItem{move}, items[1:]...)...)
}

// boardSpring is a column coming to the width it is asked for.
var boardSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond, Bounce: 0.06}}

// columnTone is a color made of a column's name: the same name, the same
// color, among the palette's eight; the column without a value is plain.
func columnTone(c *ui.Context, name string) ui.Color {
	cl := ryugo.From(c).Colors
	if name == "" {
		return cl.Muted
	}
	var h uint32 = 2166136261
	for i := 0; i < len(name); i++ {
		h = (h ^ uint32(name[i])) * 16777619
	}
	return []ui.Color{cl.C1, cl.C2, cl.C3, cl.C4, cl.C5, cl.C6, cl.C7, cl.C8}[h%8]
}

func (a *app) boardStrip(c *ui.Context, coll *Collection, v *View, s, title string, n int, id string) {
	cl := ryugo.From(c).Colors
	b := ui.Column(c.Key("strip:"+s)).Width(44).MinHeight(200).Gap(10).Padding(10, 0).Radius(18).AlignItems(ui.Center).
		Background(cl.Well).Border(1, cl.WellBorder).Label("Aç: " + title).Role(ui.RoleButton).Shrink(0)
	if b.Hovered() {
		b.Background(cl.Fill)
	}
	b.Children(func() {
		ryugo.Badge(c, strconv.Itoa(n), ryugo.ToneNeutral)
		// its name, written down the strip
		ui.Box(c).Width(20).Height(150).Children(func() {
			ui.Text(c, title).FontSize(ryugo.FsSm).FontWeight(600).TextColor(cl.Text2).NoWrap().
				Absolute().Left(-65).Top(66).Width(150).Height(18).Rotate(90)
		})
	})
	if b.Clicked() {
		a.foldColumn(coll, v, s, false)
		a.boardOpened[id] = true // an empty one the user opened stays open
	}
}

// foldColumn folds or unfolds a board's column, and keeps it so.
func (a *app) foldColumn(coll *Collection, v *View, s string, fold bool) {
	v.Collapsed = slices.DeleteFunc(v.Collapsed, func(x string) bool { return x == s })
	if fold {
		v.Collapsed = append(v.Collapsed, s)
	}
	if err := coll.save(); err != nil {
		a.saveErr = err.Error()
	}
}

// newPageRow is the last row of a view: a new page in the collection.
func (a *app) newPageRow(c *ui.Context, coll *Collection) {
	t := c.Theme()
	row := ui.Row(c).Padding(10).Gap(8).Radius(8).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Text(c, "+  Yeni "+pageNoun(coll)).FontSize(13.5).TextColor(t.TextMuted)
	})
	if row.Clicked() {
		a.newPage(coll, nil)
	}
}

// parents is the titles of the pages above p, for a sub-page in a flat view.
func parents(p *Page) string {
	var parts []string
	for q := p.Parent; q != nil; q = q.Parent {
		parts = append([]string{q.Title}, parts...)
	}
	return strings.Join(parts, " › ")
}

// ---- time, in Turkish ----

var months = []string{"Oca", "Şub", "Mar", "Nis", "May", "Haz", "Tem", "Ağu", "Eyl", "Eki", "Kas", "Ara"}

func shortDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()-1], t.Year())
}

// ago is how long ago t was, as a list says it.
func ago(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "şimdi"
	case d < time.Hour:
		return fmt.Sprintf("%d dk önce", int(d.Minutes()))
	case d < 24*time.Hour && sameDay(t, now):
		return fmt.Sprintf("%d sa önce", int(d.Hours()))
	case sameDay(t, now.AddDate(0, 0, -1)):
		return "dün"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d gün önce", daysBetween(t, now))
	}
	return shortDate(t)
}

// dateGroup is the heading a list puts t under.
func dateGroup(t, now time.Time) string {
	switch {
	case sameDay(t, now) || t.After(now): // a clock ahead, a file touched elsewhere: today
		return "Bugün"
	case sameDay(t, now.AddDate(0, 0, -1)):
		return "Dün"
	case now.Sub(t) < 7*24*time.Hour:
		return "Son 7 gün"
	case now.Sub(t) < 30*24*time.Hour:
		return "Son 30 gün"
	case t.Year() == now.Year():
		return months[t.Month()-1] + " " + fmt.Sprint(t.Year())
	}
	return fmt.Sprint(t.Year())
}

// daysBetween is how many calendar days lie from a to b.
func daysBetween(a, b time.Time) int {
	day := func(t time.Time) time.Time { y, m, d := t.Date(); return time.Date(y, m, d, 12, 0, 0, 0, time.UTC) }
	return int(day(b).Sub(day(a)).Hours()+1) / 24
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

var iconDice = icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="3.5"/><circle cx="9" cy="9" r=".8"/><circle cx="15" cy="9" r=".8"/><circle cx="12" cy="12" r=".8"/><circle cx="9" cy="15" r=".8"/><circle cx="15" cy="15" r=".8"/>`)
