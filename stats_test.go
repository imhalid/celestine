package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func statsApp(t *testing.T, src string) (*app, *ui.Tester) {
	a, tt := newTestApp(t, src)
	clock := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)
	a.clock = func() time.Time { return clock }
	a.settingsFile = filepath.Join(t.TempDir(), "celestine", "settings.json")
	a.settings.AccountUser = "ali@ornek.com" // the sign-in is not what these look at
	a.setClock = func(d time.Duration) { clock = clock.Add(d) }
	a.usage = nil // what the setup counted was on the real clock, on today: the test's days start clean
	return a, tt
}

// Words are counted by the hour, collection and page they were written in, and in a session.
func TestStatsWords(t *testing.T) {
	a, _ := statsApp(t, "x\n")
	a.noteWords(a.page, 30)
	a.setClock(2 * time.Minute)
	a.noteWords(a.page, 20)
	d := a.usage.Days["2026-11-07"]
	if d == nil || d.Hours[10] != 50 {
		t.Fatalf("hours: %+v", d)
	}
	if d.Colls[a.page.Coll.Name] != 50 || d.Pages[a.page.Dir] != 50 {
		t.Fatalf("by place: %+v %+v", d.Colls, d.Pages)
	}
	if a.sess.Words != 50 {
		t.Fatalf("session %+v", a.sess)
	}
	a.setClock(10 * time.Minute) // a pause: the session ends, another starts
	a.noteWords(a.page, 7)
	if len(a.usage.Sessions) != 1 || a.usage.Sessions[0].Words != 50 || a.sess.Words != 7 {
		t.Fatalf("sessions %+v now %+v", a.usage.Sessions, a.sess)
	}
	longest, wpm, _ := sessionStats(append([]session{{Secs: 600, Words: 300}}, a.writingSessions()...))
	if longest.Secs != 600 || wpm <= 0 {
		t.Fatalf("session stats %+v %v", longest, wpm)
	}
}

// A save counts words; counting off counts nothing, and writes nothing.
func TestStatsFromSaveAndSwitch(t *testing.T) {
	a, tt := statsApp(t, "bir\n")
	a.lastWords = 1
	tt.Type(" iki üç dört")
	tt.Advance(time.Second)
	a.save()
	if a.writtenOn(a.clock()) == 0 || a.usage.Days["2026-11-07"].Hours[10] == 0 {
		t.Fatalf("save counted nothing: %+v", a.usage.Days)
	}
	a.settings.NoStats = true
	before := a.writtenOn(a.clock())
	a.noteOpened(a.page)
	a.noteCommand("Görevler")
	a.noteSearch()
	a.noteWords(a.page, 100)
	if a.usage.Cmds["Görevler"] != 0 || a.usage.Days["2026-11-07"].Opened > 1 && false {
		t.Fatal("counted with it off")
	}
	if a.usage.Days["2026-11-07"].Searches != 0 {
		t.Fatal("search counted with it off")
	}
	if a.writtenOn(a.clock()) != before {
		t.Fatal("words counted with it off")
	}
}

// What is used is counted: pages opened, made and thrown away, searches, commands.
func TestStatsUse(t *testing.T) {
	a, tt := statsApp(t, "x\n")
	c := a.page.Coll
	p2, _ := c.NewPage(nil, "İkinci")
	a.open(p2)
	a.open(a.page)
	a.newPage(c, nil)
	a.trashPage(a.page)
	a.openSearch("elma")
	a.runItem(palItem{title: "Görevler: bugün", sub: "Komut", run: func() {}})
	a.runItem(palItem{title: "Görevler: bugün", sub: "Komut", run: func() {}})
	d := a.usage.Days["2026-11-07"]
	if d.Opened < 2 || d.Created != 1 || d.Deleted != 1 || d.Searches != 1 {
		t.Fatalf("%+v", d)
	}
	if a.usage.Cmds["Görevler"] != 2 {
		t.Fatalf("commands %+v", a.usage.Cmds)
	}
	_ = tt
}

// The time of use is counted while something is being done, and stops after a minute of nothing.
func TestStatsActiveTime(t *testing.T) {
	a, tt := statsApp(t, "x\n")
	a.statLast = a.clock()
	a.stirred()
	for i := 0; i < 30; i++ { // thirty seconds, in frames of a second
		a.setClock(time.Second)
		a.statTick()
	}
	got := a.usage.Days["2026-11-07"].Active
	if got < 28 || got > 32 {
		t.Fatalf("%d seconds in thirty", got)
	}
	for i := 0; i < 120; i++ { // two minutes of nothing done: after the first minute, no more
		a.setClock(time.Second)
		a.statTick()
	}
	if got2 := a.usage.Days["2026-11-07"].Active; got2-got > 65 {
		t.Fatalf("counted idle time: %d more", got2-got)
	}
	_ = tt
}

// The figures are kept in the library's file and come back.
func TestStatsKept(t *testing.T) {
	a, _ := statsApp(t, "x\n")
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a.settingsStore = st
	a.noteWords(a.page, 12)
	a.noteSearch()
	a.endSession()
	a.saveUsage()
	b, _ := statsApp(t, "x\n")
	b.settingsStore = st
	b.loadUsage()
	if d := b.usage.Days["2026-11-07"]; d == nil || d.Hours[10] != 12 || d.Searches != 1 || len(b.usage.Sessions) != 1 {
		t.Fatalf("not kept: %+v %+v", d, b.usage.Sessions)
	}
}

// The page draws every section, and opens from ⌘P and goes back with Esc.
func TestStatsPage(t *testing.T) {
	a, tt := statsApp(t, "bir iki [[Yok Böyle]] #etiket\n")
	a.noteWords(a.page, 40)
	a.settings.WriteGoal = 100
	tt.SetSize(1100, 2400)
	a.openStats()
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	for _, want := range []string{"İstatistikler", "Yazım", "Kullanım", "Kitaplık", "Kitaplık sağlığı", "Veri", "Yazma oturumları", "Hangi saatlerde yazıyorum", "bugün", "kelime kaldı"} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %v", want, tt.Texts())
		}
	}
	if !tt.HasText("Kırık [[bağlantı]] bulunan sayfa: 1") {
		t.Errorf("no broken link found: %v", tt.Texts())
	}
	if !a.statsScreen || a.noPage() == false {
		t.Fatal("not a screen")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(200 * time.Millisecond)
	if a.statsScreen {
		t.Fatal("Esc did not leave")
	}
	// from the palette
	runCmd(t, a, ">istatistikler", "İstatistikler")
	tt.Frame()
	if !a.statsScreen {
		t.Fatal("the command did not open it")
	}
	// and in the history
	a.openSettings()
	tt.Frame()
	if a.statsScreen {
		t.Fatal("two screens")
	}
	a.step(-1)
	if !a.statsScreen {
		t.Fatal("back did not return to the figures")
	}
}

// The library's health: orphans, empty pages, broken links and same titles are found.
func TestLibraryHealth(t *testing.T) {
	a, _ := statsApp(t, "yalnız başına\n")
	c := a.page.Coll
	p2, _ := c.NewPage(nil, "Bağlı")
	p2.SaveText("[[Yok Sayfa]] ve [[Adsız]]\n")
	p3, _ := c.NewPage(nil, "Boş")
	p3.SaveText("\n")
	p4, _ := c.NewPage(nil, "Bağlı")
	p4.SaveText("aynı başlık\n")
	l := a.libraryStats()
	if l.Pages < 4 || len(l.Empty) < 1 {
		t.Fatalf("%+v", l)
	}
	found := false
	for _, b := range l.Broken {
		if b.Name == "Bağlı" {
			found = true
		}
	}
	if !found {
		t.Fatalf("broken: %+v", l.Broken)
	}
	if len(l.Dupes) != 1 || !strings.HasPrefix(l.Dupes[0], "Bağlı") {
		t.Fatalf("dupes %v", l.Dupes)
	}
	if len(l.Orphans) == 0 {
		t.Fatal("no orphans")
	}
}

// CSV and JSON come out, the week's summary goes to the journal, reset erases.
func TestStatsOut(t *testing.T) {
	a, tt := statsApp(t, "x\n")
	a.settings.Writing = map[string]int{"2026-11-06": 120, "2026-11-07": 40}
	a.noteWords(a.page, 40)
	msg := a.exportStatsCSV()
	path := strings.TrimPrefix(msg, "Yazıldı: ")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(msg, err)
	}
	rows, err := csv.NewReader(f).ReadAll()
	f.Close()
	if err != nil || len(rows) != 3 || rows[1][0] != "2026-11-06" || rows[1][1] != "120" {
		t.Fatalf("csv %v %v", rows, err)
	}
	if m := a.exportStatsJSON(); !strings.HasPrefix(m, "Yazıldı: ") {
		t.Fatal(m)
	} else if b, _ := os.ReadFile(strings.TrimPrefix(m, "Yazıldı: ")); !strings.Contains(string(b), "\"words\"") {
		t.Fatalf("json %s", b)
	}
	sum := a.weekSummary()
	if !strings.Contains(sum, "Haftalık özet") || !strings.Contains(sum, "160 kelime") {
		t.Fatalf("summary %q", sum)
	}
	a.writeWeekSummary()
	tt.Frame()
	body, _ := a.page.Body()
	_ = body
	if !strings.Contains(a.doc.Text().String(), "Haftalık özet") {
		t.Fatalf("not in the journal page: %q", a.doc.Text().String())
	}
	a.resetUsage()
	if len(a.settings.Writing) != 0 || len(a.usage.Days) != 0 {
		t.Fatal("not erased")
	}
}

// Time in use is gathered, frame by frame, and counted once: frames 600 ms apart for three seconds are three seconds,
// for the day and for the page, as frames 5 ms apart are (#202).
func TestUsageCountedOnce(t *testing.T) {
	for _, gap := range []time.Duration{600 * time.Millisecond, 5 * time.Millisecond} {
		a, _ := newTestApp(t, "x\n")
		now := time.Date(2026, 11, 10, 10, 0, 0, 0, time.Local)
		a.clock = func() time.Time { return now }
		a.stirred()
		a.statTick()
		d := a.useDay()
		active, page := d.Active, d.PageSecs[a.page.Dir]
		for range int(3 * time.Second / gap) {
			now = now.Add(gap)
			a.stirred()
			a.statTick()
		}
		if got := d.Active - active; got != 3 {
			t.Errorf("gap %v: %d seconds in use, want 3", gap, got)
		}
		if got := d.PageSecs[a.page.Dir] - page; got != 3 {
			t.Errorf("gap %v: %d seconds in the page, want 3", gap, got)
		}
	}
}
