package main

import (
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The timeline of the page's history, in the sidebar: while the history is open the tree fades out and this fades in
// in its place (sidebar), and the stack of copies has the page's room to itself. A row a day, today at the top: a dot
// for each copy kept that day, a short line, the letter of the weekday and the day of the month, a weekend in the
// warm color, the month written at the start of each week. A click on a day brings its newest copy to the front (and
// again, the one before it); a click on a dot, that copy. A long run of days with no copy is one row of dots.

// tmDay is a row of the timeline: a day and the copies kept on it, or a run of days with none.
type tmDay struct {
	day   time.Time // the day, at its start
	first int       // the place of its newest copy: 0 the page now, i the copy vers[i-1]
	n     int       // how many copies it has
	skip  int       // not a day: this many days with no copy, left out
	month bool      // the month is written beside it
}

// tmSideState is what the timeline shows, kept apart from the history so that it can fade out after the history is
// gone.
type tmSideState struct {
	days  []tmDay
	n     int // how many copies are kept
	sel   int
	title string
}

// The measures of a row of the timeline, from its right edge inwards: the day of the month, the letter of the
// weekday, the line, the dots.
const (
	tmRowH   = 24
	tmPadR   = 14
	tmNumW   = 20
	tmLetW   = 14
	tmGap    = 6
	tmLineW  = 26
	tmPitch  = 7 // from a dot to the next
	tmMonthW = 58
	tmPadL   = 10
	tmSkipTo = 6 // more days than this with no copy are left out
)

// tmRight is how far the line's right end is from the row's.
const tmRight = tmPadR + tmNumW + tmGap + tmLetW + tmGap

// tmDaysOf lays the copies out by the day, the newest first, from today back to the oldest copy's: every day
// between has a row, but a long run of empty ones is one.
func tmDaysOf(vers []Version, now time.Time) []tmDay {
	start := func(t time.Time) time.Time {
		y, m, d := t.In(now.Location()).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	}
	out := []tmDay{{day: start(now), n: 1}}
	for i, v := range vers {
		day, last := start(v.At), &out[len(out)-1]
		if !day.Before(last.day) { // the same day (or a clock that went back)
			last.n++
			continue
		}
		if gap := int(last.day.Sub(day).Hours()/24+0.5) - 1; gap > tmSkipTo {
			out = append(out, tmDay{skip: gap})
		} else {
			for k := 1; k <= gap; k++ {
				out = append(out, tmDay{day: last.day.AddDate(0, 0, -k), first: i + 1})
			}
		}
		out = append(out, tmDay{day: day, first: i + 1, n: 1})
	}
	since := 99 // rows since a month was written
	for i := range out {
		d := &out[i]
		since++
		if d.skip == 0 && (i == 0 || (d.day.Weekday() == time.Monday || out[i-1].skip > 0) && since >= 3) {
			d.month, since = true, 0
		}
	}
	return out
}

// tmDots is how many dots a row as wide as w has room for in a line, and how many of a day's n copies it shows.
func tmDots(w float32, n int) (per, shown int) {
	per = max(4, int((w-tmRight-tmLineW-8-tmPadL-tmMonthW-tmGap)/tmPitch))
	return per, min(n, 2*per)
}

// tmDotAt is the dot of a row at (x, y) in it, or -1: the dots go from the line leftwards, the newest first, and
// onto a second line under the first.
func tmDotAt(w, x, y float32, n int) int {
	per, shown := tmDots(w, n)
	x0 := w - tmRight - tmLineW - 8
	if x > x0+2 || x < x0-float32(per)*tmPitch {
		return -1
	}
	j := int((x0-2-x)/tmPitch + 0.5)
	if shown > per && y > tmRowH/2 {
		j += per
	}
	if j < 0 || j >= shown {
		return -1
	}
	return j
}

// tmTimeline draws the timeline over the sidebar's tree, from top down and as wide as the sidebar, at opacity k.
func (a *app) tmTimeline(c *ui.Context, top, full, k float32) {
	s := &a.tmSide
	t := &a.tm
	live := t.open && !t.closing
	cl := ryugo.From(c).Colors
	col := ui.Column(c.Key("tm-side")).Absolute().Left(0).Top(top).Bottom(0).Width(full).Opacity(k).Label(T("Timeline"))
	if !live { // on its way out: only to look at
		col.Disabled(true).DimDisabled(false)
	}
	col.Children(func() {
		ui.Row(c.Key("tm-side-head")).Padding(4, 14, 8, 18).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, itoa(s.n)).FontSize(12).FontWeight(650).TextColor(cl.Text2).Padding(1, 7).Radius(rad(7)).Background(cl.Fill)
			of := T("copies of")
			if s.n == 1 {
				of = T("copy of")
			}
			ui.Text(c, of).FontSize(13).TextColor(cl.Muted)
			ui.Text(c, s.title).FontSize(13).FontWeight(620).TextColor(cl.Text).MaxLines(1).Grow(1).MinWidth(0)
		})
		a.scrolled(c, "tm-side", nil, ui.Scroll(c.Key("tm-side-rows")).Grow(1), func() {
			ui.Column(c).FillWidth().Padding(0, 8, 16, 8).Children(func() {
				letters := []rune(T("SMTWTFS"))
				for i := range s.days {
					a.tmDayRow(c, cl, i, letters, live)
				}
			})
		})
	})
}

// tmDayRow draws row i of the timeline.
func (a *app) tmDayRow(c *ui.Context, cl ryugo.Colors, i int, letters []rune, live bool) {
	s, t := &a.tmSide, &a.tm
	d := s.days[i]
	if d.skip > 0 {
		ui.Text(c.Key("tm-skip:"+itoa(i)), "⋯").FillWidth().Height(16).Padding(0, tmPadR+tmNumW+tmGap, 0, 0).TextAlign(ui.End).
			FontSize(12).TextColor(cl.Muted.Alpha(0.7))
		return
	}
	sel := d.n > 0 && s.sel >= d.first && s.sel < d.first+d.n
	ink, faint, dot, line := cl.Text2, cl.Muted, cl.Text.Alpha(0.5), cl.Text.Alpha(0.14)
	if wd := d.day.Weekday(); wd == time.Saturday || wd == time.Sunday {
		ink, faint, dot, line = cl.Warn, cl.Warn.Alpha(0.8), cl.Warn, cl.Warn.Alpha(0.35)
	}
	if sel {
		ink, faint, line = cl.Accent, cl.Accent, cl.Accent.Alpha(0.5)
	}
	row := ui.Row(c.Key("tm-row:"+d.day.Format("2006-01-02"))).FillWidth().Height(tmRowH).Padding(0, tmPadR, 0, tmPadL).Gap(tmGap).
		AlignItems(ui.Center).Radius(rad(7)).Label(T("Page history: ") + d.day.Format("2 Jan 2006")).Role(ui.RoleButton)
	w := row.LayoutBounds().W
	per, shown := tmDots(w, d.n)
	at := -1 // the dot under the pointer
	if d.n > 0 {
		row.Cursor(ui.CursorPointer)
		if x, y, over := row.PointerPosition(); over {
			at = tmDotAt(w, x, y, d.n)
		}
		if row.Hovered() {
			row.Background(cl.Fill)
			if live {
				tip := d.day.Format("Mon 2 Jan 2006")
				if at >= 0 {
					tip = a.tmWhen(d.first + at*d.n/shown)
				}
				ryugo.Tooltip(c, row, tip)
			}
		}
	}
	selAt := -1 // the dot of the copy at the front
	if sel {
		selAt = (s.sel - d.first) * shown / d.n
	}
	row.Draw(func(p *ui.Painter, r ui.Rect) {
		right, cy := r.X+r.W-tmRight, r.Y+r.H/2
		p.Fill(ui.Rect{X: right - tmLineW, Y: cy - 0.5, W: tmLineW, H: 1}, line, 0)
		lines := (shown + per - 1) / per
		for j := 0; j < shown; j++ {
			x := right - tmLineW - 8 - float32(j%per)*tmPitch - 2
			y := cy + (float32(j/per)-float32(lines-1)/2)*6
			sz, col := float32(4), dot
			switch {
			case j == selAt:
				sz, col = 6, cl.Accent
			case j == at:
				sz, col = 5, cl.Text
			}
			p.Fill(ui.Rect{X: x - sz/2, Y: y - sz/2, W: sz, H: sz}, col, sz/2)
		}
	})
	row.Children(func() {
		month := ""
		if d.month {
			month = d.day.Format("Jan 2006")
		}
		ui.Text(c, month).Width(tmMonthW).FontSize(12).TextColor(cl.Muted.Alpha(0.85)).MaxLines(1).PassThrough()
		ui.Box(c).Grow(1).PassThrough()
		ui.Text(c, string(letters[int(d.day.Weekday())%len(letters)])).Width(tmLetW).TextAlign(ui.Center).FontSize(12.5).FontWeight(650).
			TextColor(ink).PassThrough()
		ui.Text(c, itoa(d.day.Day())).Width(tmNumW).TextAlign(ui.End).FontSize(12.5).TextColor(faint).PassThrough()
	})
	if !live {
		return
	}
	if d.n > 0 && row.Clicked() {
		switch {
		case at >= 0: // that copy
			t.sel = d.first + at*d.n/shown
		case sel: // the day again: the copy before
			t.sel = d.first + (t.sel-d.first+1)%d.n
		default:
			t.sel = d.first
		}
	}
	if sel && t.shown != t.sel { // the keys went to a day out of sight
		row.ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollNearest})
		t.shown = t.sel
	}
}
