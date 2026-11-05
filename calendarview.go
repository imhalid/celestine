package main

import (
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The calendar view, which every collection has beside its list and table:
// a month, a week a row from Monday, the pages on the day they are dated -
// by default the day they were made, or when they were last changed, or a
// date property's value (View.DateField). A page is opened by a click; on a
// date property's calendar a day has a + that makes a page for it, in the
// journal today has its page's button (the past is not added to).

var weekdayShort = []string{"Pzt", "Sal", "Çar", "Per", "Cum", "Cmt", "Paz"}

// pageDate is the day p is on in a calendar dated by field.
func pageDate(p *Page, field string) (time.Time, bool) {
	var d time.Time
	switch {
	case field == "modified":
		d = p.Modified
	case strings.HasPrefix(field, "prop:"):
		v, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(p.Props[strings.TrimPrefix(field, "prop:")]), time.Local)
		if err != nil {
			return d, false
		}
		d = v
	default:
		d = p.Created
	}
	d = d.Local()
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.Local), !d.IsZero()
}

// The days are quiet: a number, and a dot for each page on it. A click on a
// day with pages opens it - the day grows into a card two cells wide and
// three tall, out of its own cell, and the other days draw back from it,
// faint - with its pages in a list; Esc, the cross or another day closes it.

const (
	calGap = 4
	calRow = 84 // a day's height
)

// calSpring is the card growing out of its day, and the days making way.
var calSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 360 * time.Millisecond, Bounce: 0.08}}

func (a *app) calendarView(c *ui.Context, coll *Collection, v *View, pages []*Page) {
	t := ryugo.From(c)
	cl := t.Colors
	now := a.clock()
	if a.calMonth.IsZero() {
		a.calMonth = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	}
	month := a.calMonth
	field := v.DateField
	if field == "" {
		field = "created"
	}
	byDay := map[string][]*Page{}
	for _, p := range pages {
		if d, ok := pageDate(p, field); ok {
			k := d.Format("2006-01-02")
			byDay[k] = append(byDay[k], p)
		}
	}
	for _, l := range byDay {
		sort.SliceStable(l, func(i, j int) bool { return l[i].Created.Before(l[j].Created) })
	}
	offset := (int(month.Weekday()) + 6) % 7
	start := month.AddDate(0, 0, -offset)
	days := month.AddDate(0, 1, -1).Day()
	weeks := (offset + days + 6) / 7
	if a.calOpen && c.Shortcut(0, ui.KeyEscape) {
		a.calOpen = false
	}
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		// the month, and the way through the months
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, monthNames[month.Month()-1]+" "+pad(month.Year(), 4)).FontSize(20).FontWeight(680)
			ui.Spacer(c)
			if a.iconButton(c, iconBack, "Önceki ay") {
				a.calMonth, a.calOpen = month.AddDate(0, -1, 0), false
			}
			if ryugo.Button(c.Key("cal-today"), "Bugün", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
				a.calMonth, a.calOpen = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local), false
			}
			if a.iconButton(c, iconChevron, "Sonraki ay") {
				a.calMonth, a.calOpen = month.AddDate(0, 1, 0), false
			}
		})
		ui.Row(c).FillWidth().Gap(calGap).Children(func() {
			for _, d := range weekdayShort {
				ui.Text(c.Key("wd:"+d), d).FontSize(11.5).FontWeight(650).TextColor(cl.Muted).Grow(1).Basis(0).Padding(0, 8)
			}
		})
		grid := ui.Column(c.Key("cal-grid")).FillWidth().Gap(calGap)
		// how far the open day has come out of its cell, 0 to 1
		prog := grid.AnimateTo("open", map[bool]float32{true: 1, false: 0}[a.calOpen], calSpring)
		gb := grid.LayoutBounds()
		colW := (gb.W - 6*calGap) / 7
		openCols := float32(2)
		if colW < 110 {
			openCols = 3
		}
		openRows := min(3, weeks)
		// the card's cells: it holds the day's own, so it starts where it can
		oc, orow := int(a.calDay.Sub(start).Hours()/24+0.5)%7, int(a.calDay.Sub(start).Hours()/24+0.5)/7
		c0 := min(oc, 7-int(openCols))
		r0 := min(orow, weeks-openRows)
		cardTo := ui.Rect{X: gb.X + float32(c0)*(colW+calGap), Y: gb.Y + float32(r0)*(calRow+calGap),
			W: openCols*colW + (openCols-1)*calGap, H: float32(openRows)*calRow + float32(openRows-1)*calGap}
		cardFrom := ui.Rect{X: gb.X + float32(oc)*(colW+calGap), Y: gb.Y + float32(orow)*(calRow+calGap), W: colW, H: calRow}
		grid.Children(func() {
			for w := 0; w < weeks; w++ {
				ui.Row(c.Key("week:" + start.AddDate(0, 0, 7*w).Format("2006-01-02"))).FillWidth().Gap(calGap).Children(func() {
					for i := 0; i < 7; i++ {
						d := start.AddDate(0, 0, 7*w+i)
						a.calendarDay(c, coll, field, d, month, now, byDay, w, i, prog, [2]int{r0, c0}, [2]int{openRows, int(openCols)})
					}
				})
			}
		})
		a.calGrid = gb
		if prog > 0.002 && gb.W > 0 && !a.calDay.IsZero() {
			a.calendarCard(c, coll, field, a.calDay, byDay[a.calDay.Format("2006-01-02")], now, cardFrom, cardTo, prog)
		}
	})
}

// calendarDay is one day: its number and a dot for each page; it draws back
// from the card that is open.
func (a *app) calendarDay(c *ui.Context, coll *Collection, field string, d, month, now time.Time, byDay map[string][]*Page, row, col int, prog float32, at, size [2]int) {
	t := ryugo.From(c)
	cl := t.Colors
	key := d.Format("2006-01-02")
	list := byDay[key]
	today := sameDay(d, now)
	inMonth := d.Month() == month.Month()
	canMake := a.canMakeOn(coll, field, d, now, list)
	cell := ui.Column(c.Key("day:"+key)).Grow(1).Basis(0).MinWidth(0).Height(calRow).Padding(8).Gap(6).Radius(10).Border(1, cl.Hairline).
		Label("Gün: " + key).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	switch {
	case today:
		cell.Background(ryugo.Mix(cl.Card, cl.Accent, 0.09)).Border(1, cl.Accent.Alpha(0.6))
	case !inMonth:
		cell.Background(cl.Card.Alpha(0.35))
	default:
		cell.Background(cl.Card)
	}
	if cell.Hovered() && prog < 0.5 {
		cell.Background(cl.Fill)
	}
	// the days away from the card draw back: away from its middle, fainter
	inCard := row >= at[0] && row < at[0]+size[0] && col >= at[1] && col < at[1]+size[1]
	if !a.calDay.IsZero() && inCard {
		cell.Opacity(1 - prog) // under the card: it goes, so none of it shows at the card's edge
	}
	if !a.calDay.IsZero() && !inCard {
		cr, cc := float32(at[0])+float32(size[0]-1)/2, float32(at[1])+float32(size[1]-1)/2
		dx, dy := float32(col)-cc, float32(row)-cr
		cell.Translate(sign(dx)*14*prog, sign(dy)*10*prog).Opacity(1 - 0.6*prog)
	}
	cell.Children(func() {
		col := cl.Text2
		if !inMonth {
			col = cl.Muted.Alpha(0.6)
		}
		if today {
			col = cl.Accent
		}
		ui.Text(c, pad(d.Day(), 0)).FontSize(12.5).FontWeight(map[bool]int{true: 700, false: 560}[today]).TextColor(col)
		if len(list) > 0 {
			ui.Row(c).Wrap().Gap(3).Children(func() {
				for i := range list {
					if i == 12 {
						ui.Text(c, "+").FontSize(10).TextColor(cl.Muted)
						break
					}
					// a small soft button: a gentle gradient of the accent and its hairline
					ui.Box(c).Size(9, 9).Radius(3).LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true,
						From: ryugo.Mix(cl.Card, cl.Accent, 0.42), To: ryugo.Mix(cl.Card, cl.Accent, 0.26)}).Border(1, cl.Accent.Alpha(0.5))
				}
			})
		}
	})
	if cell.Clicked() {
		switch {
		case !a.calDay.IsZero() && a.calOpen && sameDay(a.calDay, d):
			a.calOpen = false
		case len(list) > 0 || canMake:
			a.calDay, a.calOpen = d, true
		default:
			a.calOpen = false
		}
	}
}

func sign(v float32) float32 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// canMakeOn says whether a page can be made for day d: in the journal today's
// when it is not there, on a calendar by a date property any day's.
func (a *app) canMakeOn(coll *Collection, field string, d, now time.Time, list []*Page) bool {
	return (coll.Journal && sameDay(d, now) && len(list) == 0) || (!coll.Journal && strings.HasPrefix(field, "prop:"))
}

// calendarCard is the open day: it grows out of its cell into the room of two
// by three, with the day's pages and a way to make one.
func (a *app) calendarCard(c *ui.Context, coll *Collection, field string, d time.Time, list []*Page, now time.Time, from, to ui.Rect, prog float32) {
	t := ryugo.From(c)
	cl := t.Colors
	lerp := func(a, b float32) float32 { return a + (b-a)*prog }
	r := ui.Rect{X: lerp(from.X, to.X), Y: lerp(from.Y, to.Y), W: lerp(from.W, to.W), H: lerp(from.H, to.H)}
	ui.Overlay(c, func() {
		card := ui.Box(c.Key("day-card")).Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).Radius(12).Clip().
			Background(cl.Card).Border(1, cl.Hairline).Opacity(min(1, prog*2.4))
		t.Shadows.ShadowPop.Apply(card)
		card.Children(func() {
			// laid out at its full size, shown as far as it has grown
			ui.Column(c.Key("day-card-in")).Width(to.W).Height(to.H).Padding(10).Gap(6).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Text(c, formatDay("D MMMM YYYY, dddd", d)).FontSize(13).FontWeight(650).Grow(1).MaxLines(1)
					if a.iconButton(c, iconClose, "Günü kapat") {
						a.calOpen = false
					}
				})
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						for _, p := range list {
							p := p
							row := ui.Row(c.Key("dc:"+p.Dir)).FillWidth().Height(30).Padding(0, 8).Gap(8).Radius(7).AlignItems(ui.Center).
								Label("Aç: " + p.Title).Role(ui.RoleButton).Cursor(ui.CursorPointer)
							if row.Hovered() {
								row.Background(cl.Fill)
							}
							a.contextOn(row, func() []menuItem { return a.pageMenu(p, false) })
							row.Children(func() {
								ui.Icon(c, iconPage).Width(14).Height(14).TextColor(cl.Muted)
								ui.Text(c, p.Title).FontSize(13).MaxLines(1)
							})
							if row.Clicked() {
								a.calOpen = false
								a.open(p)
							}
						}
					})
				})
				if a.canMakeOn(coll, field, d, now, list) {
					if ryugo.Button(c.Key("day-make"), map[bool]string{true: "Bugünün sayfası", false: "Bu güne sayfa ekle"}[coll.Journal],
						ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconPlus, Color: cl.Accent, Variant: ryugo.Soft}).Clicked() {
						a.calOpen = false
						a.makeOnDay(coll, field, d)
					}
				}
			})
		})
	})
}

// makeOnDay makes a page for day d: in the journal, today's; elsewhere a
// page with the date property set to that day.
func (a *app) makeOnDay(coll *Collection, field string, d time.Time) {
	if coll.Journal {
		a.openToday()
		return
	}
	p, err := coll.NewPage(nil, "Adsız")
	if err != nil {
		a.saveErr = err.Error()
		return
	}
	if id, ok := strings.CutPrefix(field, "prop:"); ok {
		if err := p.SetValue(id, d.Format("2006-01-02")); err != nil {
			a.saveErr = err.Error()
		}
	}
	a.open(p)
	a.fresh = true
}
