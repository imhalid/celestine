package main

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// What is written and what is used, by the day, kept on this computer alone: in the library's file, or
// beside the settings. The words are those of Settings.Writing (writing.go); here are the hours they were
// written in, the sessions, the time the app was used, the pages opened, made and thrown away, the
// searches, the commands and the pages most written in. Counting costs a few additions in memory; it is
// written every little while. One switch turns it off (Settings.NoStats); it can be exported as CSV or JSON,
// and erased.

const (
	usageKeep      = 800 // days kept
	usageKey       = "usage"
	sessionGap     = 5 * time.Minute // a pause longer than this ends a writing session
	activeWindow   = 60 * time.Second
	sessionsKept   = 300
	usageSaveEvery = 20 * time.Second
)

// A dayUse is what happened on one day.
type dayUse struct {
	Active   int            `json:"active,omitempty"`   // seconds the app was used
	Opened   int            `json:"opened,omitempty"`   // pages opened
	Created  int            `json:"created,omitempty"`  // pages made
	Deleted  int            `json:"deleted,omitempty"`  // pages put in the trash
	Searches int            `json:"searches,omitempty"` // searches over the library
	Hours    [24]int        `json:"hours"`              // words, by the hour they were written in
	Colls    map[string]int `json:"colls,omitempty"`    // words, by collection
	Pages    map[string]int `json:"pages,omitempty"`    // words, by page (its folder)
	Tags     map[string]int `json:"tags,omitempty"`     // words, by the tags of the page they were written in
	PageSecs map[string]int `json:"pageSecs,omitempty"` // seconds of use, by the page that was open
}

// A session is a stretch of writing with no pause of five minutes in it.
type session struct {
	Start int64 `json:"start"` // unix seconds
	Secs  int   `json:"secs"`
	Words int   `json:"words"`
}

// usageData is what is kept.
type usageData struct {
	Days     map[string]*dayUse `json:"days"`
	Sessions []session          `json:"sessions,omitempty"`
	Cmds     map[string]int     `json:"cmds,omitempty"`  // commands run, in all
	Opens    map[string]int     `json:"opens,omitempty"` // pages opened, in all, by folder
}

func newUsage() *usageData {
	return &usageData{Days: map[string]*dayUse{}, Cmds: map[string]int{}, Opens: map[string]int{}}
}

// usageFile is where the figures are kept when the library is not a file: beside the settings.
func (a *app) usageFile() string {
	if a.settingsFile != "" {
		return filepath.Join(filepath.Dir(a.settingsFile), "usage.json")
	}
	return ""
}

// loadUsage reads what was kept.
func (a *app) loadUsage() {
	a.usage = newUsage()
	var data []byte
	if a.settingsStore != nil {
		data = a.settingsStore.Meta(usageKey)
	} else if f := a.usageFile(); f != "" {
		data, _ = os.ReadFile(f)
	}
	if data != nil {
		json.Unmarshal(data, a.usage)
	}
	if a.usage.Days == nil {
		a.usage.Days = map[string]*dayUse{}
	}
	if a.usage.Cmds == nil {
		a.usage.Cmds = map[string]int{}
	}
	if a.usage.Opens == nil {
		a.usage.Opens = map[string]int{}
	}
}

// saveUsage writes it, now.
func (a *app) saveUsage() {
	if a.usage == nil {
		return
	}
	a.usageDirty = false
	data, _ := json.Marshal(a.usage)
	if a.settingsStore != nil {
		a.settingsStore.SetMeta(usageKey, data)
		return
	}
	if f := a.usageFile(); f != "" {
		os.MkdirAll(filepath.Dir(f), 0o755)
		os.WriteFile(f, data, 0o644)
	}
}

// useDay is today's figures, made when there are none; nil with the counting off.
func (a *app) useDay() *dayUse {
	if a.settings.NoStats {
		return nil
	}
	if a.usage == nil {
		a.loadUsage()
	}
	key := a.clock().Format("2006-01-02")
	d := a.usage.Days[key]
	if d == nil {
		d = &dayUse{}
		a.usage.Days[key] = d
		if len(a.usage.Days) > usageKeep {
			var keys []string
			for k := range a.usage.Days {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys[:len(keys)-usageKeep] {
				delete(a.usage.Days, k)
			}
		}
	}
	a.usageDirty = true
	return d
}

// keepUsage writes the figures when they changed and some time has gone since the last time.
func (a *app) keepUsage() {
	if a.usageDirty && a.clock().Sub(a.usageSaved) > usageSaveEvery {
		a.usageSaved = a.clock()
		a.saveUsage()
	}
}

// stirred notes that the app is being used now: the time it is, till a minute of nothing, is counted.
func (a *app) stirred() { a.activeAt = a.clock() }

// statTick counts the time since the last frame as use, while there was something done in the last minute.
func (a *app) statTick() {
	now := a.clock()
	if a.doc != nil && a.doc.Version() != a.statVer {
		a.statVer = a.doc.Version()
		a.stirred()
	}
	last := a.statLast
	a.statLast = now
	if last.IsZero() || a.settings.NoStats || now.Sub(a.activeAt) > activeWindow {
		return
	}
	if dt := now.Sub(last); dt > 0 {
		if d := a.useDay(); d != nil {
			d.Active += int(dt.Round(time.Second).Seconds())
			if a.page != nil && !a.noPage() && dt >= time.Second {
				if d.PageSecs == nil {
					d.PageSecs = map[string]int{}
				}
				d.PageSecs[a.page.Dir] += int(dt.Round(time.Second).Seconds())
			}
			if dt < time.Second { // a frame is a few milliseconds: they are gathered, not lost
				a.activeFrac += dt
				if a.activeFrac >= time.Second {
					d.Active += int(a.activeFrac.Seconds())
					a.activeFrac = 0
				}
			}
		}
	}
	a.keepUsage()
}

// noteWords counts the words a save added: by hour, by collection, by page, and in the session.
func (a *app) noteWords(p *Page, n int) {
	d := a.useDay()
	if d == nil || n <= 0 {
		return
	}
	now := a.clock()
	a.stirred()
	d.Hours[now.Hour()] += n
	if p != nil {
		if d.Colls == nil {
			d.Colls = map[string]int{}
		}
		d.Colls[p.Coll.Name] += n
		if d.Pages == nil {
			d.Pages = map[string]int{}
		}
		d.Pages[p.Dir] += n
		if len(d.Pages) > 60 { // a day keeps the pages it wrote most in
			type kv struct {
				k string
				n int
			}
			var all []kv
			for k, v := range d.Pages {
				all = append(all, kv{k, v})
			}
			sort.Slice(all, func(i, j int) bool { return all[i].n > all[j].n })
			for _, e := range all[40:] {
				delete(d.Pages, e.k)
			}
		}
	}
	if p != nil {
		if e := a.links.pages[p]; e != nil && len(e.tags) > 0 {
			if d.Tags == nil {
				d.Tags = map[string]int{}
			}
			for _, tg := range e.tags {
				d.Tags[tg] += n
			}
		}
	}
	// the session: a gap of five minutes ends it
	s := &a.sess
	if s.Words > 0 && now.Sub(a.sessLast) > sessionGap {
		a.endSession()
	}
	if s.Words == 0 {
		s.Start = now.Unix()
	}
	s.Words += n
	s.Secs = int(now.Sub(time.Unix(s.Start, 0)).Seconds()) + 1
	a.sessLast = now
}

// endSession puts the session that was going in the list.
func (a *app) endSession() {
	if a.sess.Words > 0 && a.usage != nil && !a.settings.NoStats {
		a.usage.Sessions = append(a.usage.Sessions, a.sess)
		if len(a.usage.Sessions) > sessionsKept {
			a.usage.Sessions = a.usage.Sessions[len(a.usage.Sessions)-sessionsKept:]
		}
		a.usageDirty = true
	}
	a.sess = session{}
}

// noteOpened counts a page opened.
func (a *app) noteOpened(p *Page) {
	if d := a.useDay(); d != nil {
		d.Opened++
		a.usage.Opens[p.Dir]++
		if len(a.usage.Opens) > 400 {
			for k, v := range a.usage.Opens {
				if v <= 1 {
					delete(a.usage.Opens, k)
				}
			}
		}
	}
	a.stirred()
}

func (a *app) noteCreated() {
	if d := a.useDay(); d != nil {
		d.Created++
	}
	a.stirred()
}

func (a *app) noteDeleted() {
	if d := a.useDay(); d != nil {
		d.Deleted++
	}
	a.stirred()
}

func (a *app) noteSearch() {
	if d := a.useDay(); d != nil {
		d.Searches++
	}
	a.stirred()
}

// noteCommand counts a command run.
func (a *app) noteCommand(title string) {
	if d := a.useDay(); d != nil {
		name, _, _ := strings.Cut(title, ": ") // the kind, not what it was set to
		name, _, _ = strings.Cut(name, " → ")
		a.usage.Cmds[name]++
		if len(a.usage.Cmds) > 300 {
			for k, v := range a.usage.Cmds {
				if v <= 1 {
					delete(a.usage.Cmds, k)
				}
			}
		}
	}
	a.stirred()
}

// resetUsage erases the figures: the usage, and the words by day.
func (a *app) resetUsage() {
	a.usage = newUsage()
	a.settings.Writing = nil
	a.sess = session{}
	a.saveUsage()
	a.saveSettings()
}

// ---- reading what was kept ----

// A DayRow is a day of the figures, with its words.
type dayRow struct {
	Day                                               string
	Words, Active, Opened, Created, Deleted, Searches int
}

// dayRows are all the days with anything in them, the first first.
func (a *app) dayRows() []dayRow {
	if a.usage == nil {
		a.loadUsage()
	}
	keys := map[string]bool{}
	for k := range a.usage.Days {
		keys[k] = true
	}
	for k := range a.settings.Writing {
		keys[k] = true
	}
	var days []string
	for k := range keys {
		days = append(days, k)
	}
	sort.Strings(days)
	out := make([]dayRow, 0, len(days))
	for _, k := range days {
		r := dayRow{Day: k, Words: a.settings.Writing[k]}
		if d := a.usage.Days[k]; d != nil {
			r.Active, r.Opened, r.Created, r.Deleted, r.Searches = d.Active, d.Opened, d.Created, d.Deleted, d.Searches
		}
		out = append(out, r)
	}
	return out
}

// activeOver is the seconds of use in the days from..to (inclusive).
func (a *app) activeOver(from, to time.Time) int {
	if a.usage == nil {
		a.loadUsage()
	}
	n := 0
	for d := startOfDay(from); !d.After(to); d = d.AddDate(0, 0, 1) {
		if u := a.usage.Days[d.Format("2006-01-02")]; u != nil {
			n += u.Active
		}
	}
	return n
}

// useOver sums a count of the days from..to.
func (a *app) useOver(from, to time.Time, f func(*dayUse) int) int {
	if a.usage == nil {
		a.loadUsage()
	}
	n := 0
	for d := startOfDay(from); !d.After(to); d = d.AddDate(0, 0, 1) {
		if u := a.usage.Days[d.Format("2006-01-02")]; u != nil {
			n += f(u)
		}
	}
	return n
}

// hourGrid is the words written by weekday (Monday first) and hour, over all the days kept.
func (a *app) hourGrid() (grid [7][24]int, best [2]int, hi int) {
	if a.usage == nil {
		a.loadUsage()
	}
	for k, u := range a.usage.Days {
		t, err := time.Parse("2006-01-02", k)
		if err != nil {
			continue
		}
		wd := (int(t.Weekday()) + 6) % 7
		for h, n := range u.Hours {
			grid[wd][h] += n
			if grid[wd][h] > hi {
				hi, best = grid[wd][h], [2]int{wd, h}
			}
		}
	}
	return grid, best, hi
}

// A count is a name and a number, for the lists of most.
type count struct {
	Name string
	N    int
}

func top(m map[string]int, n int) []count {
	var all []count
	for k, v := range m {
		all = append(all, count{k, v})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].N != all[j].N {
			return all[i].N > all[j].N
		}
		return all[i].Name < all[j].Name
	})
	if len(all) > n {
		all = all[:n]
	}
	return all
}

// writtenIn sums the words by collection (or page) over the days kept.
func (a *app) writtenIn(pages bool) map[string]int {
	return a.sumBy(func(u *dayUse) map[string]int {
		if pages {
			return u.Pages
		}
		return u.Colls
	})
}

// sumBy sums one of the days' maps over all the days kept.
func (a *app) sumBy(pick func(*dayUse) map[string]int) map[string]int {
	if a.usage == nil {
		a.loadUsage()
	}
	out := map[string]int{}
	for _, u := range a.usage.Days {
		src := pick(u)
		for k, v := range src {
			out[k] += v
		}
	}
	return out
}

// writingSessions are the sessions, with the one that goes on.
func (a *app) writingSessions() []session {
	if a.usage == nil {
		a.loadUsage()
	}
	out := append([]session(nil), a.usage.Sessions...)
	if a.sess.Words > 0 {
		out = append(out, a.sess)
	}
	return out
}

// sessionStats are the longest session, the average speed and the average length of a session.
func sessionStats(list []session) (longest session, wpm float64, avgMin float64) {
	var words, secs int
	for _, s := range list {
		if s.Secs > longest.Secs {
			longest = s
		}
		if s.Secs >= 30 { // under half a minute is no speed
			words += s.Words
			secs += s.Secs
		}
	}
	if secs > 0 {
		wpm = float64(words) / (float64(secs) / 60)
	}
	if len(list) > 0 {
		t := 0
		for _, s := range list {
			t += s.Secs
		}
		avgMin = float64(t) / float64(len(list)) / 60
	}
	return
}

// ---- the library ----

type libStats struct {
	Collections, Pages, Tags, Links, Tasks int
	Words                                  int
	Biggest                                []count
	Oldest, Newest                         *Page
	Orphans, Empty                         []*Page
	Broken                                 []count  // [[links]] to nothing, by page title
	Dupes                                  []string // titles used by more than one page
}

var dayKey = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// libraryStats reads the library: the counts, the biggest pages, and what needs tidying.
func (a *app) libraryStats() libStats {
	var s libStats
	a.linksStale() // the index keeps what each page said and reads again only a page that changed, so this is cheap
	a.linksTo(a.page, a.clock(), true)
	titles := map[string][]*Page{}
	sizes := map[string]int{}
	for _, c := range a.lib.Collections {
		s.Collections++
		for _, p := range c.All() {
			s.Pages++
			key := strings.ToLower(strings.TrimSpace(p.Title))
			titles[key] = append(titles[key], p)
			if !p.Modified.IsZero() {
				if s.Oldest == nil || p.Modified.Before(s.Oldest.Modified) {
					s.Oldest = p
				}
				if s.Newest == nil || p.Modified.After(s.Newest.Modified) {
					s.Newest = p
				}
			}
			e := a.links.pages[p]
			if e != nil {
				s.Words += e.words
				sizes[p.Title+" · "+c.Name] = e.words
				if e.empty {
					s.Empty = append(s.Empty, p)
				}
				s.Tasks += len(e.tasks)
				s.Links += len(e.links)
				if len(e.links) == 0 && len(a.links.into[key]) == 0 && !c.Journal {
					s.Orphans = append(s.Orphans, p)
				}
			}
		}
	}
	s.Tags = len(a.links.tags)
	s.Biggest = top(sizes, 8)
	broken := map[string]int{}
	for p, e := range a.links.pages {
		for _, l := range e.links {
			if _, ok := titles[l]; !ok && !dayKey.MatchString(l) {
				broken[p.Title]++
			}
		}
	}
	s.Broken = top(broken, 10)
	for k, ps := range titles {
		if len(ps) > 1 {
			s.Dupes = append(s.Dupes, ps[0].Title+" ("+strconv.Itoa(len(ps))+")")
			_ = k
		}
	}
	sort.Strings(s.Dupes)
	return s
}

// ---- out ----

// exportStatsCSV writes the days as CSV beside the settings, and says where.
func (a *app) exportStatsCSV() string {
	dir := filepath.Dir(a.settingsFile)
	if a.settingsFile == "" {
		dir = os.TempDir()
	}
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "istatistik-"+a.clock().Format("20060102-150405")+".csv")
	f, err := os.Create(path)
	if err != nil {
		return "Olmadı: " + err.Error()
	}
	w := csv.NewWriter(f)
	w.Write([]string{"gün", "kelime", "kullanım_sn", "açılan", "yeni", "silinen", "arama"})
	for _, r := range a.dayRows() {
		w.Write([]string{r.Day, strconv.Itoa(r.Words), strconv.Itoa(r.Active), strconv.Itoa(r.Opened), strconv.Itoa(r.Created), strconv.Itoa(r.Deleted), strconv.Itoa(r.Searches)})
	}
	w.Flush()
	if err := f.Close(); err != nil || w.Error() != nil {
		return "Olmadı: yazılamadı"
	}
	return "Yazıldı: " + path
}

// exportStatsJSON writes all of it as JSON beside the settings.
func (a *app) exportStatsJSON() string {
	if a.usage == nil {
		a.loadUsage()
	}
	out := struct {
		Words map[string]int `json:"words"`
		*usageData
	}{a.settings.Writing, a.usage}
	data, _ := json.MarshalIndent(out, "", "  ")
	dir := filepath.Dir(a.settingsFile)
	if a.settingsFile == "" {
		dir = os.TempDir()
	}
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "istatistik-"+a.clock().Format("20060102-150405")+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "Olmadı: " + err.Error()
	}
	return "Yazıldı: " + path
}

// duration says seconds as "2 sa 5 dk" or "40 sn".
func duration(secs int) string {
	switch {
	case secs >= 3600:
		return strconv.Itoa(secs/3600) + " sa " + strconv.Itoa(secs%3600/60) + " dk"
	case secs >= 60:
		return strconv.Itoa(secs/60) + " dk"
	}
	return strconv.Itoa(secs) + " sn"
}

// weekSummary is the week up to today in Markdown: for the journal.
func (a *app) weekSummary() string {
	now := startOfDay(a.clock())
	from := now.AddDate(0, 0, -6)
	words := 0
	for d := from; !d.After(now); d = d.AddDate(0, 0, 1) {
		words += a.writtenOn(d)
	}
	var b strings.Builder
	b.WriteString("## Haftalık özet: " + from.Format("2 Jan") + " – " + now.Format("2 Jan 2006") + "\n\n")
	b.WriteString("- Yazılan: " + strconv.Itoa(words) + " kelime\n")
	b.WriteString("- Kullanım: " + duration(a.activeOver(from, now)) + "\n")
	b.WriteString("- Açılan sayfa: " + strconv.Itoa(a.useOver(from, now, func(u *dayUse) int { return u.Opened })) + ", yeni: " + strconv.Itoa(a.useOver(from, now, func(u *dayUse) int { return u.Created })) + "\n")
	cs := map[string]int{}
	for d := from; !d.After(now); d = d.AddDate(0, 0, 1) {
		if u := a.usage.Days[d.Format("2006-01-02")]; u != nil {
			for k, v := range u.Colls {
				cs[k] += v
			}
		}
	}
	if t := top(cs, 3); len(t) > 0 {
		b.WriteString("- En çok yazılan koleksiyonlar: ")
		for i, e := range t {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(e.Name + " (" + strconv.Itoa(e.N) + ")")
		}
		b.WriteString("\n")
	}
	return b.String()
}
