package main

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The time-lapse of a page, after Lev Manovich's Text Time-lapse: the text,
// in place of the editor while it is on, each word in the color of the day it
// was written. A day is a version: the copies kept of the page that day, the
// last of them, and the page as it is now for the day it was last written.
// The status bar names the days, v1 v2 …: a click lights a day's words, the
// rest faint; a double click leaves its words alone, the rest going and the
// ones that stay moving up into their places.

var iconLapse = icon(`<path d="M4 6.5h9"/><path d="M4 12h16"/><path d="M4 17.5h6"/><circle cx="17.5" cy="6.5" r="2"/><circle cx="14" cy="17.5" r="2"/>`)

type lapseState struct {
	on    bool
	page  *Page
	words []Word
	days  []time.Time // the day of each version, from the oldest
	sizes []int       // the words of each version
	busy  bool        // the words are being worked out
	lit   int         // the version clicked, from 1: its words bright, the rest faint
	solo  int         // the version double-clicked: its words alone
	held  int         // the version double-clicked, whose release is still to come: it is no click of its own
}

// lapseWord is how a word moves: to its new place when words before it go,
// fading as it goes or comes; one that goes is gone before the others are
// far on their way, so that they do not run over it.
var lapseWord = ui.ElementTransition{
	Duration:     420 * time.Millisecond,
	Ease:         ui.EaseInOut,
	ExitDuration: 140 * time.Millisecond,
	Position:     true,
	Enter:        &ui.Motion{Opacity: 0, Scale: 0.9},
	Exit:         &ui.Motion{Opacity: 0, Scale: 0.9},
}

// lapsePara is how a paragraph moves: up when the ones above it shrink.
var lapsePara = ui.ElementTransition{
	Duration: 420 * time.Millisecond,
	Ease:     ui.EaseInOut,
	Position: true,
	Height:   true,
	Enter:    &ui.Motion{Opacity: 0},
	Exit:     &ui.Motion{Opacity: 0, Collapse: true},
}

// toggleLapse shows the time-lapse of the page in place of its editor, or
// the editor again.
func (a *app) toggleLapse() {
	if a.lapse.on {
		a.lapse = lapseState{}
		a.refocus = true
		return
	}
	if a.page == nil || a.noPage() {
		return
	}
	if _, ok := historyOf(a.page); !ok {
		a.notice = "Sürüm geçmişi yalnız tek dosya kitaplığında tutulur"
		return
	}
	a.flush()
	a.lapse = lapseState{on: true, page: a.page}
}

// startLapse works out the time-lapse of the page: the copies are read here,
// the words set against each other outside the lock.
func (a *app) startLapse() {
	l := &a.lapse
	p := l.page
	h, ok := historyOf(p)
	if !ok {
		return
	}
	// the copies of the week, the named ones too, from the oldest
	week := a.clock().AddDate(0, 0, -HistoryDays)
	vers := slices.DeleteFunc(h.Versions(p.File()), func(v Version) bool { return v.Label != "" && v.At.Before(week) })
	slices.SortStableFunc(vers, func(x, y Version) int { return x.At.Compare(y.At) })
	var datas [][]byte
	var times []time.Time
	for _, v := range vers {
		if d, ok := h.VersionData(p.File(), v.ID); ok {
			datas = append(datas, d)
			times = append(times, v.At)
		}
	}
	now, _ := p.Text()
	at := p.Modified
	if at.IsZero() {
		at = a.clock()
	}
	l.busy = true
	go func() {
		defer a.guard()
		var texts []string
		for _, d := range datas {
			_, body := SplitPage(d)
			texts = append(texts, lapseText(bodyText(body)))
		}
		if now := lapseText(now); len(texts) == 0 || texts[len(texts)-1] != now {
			texts = append(texts, now)
			times = append(times, at)
		}
		var days []string
		var when []time.Time
		for _, i := range LastOfDays(times) {
			days = append(days, texts[i])
			when = append(when, times[i])
		}
		ws := WordHistory(days)
		sizes := make([]int, len(days))
		for _, w := range ws {
			if w.Para() {
				continue
			}
			for k := w.Born; k <= len(days) && (w.Gone == 0 || k < w.Gone); k++ {
				sizes[k-1]++
			}
		}
		a.do(func() error {
			l := &a.lapse
			if l.page != p || !l.busy {
				return nil
			}
			l.busy, l.words, l.days, l.sizes = false, ws, when, sizes
			return nil
		})
	}()
}

// lapseText is a page's Markdown as the words a reader sees, a paragraph a
// line: the marks of headings, lists and emphasis, and the dividers, are not
// words of the text.
func lapseText(md string) string {
	var lines []string
	for _, line := range strings.Split(md, "\n") {
		if t := strings.TrimSpace(line); len(t) >= 3 && strings.Trim(t, "-*_") == "" {
			continue
		}
		if s := plainText(line); s != "" {
			lines = append(lines, s)
		}
	}
	return strings.Join(lines, "\n")
}

// lapseColor is the color of version k of n: from emerald through sky and
// indigo to fuchsia, the oldest the coolest and today the warmest; lighter on
// a dark ground.
func lapseColor(k, n int, dark bool) ui.Color {
	var t float32
	if n > 1 {
		t = float32(k-1) / float32(n-1)
	}
	h := 160 + 170*t
	if dark {
		return ui.Oklch(0.78, 0.13, h)
	}
	return ui.Oklch(0.55, 0.16, h)
}

// lapseOn reports whether the time-lapse is shown in place of p's editor.
func (a *app) lapseOn(p *Page) bool { return a.lapse.on && a.lapse.page == p }

// lapseView is the time-lapse, where the editor is.
func (a *app) lapseView(c *ui.Context) {
	l := &a.lapse
	t := c.Theme()
	if l.words == nil {
		if !l.busy {
			a.startLapse()
		}
		ui.Text(c, T("Reading the versions…")).TextColor(t.TextMuted).Padding(8, 0)
		return
	}
	dark := ryugo.From(c).Dark
	n := len(l.days)
	size := a.font.Size
	lh := a.font.LineHeight
	if lh == 0 {
		lh = 1.6
	}
	ui.Column(c.Key("lapse")).FillWidth().Gap(size*0.75).Padding(2, 0, 24, 0).Label(T("Time-lapse")).Children(func() {
		var words []int
		para := 0
		flush := func() {
			if len(words) > 0 {
				ui.Row(c.Key("lp" + strconv.Itoa(para))).FillWidth().Wrap().GapX(size * 0.28).Transition(lapsePara).Children(func() {
					for _, i := range words {
						w := l.words[i]
						word := ui.Text(c.Key("lw"+strconv.Itoa(i)), w.Text).Font(a.font.Family).FontSize(size).LineHeight(lh).
							TextColor(lapseColor(w.Born, n, dark)).Transition(lapseWord)
						faint := float32(1)
						if l.lit != 0 && w.Born != l.lit {
							faint = 0.18
						}
						word.Opacity(word.AnimateTo("lit", faint, ui.Animation{Duration: 220 * time.Millisecond, NonMotion: true}))
					}
				})
			}
			words = words[:0]
		}
		for i, w := range l.words {
			switch {
			case w.Gone != 0: // the text as it is
			case w.Para():
				flush()
				para++
			case l.solo == 0 || w.Born == l.solo:
				words = append(words, i)
			}
		}
		flush()
	})
}

// lapseChips are the versions in the status bar, each in its color, its day
// on hover: a click lights it, a double click leaves it alone.
func (a *app) lapseChips(c *ui.Context) {
	l := &a.lapse
	if l.on && l.page != a.page { // another page is open: its editor
		*l = lapseState{}
	}
	if !l.on || len(l.days) == 0 {
		return
	}
	dark := ryugo.From(c).Dark
	n := len(l.days)
	ui.Row(c.Key("lapse-chips")).Gap(2).AlignItems(ui.Center).Children(func() {
		for k := 1; k <= n; k++ {
			col := lapseColor(k, n, dark)
			chip := ui.Row(c.Key("lc"+strconv.Itoa(k))).Gap(4).Padding(2, 6).Radius(rad(6)).AlignItems(ui.Center).
				Label(Tf("Version %d", k)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			on := l.lit == k || l.solo == k
			if on || chip.Hovered() {
				chip.Background(col.Alpha(0.16))
			}
			if (l.lit != 0 || l.solo != 0) && !on {
				chip.Opacity(0.45)
			}
			chip.Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(col)
				ui.Text(c, "v"+strconv.Itoa(k)).FontSize(11.5).FontWeight(650).TextColor(col)
			})
			day := l.days[k-1].Format("Mon, 2 Jan 2006")
			if sameDayAs(l.days[k-1], a.clock()) {
				day = T("Today")
			}
			ryugo.Tooltip(c, chip, Tf("%s · %d words", day, l.sizes[k-1]))
			switch {
			case chip.DoubleClicked(): // its first click lit it; a click again brings the rest back
				l.solo, l.lit, l.held = k, 0, k
			case chip.Clicked():
				switch {
				case l.held == k: // the double click let go
					l.held = 0
				case l.solo != 0: // back to all the text, this one lit
					l.solo, l.lit = 0, k
				case l.lit == k:
					l.lit = 0
				default:
					l.lit = k
				}
			}
		}
	})
}

// sameDayAs reports whether a and b are on the same day where the computer is.
func sameDayAs(a, b time.Time) bool {
	ay, am, ad := a.Local().Date()
	by, bm, bd := b.Local().Date()
	return ay == by && am == bm && ad == bd
}
