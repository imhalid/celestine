package main

import (
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The calendar view, which every collection has beside its list and table:
// a month, a week a row from Monday, the pages on the day they are dated -
// by default the day they were made, or when they were last changed, or a
// date property's value (View.DateField). A page is opened by a click; on a
// date property's calendar a day has a + that makes a page for it, in the
// journal today has its page's button (the past is not added to). On a date
// property's calendar a page in an open day is dragged to another day: its
// date becomes that day.

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
	case p.Coll != nil && p.Coll.Journal: // a day of the journal is its own, wherever it is looked at from (#144)
		return dayOf(p), true
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

// calKept is the calendar's pages by day, as they were last sorted, and what from.
type calKept struct {
	field string
	made  int // the view's pages as they were made (viewPages): new ones are new pages
	lib   libKey
	days  map[string][]*Page
}

// calendarDays are the pages by their day of field, each day's in the order they were made: kept while the view's
// pages and the library are as they were, where every page was put in its day again at every frame (#194).
func (a *app) calendarDays(pages []*Page, field string) map[string][]*Page {
	d := &a.calDays
	if k := a.libKeyNow(); d.days != nil && d.field == field && d.made == a.viewCache.made && d.lib == k {
		return d.days
	}
	byDay := map[string][]*Page{}
	for _, p := range pages {
		if t, ok := pageDate(p, field); ok {
			k := t.Format("2006-01-02")
			byDay[k] = append(byDay[k], p)
		}
	}
	for _, l := range byDay {
		sort.SliceStable(l, func(i, j int) bool { return l[i].Created.Before(l[j].Created) })
	}
	*d = calKept{field, a.viewCache.made, a.libKeyNow(), byDay}
	return byDay
}

func (a *app) calendarView(c *ui.Context, pn *paneState, coll *Collection, v *View, pages []*Page) {
	t := ryugo.From(c)
	cl := t.Colors
	now := a.clock()
	if pn.calMonth.IsZero() {
		pn.calMonth = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	}
	month := pn.calMonth
	field := v.DateField
	if field == "" {
		field = "created"
	}
	byDay := a.calendarDays(pages, field)
	offset := weekIndex(month)
	start := month.AddDate(0, 0, -offset)
	days := month.AddDate(0, 1, -1).Day()
	weeks := (offset + days + 6) / 7
	if (pn.calOpen || !pn.calNext.IsZero()) && c.Shortcut(0, ui.KeyEscape) {
		pn.calOpen, pn.calNext = false, time.Time{}
	}
	dragging := pn.calDrag.page != nil
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		// the month, and the way through the months
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, monthYear(month, false)).FontSize(20).FontWeight(680)
			ui.Spacer(c)
			if a.iconButton(c, iconBack, locales.CommonPreviousMonth()) {
				pn.calMonth, pn.calOpen, pn.calNext = month.AddDate(0, -1, 0), false, time.Time{}
			}
			if ryugo.Button(c.Key("cal-today"), locales.CommonToday(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
				pn.calMonth, pn.calOpen, pn.calNext = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local), false, time.Time{}
			}
			if a.iconButton(c, iconChevron, locales.CommonNextMonth()) {
				pn.calMonth, pn.calOpen, pn.calNext = month.AddDate(0, 1, 0), false, time.Time{}
			}
		})
		ui.Row(c).FillWidth().Gap(calGap).Children(func() {
			for _, d := range weekdayHeads(false) {
				ui.Text(c.Key("wd:"+d), d).FontSize(11.5).FontWeight(650).TextColor(cl.Muted).Grow(1).Basis(0).Padding(0, 8, 6, 8) // room under the names, off the days
			}
		})
		grid := ui.Column(c.Key("cal-grid")).FillWidth().Gap(calGap)
		// how far the open day has come out of its cell, 0 to 1
		prog := grid.AnimateTo("open", map[bool]float32{true: 1, false: 0}[pn.calOpen], calSpring)
		// another day was chosen while one was open: it opens once the first has nearly gone back into its cell
		if !pn.calNext.IsZero() && !pn.calOpen && prog < 0.2 {
			pn.calDay, pn.calOpen, pn.calNext = pn.calNext, true, time.Time{}
		}
		dayProg := prog // while a page is dragged the days are all there to be dropped on, the card unseen over them
		if dragging {
			dayProg = 0
		}
		gb := grid.LayoutBounds()
		colW := (gb.W - 6*calGap) / 7
		openCols := float32(2)
		if colW < 110 {
			openCols = 3
		}
		openRows := min(3, weeks)
		// the card's cells: it holds the day's own, so it starts where it can
		oc, orow := int(pn.calDay.Sub(start).Hours()/24+0.5)%7, int(pn.calDay.Sub(start).Hours()/24+0.5)/7
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
						a.calendarDay(c, pn, coll, field, d, month, now, byDay, w, i, dayProg, [2]int{r0, c0}, [2]int{openRows, int(openCols)})
					}
				})
			}
		})
		pn.calGrid = gb
		if dragging {
			pn.calDrag.over = calDayAt(pn.calDrag.pos, gb, colW, start, weeks)
		}
		if prog > 0.002 && gb.W > 0 && !pn.calDay.IsZero() {
			a.calendarCard(c, pn, coll, field, pn.calDay, byDay[pn.calDay.Format("2006-01-02")], now, cardFrom, cardTo, prog)
		}
		if dragging {
			a.calDragGhost(c, pn)
		}
	})
}

// calendarDay is one day: its number and a dot for each page; it draws back
// from the card that is open.
func (a *app) calendarDay(c *ui.Context, pn *paneState, coll *Collection, field string, d, month, now time.Time, byDay map[string][]*Page, row, col int, prog float32, at, size [2]int) {
	t := ryugo.From(c)
	cl := t.Colors
	key := d.Format("2006-01-02")
	list := byDay[key]
	today := sameDay(d, now)
	inMonth := d.Month() == month.Month()
	canMake := a.canMakeOn(coll, field, d, now, list)
	cell := ui.Column(c.Key("day:"+key)).Grow(1).Basis(0).MinWidth(0).Height(calRow).Padding(8).Gap(6).Radius(rad(10)).Border(1, cl.Hairline).
		Label(locales.CalendarDayLabel(key)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	switch {
	case today:
		cell.Background(ryugo.Mix(cl.Card, cl.Accent, 0.09)).Border(1, cl.Accent.Alpha(0.6))
	case !inMonth:
		cell.Background(cl.Card.Alpha(0.35))
	default:
		cell.Background(cl.Card)
	}
	if cell.Hovered() && prog < 0.5 && pn.calDrag.page == nil {
		cell.Background(cl.Fill)
	}
	if pn.calDrag.page != nil && sameDay(pn.calDrag.over, d) { // where the page dragged would go
		cell.Background(ryugo.Mix(cl.Card, cl.Accent, 0.16)).Border(1.5, cl.Accent)
	}
	// the days away from the card draw back: away from its middle, fainter
	inCard := row >= at[0] && row < at[0]+size[0] && col >= at[1] && col < at[1]+size[1]
	if !pn.calDay.IsZero() && inCard {
		cell.Opacity(1 - prog) // under the card: it goes, so none of it shows at the card's edge
	}
	if !pn.calDay.IsZero() && !inCard {
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
					ui.Box(c).Size(9, 9).Radius(rad(3)).LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true,
						From: ryugo.Mix(cl.Card, cl.Accent, 0.42), To: ryugo.Mix(cl.Card, cl.Accent, 0.26)}).Border(1, cl.Accent.Alpha(0.5))
				}
			})
		}
	})
	a.kbMark(cell, "d:"+d.Format("2006-01-02"), true, func() {
		switch {
		case !pn.calDay.IsZero() && pn.calOpen && sameDay(pn.calDay, d):
			pn.calOpen = false
		case len(list) > 0 || canMake:
			a.openCalDayIn(pn, d)
		default:
			pn.calOpen, pn.calNext = false, time.Time{}
		}
	})
	if cell.Clicked() {
		switch {
		case !pn.calDay.IsZero() && pn.calOpen && sameDay(pn.calDay, d):
			pn.calOpen = false
		case len(list) > 0 || canMake:
			a.openCalDayIn(pn, d)
		default:
			pn.calOpen, pn.calNext = false, time.Time{}
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
func (a *app) calendarCard(c *ui.Context, pn *paneState, coll *Collection, field string, d time.Time, list []*Page, now time.Time, from, to ui.Rect, prog float32) {
	t := ryugo.From(c)
	cl := t.Colors
	lerp := func(a, b float32) float32 { return a + (b-a)*prog }
	r := ui.Rect{X: lerp(from.X, to.X), Y: lerp(from.Y, to.Y), W: lerp(from.W, to.W), H: lerp(from.H, to.H)}
	ui.Overlay(c, func() {
		seen := min(1, prog*2.4)
		if pn.calDrag.page != nil { // still there, so that the drag goes on, but out of the way of the days
			seen = 0
		}
		card := ui.Box(c.Key("day-card")).Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).Radius(rad(12)).Clip().
			Background(cl.Card).Border(1, cl.Hairline).Opacity(seen)
		t.Shadows.ShadowPop.Apply(card)
		card.Children(func() {
			// laid out at its full size, shown as far as it has grown
			ui.Column(c.Key("day-card-in")).Width(to.W).Height(to.H).Padding(10).Gap(6).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Text(c, formatDay("D MMMM YYYY, dddd", d)).FontSize(13).FontWeight(650).Grow(1).MaxLines(1)
					if a.iconButton(c, iconClose, locales.CalendarCloseDay()) {
						pn.calOpen = false
					}
				})
				a.scrolled(c, "calendarview:250", nil, ui.Scroll(c).Grow(1), func() {
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						for _, p := range list {
							p := p
							row := ui.Row(c.Key("dc:"+p.Dir)).FillWidth().Height(30).Padding(0, 8).Gap(8).Radius(rad(7)).AlignItems(ui.Center).
								Label(locales.ViewsOpenLabel(p.Title)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
							if row.Hovered() {
								row.Background(cl.Fill)
							}
							a.contextOn(row, func() []menuItem { return a.pageMenu(p, false) })
							row.Children(func() {
								ui.Icon(c, iconPage).Width(14).Height(14).TextColor(cl.Muted)
								ui.Text(c, p.Title).FontSize(13).MaxLines(1)
							})
							a.kbMark(row, "c:"+p.Dir, true, func() { pn.calOpen = false; a.openIn(pn, p) })
							held := false
							if strings.HasPrefix(field, "prop:") && !coll.Journal {
								held = a.calDragRow(c, pn, row, coll, field, p)
							}
							if row.Clicked() && !held {
								pn.calOpen = false
								a.openIn(pn, p)
							}
						}
					})
				})
				if a.canMakeOn(coll, field, d, now, list) {
					if ryugo.Button(c.Key("day-make"), map[bool]string{true: "todaysPage", false: addToDayLabel(coll)}[coll.Journal],
						ryugo.ButtonOptions{Size: ryugo.Sm, Icon: iconPlus, Color: cl.Accent, Variant: ryugo.Soft}).Clicked() {
						pn.calOpen = false
						a.makeOnDayIn(pn, coll, field, d)
					}
				}
			})
		})
	})
}

// calDrag is a page of an open day being dragged to another: the page, the press that may become a drag and how far
// it went, where the pointer is, the day under it, and the date property the drop sets.
type calDrag struct {
	page, pressed *Page
	dropped       *Page // let go a frame ago: the click its release makes is not one
	moved         float32
	pos           ui.Point
	over          time.Time
	coll          *Collection
	field         string
}

// calDragRow follows a press on page p's row in the open day: a press that went a few DIPs picks the page up, and
// letting it go over a day gives the page that day's date. It reports whether the page is held or was let go this
// frame: the press is a drag, not a click.
func (a *app) calDragRow(c *ui.Context, pn *paneState, row ui.Element, coll *Collection, field string, p *Page) bool {
	d := &pn.calDrag
	dx, dy, pressed := row.Dragged()
	pointer := func() ui.Point {
		b := row.LayoutBounds()
		px, py, _ := row.PointerPosition()
		return ui.Point{X: b.X + px, Y: b.Y + py}
	}
	switch {
	case pressed && d.page == p:
		d.pos = pointer()
		c.AnimationFrame()
		return true
	case pressed && d.page == nil:
		if d.pressed != p {
			d.pressed, d.moved = p, 0
		}
		if d.moved += abs32(dx) + abs32(dy); d.moved > dragStart {
			d.page, d.coll, d.field, d.pos = p, coll, field, pointer()
			return true
		}
	case !pressed && d.page == p: // let go
		day := d.over
		*d = calDrag{dropped: p}
		pn.calOpen = false
		if !day.IsZero() {
			if id, ok := strings.CutPrefix(field, "prop:"); ok {
				if err := p.SetValue(id, day.Format("2006-01-02")); err != nil {
					a.fail(err)
				}
			}
		}
		return true
	case !pressed && d.dropped == p:
		d.dropped = nil
		return true
	case !pressed && d.pressed == p:
		d.pressed, d.moved = nil, 0
	}
	return false
}

// calDayAt is the day of the grid under point pt, or zero when it is over none.
func calDayAt(pt ui.Point, grid ui.Rect, colW float32, start time.Time, weeks int) time.Time {
	if colW <= 0 || pt.X < grid.X || pt.Y < grid.Y {
		return time.Time{}
	}
	col, row := int((pt.X-grid.X)/(colW+calGap)), int((pt.Y-grid.Y)/(calRow+calGap))
	if col > 6 || row >= weeks {
		return time.Time{}
	}
	return start.AddDate(0, 0, 7*row+col)
}

// calDragGhost is the page being dragged: its title on a little card under the pointer.
func (a *app) calDragGhost(c *ui.Context, pn *paneState) {
	cl := ryugo.From(c).Colors
	// the page and the place taken now: the overlay is built after the view, by when a drop may have let the page go
	p, pos := pn.calDrag.page, pn.calDrag.pos
	if p == nil {
		return
	}
	ui.Overlay(c, func() {
		ghost := ui.Row(c.Key("cal-ghost")).Absolute().Left(pos.X+10).Top(pos.Y-14).Height(30).Padding(0, 10).Gap(8).
			AlignItems(ui.Center).Radius(rad(8)).Background(cl.Card).Border(1, cl.Accent.Alpha(0.6))
		ryugo.From(c).Shadows.ShadowPop.Apply(ghost)
		ghost.Children(func() {
			ui.Icon(c, iconPage).Width(14).Height(14).TextColor(cl.Accent)
			ui.Text(c, p.Title).FontSize(13).MaxLines(1)
		})
	})
}

// makeOnDay makes a page for day d: in the journal, today's; elsewhere a
// page with the date property set to that day.
func (a *app) makeOnDay(coll *Collection, field string, d time.Time) {
	a.makeOnDayIn(a.pane(), coll, field, d)
}

func (a *app) makeOnDayIn(pn *paneState, coll *Collection, field string, d time.Time) {
	if coll.Journal {
		a.openToday()
		return
	}
	p, err := coll.NewPage(nil, msg("common_untitled"))
	if err != nil {
		a.fail(err)
		return
	}
	if id, ok := strings.CutPrefix(field, "prop:"); ok {
		if err := p.SetValue(id, d.Format("2006-01-02")); err != nil {
			a.fail(err)
		}
	}
	a.openIn(pn, p)
	pn.fresh = true
}

// openCalDay opens day d on the calendar. With another day open, that one closes first and d opens as it goes
// (calendarView), so the card goes back into its cell and comes out of the new one instead of jumping there.
func (a *app) openCalDay(d time.Time) { a.openCalDayIn(a.pane(), d) }

func (a *app) openCalDayIn(pn *paneState, d time.Time) {
	if (pn.calOpen || !pn.calNext.IsZero()) && !pn.calDay.IsZero() && !sameDay(pn.calDay, d) {
		pn.calNext, pn.calOpen = d, false
		return
	}
	pn.calDay, pn.calOpen, pn.calNext = d, true, time.Time{}
}
