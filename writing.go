package main

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// What is written, by the day: the words each save adds to the page (what is
// deleted does not take them away) are counted into the day's, kept with the
// settings. A dialog shows today, the week, the month, the best day, the
// streak of days written, and the last thirty days as bars; the status bar
// can have today's count.

const writingKeep = 400 // days kept

// wordsOf counts the words of Markdown md as one reads them.
func wordsOf(md string) int { return len(strings.Fields(plainText(md))) }

// countWriting adds to today's count the words the save of md added to prev.
func (a *app) countWriting(prev, md string) {
	if prev == "" {
		return
	}
	n := wordsOf(md) - wordsOf(prev)
	if n <= 0 {
		return
	}
	if a.settings.Writing == nil {
		a.settings.Writing = map[string]int{}
	}
	a.settings.Writing[a.clock().Format("2006-01-02")] += n
	if len(a.settings.Writing) > writingKeep {
		days := a.writingDays()
		for _, d := range days[:len(days)-writingKeep] {
			delete(a.settings.Writing, d)
		}
	}
	// the settings are written now and then, not at each save
	if a.clock().Sub(a.writingSaved) > 20*time.Second {
		a.writingSaved = a.clock()
		a.saveSettings()
	}
}

// writingDays are the days with a count, the first first.
func (a *app) writingDays() []string {
	var days []string
	for d := range a.settings.Writing {
		days = append(days, d)
	}
	sort.Strings(days)
	return days
}

// writtenOn is the words written on day d.
func (a *app) writtenOn(d time.Time) int { return a.settings.Writing[d.Format("2006-01-02")] }

// writingStats are the figures the dialog shows.
type writingStats struct {
	today, week, month, total, best int
	bestDay                         string
	streak                          int
	last                            [30]int // the last 30 days, today last
}

// stats counts what was written up to now. The week starts on Monday; the
// streak counts the days in a row, back from today (or yesterday, while
// today has nothing yet), on which the goal - at least a word - was reached.
func (a *app) stats() writingStats {
	var s writingStats
	now := startOfDay(a.clock())
	weekStart := now.AddDate(0, 0, -((int(now.Weekday()) + 6) % 7))
	goal := max(a.settings.WriteGoal, 1)
	for d, n := range a.settings.Writing {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			continue
		}
		s.total += n
		if n > s.best {
			s.best, s.bestDay = n, d
		}
		if !t.Before(weekStart) && !t.After(now) {
			s.week += n
		}
		if t.Year() == now.Year() && t.Month() == now.Month() {
			s.month += n
		}
	}
	s.today = a.writtenOn(now)
	for i := range s.last {
		s.last[i] = a.writtenOn(now.AddDate(0, 0, i-29))
	}
	day := now
	if a.writtenOn(day) < goal {
		day = day.AddDate(0, 0, -1)
	}
	for a.writtenOn(day) >= goal {
		s.streak++
		day = day.AddDate(0, 0, -1)
	}
	return s
}

var iconPen = icon(`<path d="M14.5 5.5 18.5 9.5 9 19H5v-4z"/><path d="m12.5 7.5 4 4M4 21h16"/>`)

// writingView is the dialog.
func (a *app) writingView(c *ui.Context) {
	if !a.statsOpen {
		return
	}
	cl := ryugo.From(c).Colors
	s := a.stats()
	ryugo.Dialog(c.Key("writing"), &a.statsOpen, ryugo.DialogOptions{Title: "Yazma istatistikleri", Width: 640}, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Row(c).FillWidth().Gap(10).Children(func() {
				stat := func(key, label, value string) {
					ui.Column(c.Key(key)).Grow(1).Padding(12, 14).Gap(2).Radius(12).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
						ui.Text(c, value).FontSize(22).FontWeight(700).TextColor(cl.Text)
						ui.Text(c, label).FontSize(12).TextColor(cl.Muted)
					})
				}
				stat("st-today", "bugün", strconv.Itoa(s.today))
				stat("st-week", "bu hafta", strconv.Itoa(s.week))
				stat("st-month", "bu ay", strconv.Itoa(s.month))
				stat("st-streak", "gün üst üste", strconv.Itoa(s.streak))
			})
			if goal := a.settings.WriteGoal; goal > 0 { // today against the goal
				frac := float32(min(s.today, goal)) / float32(goal)
				ui.Column(c.Key("goal")).FillWidth().Gap(5).Children(func() {
					ui.Row(c).Children(func() {
						ui.Text(c, "Günlük hedef: "+strconv.Itoa(s.today)+" / "+strconv.Itoa(goal)+" kelime").FontSize(13).TextColor(cl.Text2).Grow(1)
						if s.today >= goal {
							ui.Text(c, "tamam").FontSize(12.5).FontWeight(650).TextColor(cl.Down)
						}
					})
					bar := ui.Box(c.Key("goal-bar")).FillWidth().Height(8).Radius(4).Background(cl.Well)
					bar.Draw(func(p *ui.Painter, r ui.Rect) {
						p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * frac, H: r.H}, map[bool]ui.Color{true: cl.Down, false: cl.Accent}[s.today >= goal], 4)
					})
				})
			}
			a.writingChart(c, cl, s)
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "En iyi gün: "+strconv.Itoa(s.best)+" kelime"+map[bool]string{true: " (" + s.bestDay + ")", false: ""}[s.best > 0]+"  ·  toplam "+strconv.Itoa(s.total)).
					FontSize(12.5).TextColor(cl.Muted).Grow(1)
				ui.Text(c, "Günlük hedef").FontSize(12.5).TextColor(cl.Muted)
				goal := float64(a.settings.WriteGoal)
				ryugo.NumberInput(c.Key("write-goal"), &goal, ryugo.NumberInputOptions{Label: "Günlük hedef", Size: ryugo.Sm, Step: 50}).Width(130)
				if int(goal) != a.settings.WriteGoal {
					a.settings.WriteGoal = max(int(goal), 0)
					a.saveSettings()
				}
			})
			ui.Text(c, "Sayılan, her kayıtta sayfaya eklenen kelimelerdir; silinenler düşmez. Hedef 0 ise yalnız bir kelime yazmak günü sayar.").
				FontSize(12).TextColor(cl.Muted).Wrap()
		})
	})
}

// writingChart draws the last thirty days as bars.
func (a *app) writingChart(c *ui.Context, cl ryugo.Colors, s writingStats) {
	hi := 1
	for _, n := range s.last {
		hi = max(hi, n)
	}
	goal := a.settings.WriteGoal
	hi = max(hi, goal)
	now := startOfDay(a.clock())
	box := ui.Box(c.Key("chart")).FillWidth().Height(150).Label("Son 30 günde yazılan kelimeler")
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		const gap = 3
		w := (r.W - gap*29) / 30
		plot := r.H - 22
		if goal > 0 { // the goal's line
			y := r.Y + plot*(1-float32(goal)/float32(hi))
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: 1}, cl.Down.Alpha(0.55), 0)
		}
		for i, n := range s.last {
			x := r.X + float32(i)*(w+gap)
			h := max(plot*float32(n)/float32(hi), map[bool]float32{true: 3, false: 0}[n > 0])
			col := cl.Accent.Alpha(0.55)
			if i == 29 {
				col = cl.Accent
			}
			if n == 0 {
				col = cl.Fill
				h = 3
			}
			p.Fill(ui.Rect{X: x, Y: r.Y + plot - h, W: w, H: h}, col, min(3, w/2))
			if d := now.AddDate(0, 0, i-29); i%7 == 1 || i == 29 {
				sp := ui.Span{Text: strconv.Itoa(d.Day()) + " " + months[d.Month()-1], Size: 11, Color: cl.Muted}
				tw, _ := p.MeasureText(0, sp)
				p.RichText(min(max(x+w/2-tw/2, r.X), r.X+r.W-tw), r.Y+plot+6, tw+2, sp)
			}
		}
	})
}
