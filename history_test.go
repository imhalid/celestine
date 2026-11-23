package main

import (
	"fmt"
	"github.com/imhalid/celestine/locales"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestDiffLines(t *testing.T) {
	ops := DiffLines([]string{"a", "b", "c"}, []string{"a", "x", "c", "d"})
	var got []string
	for _, o := range ops {
		got = append(got, string(o.Kind)+o.Text)
	}
	if !equal(got, []string{"=a", "+b", "-x", "=c", "-d"}) {
		t.Fatalf("%q", got)
	}
}

// storeAt is a store whose clock is the one returned.
func storeAt(t *testing.T) (*Store, *time.Time) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	st.Now = func() time.Time { return now }
	return st, &now
}

// TestStoreVersions: a page file written over keeps what it was, at most
// every few minutes, and the copies move with the file and go with it.
func TestStoreVersions(t *testing.T) {
	st, now := storeAt(t)
	st.Mkdir("/a")
	write := func(s string) {
		st.WriteFile("/a/"+PageFileName, JoinPage(map[string]string{"title": "A"}, encodeBody(importMarkdown(s))))
	}
	write("bir") // nothing before it
	write("iki") // keeps "bir"
	write("üç")  // too soon
	*now = now.Add(6 * time.Minute)
	write("dört") // keeps "üç"
	vs := st.Versions("/a/" + PageFileName)
	if len(vs) != 2 {
		t.Fatalf("versions: %+v", vs)
	}
	if d, _ := st.VersionData("/a/"+PageFileName, vs[1].ID); !strings.Contains(string(d), "bir") {
		t.Fatalf("the oldest: %q", d)
	}
	if d, _ := st.VersionData("/a/"+PageFileName, vs[0].ID); !strings.Contains(string(d), "üç") {
		t.Fatalf("the latest: %q", d)
	}
	st.SnapshotNow("/a/" + PageFileName) // "dört", whatever was kept lately
	if n := len(st.Versions("/a/" + PageFileName)); n != 3 {
		t.Fatalf("after SnapshotNow: %d", n)
	}
	st.SnapshotNow("/a/" + PageFileName) // the same: not again
	if n := len(st.Versions("/a/" + PageFileName)); n != 3 {
		t.Fatalf("twice: %d", n)
	}
	st.Mkdir("/b")
	if err := st.Rename("/a", "/b/a"); err != nil {
		t.Fatal(err)
	}
	if n := len(st.Versions("/b/a/" + PageFileName)); n != 3 || len(st.Versions("/a/"+PageFileName)) != 0 {
		t.Fatalf("after the move: %d", n)
	}
	st.RemoveAll("/b/a")
	if n := len(st.Versions("/b/a/" + PageFileName)); n != 0 {
		t.Fatalf("after removal: %d", n)
	}
}

// TestVersionDialog: the page menu opens the copies; one is set against the
// page, and put back - the page as it was, the text of before kept as a copy.
func TestVersionDialog(t *testing.T) {
	st, now := storeAt(t)
	lib, _ := OpenLibraryIn(st, StoreRoot)
	seed(lib)
	a := newApp(lib)
	a.clock = func() time.Time { return *now }
	tt := ui.NewTester(a.view, 1100, 800)
	tt.Advance(50 * time.Millisecond)
	p := a.page
	p.SaveText("birinci metin\nortak satır\n")
	*now = now.Add(10 * time.Minute)
	p.SaveText("ikinci metin\nortak satır\nyeni satır\n")
	*now = now.Add(10 * time.Minute)
	p.SaveText("üçüncü metin\nortak satır\n")
	a.takeOutside(p)
	tt.Frame()

	// the dialog of the differences, as the server's versions open it: the page menu opens the history as a stack (timemachine.go)
	a.vhist = histView{open: true, page: p, vers: st.Versions(p.File()), opsFor: -1}
	tt.Frame()
	tt.Advance(400 * time.Millisecond)
	if !a.vhist.open || len(a.vhist.vers) < 2 {
		t.Fatalf("versions: %+v", a.vhist.vers)
	}
	if !tt.HasText("satır geri gelir") {
		t.Fatalf("no difference: %v", tt.Texts())
	}
	// the one that says "birinci"
	for i, v := range a.vhist.vers {
		if d, _ := st.VersionData(p.File(), v.ID); strings.Contains(string(d), "birinci metin") {
			a.vhist.sel = i
		}
	}
	tt.Advance(200 * time.Millisecond)
	if err := tt.Click("Bu sürüme dön"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	body, _ := p.Text()
	if !strings.Contains(body, "birinci metin") || a.vhist.open {
		t.Fatalf("body %q, open %v", body, a.vhist.open)
	}
	if md := a.pageMarkdown(a.page, a.doc.Text()); !strings.Contains(md, "birinci metin") {
		t.Fatalf("the editor: %q", md)
	}
	last := st.Versions(p.File())[0]
	if d, _ := st.VersionData(p.File(), last.ID); !strings.Contains(string(d), "üçüncü metin") {
		t.Fatalf("the text before the return is not kept: %q", d)
	}
}

func TestPairWords(t *testing.T) {
	ops := DiffLines([]string{"bir iki üç dört"}, []string{"bir iki beş dört"})
	PairWords(ops)
	if len(ops) != 2 || ops[0].Kind != '+' || ops[1].Kind != '-' {
		t.Fatalf("%+v", ops)
	}
	marked := func(o DiffOp) (out []string) {
		for _, w := range o.Words {
			if w.Kind != '=' {
				out = append(out, w.Text)
			}
		}
		return
	}
	if !equal(marked(ops[0]), []string{"üç"}) || !equal(marked(ops[1]), []string{"beş"}) {
		t.Fatalf("%v %v", marked(ops[0]), marked(ops[1]))
	}
}

// TestNamedVersion: a copy the user names is kept whatever was kept lately,
// stays when the copies of more than a week ago go, and has its name in the
// list.
func TestNamedVersion(t *testing.T) {
	st, now := storeAt(t)
	st.Mkdir("/a")
	st.WriteFile("/a/"+PageFileName, JoinPage(map[string]string{"title": "A"}, encodeBody(importMarkdown("ilk metin"))))
	if !st.SaveNamed("/a/"+PageFileName, "Yayına giden") {
		t.Fatal("not kept")
	}
	for i := 0; i < 24*(HistoryDays+3)/2; i++ { // every two hours, for ten days
		*now = now.Add(2 * time.Hour)
		st.WriteFile("/a/"+PageFileName, JoinPage(map[string]string{"title": "A"}, encodeBody(importMarkdown("metin "+strings.Repeat("x", i+1)))))
	}
	vs := st.Versions("/a/" + PageFileName) // a copy for every two hours of the week, the one of a week ago to the minute, and the named
	named := 0
	for _, v := range vs {
		if v.Label == "Yayına giden" {
			named++
		}
	}
	if want := 24*HistoryDays/2 + 2; named != 1 || len(vs) != want {
		t.Fatalf("named %d, copies %d, not %d", named, len(vs), want)
	}
}

// TestTimeMachineKeepNamed: while Now is at the front, the history's foot keeps the page as it is under a name, and
// a toast says so, as the copy is not in the stack (#223).
func TestTimeMachineKeepNamed(t *testing.T) {
	st, now := storeAt(t)
	lib, _ := OpenLibraryIn(st, StoreRoot)
	seed(lib)
	a := newApp(lib)
	a.clock = func() time.Time { return *now }
	tt := ui.NewTester(a.view, 1100, 800)
	tt.Advance(50 * time.Millisecond)
	a.openTimeMachine(a.page)
	tt.Advance(900 * time.Millisecond)
	a.tm.label = "Bugünkü hâl"
	tt.Frame()
	r, ok := tt.Find("Sakla")
	if !ok {
		t.Fatalf("no Keep in the foot: %v", tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2) // the pointer is there a frame before the click, as it is for a hand
	tt.Frame()
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(200 * time.Millisecond)
	if vers := st.Versions(a.page.File()); len(vers) == 0 || vers[0].Label != "Bugünkü hâl" {
		t.Fatalf("%+v", vers)
	}
	said := false
	for _, it := range a.toasts.items {
		said = said || strings.Contains(it.text, "Bugünkü hâl")
	}
	if a.tm.label != "" || !said {
		t.Fatalf("not said, label %q", a.tm.label)
	}
}

// lapsePage is a page whose copies were written on three days, the page
// now on a fourth: the cat sat, the black cat sat, the black cat lay, and
// the black cat lay on the rug.
func lapsePage(t *testing.T) (*app, *ui.Tester) {
	st, now := storeAt(t)
	lib, _ := OpenLibraryIn(st, StoreRoot)
	seed(lib)
	a := newApp(lib)
	a.clock = func() time.Time { return *now }
	tt := ui.NewTester(a.view, 1100, 800)
	tt.Advance(50 * time.Millisecond)
	p := a.page
	p.Save("") // the seeded text is not one of the days:
	// its copy, stamped with the computer's clock, would fall on one of the days below after 15:00 and be its last copy
	st.DB.Exec(`DELETE FROM versions WHERE path = ?`, Clean(p.File()))
	for _, s := range []string{"The cat sat on the mat", "The black cat sat on the mat", "The black cat lay on the mat", "The black cat lay on the rug"} {
		*now = now.Add(10 * time.Minute)
		p.SaveText(s + "\n")
	}
	vs := st.Versions(p.File()) // the latest first: the copies of the 9th, the 8th, the 6th (the store stamps them with the time of the computer)
	for i, day := range []int{9, 8, 6} {
		st.DB.Exec(`UPDATE versions SET at = ? WHERE id = ?`, time.Date(2026, 11, day, 15, 0, 0, 0, time.Local).UnixNano(), vs[i].ID)
	}
	a.takeOutside(p)
	p.Modified = time.Date(2026, 11, 10, 9, 0, 0, 0, time.Local)
	*now = p.Modified
	tt.Frame()
	return a, tt
}

// TestTimeLapse: the time-lapse is on from the start - the editor itself, each word in the color of the day it was
// written, a day a version named in the status bar. A click on a day lights it, a double click leaves its words alone
// (a view to look at), Esc takes that back; the page's button turns the colors off and on.
func TestTimeLapse(t *testing.T) {
	a, tt := lapsePage(t) // the page was changed under the open one: its words are worked out again once the text has been still
	for range 200 {
		tt.Advance(20 * time.Millisecond)
		if len(a.lapse.days) == 4 && !a.lapse.busy {
			break
		}
	}
	tt.Advance(100 * time.Millisecond)
	tt.Frame() // (the window is told by the app; the tester is not)
	l := &a.lapse
	if !l.on || len(l.days) != 4 || l.days[0].Day() != 6 || l.days[3].Day() != 10 {
		t.Fatalf("on %v, days: %v", l.on, l.days)
	}
	if !equal([]string{strconv.Itoa(l.sizes[0]), strconv.Itoa(l.sizes[3])}, []string{"6", "7"}) {
		t.Fatalf("sizes: %v", l.sizes)
	}
	for _, s := range []string{"v1", "v4"} {
		if !tt.HasText(s) {
			t.Fatalf("no %q: %v", s, tt.Texts())
		}
	}
	if _, ok := tt.Find("Sayfa"); !ok {
		t.Fatal("the editor is not there: one writes in the time-lapse")
	}
	// the words in the colors of their days: The 1, black 2, cat 1, lay 3, on 1, the 1, rug 4
	line := "The black cat lay on the rug"
	colors := func(text string) map[string]ui.Color {
		out, ok := a.lapseSpans(0, []ui.Span{{Text: text}})
		if !ok {
			t.Fatalf("no colors for %q", text)
		}
		got := map[string]ui.Color{}
		for _, sp := range out {
			if w := strings.TrimSpace(sp.Text); w != "" {
				got[w] = sp.Color
			}
		}
		return got
	}
	dark := l.dark
	got := colors(line)
	for w, born := range map[string]int{"The": 1, "black": 2, "cat": 1, "lay": 3, "rug": 4} {
		if got[w] != lapseColor(born, 4, dark) {
			t.Errorf("%q is %v, the color of day %d is %v", w, got[w], born, lapseColor(born, 4, dark))
		}
	}
	// what is typed is today's, before the words are worked out again: at the end, and in the middle
	got = colors(line + " tonight")
	if got["tonight"] != lapseColor(4, 4, dark) || got["rug"] != lapseColor(4, 4, dark) || got["black"] != lapseColor(2, 4, dark) {
		t.Errorf("typed at the end: %v", got)
	}
	got = colors("The black big cat lay on the rug")
	if got["big"] != lapseColor(4, 4, dark) || got["cat"] != lapseColor(1, 4, dark) || got["lay"] != lapseColor(3, 4, dark) {
		t.Errorf("typed in the middle: %v", got)
	}
	// a click lights the day, a double click leaves its words alone
	if err := tt.Click(locales.HistoryVersionN(2)); err != nil {
		t.Fatal(err)
	}
	tt.Advance(50 * time.Millisecond)
	if l.lit != 2 {
		t.Fatalf("lit %d", l.lit)
	}
	if got := colors(line); got["black"] != lapseColor(2, 4, dark) || got["rug"].A >= lapseColor(4, 4, dark).A {
		t.Errorf("a day lit: the others are not faint: %v", got)
	}
	r, _ := tt.Find(locales.HistoryVersionN(4))
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(30 * time.Millisecond)
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(600 * time.Millisecond)
	if l.solo != 4 || !tt.HasText("rug") || tt.HasText("black") {
		t.Fatalf("solo %d: %v", l.solo, tt.Texts())
	}
	if _, ok := tt.Find("Sayfa"); ok {
		t.Fatal("the editor is there in the view of one day")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if !a.lapse.on || a.lapse.solo != 0 {
		t.Fatalf("Esc did not take the view back: on %v, solo %d", a.lapse.on, a.lapse.solo)
	}
	if _, ok := tt.Find("Sayfa"); !ok || a.kb.normal {
		t.Fatalf("the editor is not back (normal mode %v)", a.kb.normal)
	}
	// with nothing lit, Esc is the editor's again: the time-lapse is not left by it
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if !a.lapse.on {
		t.Fatal("Esc turned the time-lapse off")
	}
	// the page's button turns the colors off, and on
	if err := tt.Click(msg("common_backText")); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(100 * time.Millisecond)
	if !a.settings.NoLapse || a.lapse.on || tt.HasText("v1") {
		t.Fatalf("off: setting %v, on %v", a.settings.NoLapse, a.lapse.on)
	}
	if _, ok := a.lapseSpans(0, []ui.Span{{Text: line}}); ok {
		t.Fatal("colors with it off")
	}
	if err := tt.Click(msg("common_timeLapse")); err != nil {
		t.Fatal(err, tt.Texts())
	}
	for range 50 {
		tt.Advance(20 * time.Millisecond)
		if a.lapse.words != nil {
			break
		}
	}
	if a.settings.NoLapse || !a.lapse.on || a.lapse.words == nil {
		t.Fatalf("on again: setting %v, on %v", a.settings.NoLapse, a.lapse.on)
	}
}

// The page's history as a stack and a timeline: the arrows bring an older copy to the front, Enter puts it back
// and goes back to the editor, Esc leaves with nothing changed (#25).
func TestTimeMachine(t *testing.T) {
	a, tt := lapsePage(t)
	a.openTimeMachine(a.page)
	tt.Frame()
	tt.Advance(700 * time.Millisecond)
	if !a.tm.open || len(a.tm.vers) != 3 || !tt.HasText(locales.HistoryPageHistoryAt(locales.HistoryTodayNow())) {
		t.Fatalf("not open: %+v %v", a.tm.open, tt.Texts())
	}
	tt.Key(0, ui.KeyLeft)
	tt.Key(0, ui.KeyLeft)
	tt.Advance(700 * time.Millisecond)
	if a.tm.sel != 2 || !strings.Contains(a.tmText(2), "sat on the mat") {
		t.Fatalf("at %d: %q", a.tm.sel, a.tmText(a.tm.sel))
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(900 * time.Millisecond)
	if a.tm.open {
		t.Fatal("Esc did not leave it")
	}
	if text, _ := a.page.Text(); !strings.Contains(text, "rug") {
		t.Fatalf("Esc changed the page: %q", text)
	}
	// again: the copy of the 8th put back
	a.openTimeMachine(a.page)
	tt.Frame()
	tt.Key(0, ui.KeyLeft)
	tt.Key(0, ui.KeyLeft)
	tt.Key(0, ui.KeyEnter)
	for range 100 { // the sweep, then the going (#316)
		tt.Advance(20 * time.Millisecond)
	}
	if a.tm.open {
		t.Fatal("still open after putting back")
	}
	if text, _ := a.page.Text(); !strings.Contains(text, "The black cat sat on the mat") {
		t.Fatalf("not put back: %q", text)
	}
}

// Each copy in the history is the whole page area as it was, not its lines: the cover, the title, the properties with
// their values (the copy's own) and the text.
func TestTimeMachineShowsTheWholePage(t *testing.T) {
	a, tt := lapsePage(t)
	pr, err := a.page.Coll.AddProperty("Konu", "text", nil)
	if err != nil {
		t.Fatal(err)
	}
	a.setValue(a.page, pr.ID, "kediler")
	a.flush()
	a.openTimeMachine(a.page)
	tt.Frame()
	tt.Advance(900 * time.Millisecond)
	if !tt.HasText("Konu") || !tt.HasText("kediler") || !strings.Contains(a.tmDoc(0).Text().String(), "The black cat lay on the rug") ||
		!strings.Contains(a.tmDoc(2).Text().String(), "black cat") {
		t.Fatalf("not the whole page: %v", tt.Texts())
	}
	old := a.tmVersionPage(3)
	if old == a.page || old.Props[pr.ID] != "" || a.tmVersionPage(0).Props[pr.ID] != "kediler" {
		t.Fatalf("a copy has the page's values, not its own: %q now %q", old.Props[pr.ID], a.tmVersionPage(0).Props[pr.ID])
	}
	if old.Dir != a.page.Dir || old.Coll != a.page.Coll {
		t.Fatal("the copy is not of this page")
	}
}

// A page with one day of history has nothing to tell apart, so it is plain; the one day's chip lights it, all in its color.
func TestTimeLapseOneDay(t *testing.T) {
	a, tt := lapsePage(t)
	for range 200 {
		tt.Advance(20 * time.Millisecond)
		if len(a.lapse.days) == 4 && !a.lapse.busy {
			break
		}
	}
	l := &a.lapse
	l.days, l.sizes = l.days[:1], l.sizes[:1]
	spans := []ui.Span{{Text: "The black cat"}}
	l.paras = map[int][]lapseTok{0: {{"The", 1}, {"black", 1}, {"cat", 1}}} // all of it written on the one day
	if _, ok := a.lapseSpans(0, spans); ok {
		t.Fatal("one day colors the text without being asked")
	}
	l.lit = 1
	out, ok := a.lapseSpans(0, spans)
	if !ok {
		t.Fatal("v1 lit: nothing lights")
	}
	for _, sp := range out {
		if w := strings.TrimSpace(sp.Text); w != "" && sp.Color != lapseColor(1, 1, l.dark) {
			t.Errorf("%q is not in the color of v1: %v", w, sp.Color)
		}
	}
}

// The text of a copy is read as the editor reads a page - a link is a chip, not "[[…]]" - and it is only to look at: typing while the
// history is open changes nothing.
func TestTimeMachineReadsTextAsThePageDoes(t *testing.T) {
	a, tt := lapsePage(t)
	p := a.page
	p.SaveText("Bir paragraf.\n\nİlgili: [[Welcome]]\n")
	a.takeOutside(p)
	tt.Advance(100 * time.Millisecond)
	a.openTimeMachine(p)
	tt.Frame()
	tt.Advance(700 * time.Millisecond)
	if d := a.tmDoc(0); strings.Contains(d.Text().String(), "[[") {
		t.Fatalf("a link is raw text in the history: %q", d.Text().String())
	}
	before, _ := p.Text()
	tt.Type("zzz")
	tt.Advance(100 * time.Millisecond)
	if after, _ := p.Text(); after != before || !a.tm.open {
		t.Fatalf("typing in the history changed the page: %q -> %q (open %v)", before, after, a.tm.open)
	}
}

// While the history is open its timeline is in the sidebar, where the tree was (#71): a row a day from today back to the
// oldest copy's, the empty days too; a click on a day brings its copy to the front, ↓ goes to an older one and ↑ to a
// newer; Esc brings the tree back. A folded sidebar opens for it, and folds again.
func TestTimeMachineTimeline(t *testing.T) {
	a, tt := lapsePage(t)
	settle := func() { // the sidebar's width and the fade are on their way for a while
		for range 60 {
			tt.Advance(20 * time.Millisecond)
		}
	}
	a.sidebarClosed = true
	settle()
	if a.sidebarShown > 1 {
		t.Fatalf("the sidebar is not folded: %v", a.sidebarShown)
	}
	a.openTimeMachine(a.page)
	tt.Frame()
	settle()
	var got []int
	for _, d := range a.tm.days {
		got = append(got, d.day.Day()*10+d.n)
	}
	if want := []int{101, 91, 81, 70, 61}; !slices.Equal(got, want) { // the 10th (the page now) … the 7th, with none … the 6th
		t.Fatalf("the days are %v, not %v", got, want)
	}
	if a.sidebarShown < 100 || !tt.HasText(locales.HistoryCopies()) || !tt.HasText(a.page.Title) {
		t.Fatalf("no timeline in the sidebar (%v wide): %v", a.sidebarShown, tt.Texts())
	}
	if err := tt.Click(locales.HistoryPageHistoryAt(shortDate(time.Date(2026, 11, 8, 0, 0, 0, 0, time.Local)))); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.tm.sel != 2 {
		t.Fatalf("a click on the 8th: at %d", a.tm.sel)
	}
	tt.Key(0, ui.KeyDown)
	if a.tm.sel != 3 {
		t.Fatalf("↓ is not older: at %d", a.tm.sel)
	}
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyUp)
	if a.tm.sel != 1 {
		t.Fatalf("↑ is not newer: at %d", a.tm.sel)
	}
	if err := tt.Click(locales.HistoryPageHistoryAt("7 Nov 2026")); err == nil && a.tm.sel != 1 {
		t.Fatalf("a day with no copy chose one: at %d", a.tm.sel)
	}
	tt.Key(0, ui.KeyEscape)
	settle()
	if a.tm.open || tt.HasText(locales.HistoryCopies()) || a.sidebarShown > 1 {
		t.Fatalf("after Esc: open %v, the sidebar %v wide, %v", a.tm.open, a.sidebarShown, tt.Texts())
	}
}

// A long run of days with no copy is one row of the timeline, a day's copies are counted on its row, and the dots
// are found from the line leftwards.
func TestTimeMachineDays(t *testing.T) {
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.Local)
	at := func(day, hour int) Version { return Version{At: time.Date(2026, 11, day, hour, 0, 0, 0, time.Local)} }
	days := tmDaysOf([]Version{at(10, 8), at(10, 7), at(9, 12), at(-20, 12)}, now) // the last on the 11th of October
	if len(days) != 4 || days[0].n != 3 || days[1].first != 3 || days[2].skip != 28 || days[3].first != 4 || days[3].n != 1 {
		t.Fatalf("%+v", days)
	}
	if !days[0].month || !days[3].month || days[3].day.Month() != time.October {
		t.Fatalf("the months: %+v", days)
	}
	const w = 250
	x0 := float32(w - tmRight - tmLineW - 8)
	if j := tmDotAt(w, x0-2, 12, 3); j != 0 {
		t.Fatalf("the first dot: %d", j)
	}
	if j := tmDotAt(w, x0-2-2*tmPitch, 12, 3); j != 2 {
		t.Fatalf("the third dot: %d", j)
	}
	if j := tmDotAt(w, x0-2-3*tmPitch, 12, 3); j != -1 {
		t.Fatalf("past the dots: %d", j)
	}
	if j := tmDotAt(w, w-10, 12, 3); j != -1 {
		t.Fatalf("on the day's number: %d", j)
	}
}

// Going back to a copy brings the whole of it - its cover and values, not only its text - and keeps the page as it
// was as a copy once: going back again, to what a copy already is, keeps no second one of it (#71).
func TestTimeMachineRestoresTheWholeCopy(t *testing.T) {
	a, tt := lapsePage(t)
	a.settings.AutoCover = true // made covers are off by default (#388)
	p := a.page
	st := p.FS().(*Store)
	st.DB.Exec(`UPDATE versions SET data = replace(data, '"title"', '"cover":"none","title"') WHERE id = ?`, st.Versions(p.File())[1].ID) // the copy of the 8th had no cover
	if a.coverKind(p) == coverKindNone {
		t.Fatal("the page has no cover to begin with")
	}
	back := func(sel int) {
		a.openTimeMachine(p)
		tt.Frame()
		a.tm.sel = sel
		a.tmPutBack()
		for range 100 {
			tt.Advance(20 * time.Millisecond)
		}
	}
	back(2)
	if text, _ := p.Text(); !strings.Contains(text, "The black cat sat on the mat") || p.Cover != CoverNone {
		t.Fatalf("not the whole copy: %q, cover %q", text, p.Cover)
	}
	if n := len(st.Versions(p.File())); n != 4 {
		t.Fatalf("%d copies after going back, not the three and the page as it was", n)
	}
	// the copy of the 8th is the page now: the stack shows it once, as Now, though it stays kept (#263)
	a.openTimeMachine(p)
	tt.Frame()
	if len(a.tm.vers) != 3 {
		t.Fatalf("%d copies in the stack, not the three the page now is not", len(a.tm.vers))
	}
	for i := 1; i <= len(a.tm.vers); i++ {
		if a.tmText(i) == a.tmText(0) {
			t.Fatalf("the page now is in the stack twice: as Now and at %d", i)
		}
	}
	a.tm.sel = 2
	a.tmPutBack()
	for range 100 {
		tt.Advance(20 * time.Millisecond)
	}
	if n := len(st.Versions(p.File())); n != 4 {
		t.Fatalf("%d copies: going back to a copy kept the page, which a copy already is, again", n)
	}
	if a.tm.open {
		t.Fatal("the history is still open")
	}
}

// The differences are in the history itself, side by side: the toolbar's button turns the stack into the copy's
// lines beside the page's, and back; the arrows go on choosing the copy.
func TestTimeMachineDifferences(t *testing.T) {
	a, tt := lapsePage(t)
	a.openTimeMachine(a.page)
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	if err := tt.Click(locales.HistoryShowDifferences()); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if !a.tm.diff || a.vhist.open || !tt.HasText(locales.HistoryShowPage()) || !tt.HasText(locales.HistoryNow()) {
		t.Fatalf("the differences are not in the history: diff %v, dialog %v, %v", a.tm.diff, a.vhist.open, tt.Texts())
	}
	rows := a.tmDiff(1)
	if len(rows) != 1 || rows[0].left == nil || rows[0].right == nil || rows[0].left.Text != "The black cat lay on the mat" ||
		rows[0].right.Text != "The black cat lay on the rug" || len(rows[0].left.Words) == 0 {
		t.Fatalf("not side by side: %+v", rows)
	}
	tt.Key(0, ui.KeyDown)
	if a.tm.sel != 2 || !a.tm.diff {
		t.Fatalf("↓ in the differences: at %d, diff %v", a.tm.sel, a.tm.diff)
	}
	if err := tt.Click(locales.HistoryShowPage()); err != nil || a.tm.diff {
		t.Fatal("not back to the stack", err)
	}
	rows = tmDiffRows("a\nb\nc\nd\ne\nf\ng\nh\nold\n", "a\nb\nc\nd\ne\nf\ng\nh\nnew\nmore\n")
	if len(rows) != 7 || rows[2].same != 4 || rows[5].left.Text != "old" || rows[5].right.Text != "new" || rows[6].left != nil || rows[6].right.Text != "more" {
		t.Fatalf("%+v", rows)
	}
}

// The history comes in and goes out in a moment, and not at once: the first frame has it, still clear, and it is
// whole a little later; after Esc it fades, and then the editor is back. Only its layers move.
func TestTimeMachineComesAndGoes(t *testing.T) {
	a, tt := lapsePage(t)
	a.openTimeMachine(a.page)
	tt.Frame()
	if !tt.HasText(locales.HistoryPageHistoryAt(locales.HistoryTodayNow())) || !tt.HasText(locales.HistoryRestore()) {
		t.Fatalf("the first frame is not the history: %v", tt.Texts())
	}
	tt.Advance(60 * time.Millisecond)
	if in := a.tm.in; in <= 0.05 || in >= 0.999 {
		t.Fatalf("60ms in, it is %v in: not on its way", in)
	}
	for range 30 {
		tt.Advance(16 * time.Millisecond)
	}
	if a.tm.in < 0.999 {
		t.Fatalf("half a second in, it is %v in", a.tm.in)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(60 * time.Millisecond)
	if !a.tm.open || !a.tm.closing || a.tm.in >= 0.999 {
		t.Fatalf("60ms after Esc: open %v, closing %v, %v in", a.tm.open, a.tm.closing, a.tm.in)
	}
	for range 30 {
		tt.Advance(16 * time.Millisecond)
	}
	if a.tm.open || tt.HasText(locales.HistoryRestore()) {
		t.Fatalf("half a second after Esc it is still there: open %v", a.tm.open)
	}
	if _, ok := tt.Find("Sayfa"); !ok {
		t.Fatal("the editor is not back")
	}
}

// The history is in a page's menu where it is listed too, not only in the page's own: from the tree it opens the
// page and its history (#265).
func TestHistoryFromThePageMenu(t *testing.T) {
	a, tt := lapsePage(t)
	p := a.page
	for _, c := range a.lib.Collections {
		if !c.Journal && !c.Dynamic() && a.page == p {
			a.newPage(c, nil) // another page, which the history is opened from
		}
	}
	if a.page == p {
		t.Fatal("no other page to stand on")
	}
	tt.Frame()
	found := false
	for _, it := range a.pageMenu(p, true) {
		if it.title == "page_versionHistory" {
			found = true
			it.run()
		}
	}
	if !found {
		t.Fatal("no \"Sürüm geçmişi…\" in the page's menu")
	}
	tt.Advance(700 * time.Millisecond)
	if a.page != p || !a.tm.open || a.tm.page != p || len(a.tm.vers) != 3 {
		t.Fatalf("not the page's history: page %q, open %v, %d copies", a.page.Title, a.tm.open, len(a.tm.vers))
	}
}

// A restore is shown before the history goes (#316): the copy is written at once, then the light runs down the front
// card, the copy over the page as it was, keys and clicks not taken; only after it does the editor come back.
func TestTimeMachineRestoreSweeps(t *testing.T) {
	a, tt := lapsePage(t)
	p := a.page
	before, _ := p.Text()
	a.openTimeMachine(p)
	tt.Frame()
	a.tm.sel = 2
	copyText := a.tmText(2)
	a.tmPutBack()
	if now, _ := p.Text(); now == before || strings.TrimSpace(now) != strings.TrimSpace(copyText) {
		t.Fatalf("the copy is not written when Restore is pressed: %q", now)
	}
	tt.Frame()
	if a.tm.reveal.sel != 2 || a.tm.closing || !a.tm.open {
		t.Fatalf("no sweep: reveal %d, closing %v", a.tm.reveal.sel, a.tm.closing)
	}
	if got := a.tmText(0); a.tm.docs[tmRevealOld] == nil || strings.TrimSpace(got) != strings.TrimSpace(copyText) {
		t.Fatalf("the page as it was is not kept for the sweep, or the page now is not the copy: %q", got)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(revealLen / 2)
	if a.tm.closing || a.tm.reveal.sel != 2 {
		t.Fatal("Esc stopped the sweep half way")
	}
	if f := revealFront(0.5); f < 0.45 || f > 0.55 {
		t.Fatalf("the band is at %v half way", f)
	}
	for range 100 {
		tt.Advance(20 * time.Millisecond)
	}
	if a.tm.open {
		t.Fatal("the history is still open after the sweep")
	}
}

// A copy whose data cannot be read says so when it is put back, and the history stays open on it, instead of
// closing as if the page had been restored (#225).
func TestTimeMachineRestoreFailureIsSaid(t *testing.T) {
	a, tt := lapsePage(t)
	p := a.page
	st := p.FS().(*Store)
	before, _ := p.Text()
	a.openTimeMachine(p)
	tt.Frame()
	st.DB.Exec(`DELETE FROM versions WHERE id = ?`, a.tm.vers[1].ID) // the copy at 2 is gone under the open history
	a.tm.sel = 2
	a.tmPutBack()
	tt.Frame()
	if text, _ := p.Text(); text != before {
		t.Fatalf("the page changed: %q", text)
	}
	if !a.tm.open || a.tm.closing || a.tm.reveal.sel != 0 {
		t.Fatalf("the history closed or swept: open %v closing %v reveal %d", a.tm.open, a.tm.closing, a.tm.reveal.sel)
	}
	said := false
	for _, it := range a.toasts.items {
		said = said || it.kind == toastErr
	}
	if !said {
		t.Fatal("no error said")
	}
}

// What a goroutine hands to the next frame is done as that frame starts, under the lock, and not before: the paint,
// which runs out of the lock, reads it only after a frame has put it in (#582).
func TestDoNextFrame(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	done := false
	given := make(chan struct{})
	go func() {
		a.doNextFrame(func() { done = true })
		close(given)
	}()
	<-given
	a.mu.Lock()
	early := done
	a.mu.Unlock()
	if early {
		t.Fatal("done before a frame")
	}
	tt.Frame()
	if !done {
		t.Fatal("not done by the frame")
	}
}

// The page's history keeps in its caches only the copies near the one at the front, however far it is stepped
// through, and reads each copy's file once (#199).
func TestTimeMachineCachesNear(t *testing.T) {
	st, now := storeAt(t)
	lib, _ := OpenLibraryIn(st, StoreRoot)
	seed(lib)
	a := newApp(lib)
	a.clock = func() time.Time { return *now }
	tt := ui.NewTester(a.view, 1100, 800)
	tt.Advance(50 * time.Millisecond)
	p := a.page
	for i := range 60 {
		*now = now.Add(time.Hour)
		p.SaveText(fmt.Sprintf("copy %d\n", i))
	}
	a.takeOutside(p)
	a.openTimeMachine(p)
	tt.Frame()
	n := len(a.tm.vers)
	if n < 40 {
		t.Skipf("the store kept %d copies", n)
	}
	for sel := 0; sel <= n; sel += 4 { // stepped back, the stack moving on its spring
		a.tm.sel = sel
		tt.Advance(40 * time.Millisecond)
	}
	a.tm.sel = n
	tt.Advance(time.Second)
	for _, m := range []int{len(a.tm.raw), len(a.tm.texts), len(a.tm.docs), len(a.tm.pages)} {
		if m > 2*tmNear+2 {
			t.Fatalf("the caches hold %d, %d, %d and %d copies of %d", len(a.tm.raw), len(a.tm.texts), len(a.tm.docs), len(a.tm.pages), n)
		}
	}
	for i := range a.tm.raw {
		if i < n-tmNear {
			t.Fatalf("copy %d kept with %d at the front", i, n)
		}
	}
}
