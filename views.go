package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The collection's page: its name, its views (a list grouped by date, a
// table), a search over its pages and a button for a new one. What view is
// shown and how it is sorted is kept in the collection's collection.json.

const viewMax = 920

// openCollection shows collection c's page, saving the page that was open.
func (a *app) openCollection(c *Collection) {
	a.flush()
	a.coll, a.collName, a.search = c, c.Name, ""
	c.Open = true
}

func (a *app) collectionView(c *ui.Context) {
	t := c.Theme()
	coll := a.coll
	v := coll.View()
	ui.Column(c).Fill().MaxWidth(viewMax).Gap(10).Children(func() {
		ui.Row(c).Children(func() {
			ui.Box(c).Padding(3, 10).Radius(999).Background(t.Surface).Children(func() {
				ui.Text(c, "Koleksiyon").FontSize(12).TextColor(t.TextMuted)
			})
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
		// The toolbar: the views, then the search and a new page.
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			// The views as a segmented control: a raised pill behind the one shown,
			// which slides to the one chosen on a spring, as Mizan's Segmented does.
			sel := 0
			for i := range coll.Views {
				if &coll.Views[i] == v {
					sel = i
				}
			}
			track := ui.Row(c).Padding(3).Radius(9).Background(t.Surface)
			px := track.AnimateTo("pill-x", float32(sel)*tabW, springPill)
			track.Draw(func(p *ui.Painter, r ui.Rect) {
				pill := ui.Rect{X: r.X + 3 + px, Y: r.Y + 3, W: tabW, H: r.H - 6}
				p.Shadow(pill, 6, 0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.12))
				p.Fill(pill, t.Background, 6)
			})
			track.Children(func() {
				for i := range coll.Views {
					vw := &coll.Views[i]
					c.Key("view:" + vw.ID)
					tab := ui.Row(c).Width(tabW).Height(28).Gap(6).Justify(ui.Center).AlignItems(ui.Center).Label("Görünüm: " + vw.Label).Role(ui.RoleButton)
					tone := t.TextMuted
					if vw == v {
						tone = t.Text
					}
					tab.Children(func() {
						ui.Icon(c, viewIcon(vw.Type)).Width(14).Height(14).TextColor(tone)
						ui.Text(c, vw.Label).FontSize(13).FontWeight(560).TextColor(tone)
					})
					if tab.Clicked() && vw != v {
						if err := coll.SetView(vw.ID); err != nil {
							a.saveErr = err.Error()
						}
					}
				}
			})
			ui.Spacer(c)
			ui.Box(c).Width(220).Padding(6, 10).Radius(8).Border(1, t.Border).Children(func() {
				ui.TextInputBase(c, &a.search).Label("Ara").Placeholder("Bu koleksiyonda ara…").FontSize(13)
			})
			if ui.Button(c, "+ Yeni sayfa").Label("Yeni sayfa").Clicked() {
				a.newPage(coll, nil)
			}
		})
		pages := a.filtered(coll)
		sortPages(pages, v)
		ui.Scroll(c).Grow(1).Children(func() {
			// A body per view: switching views drops the old one at once and
			// fades the new one in; within a view, rows come, go and move one by one.
			c.Key("body:" + coll.Dir + ":" + v.ID)
			ui.Column(c).Gap(2).Padding(4, 0, 40, 0).Transition(viewSwap).Children(func() {
				switch {
				case len(pages) == 0 && a.search != "":
					ui.Text(c, "“"+a.search+"” ile eşleşen sayfa yok.").TextColor(t.TextMuted).Padding(16, 4)
				case len(pages) == 0:
					ui.Text(c, "Bu koleksiyonda henüz sayfa yok.").TextColor(t.TextMuted).Padding(16, 4)
				case v.Type == "table":
					a.tableView(c, v, pages)
				default:
					a.listView(c, v, pages)
				}
				a.newPageRow(c, coll)
			})
		})
	})
}

func viewIcon(typ string) *ui.SVG {
	if typ == "table" {
		return iconTable
	}
	return iconList
}

const tabW = 96

var springPill = ui.Animation{Spring: ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.14}}

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

// ---- the list ----

// listView shows the pages under headings of when they changed (or were made,
// as the view is sorted), each with the start of its text.
func (a *app) listView(c *ui.Context, v *View, pages []*Page) {
	t := c.Theme()
	now := a.clock()
	last := ""
	for _, p := range pages {
		if v.Sort != "title" {
			when := p.Modified
			if v.Sort == "created" {
				when = p.Created
			}
			if g := dateGroup(when, now); g != last {
				last = g
				c.Key("group:" + g)
				ui.Text(c, g).FontSize(12).FontWeight(650).TextColor(t.TextMuted).Padding(14, 8, 4, 8).Transition(rowEnter)
			}
		}
		c.Key("list:" + p.Dir)
		row := ui.Column(c).Padding(8, 10).Gap(3).Radius(8).Label("Aç: " + p.Title).Role(ui.RoleButton).Transition(rowEnter)
		if row.Hovered() {
			row.Background(t.SurfaceHover)
		} else {
			row.Background(t.SurfaceHover.Alpha(0))
		}
		row.Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if crumb := parents(p); crumb != "" {
					ui.Text(c, crumb+"  ›").FontSize(14).TextColor(t.TextMuted)
				}
				ui.Text(c, p.Title).FontSize(15).FontWeight(620).MaxLines(1).Grow(1)
				when := p.Modified
				if v.Sort == "created" {
					when = p.Created
				}
				ui.Text(c, ago(when, now)).FontSize(12).TextColor(t.TextMuted)
			})
			if ex := p.Excerpt(); ex != "" {
				ui.Text(c, ex).FontSize(13.5).TextColor(t.TextMuted).MaxLines(2)
			}
		})
		if row.Clicked() {
			a.open(p)
		}
	}
}

// ---- the table ----

type column struct {
	label, field string
	width        float32 // 0: the rest
	cell         func(p *Page, now time.Time) string
}

var tableColumns = []column{
	{label: "Başlık", field: "title", cell: func(p *Page, _ time.Time) string { return p.Title }},
	{label: "Üst sayfa", width: 170, cell: func(p *Page, _ time.Time) string { return parents(p) }},
	{label: "Oluşturulma", field: "created", width: 150, cell: func(p *Page, _ time.Time) string { return shortDate(p.Created) }},
	{label: "Değiştirilme", field: "modified", width: 140, cell: func(p *Page, now time.Time) string { return ago(p.Modified, now) }},
}

func (a *app) tableView(c *ui.Context, v *View, pages []*Page) {
	t := c.Theme()
	now := a.clock()
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
		for _, col := range tableColumns {
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
		if row.Hovered() {
			row.Background(t.SurfaceHover)
		} else {
			row.Background(t.SurfaceHover.Alpha(0))
		}
		row.Children(func() {
			for i, col := range tableColumns {
				cellBox(col).Children(func() {
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

// newPageRow is the last row of a view: a new page in the collection.
func (a *app) newPageRow(c *ui.Context, coll *Collection) {
	t := c.Theme()
	row := ui.Row(c).Padding(10).Gap(8).Radius(8).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Text(c, "+  Yeni sayfa").FontSize(13.5).TextColor(t.TextMuted)
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
	case sameDay(t, now):
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
