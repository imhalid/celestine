package main

import (
	"cmp"
	"fmt"
	"maps"
	"path/filepath"
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
	a.coll, a.collName, a.search, a.trashOpen, a.tag, a.tasksOpen, a.searchOpen, a.settingsOpen = c, c.Name, "", false, "", false, false, false
	a.statsScreen, a.profileScreen = false, false
	a.bulkClear()
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
		a.scrolled(c, "coll-scroll:"+coll.Dir, nil, ui.Scroll(c.Key("coll-scroll:"+coll.Dir)).Grow(1), func() {
			if a.topSpace > 0 { // below the title bar, which lies over the top
				ui.Box(c.Key("coll-top")).FillWidth().Height(a.topSpace)
			}
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, 32).Children(func() {
					ui.Column(c).FillWidth().MaxWidth(viewMax).Gap(10).Children(func() {
						a.bannerView(c, coll)
						// its icon and the capsule that opens its settings, as Thymer's
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, collGlyph(coll)).Width(20).Height(20).TextColor(collTint(c, coll))
							if ryugo.Button(c.Key("coll-settings-btn"), "Koleksiyon ayarları", ryugo.ButtonOptions{Icon: iconSettings, Size: ryugo.Sm}).Clicked() {
								a.openCollSettings(coll, 0)
							}
						})
						c.Key("coll-name:" + coll.Dir)
						name := ui.TextInputBase(c, &a.collName).Label(T("Koleksiyon adı")).FontSize(32).FontWeight(720).Padding(2, 0, 6, 0)
						a.fieldMenu(name, &a.collName, fieldOpts{oneLine: true, more: func() []menuItem { return a.collectionMenu(coll) }, changed: func() {
							if err := coll.Rename(cmp.Or(strings.TrimSpace(a.collName), "Adsız koleksiyon")); err != nil {
								a.saveErr = err.Error()
							}
						}})
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
						ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Wrap().Children(func() { // wraps on a small window instead of squeezing the search
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
							if n := len(v.Filters); n > 0 { // the view's filters, which its settings change
								label := Tf("%d filters", n)
								if n == 1 {
									label = T("1 filter")
								}
								if ryugo.Button(c.Key("view-filters"), label, ryugo.ButtonOptions{Icon: iconFilter, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
									a.editViewFilters(coll, v)
								}
							}
							a.fieldMenu(ryugo.Input(c.Key("search:"+coll.Dir), &a.search, ryugo.InputOptions{Label: "Ara", Placeholder: "Bu koleksiyonda ara…", PrefixIcon: iconSearch, Clearable: true, Size: ryugo.Sm}), &a.search, fieldOpts{oneLine: true}).Width(220).MinWidth(140).Shrink(1)
							if coll.Journal { // a random day, and today's page
								if ryugo.Button(c.Key("random-day"), "Rastgele gün", ryugo.ButtonOptions{Icon: iconDice, Size: ryugo.Sm}).Clicked() {
									a.openRandomDay()
								}
							}
							if v.Type == "list" || v.Type == "table" {
								if ryugo.Button(c.Key("bulk-on"), map[bool]string{true: "Seçimi bitir", false: "Seç"}[a.bulk.on], ryugo.ButtonOptions{Icon: iconTask, Size: ryugo.Sm}).Clicked() {
									if a.bulk.on {
										a.bulkClear()
									} else {
										a.bulk.on = true
									}
								}
							}
							label := newPageLabel(coll)
							if coll.Journal {
								label = "Bugünün sayfası"
							}
							if coll.Dynamic() {
								return // it has no pages to make
							}
							if ryugo.Button(c, label, ryugo.ButtonOptions{Icon: iconPlus, Color: ryugo.From(c).Colors.Accent, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
								a.newPage(coll, nil)
							}
							if !coll.Journal && len(coll.AllTemplates()) > 0 { // pages from the other templates
								if ryugo.Button(c.Key("tpl-menu"), T("Şablondan yeni sayfa"), ryugo.ButtonOptions{Icon: iconChevronDown, IconOnly: true, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
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
						if len(v.Filters) > 0 { // the view's own conditions
							pages = slices.DeleteFunc(pages, func(p *Page) bool { return !v.Match(coll, p) })
						}
						sortPages(pages, v)
						if v.Type == "board" || v.Type == "calendar" {
							a.bulkClear()
						}
						a.bulkBar(c, coll, pages)
						func() {
							// A body per view: switching views drops the old one at once and
							// fades the new one in; within a view, rows come, go and move one by one.
							c.Key("body:" + coll.Dir + ":" + v.ID)
							ui.Column(c).Gap(2).Padding(4, 0, 40, 0).Transition(viewSwap).Children(func() {
								switch {
								case v.Type == "calendar": // a month shows even with nothing in it
									a.calendarView(c, coll, v, pages)
								case len(pages) == 0 && len(v.Filters) > 0 && a.search == "":
									a.emptyState(c, iconFilter, T("No page meets this view's filters"), T("The view shows only the pages that meet its filters."), T("Edit filters"), func() { a.editViewFilters(coll, v) })
								case len(pages) == 0 && a.search != "":
									a.emptyState(c, iconSearch, "Eşleşen sayfa yok", "“"+a.search+"” bu koleksiyonda bir sayfayla eşleşmiyor.", "Aramayı temizle", func() { a.search = "" })
								case len(pages) == 0 && coll.Dynamic():
									a.emptyState(c, iconSearch, "Sorguyla eşleşen sayfa yok", "Bu koleksiyon, sorgusunun bulduğu sayfaları gösterir: "+coll.Query, "", nil)
								case len(pages) == 0:
									a.emptyState(c, collGlyph(coll), "Bu koleksiyon boş", "Aşağıdaki satırla ilk sayfayı yaz.", "", nil)
								case v.Type == "board":
									a.boardView(c, coll, v, pages)
								case v.Type == "table":
									a.tableView(c, coll, v, pages)
								case v.Type == "gallery":
									a.galleryView(c, v, pages)
								default:
									a.listView(c, coll, v, pages)
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
		case "manual":
			return CompareManual(x, y)
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
		if v.Desc && v.Sort != "manual" { // the own order is as it was dragged, never the other way
			return d > 0
		}
		return d < 0
	})
}

// compareValues orders two values of property pr: a choice by its
// options, a number as a number, the rest as text; no value last.
func compareValues(pr *Property, a, b string) int { return CompareValues(pr, a, b) }

// ---- the list ----

// listView shows the pages under headings of when they changed (or were made,
// as the view is sorted), each with the start of its text; or, grouped by a
// choice, under its options (listGroups).
func (a *app) listView(c *ui.Context, coll *Collection, v *View, pages []*Page) {
	if pr := coll.Prop(v.GroupBy); pr != nil && pr.Type == PropChoice {
		a.listGroups(c, coll, v, pr, pages)
		return
	}
	if v.Sort == "manual" && !coll.Dynamic() {
		a.manualList(c, coll, v, pages)
		return
	}
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
		a.listRow(c, v, p, now)
	}
}

// listGroups shows the pages under the options of choice pr, in the options' order, those with none last; each
// group's heading folds it (kept in the view, as a board's columns are), and an empty group is not shown.
func (a *app) listGroups(c *ui.Context, coll *Collection, v *View, pr *Property, pages []*Page) {
	t := c.Theme()
	now := a.clock()
	for _, s := range append(slices.Clone(pr.Options), "") {
		var group []*Page
		for _, p := range pages {
			if p.Value(pr) == s {
				group = append(group, p)
			}
		}
		if len(group) == 0 {
			continue
		}
		title := s
		if s == "" {
			title = Tf("No %s", pr.Name)
		}
		folded := slices.Contains(v.Collapsed, s)
		c.Key("lgroup:" + s)
		head := ui.Row(c).Gap(7).AlignItems(ui.Center).Padding(14, 8, 4, 8).Radius(rad(6)).Transition(rowEnter).
			Label(map[bool]string{true: T("Expand: "), false: T("Collapse: ")}[folded] + title).Role(ui.RoleButton).Cursor(ui.CursorPointer)
		head.Children(func() {
			glyph := iconChevronDown
			if folded {
				glyph = iconChevron
			}
			ui.Icon(c, glyph).Width(12).Height(12).TextColor(t.TextMuted)
			ui.Box(c).Size(8, 8).Radius(rad(3)).Background(columnTone(c, s))
			ui.Text(c, title).FontSize(12).FontWeight(650).TextColor(t.TextMuted)
			ui.Text(c, strconv.Itoa(len(group))).FontSize(12).TextColor(t.TextMuted.Alpha(0.7))
		})
		if head.Clicked() {
			a.foldColumn(coll, v, s, !folded)
		}
		if folded {
			continue
		}
		for _, p := range group {
			a.listRow(c, v, p, now)
		}
	}
}

// manualList is a list in the collection's own order: its rows are dragged into place (Ryugo's Sortable), and the
// order is kept in the pages (Collection.SetManual), the rows a search or a filter hides keeping theirs.
func (a *app) manualList(c *ui.Context, coll *Collection, v *View, pages []*Page) {
	t := c.Theme()
	key := coll.Dir + "|" + v.ID
	if a.sortLists == nil {
		a.sortLists = map[string]*manualState{}
	}
	ms := a.sortLists[key]
	if ms == nil {
		ms = &manualState{list: new(ui.ListState)}
		a.sortLists[key] = ms
	}
	// Sortable moves a row in the list it was given when the row was let go, a frame later: the list is kept here
	// across frames, and taken anew when the pages shown are others (a page made, a search)
	if !samePages(ms.items, pages) {
		ms.items = slices.Clone(pages)
	}
	items := &ms.items
	ryugo.Sortable(c.Key("manual:"+key), ms.list, items, ryugo.SortableOptions{
		Label: func(i int) string { return T("Sırala: ") + (*items)[i].Title },
		Key:   func(i int) any { return (*items)[i] },
	}, func(p *Page) {
		// the whole row is what is dragged: nothing in it takes the pointer but its arrow, which opens the page (as
		// the keys do) and has its menu
		row := ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(4, 6)
		row.Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, p.Title).FontSize(15).FontWeight(620).MaxLines(1).Grow(1)
					a.fieldChips(c, p, v)
				})
				if ex := p.Excerpt(); ex != "" && !v.NoExcerpt {
					ui.Text(c, ex).FontSize(13.5).TextColor(t.TextMuted).MaxLines(1)
				}
			})
			open := ryugo.Button(c.Key("open:"+p.Dir), T("Aç: ")+p.Title, ryugo.ButtonOptions{Icon: iconChevron, IconOnly: true, Variant: ryugo.Soft, Size: ryugo.Sm})
			a.contextOn(open, func() []menuItem { return a.pageMenu(p, false) })
			if open.Clicked() {
				a.open(p)
			}
		})
		a.kbMark(row, "p:"+p.Dir, true, func() { a.open(p) })
	})
	if !slices.Equal(*items, pages) { // a row was moved
		if err := coll.SetManual(*items); err != nil {
			a.saveErr = err.Error()
		}
	}
}

// manualState is a list in its own order across frames: Ryugo's state of it, and its rows as last shown or moved.
type manualState struct {
	list  *ui.ListState
	items []*Page
}

// samePages is whether a and b hold the same pages, in any order.
func samePages(a, b []*Page) bool {
	if len(a) != len(b) {
		return false
	}
	in := make(map[*Page]bool, len(a))
	for _, p := range a {
		in[p] = true
	}
	for _, p := range b {
		if !in[p] {
			return false
		}
	}
	return true
}

// listRow is page p's row in a list: its title, the properties the view shows, when it changed, the start of its
// text and its cover.
func (a *app) listRow(c *ui.Context, v *View, p *Page, now time.Time) {
	t := c.Theme()
	c.Key("list:" + p.Dir)
	row := ui.Row(c).Padding(8, 10).AlignItems(ui.Center).Radius(rad(8)).Label(T("Aç: ") + p.Title).Role(ui.RoleButton).Transition(rowEnter)
	a.contextOn(row, func() []menuItem { return a.pageMenu(p, false) })
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	} else {
		row.Background(t.SurfaceHover.Alpha(0))
	}
	if a.bulk.sel[p] {
		row.Background(ryugo.From(c).Colors.Accent.Alpha(0.12))
	}
	row.Children(func() {
		a.bulkMark(c, p)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, p.Title).FontSize(15).FontWeight(620).MaxLines(1).Grow(1)
				a.fieldChips(c, p, v)
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
		if p.ImageCover() {
			ui.Box(c.Key("thumb-gap")).Width(12).Height(1).Shrink(0)
			a.coverThumb(c, p, 72, 52, 8)
		}
	})
	a.kbMark(row, "p:"+p.Dir, true, func() { a.open(p) })
	if a.bulkRowClicked(p, row.Clicked()) {
		// chosen, not opened
	} else if row.Clicked() {
		a.open(p)
	}
}

// galleryView shows the pages as cards in rows that wrap: each its picture across the top (its cover, or else the
// first picture in its text, or else a field of a color its title gives, with its first letter), then its title,
// the start of its text and the properties the view shows.
func (a *app) galleryView(c *ui.Context, v *View, pages []*Page) {
	t := c.Theme()
	cl := ryugo.From(c).Colors
	ui.Row(c.Key("gallery")).FillWidth().Wrap().Gap(14).Padding(6, 0, 0, 0).Children(func() {
		for _, p := range pages {
			c.Key("gal:" + p.Dir)
			card := ui.Column(c).Width(galleryCard).Gap(8).Padding(8).Radius(rad(12)).Border(1, cl.Border).
				Label(T("Aç: ") + p.Title).Role(ui.RoleButton).Cursor(ui.CursorPointer).Transition(rowEnter)
			if card.Hovered() {
				card.Background(t.SurfaceHover)
			} else {
				card.Background(t.SurfaceHover.Alpha(0))
			}
			a.contextOn(card, func() []menuItem { return a.pageMenu(p, false) })
			card.Children(func() {
				a.galleryPicture(c, p, galleryCard-16, 132)
				ui.Text(c, p.Title).FontSize(14.5).FontWeight(620).MaxLines(2).Padding(0, 2)
				if ex := p.Excerpt(); ex != "" && !v.NoExcerpt {
					ui.Text(c, ex).FontSize(12.5).TextColor(t.TextMuted).MaxLines(3).Padding(0, 2)
				}
				if len(v.Fields) > 0 {
					ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Padding(0, 2).Children(func() { a.fieldChips(c, p, v) })
				}
			})
			a.kbMark(card, "g:"+p.Dir, true, func() { a.open(p) })
			if card.Clicked() {
				a.open(p)
			}
		}
	})
}

// galleryCard is the width of a gallery's card.
const galleryCard = 232

// galleryPicture is the top of p's card in a gallery, w by h: its cover, the first picture in its text, or a field
// of its title's color with its first letter.
func (a *app) galleryPicture(c *ui.Context, p *Page, w, h float32) {
	if p.ImageCover() {
		if src := a.loadImage(p, filepath.Join(p.Dir, p.Cover)); src != nil {
			a.coverThumb(c, p, w, h, 8)
			return
		}
	}
	if file := imageFile(p, p.FirstImage()); file != "" {
		if src := a.loadImage(p, file); src != nil {
			ui.Box(c.Key("galpic:" + p.Dir)).Width(w).Height(h).Radius(rad(8)).Clip().Shrink(0).
				Draw(func(pt *ui.Painter, r ui.Rect) { pt.Clip(r, 8, func() { pt.Image(src, r, ui.Cover) }) })
			return
		}
	}
	tone := columnTone(c, p.Title)
	letter := "·"
	if r := []rune(strings.TrimSpace(p.Title)); len(r) > 0 {
		letter = strings.ToUpper(string(r[0]))
	}
	ui.Column(c.Key("galtone:" + p.Dir)).Width(w).Height(h).Radius(rad(8)).Background(tone.Alpha(0.18)).Shrink(0).
		AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Text(c, letter).FontSize(40).FontWeight(700).TextColor(tone.Alpha(0.75))
	})
}

// fieldChips are the values of the properties view v shows on page p's
// row or card: a choice as a badge, tags a badge each, the rest as muted text (a page as its title).
func (a *app) fieldChips(c *ui.Context, p *Page, v *View) {
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
		switch pr.Type {
		case PropChoice:
			ryugo.Badge(c.Key("chip:"+id), val, choiceTone(pr, val))
		case PropTags:
			for _, tag := range SplitTags(val) {
				ryugo.Badge(c.Key("chip:"+id+":"+tag), tag, ryugo.ToneNeutral)
			}
		default:
			ui.Text(c.Key("chip:"+id), a.propLabel(p, pr)).FontSize(12.5).TextColor(t.TextMuted).MaxLines(1)
		}
	}
}

// ---- the table ----

type column struct {
	label, field string
	prop         *Property // the property it shows; a choice's value is a badge
	width        float32   // 0: the rest
	right        bool      // its values go to the end of the cell: numbers
	cell         func(p *Page, now time.Time) string
}

// tableColumns are the table's columns: the title, the collection's
// properties, then where it is and when.
func tableColumns(coll *Collection, v *View) []column {
	cols := []column{{label: "Başlık", field: "title", cell: func(p *Page, _ time.Time) string { return p.Title }}}
	byKey := map[string]column{
		"parent":   {label: "Üst sayfa", width: 150, cell: func(p *Page, _ time.Time) string { return Parents(p) }},
		"created":  {label: "Oluşturulma", field: "created", width: 140, cell: func(p *Page, _ time.Time) string { return shortDate(p.Created) }},
		"modified": {label: "Değiştirilme", field: "modified", width: 130, cell: func(p *Page, now time.Time) string { return ago(p.Modified, now) }},
	}
	for i := range coll.Properties {
		pr := &coll.Properties[i]
		byKey["prop:"+pr.ID] = column{label: pr.Name, field: "prop:" + pr.ID, width: 140, prop: pr, right: pr.Type == PropNumber,
			cell: func(p *Page, _ time.Time) string { return p.Value(pr) }}
	}
	for _, k := range viewColumns(coll, v) { // the view's, in its order
		if col, ok := byKey[k]; ok {
			cols = append(cols, col)
		}
	}
	return cols
}

// tablePageSize is how many rows the table shows at a time, with pages under
// it: the rows are built as they show, and the page around the table scrolls as
// a whole, so the table is as high as its rows.
const tablePageSize = 50

// tableTitleMin is the narrowest the title's column goes.
const tableTitleMin = 200

// tableUI is a table's own state, kept with the view it is of: Ryugo's (the sort
// it shows, the columns' order and widths, the rows chosen, the page), and the
// sort as the view has it, to tell which of the two the user changed.
type tableUI struct {
	st      ryugo.TableState
	applied []ryugo.SortKey
	layout  tableLayout // the columns as last given to the table or taken from it (takeLayout)
	dirtyAt time.Time   // when the user last changed them, not yet saved; zero when saved
}

// tableLayout is how a table's columns are: their widths, order and those hidden.
type tableLayout struct {
	widths map[string]float32
	order  []string
	hidden []string
}

func (l tableLayout) equal(o tableLayout) bool {
	return maps.Equal(l.widths, o.widths) && slices.Equal(l.order, o.order) && slices.Equal(l.hidden, o.hidden)
}

// layoutOf is the layout of the table's state, the hidden columns sorted.
func layoutOf(st *ryugo.TableState) tableLayout {
	l := tableLayout{widths: maps.Clone(st.Widths), order: slices.Clone(st.Order)}
	for id, h := range st.Hidden {
		if h {
			l.hidden = append(l.hidden, id)
		}
	}
	slices.Sort(l.hidden)
	return l
}

// takeLayout keeps the columns as the user leaves them, as takeSort does the sort: a width dragged, a column
// moved or hidden goes into the view, and is saved once it has been still a moment (not each frame of a drag);
// a view changed elsewhere (read again, another pane) is given to the table.
func (a *app) takeLayout(c *ui.Context, coll *Collection, v *View, tu *tableUI) {
	now := c.Now()
	got := layoutOf(&tu.st)
	switch {
	case !got.equal(tu.layout): // the user changed them
		v.Widths, v.Order, v.Hidden = got.widths, got.order, got.hidden
		tu.layout, tu.dirtyAt = got, now
		c.After(layoutSettle + 50*time.Millisecond)
	case !tu.dirtyAt.IsZero():
		if now.Sub(tu.dirtyAt) < layoutSettle {
			c.After(layoutSettle - now.Sub(tu.dirtyAt) + 20*time.Millisecond)
			return
		}
		tu.dirtyAt = time.Time{}
		if err := coll.Save(); err != nil {
			a.saveErr = err.Error()
		}
	default:
		want := tableLayout{widths: v.Widths, order: v.Order, hidden: v.Hidden}
		if !want.equal(tu.layout) {
			tu.st.Widths, tu.st.Order = maps.Clone(v.Widths), slices.Clone(v.Order)
			tu.st.Hidden = nil
			for _, id := range v.Hidden {
				if tu.st.Hidden == nil {
					tu.st.Hidden = map[string]bool{}
				}
				tu.st.Hidden[id] = true
			}
			tu.layout = layoutOf(&tu.st)
		}
	}
}

// layoutSettle is how long the columns are still before what the user did to them is saved.
const layoutSettle = 400 * time.Millisecond

// tableLabels are the table's words.
var tableLabels = ryugo.TableLabels{
	Search: "Ara", Selected: "%d seçili", Clear: "Temizle",
	SelectRow: "Satırı seç", SelectAll: "Hepsini seç", Details: "Ayrıntı",
	NoRows: "Sayfa yok", LoadError: "Sayfalar okunamadı", TryAgain: "Yeniden dene",
	Pagination: ryugo.PaginationLabels{Group: "Sayfalar", Of: "/", Previous: "Önceki sayfa", Next: "Sonraki sayfa", Page: "Sayfa", Current: "geçerli"},
}

// tableOf is the state of the table of view v of collection coll.
func (a *app) tableOf(coll *Collection, v *View) *tableUI {
	if a.tables == nil {
		a.tables = map[string]*tableUI{}
	}
	key := coll.Dir + "|" + v.ID
	tu := a.tables[key]
	if tu == nil {
		tu = &tableUI{}
		a.tables[key] = tu
	}
	return tu
}

// viewSortKeys is the view's sort as Ryugo's table has it.
func viewSortKeys(v *View) []ryugo.SortKey {
	if v.Sort == "" {
		return nil
	}
	return []ryugo.SortKey{{Column: v.Sort, Desc: v.Desc}}
}

// takeSort tells which of the table's sort and the view's was changed since
// they were the same, and brings the other to it: a click on a header changes
// the table's, the collection's settings the view's.
func (a *app) takeSort(coll *Collection, v *View, tu *tableUI) {
	want := viewSortKeys(v)
	switch {
	case !slices.Equal(tu.st.Sort, tu.applied): // the user sorted
		if k := tu.st.Sort; len(k) > 0 {
			v.Sort, v.Desc = k[0].Column, k[0].Desc
		} else {
			v.Sort, v.Desc = "title", false // sorted by nothing: by title
		}
		if err := coll.Save(); err != nil {
			a.saveErr = err.Error()
		}
		tu.st.Sort = viewSortKeys(v)
	case !slices.Equal(want, tu.applied):
		tu.st.Sort = want
	}
	tu.applied = slices.Clone(tu.st.Sort)
}

// pageSortValue is the value a page is sorted by in column col: nil when it has none.
func pageSortValue(col column, p *Page) any {
	switch {
	case col.field == "title":
		return p.Title
	case col.field == "created":
		return p.Created
	case col.field == "modified":
		return p.Modified
	case col.prop != nil:
		v := p.Value(col.prop)
		if v == "" {
			return nil
		}
		switch col.prop.Type {
		case PropChoice:
			return float64(slices.Index(col.prop.Options, v))
		case PropNumber:
			f, _ := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
			return f
		}
		return v
	}
	return nil
}

// tableView is the pages as Ryugo's table: the header in its tint, rows divided
// by hairlines, a column sorted by a click on its header, moved by dragging it
// and resized by its edge, pages under it. A click on a row opens its page (or
// chooses it, while rows are chosen), a right click is the page's menu.
func (a *app) tableView(c *ui.Context, coll *Collection, v *View, pages []*Page) {
	t := c.Theme()
	now := a.clock()
	tu := a.tableOf(coll, v)
	a.takeSort(coll, v, tu)
	a.takeLayout(c, coll, v, tu)
	var cols []ryugo.Column[*Page]
	for i, col := range tableColumns(coll, v) {
		col, first := col, i == 0
		id := col.field
		if id == "" {
			id = col.label
		}
		rc := ryugo.Column[*Page]{ID: id, Title: col.label, Sortable: col.field != "", Width: col.width,
			Value: func(p *Page) any { return pageSortValue(col, p) },
			Cell: func(c *ui.Context, p *Page) {
				if col.prop != nil && col.prop.Type == PropChoice {
					if val := p.Value(col.prop); val != "" {
						ryugo.Badge(c, val, choiceTone(col.prop, val))
					}
					return
				}
				if col.prop != nil && col.prop.Type == PropTags {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Clip().Children(func() {
						for _, tag := range SplitTags(p.Value(col.prop)) {
							ryugo.Badge(c.Key(tag), tag, ryugo.ToneNeutral)
						}
					})
					return
				}
				text := col.cell(p, now)
				if col.prop != nil {
					text = a.propLabel(p, col.prop) // a page as its title
				}
				tx := ui.Text(c, text).FontSize(13.5).SingleLine().Ellipsis("…")
				if col.right {
					tx.FontFeatures("tnum")
				}
				if first {
					tx.FontWeight(560)
				} else {
					tx.TextColor(t.TextMuted)
				}
			}}
		if col.width == 0 {
			rc.MinWidth = tableTitleMin
		}
		if col.right {
			rc.Align = ui.End
		}
		cols = append(cols, rc)
	}
	// the rows chosen are the app's (a.bulk): the table shows them, and says what is chosen in it
	tu.st.Selected = nil
	if a.bulk.on && len(a.bulk.sel) > 0 {
		tu.st.Selected = map[any]bool{}
		for p := range a.bulk.sel {
			tu.st.Selected[p] = true
		}
	}
	head, row := ryugo.TableSize(ryugo.Md)
	rows := min(len(pages), tablePageSize)
	if left := len(pages) - tu.st.Page*tablePageSize; left < rows {
		rows = max(left, 0)
	}
	height := head + float32(rows)*row
	if len(pages) > tablePageSize {
		height += ryugo.PaginationHeight(c, ryugo.Md) + 10 // the pages under the table, and the gap
	}
	ryugo.Table(c.Key("table:"+coll.Dir+"|"+v.ID), &tu.st, cols, pages, ryugo.TableOptions[*Page]{
		Key: func(p *Page) any { return p }, Label: "Sayfalar", Height: height + 2,
		Selectable: a.bulk.on, HideSelectionBar: true, PageSize: tablePageSize, Labels: tableLabels,
		Pin: true, // the title stays as the table is scrolled sideways
		Open: func(p *Page) {
			if a.bulk.on {
				a.bulkToggle(p) // chosen, not opened; the table's own choice follows, or it would be taken back below
				if tu.st.Selected == nil {
					tu.st.Selected = map[any]bool{}
				}
				if a.bulk.sel[p] {
					tu.st.Selected[p] = true
				} else {
					delete(tu.st.Selected, p)
				}
				return
			}
			a.open(p)
		},
		Row: func(row ui.Element, p *Page) {
			a.contextOn(row, func() []menuItem { return a.pageMenu(p, false) })
			a.kbMark(row, "p:"+p.Dir, true, func() { a.open(p) }) // the keys move over the rows of the table, too
		},
	})
	if a.bulk.on { // what was chosen in the table, by its boxes
		a.bulk.sel = map[*Page]bool{}
		for k := range tu.st.Selected {
			if p, ok := k.(*Page); ok {
				a.bulk.sel[p] = true
			}
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
	ui.Column(c.Key("board-wrap")).FillWidth().Children(func() { // the fades at its edges lie over the scroll
		a.scrolledSideways(c, "board:"+coll.Dir+"|"+v.ID, a.boardScroll, ui.ScrollHorizontal(c), func() {
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
					look := cardLookOf(v)
					want := look.width
					if focus == id {
						want = look.focused
					}
					wrap.Width(wrap.AnimateTo("w", want, boardSpring))
					wrap.Children(func() {
						// its head: a dot of its color, name, count, and a fold; the name is the focus
						head := ui.Row(c).Gap(8).Padding(0, 4, 0, 6).AlignItems(ui.Center)
						head.Children(func() {
							name := ui.Row(c.Key("colname:"+s)).Gap(7).AlignItems(ui.Center).Padding(3, 6).Radius(rad(7)).Grow(1).MinWidth(0).
								Label("Odakla: " + title).Role(ui.RoleButton).Cursor(ui.CursorPointer)
							if name.Hovered() {
								name.Background(cl.Fill)
							}
							name.Children(func() {
								ui.Box(c).Size(8, 8).Radius(rad(3)).Background(tone)
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
									if look.cover > 0 {
										a.coverThumb(c, p, 0, look.cover, 8) // its cover, across the top
									}
									ui.Box(c).FillWidth().Height(3).Radius(rad(2)).Background(tone.Alpha(0.75)) // its column's color
									ui.Text(c, p.Title).FontSize(look.font).FontWeight(600).MaxLines(look.lines)
									full := focus == id || look.always
									if ex := p.Excerpt(); ex != "" && !v.NoExcerpt && full {
										ui.Text(c, ex).FontSize(12.5).TextColor(t.TextMuted).MaxLines(3)
									}
									if len(v.Fields) > 0 && full {
										ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Children(func() { a.fieldChips(c, p, v) })
									}
								}).Label("Kart: " + p.Title)
								a.contextOn(card, func() []menuItem { return a.boardCardMenu(c, coll, pr, p) })
								a.kbMark(card, "b:"+p.Dir, true, func() { a.open(p) })
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
		a.edgeFades(c, a.boardScroll)
	})
}

// cardLook is how large a board's cards are: its columns' width, and when one is the focus; the cover's height
// (none on the small ones); the title's size and lines; and whether every card shows what it says, not only the
// focused column's.
type cardLook struct {
	width, focused, cover, font float32
	lines                       int
	always                      bool
}

// cardLookOf is the look of view v's cards, by its CardSize.
func cardLookOf(v *View) cardLook {
	switch v.CardSize {
	case "small":
		return cardLook{width: 200, focused: 300, font: 13, lines: 1}
	case "large":
		return cardLook{width: 330, focused: 440, cover: 150, font: 15, lines: 3, always: true}
	}
	return cardLook{width: 250, focused: 360, cover: 84, font: 14, lines: 2}
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
	b := ui.Column(c.Key("strip:"+s)).Width(44).MinHeight(200).Gap(10).Padding(10, 0).Radius(rad(18)).AlignItems(ui.Center).
		Background(cl.Well).Border(1, cl.WellBorder).Label(T("Aç: ") + title).Role(ui.RoleButton).Shrink(0)
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
	if err := coll.Save(); err != nil {
		a.saveErr = err.Error()
	}
}

// newPageRow is the last row of a view: a new page in the collection.
func (a *app) newPageRow(c *ui.Context, coll *Collection) {
	t := c.Theme()
	row := ui.Row(c).Padding(10).Gap(8).Radius(rad(8)).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		// the plus is a mark of its own, in the middle of the line: as a character it sat on the text's baseline,
		// lower than the words
		ui.Icon(c, iconPlus).Width(14).Height(14).TextColor(t.TextMuted)
		ui.Text(c, newPageLabel(coll)).FontSize(13.5).TextColor(t.TextMuted)
	})
	if row.Clicked() {
		a.newPage(coll, nil)
	}
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
