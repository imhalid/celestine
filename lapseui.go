package main

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The time-lapse of a page, after Lev Manovich's Text Time-lapse: the text,
// each word in the color of the day it was written. A day is a version: the
// copies kept of the page that day, the last of them, and the page as it is
// now for the day it was last written. It is on by default (NoLapse in the
// settings turns it off), and it is the editor itself that is colored (the
// editor's Spans), so that one writes in it as anywhere: what is typed is
// today's, until the page is worked out again once the text has been still a
// moment. The status bar names the days, v1 v2 …: a click lights a day's words,
// the rest faint; a double click leaves its words alone, the rest going and the
// ones that stay moving up into their places: that is a view to look at, with
// nothing to type in, and Esc leaves it.

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

	paras   map[int][]lapseTok // the words of each paragraph of the editor, and the version each was written in, as of when they were worked out
	ver     int                // changes when the colors do: the editor lays its paragraphs out again (spansVersion)
	dark    bool               // the colors are for a dark ground
	docVer  uint64             // the version of the document the words were worked out from
	docSeen uint64             // the version of the document last seen, and when it was: the words are worked out again once it has been still
	seenAt  time.Time
}

// lapseTok is a word of a paragraph of the editor and the version it was written in.
type lapseTok struct {
	text string
	born int
}

// lapseStill is how long the text is still before the words are worked out again.
const lapseStill = 900 * time.Millisecond

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

// toggleLapse turns the time-lapse's colors on or off for every page: it is a way the editor looks, kept in the settings.
func (a *app) toggleLapse() {
	if a.page == nil || a.noPage() {
		return
	}
	if a.settings.NoLapse {
		if _, ok := historyOf(a.page); !ok {
			a.notice = "Sürüm geçmişi yalnız tek dosya kitaplığında tutulur"
			return
		}
	}
	a.settings.NoLapse = !a.settings.NoLapse
	a.saveSettings()
	a.lapse = lapseState{}
	a.refocus = true
}

// lapseAuto keeps the time-lapse as the settings say, for the page the active pane shows: it is on when the page has a history, worked
// out in the background; and it is worked out again once the text has been still, so that what was written since is counted.
func (a *app) lapseAuto(c *ui.Context, p *Page) {
	l := &a.lapse
	if a.inactivePane() || a.doc == nil {
		return
	}
	_, ok := historyOf(p)
	switch want := !a.settings.NoLapse && ok; {
	case want && !(l.on && l.page == p):
		*l = lapseState{on: true, page: p}
	case !want && l.on:
		*l = lapseState{}
	}
	if !l.on || l.page != p {
		return
	}
	if dark := ryugo.From(c).Dark; dark != l.dark {
		l.dark, l.ver = dark, l.ver+1
	}
	if l.words == nil {
		if !l.busy {
			a.startLapse()
		}
		return
	}
	if v := a.doc.Version(); v != l.docSeen {
		l.docSeen, l.seenAt = v, c.Now()
	}
	if l.docSeen != l.docVer && !l.busy {
		if wait := lapseStill - c.Now().Sub(l.seenAt); wait > 0 {
			c.After(wait)
		} else {
			a.startLapse()
		}
	}
}

// spansVersion tells the editor when what its spans do has changed: the letters' spacing, the theme, the time-lapse's colors.
func (a *app) spansVersion() int {
	v := a.settings.LetterSpacing*100 + a.settings.AppTheme
	if l := &a.lapse; l.on && l.page == a.page {
		v += l.ver * 100000
	}
	if a.grip.dragging { // the block dragged, drawn faded (blockSpans)
		v = -v - 1
	}
	return v
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
	file := p.File()
	now := a.currentMarkdown(p) // what is typed too: the page is saved first
	at := p.Modified
	if at.IsZero() {
		at = a.clock()
	}
	l.busy, l.docVer, l.docSeen = true, a.doc.Version(), a.doc.Version()
	paras := strings.Split(a.doc.Text().String(), "\n") // the editor's paragraphs, which the words are given to
	go func() {
		defer a.guard()
		// the copies read here, away from the window: every one the page kept, at each page opened and each pause in
		// the typing, was read under the lock (#199)
		var datas [][]byte
		var times []time.Time
		for _, v := range vers {
			if d, ok := h.VersionData(file, v.ID); ok {
				datas = append(datas, d)
				times = append(times, v.At)
			}
		}
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
		toks := lapseParas(ws, paras)
		a.do(func() error {
			l := &a.lapse
			if l.page != p || !l.busy {
				return nil
			}
			l.busy, l.words, l.days, l.sizes, l.paras = false, ws, when, sizes, toks
			l.ver++ // the editor's words are colored anew
			return nil
		})
	}()
}

// lapseParas gives the words of the page as it is (the ones the last version has) to the paragraphs of the editor, in order: each word
// of a paragraph takes the next word of that text that is the same, looking a few ahead, since the editor's text and the Markdown's
// words may differ in a mark; one that is not found is not written in any version yet.
func lapseParas(ws []Word, paras []string) map[int][]lapseTok {
	var now []Word
	for _, w := range ws {
		if w.Gone == 0 && !w.Para() {
			now = append(now, w)
		}
	}
	out := map[int][]lapseTok{}
	j := 0
	const ahead = 4
	for p, text := range paras {
		for _, f := range strings.Fields(text) {
			tok := lapseTok{text: f}
			for k := j; k < len(now) && k < j+ahead; k++ {
				if now[k].Text == f {
					tok.born, j = now[k].Born, k+1
					break
				}
			}
			out[p] = append(out[p], tok)
		}
	}
	return out
}

// lapseSpans colors the words of paragraph p of the editor by the version each was written in; ok is false where the time-lapse has
// nothing to say (it is off, there is one version only, the paragraph is a block of code, or a view of its own is up). What is typed
// since the words were worked out is told by what is the same before it and after it: the rest in between is the latest version's.
func (a *app) lapseSpans(p int, spans []ui.Span) (out []ui.Span, ok bool) {
	l := &a.lapse
	// one day only has nothing to tell apart: it is colored when its chip is clicked (v1), as the one day lit
	if !l.on || l.page != a.page || l.solo != 0 || len(l.days) == 0 || (len(l.days) < 2 && l.lit == 0) || len(spans) == 0 {
		return nil, false
	}
	var text strings.Builder
	for _, s := range spans {
		text.WriteString(s.Text)
	}
	s := text.String()
	type rng struct {
		from, to int
		col      ui.Color
	}
	var toks []string
	var at [][2]int
	for i := 0; i < len(s); {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
			i++
		}
		from := i
		for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' {
			i++
		}
		if i > from {
			toks, at = append(toks, s[from:i]), append(at, [2]int{from, i})
		}
	}
	old := l.paras[p]
	pre := 0
	for pre < len(old) && pre < len(toks) && old[pre].text == toks[pre] {
		pre++
	}
	suf := 0
	for suf < len(old)-pre && suf < len(toks)-pre && old[len(old)-1-suf].text == toks[len(toks)-1-suf] {
		suf++
	}
	n := len(l.days)
	var rs []rng
	for k := range toks {
		born := n
		switch {
		case k < pre:
			born = old[k].born
		case k >= len(toks)-suf:
			born = old[len(old)-(len(toks)-k)].born
		}
		if born == 0 {
			born = n // a word the history has no version for
		}
		col := lapseColor(born, n, l.dark)
		if l.lit != 0 && born != l.lit {
			col = col.Alpha(0.18)
		}
		rs = append(rs, rng{at[k][0], at[k][1], col})
	}
	// the spans cut where a word begins and ends
	pos, r := 0, 0
	for _, sp := range spans {
		text := sp.Text
		for len(text) > 0 {
			for r < len(rs) && rs[r].to <= pos {
				r++
			}
			piece := sp
			cut := len(text)
			colored := false
			switch {
			case r < len(rs) && rs[r].from <= pos: // inside a word
				cut, colored = min(cut, rs[r].to-pos), true
			case r < len(rs): // before one
				cut = min(cut, rs[r].from-pos)
			}
			piece.Text = text[:cut]
			if colored && sp.ObjectWidth == 0 {
				piece.Color = rs[r].col
			}
			out = append(out, piece)
			text, pos = text[cut:], pos+cut
		}
	}
	return out, true
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
		ui.Text(c, locales.HistoryReadingVersions()).TextColor(t.TextMuted).Padding(8, 0)
		return
	}
	dark := ryugo.From(c).Dark
	n := len(l.days)
	size := a.font.Size
	lh := a.font.LineHeight
	if lh == 0 {
		lh = 1.6
	}
	ui.Column(c.Key("lapse")).FillWidth().Gap(size*0.75).Padding(2, 0, 24, 0).Label(locales.CommonTimeLapse()).Children(func() {
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
	lit, solo := l.lit, l.solo
	defer func() { // the editor's words are colored anew when a day is lit or left alone
		if l.lit != lit || l.solo != solo {
			l.ver++
		}
	}()
	ui.Row(c.Key("lapse-chips")).Gap(2).AlignItems(ui.Center).Children(func() {
		for k := 1; k <= n; k++ {
			col := lapseColor(k, n, dark)
			chip := ui.Row(c.Key("lc"+strconv.Itoa(k))).Gap(4).Padding(2, 6).Radius(rad(6)).AlignItems(ui.Center).
				Label(locales.HistoryVersionN(k)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
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
			day := weekdayDate(l.days[k-1])
			if sameDayAs(l.days[k-1], a.clock()) {
				day = locales.HistoryToday()
			}
			ryugo.Tooltip(c, chip, locales.HistoryDayWords(day, l.sizes[k-1]))
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
