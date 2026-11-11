package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The page of figures: a screen of the window, like the settings - writing, use, the library and what
// the figures are. It opens from ⌘P ("İstatistikler"), is in the history, and can be in a pane of its own.

var iconStats = icon(`<path d="M5 20V11M12 20V5M19 20v-7"/>`)

// openStats shows the page of figures.
func (a *app) openStats() {
	if a.statsScreen {
		return
	}
	a.flush()
	a.statsScreen, a.profileScreen, a.settingsOpen, a.coll, a.tag, a.trashOpen, a.tasksOpen, a.searchOpen = true, false, false, nil, "", false, false, false
	a.statCache = libStats{}
	a.statCacheAt = time.Time{}
	a.statLoading = false
	a.visit()
}

// closeStats goes back to where the page was opened from.
func (a *app) closeStats() {
	if !a.step(-1) {
		a.statsScreen = false
	}
}

// libStatsNow is the library's figures, read again at most every ten seconds.
func (a *app) libStatsNow() libStats {
	if a.statCacheAt.IsZero() || a.clock().Sub(a.statCacheAt) > 10*time.Second {
		a.statCache, a.statCacheAt = a.libraryStats(), a.clock()
	}
	return a.statCache
}

// statTile is a figure and what it is of.
func statTile(c *ui.Context, key, label, value string) {
	label = T(label)
	cl := ryugo.From(c).Colors
	ui.Column(c.Key(key)).Grow(1).MinWidth(110).Padding(12, 14).Gap(2).Radius(rad(12)).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
		ui.Text(c, value).FontSize(22).FontWeight(700).TextColor(cl.Text).MaxLines(1)
		ui.Text(c, label).FontSize(12).TextColor(cl.Muted).MaxLines(1)
	})
}

// statHead is a section's title and what it counts.
func statHead(c *ui.Context, title, note string) {
	title, note = T(title), T(note)
	cl := ryugo.From(c).Colors
	ui.Column(c).FillWidth().Gap(2).Padding(14, 0, 0, 0).Children(func() {
		ui.Text(c, title).FontSize(17).FontWeight(680).TextColor(cl.Text)
		if note != "" {
			ui.Text(c, note).FontSize(ryugo.FsSm).TextColor(cl.Muted).Wrap()
		}
	})
}

// statList is a list of names and numbers, with a bar behind each.
func statList(c *ui.Context, key string, rows []count, empty string, open func(count)) {
	cl := ryugo.From(c).Colors
	if len(rows) == 0 {
		ui.Text(c, empty).FontSize(13).TextColor(cl.Muted)
		return
	}
	hi := 1
	for _, r := range rows {
		hi = max(hi, r.N)
	}
	ui.Column(c.Key(key)).FillWidth().Gap(3).Children(func() {
		for i, r := range rows {
			r := r
			row := ui.Row(c.Key(key+":"+strconv.Itoa(i))).FillWidth().Height(28).Padding(0, 10).Gap(10).Radius(rad(7)).AlignItems(ui.Center).Label(r.Name)
			if open != nil {
				row.Role(ui.RoleButton).Cursor(ui.CursorPointer)
			}
			frac := float32(r.N) / float32(hi)
			row.Draw(func(p *ui.Painter, rc ui.Rect) {
				p.Fill(ui.Rect{X: rc.X, Y: rc.Y, W: rc.W * frac, H: rc.H}, cl.Accent.Alpha(0.14), rad(7))
			})
			row.Children(func() {
				ui.Text(c, r.Name).FontSize(13).TextColor(cl.Text).MaxLines(1).Grow(1)
				ui.Text(c, strconv.Itoa(r.N)).FontSize(13).FontWeight(620).TextColor(cl.Text2)
			})
			if open != nil && row.Clicked() {
				open(r)
			}
		}
	})
}

// statsView is the page.
func (a *app) statsView(c *ui.Context) {
	cl := ryugo.From(c).Colors
	s := a.stats()
	now := startOfDay(a.clock())
	weekFrom := weekStart(now)
	// the scroll bar is the window's: the page is as wide as the area, its column is centered inside it
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Row(c).FillWidth().MaxWidth(viewMax).Gap(12).AlignItems(ui.Center).Padding(2, 0, 0, 0).Children(func() {
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Text(c, locales.CommonStatistics()).FontSize(32).FontWeight(720).TextColor(cl.Text)
					ui.Text(c, locales.StatsWritingUseAllStays()).FontSize(ryugo.FsMd).TextColor(cl.Muted)
				})
				on := !a.settings.NoStats
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Topla").FontSize(13).TextColor(cl.Muted)
					ryugo.Switch(c.Key("stats-on"), &on, ryugo.SwitchOptions{}).Label(locales.CommonCollectStatistics())
				})
				if on != !a.settings.NoStats {
					a.settings.NoStats = !on
					a.saveSettings()
				}
			})
		})
		a.scrolled(c, "stats-scroll", nil, ui.Scroll(c.Key("stats-scroll")).FillWidth().Grow(1).MinHeight(0), func() {
			ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
				ui.Column(c.Key("stats-body")).FillWidth().MaxWidth(viewMax).Gap(12).Padding(0, 4, 40, 0).Children(func() {
					if a.statCacheAt.IsZero() && !a.statLoading { // the screen first, then the figures: opening is never the wait
						a.statLoading = true
						ui.Text(c, locales.StatsCalculating()).FontSize(13).TextColor(cl.Muted)
						c.Invalidate()
						return
					}
					if a.settings.NoStats {
						ui.Text(c, locales.StatsStatisticsAreOffNothing()).FontSize(13).TextColor(cl.Warn)
					}
					a.statsWriting(c, cl, s, weekFrom, now)
					a.statsUse(c, cl, now, weekFrom)
					a.statsLibrary(c, cl)
					a.statsData(c, cl)
				})
			})
		})
	})
}

func (a *app) statsWriting(c *ui.Context, cl ryugo.Colors, s writingStats, weekFrom, now time.Time) {
	statHead(c, locales.StatsWriting(), "Sayılan, her kayıtta sayfaya eklenen kelimelerdir; silinenler düşmez, taşınan ve içe aktarılan metin yazılmış sayılmaz.")
	ui.Row(c.Key("st-tiles")).FillWidth().Gap(10).Wrap().Children(func() {
		statTile(c, "st-today", locales.StatsToday(), strconv.Itoa(s.today))
		statTile(c, "st-week", locales.StatsThisWeek(), strconv.Itoa(s.week))
		statTile(c, "st-month", locales.StatsThisMonth(), strconv.Itoa(s.month))
		statTile(c, "st-streak", locales.StatsDaysRow(), strconv.Itoa(s.streak))
		statTile(c, "st-best", locales.StatsBestDay(), strconv.Itoa(s.best))
		statTile(c, "st-total", locales.StatsTotal(), strconv.Itoa(s.total))
	})
	goal := a.settings.WriteGoal
	if goal > 0 {
		frac := float32(min(s.today, goal)) / float32(goal)
		ui.Column(c.Key("goal")).FillWidth().Gap(5).Children(func() {
			ui.Row(c).Children(func() {
				ui.Text(c, locales.StatsDailyGoalProgress(s.today, goal)).FontSize(13).TextColor(cl.Text2).Grow(1)
				switch {
				case s.today >= goal:
					ui.Text(c, "tamam").FontSize(12.5).FontWeight(650).TextColor(cl.Down)
				default:
					ui.Text(c, strconv.Itoa(goal-s.today)+" kelime kaldı").FontSize(12.5).TextColor(cl.Muted)
				}
			})
			ui.Box(c.Key("goal-bar")).FillWidth().Height(8).Radius(rad(4)).Background(cl.Well).Draw(func(p *ui.Painter, r ui.Rect) {
				p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * frac, H: r.H}, map[bool]ui.Color{true: cl.Down, false: cl.Accent}[s.today >= goal], rad(4))
			})
			if s.today < goal && s.streak > 0 {
				ui.Text(c, strconv.Itoa(s.streak)+" günlük serin var: bugün hedefi tutturursan "+strconv.Itoa(s.streak+1)+" olur.").FontSize(12).TextColor(cl.Muted)
			}
		})
	}
	ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, locales.StatsDailyGoalWords()).FontSize(13).TextColor(cl.Muted).Grow(1)
		g := goal
		a.countField(c, "write-goal", &g, locales.StatsDailyGoal2()).Width(130)
		if g != a.settings.WriteGoal {
			a.settings.WriteGoal = g
			a.saveSettings()
		}
	})
	a.writingChart(c, cl, s)
	statHead(c, locales.StatsYear(), "Her kare bir gün; koyu olan çok yazılandır.")
	a.yearHeat(c, cl, now)
	grid, best, hi := a.hourGrid()
	statHead(c, locales.StatsWhenDoIWrite(), "Gün ve saate göre yazılan kelime.")
	a.hourHeat(c, cl, grid, hi)
	if hi > 0 {
		ui.Text(c, locales.StatsMostFruitful(dayName((firstWeekday()+time.Weekday(best[0]))%7, false), clockTime(time.Date(2000, 1, 1, best[1], 0, 0, 0, time.Local)), hi)).FontSize(13).TextColor(cl.Text2)
	} else {
		ui.Text(c, locales.StatsNoHourlyDataYet()).FontSize(13).TextColor(cl.Muted)
	}
	ses := a.writingSessions()
	long, wpm, avg := sessionStats(ses)
	statHead(c, locales.StatsWritingSessions(), "Beş dakikadan uzun bir ara oturumu bitirir. Hız, en az yarım dakikalık oturumlardan hesaplanır.")
	ui.Row(c.Key("ses-tiles")).FillWidth().Gap(10).Wrap().Children(func() {
		statTile(c, "ses-n", locales.StatsSessions(), strconv.Itoa(len(ses)))
		statTile(c, "ses-long", "en uzun oturum"+map[bool]string{true: " (" + strconv.Itoa(long.Words) + " kelime)", false: ""}[long.Secs > 0], map[bool]string{true: duration(long.Secs), false: "—"}[long.Secs > 0])
		statTile(c, "ses-wpm", locales.StatsAverageSpeed(), map[bool]string{true: strconv.Itoa(int(wpm)) + " kelime/dk", false: "—"}[wpm > 0])
		statTile(c, "ses-avg", locales.StatsAverageLength(), map[bool]string{true: strconv.FormatFloat(avg, 'f', 1, 64) + " dk", false: "—"}[avg > 0])
	})
	statHead(c, locales.StatsWhereIWrite(), "")
	if tags := top(a.sumBy(func(u *dayUse) map[string]int { return u.Tags }), 6); len(tags) > 0 {
		ui.Text(c, locales.CommonTags()).FontSize(13).FontWeight(620).TextColor(cl.Muted)
		statList(c, "by-tag", tags, "", nil)
	}
	ui.Row(c).FillWidth().Gap(18).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, "Koleksiyonlar").FontSize(13).FontWeight(620).TextColor(cl.Muted)
			statList(c, "by-coll", top(a.writtenIn(false), 6), "Henüz yok.", nil)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, locales.StatsMostWrittenPages()).FontSize(13).FontWeight(620).TextColor(cl.Muted)
			var rows []count
			for _, e := range top(a.writtenIn(true), 6) {
				if p := a.pageAt(e.Name); p != nil {
					rows = append(rows, count{p.Title, e.N})
				}
			}
			statList(c, "by-page", rows, "Henüz yok.", func(r count) {
				for _, e := range top(a.writtenIn(true), 12) {
					if p := a.pageAt(e.Name); p != nil && p.Title == r.Name {
						a.open(p)
						return
					}
				}
			})
		})
	})
}

// pageAt is the page of a folder, or nil.
func (a *app) pageAt(dir string) *Page {
	for _, c := range a.lib.Collections {
		for _, p := range c.All() {
			if p.Dir == dir {
				return p
			}
		}
	}
	return nil
}

func (a *app) statsUse(c *ui.Context, cl ryugo.Colors, now, weekFrom time.Time) {
	statHead(c, locales.StatsUse(), "Uygulamanın açık olup bir şey yaptığın süre: son işlemden bir dakika sonra sayılmaz.")
	opened := func(u *dayUse) int { return u.Opened }
	created := func(u *dayUse) int { return u.Created }
	deleted := func(u *dayUse) int { return u.Deleted }
	searches := func(u *dayUse) int { return u.Searches }
	ui.Row(c.Key("use-tiles")).FillWidth().Gap(10).Wrap().Children(func() {
		statTile(c, "use-today", "bugün kullanım", duration(a.activeOver(now, now)))
		statTile(c, "use-week", "bu hafta kullanım", duration(a.activeOver(weekFrom, now)))
		statTile(c, "use-open", "açılan sayfa (hafta)", strconv.Itoa(a.useOver(weekFrom, now, opened)))
		statTile(c, "use-new", "yeni sayfa (hafta)", strconv.Itoa(a.useOver(weekFrom, now, created)))
		statTile(c, "use-del", "silinen (hafta)", strconv.Itoa(a.useOver(weekFrom, now, deleted)))
		statTile(c, "use-search", "arama (hafta)", strconv.Itoa(a.useOver(weekFrom, now, searches)))
	})
	ui.Row(c).FillWidth().Gap(18).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, locales.StatsMostUsedCommands()).FontSize(13).FontWeight(620).TextColor(cl.Muted)
			statList(c, "by-cmd", top(a.usage.Cmds, 8), "Henüz komut çalıştırmadın.", nil)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Text(c, locales.StatsMostOpenedPages()).FontSize(13).FontWeight(620).TextColor(cl.Muted)
			var rows []count
			for _, e := range top(a.usage.Opens, 8) {
				if p := a.pageAt(e.Name); p != nil {
					rows = append(rows, count{p.Title, e.N})
				}
			}
			statList(c, "by-open", rows, "Henüz yok.", func(r count) {
				for _, e := range top(a.usage.Opens, 16) {
					if p := a.pageAt(e.Name); p != nil && p.Title == r.Name {
						a.open(p)
						return
					}
				}
			})
		})
	})
	var timed []count
	for _, e := range top(a.sumBy(func(u *dayUse) map[string]int { return u.PageSecs }), 6) {
		if p := a.pageAt(e.Name); p != nil {
			timed = append(timed, count{p.Title + " · " + duration(e.N), e.N})
		}
	}
	if len(timed) > 0 {
		ui.Text(c, locales.StatsPagesYouSpentMost()).FontSize(13).FontWeight(620).TextColor(cl.Muted)
		statList(c, "by-time", timed, "", nil)
	}
	if un := a.unusedFeatures(6); len(un) > 0 {
		ui.Text(c, locales.StatsNeverUsedList(strings.Join(un, " · "))).FontSize(12.5).TextColor(cl.Muted).Wrap()
	}
}

// unusedFeatures are n of the commands that have not been run, for a hint to look at them.
func (a *app) unusedFeatures(n int) []string {
	var out []string
	seen := map[string]bool{}
	for _, it := range a.commands() {
		if it.sub != "Komut" || it.off != "" {
			continue
		}
		name, _, _ := strings.Cut(it.title, ": ")
		if seen[name] || a.usage.Cmds[name] > 0 || strings.Contains(it.title, "Yedekten") || strings.Contains(it.title, "→") {
			continue
		}
		seen[name] = true
		out = append(out, name)
		if len(out) >= n {
			break
		}
	}
	return out
}

func (a *app) statsLibrary(c *ui.Context, cl ryugo.Colors) {
	l := a.libStatsNow()
	statHead(c, locales.CommonLibrary(), "")
	ui.Row(c.Key("lib-tiles")).FillWidth().Gap(10).Wrap().Children(func() {
		statTile(c, "lib-coll", locales.CommonCollections(), strconv.Itoa(l.Collections))
		statTile(c, "lib-pages", locales.CommonPages(), strconv.Itoa(l.Pages))
		statTile(c, "lib-words", locales.StatsWords2(), strconv.Itoa(l.Words))
		statTile(c, "lib-tags", locales.CommonTags2(), strconv.Itoa(l.Tags))
		statTile(c, "lib-links", locales.StatsLinks(), strconv.Itoa(l.Links))
		statTile(c, "lib-tasks", locales.StatsTasks(), strconv.Itoa(l.Tasks))
	})
	if l.Oldest != nil && l.Newest != nil {
		ui.Text(c, locales.StatsOldestNewest(l.Oldest.Title, shortDate(l.Oldest.Modified), l.Newest.Title, dateTime(l.Newest.Modified, false))).FontSize(12.5).TextColor(cl.Muted).Wrap()
	}
	ui.Text(c, locales.StatsBiggestPages()).FontSize(13).FontWeight(620).TextColor(cl.Muted)
	statList(c, "big-pages", l.Biggest, "Sayfa yok.", nil)
	statHead(c, locales.StatsLibraryHealth(), "Temizlik önerisi: tıklayınca sayfa açılır.")
	health := func(key, title string, pages []*Page) {
		if len(pages) == 0 {
			ui.Text(c, title+": yok ✓").FontSize(13).TextColor(cl.Down)
			return
		}
		ui.Text(c, title+": "+strconv.Itoa(len(pages))).FontSize(13).FontWeight(620).TextColor(cl.Warn)
		var rows []count
		for i, p := range pages {
			if i >= 6 {
				break
			}
			rows = append(rows, count{p.Title, 1})
		}
		ui.Row(c.Key(key)).FillWidth().Gap(6).Wrap().Children(func() {
			for i, r := range rows {
				p := pages[i]
				b := ui.Row(c.Key(key+":"+strconv.Itoa(i))).Padding(4, 10).Radius(rad(7)).Background(cl.Fill).Label(r.Name).Role(ui.RoleButton).Cursor(ui.CursorPointer)
				b.Children(func() { ui.Text(c, r.Name).FontSize(12.5).TextColor(cl.Text).MaxLines(1) })
				if b.Clicked() {
					a.open(p)
				}
			}
			if len(pages) > len(rows) {
				ui.Text(c, "… "+strconv.Itoa(len(pages)-len(rows))+" tane daha").FontSize(12).TextColor(cl.Muted)
			}
		})
	}
	health("orph", "Yetim sayfa (ne bağlanan ne bağlanılan)", l.Orphans)
	health("empty", "Boş sayfa", l.Empty)
	if len(l.Broken) == 0 {
		ui.Text(c, locales.StatsBrokenLinksNone()).FontSize(13).TextColor(cl.Down)
	} else {
		ui.Text(c, locales.StatsBrokenLinkCount(len(l.Broken))).FontSize(13).FontWeight(620).TextColor(cl.Warn)
		statList(c, "broken", l.Broken, "", nil)
	}
	if len(l.Dupes) == 0 {
		ui.Text(c, locales.StatsPagesWithSameTitle()).FontSize(13).TextColor(cl.Down)
	} else {
		ui.Text(c, locales.StatsSameTitleList(strings.Join(l.Dupes, ", "))).FontSize(13).TextColor(cl.Warn).Wrap()
	}
}

func (a *app) statsData(c *ui.Context, cl ryugo.Colors) {
	statHead(c, locales.StatsData(), "Rakamlar düz JSON/CSV olarak her an dışa aktarılır; hiçbiri ağa gitmez, senkronizasyon açılsa bile ayrı bir seçenek olmadan gönderilmez.")
	ui.Row(c.Key("stats-actions")).FillWidth().Gap(8).Wrap().Children(func() {
		if ryugo.Button(c.Key("st-csv"), "CSV olarak dışa aktar", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			a.notice = a.exportStatsCSV()
		}
		if ryugo.Button(c.Key("st-json"), "JSON olarak dışa aktar", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			a.notice = a.exportStatsJSON()
		}
		if ryugo.Button(c.Key("st-week-md"), "Haftalık özeti günlüğe yaz", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			a.writeWeekSummary()
		}
		label := "Sıfırla…"
		if a.statReset {
			label = "Emin misin? Hepsi silinir: tekrar tıkla"
		}
		if ryugo.Button(c.Key("st-reset"), label, ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			if a.statReset {
				a.resetUsage()
				a.statReset = false
				a.notice = "İstatistikler silindi."
			} else {
				a.statReset = true
			}
		}
	})
	ui.Text(c, locales.StatsHowCountedWordPiece()).FontSize(12).TextColor(cl.Muted).Wrap()
}

// writeWeekSummary puts the week's summary at the end of today's page of the journal.
func (a *app) writeWeekSummary() {
	md := a.weekSummary()
	p, err := a.todayPage()
	if err != nil {
		a.notice = "Olmadı: " + err.Error()
		return
	}
	a.open(p)
	if a.doc == nil {
		return
	}
	n := a.doc.Text().Len()
	a.doc.SetSelection(n, n)
	a.doc.InsertText("\n")
	a.doc.Insert(a.parsePage(p, encodeBody(importMarkdown(md))))
	a.notice = "Haftalık özet günlüğe yazıldı."
}

// yearHeat draws the year as a grid of days, a column a week.
func (a *app) yearHeat(c *ui.Context, cl ryugo.Colors, now time.Time) {
	hi := 1
	start := weekStart(now).AddDate(0, 0, -52*7)
	for d := start; !d.After(now); d = d.AddDate(0, 0, 1) {
		hi = max(hi, a.writtenOn(d))
	}
	ui.Box(c.Key("year-heat")).FillWidth().Height(7*14 + 2).Label(locales.StatsYearlyWritingHeatMap()).Draw(func(p *ui.Painter, r ui.Rect) {
		cell := min(float32(13), (r.W-52*2)/53)
		col := 0
		for d := start; !d.After(now); d = d.AddDate(0, 0, 1) {
			wd := weekIndex(d)
			if wd == 0 && !d.Equal(start) {
				col++
			}
			n := a.writtenOn(d)
			f := cl.Fill
			if n > 0 {
				f = cl.Accent.Alpha(0.25 + 0.75*float32(n)/float32(hi))
			}
			p.Fill(ui.Rect{X: r.X + float32(col)*(cell+2), Y: r.Y + float32(wd)*(cell+2), W: cell, H: cell}, f, rad(3))
		}
	})
}

// hourHeat draws the weekday by hour grid.
func (a *app) hourHeat(c *ui.Context, cl ryugo.Colors, grid [7][24]int, hi int) {
	ui.Box(c.Key("hour-heat")).FillWidth().Height(7*18 + 22).Label(locales.StatsWordsWrittenByHour()).Draw(func(p *ui.Painter, r ui.Rect) {
		left := float32(34)
		cell := (r.W - left) / 24
		heads := weekdayHeads(false)
		for wd := 0; wd < 7; wd++ {
			sp := ui.Span{Text: heads[wd], Size: 11, Color: cl.Muted}
			p.RichText(r.X, r.Y+float32(wd)*18+2, left, sp)
			for h := 0; h < 24; h++ {
				n := grid[wd][h]
				f := cl.Fill
				if n > 0 && hi > 0 {
					f = cl.Accent.Alpha(0.2 + 0.8*float32(n)/float32(hi))
				}
				p.Fill(ui.Rect{X: r.X + left + float32(h)*cell, Y: r.Y + float32(wd)*18, W: cell - 2, H: 16}, f, rad(3))
			}
		}
		for _, h := range []int{0, 6, 12, 18, 23} {
			sp := ui.Span{Text: pad(h, 2), Size: 10.5, Color: cl.Muted}
			p.RichText(r.X+left+float32(h)*cell, r.Y+7*18+4, 24, sp)
		}
	})
}
