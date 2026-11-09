package main

import (
	"path/filepath"
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

	a.openHistory(p) // the dialog of the differences: the page menu opens the history as a stack (timemachine.go)
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

// TestVersionDialogName: the dialog's field keeps the page as it is under a name.
func TestVersionDialogName(t *testing.T) {
	st, now := storeAt(t)
	lib, _ := OpenLibraryIn(st, StoreRoot)
	seed(lib)
	a := newApp(lib)
	a.clock = func() time.Time { return *now }
	tt := ui.NewTester(a.view, 1100, 800)
	tt.Advance(50 * time.Millisecond)
	a.openHistory(a.page)
	tt.Advance(900 * time.Millisecond)
	a.vhist.label = "Bugünkü hâl"
	tt.Frame()
	r, _ := tt.Find("Sakla")
	tt.Move(r.X+r.W/2, r.Y+r.H/2) // the pointer is there a frame before the click, as it is for a hand
	tt.Frame()
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(200 * time.Millisecond)
	if len(a.vhist.vers) == 0 || a.vhist.vers[0].Label != "Bugünkü hâl" {
		t.Fatalf("%+v", a.vhist.vers)
	}
	if !tt.HasText("Bugünkü hâl") {
		t.Fatalf("not listed: %v", tt.Texts())
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
	if err := tt.Click(Tf("Version %d", 2)); err != nil {
		t.Fatal(err)
	}
	tt.Advance(50 * time.Millisecond)
	if l.lit != 2 {
		t.Fatalf("lit %d", l.lit)
	}
	if got := colors(line); got["black"] != lapseColor(2, 4, dark) || got["rug"].A >= lapseColor(4, 4, dark).A {
		t.Errorf("a day lit: the others are not faint: %v", got)
	}
	r, _ := tt.Find(Tf("Version %d", 4))
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
	if err := tt.Click(T("Back to the text")); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(100 * time.Millisecond)
	if !a.settings.NoLapse || a.lapse.on || tt.HasText("v1") {
		t.Fatalf("off: setting %v, on %v", a.settings.NoLapse, a.lapse.on)
	}
	if _, ok := a.lapseSpans(0, []ui.Span{{Text: line}}); ok {
		t.Fatal("colors with it off")
	}
	if err := tt.Click(T("Time-lapse")); err != nil {
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
	if !a.tm.open || len(a.tm.vers) != 3 || !tt.HasText(T("Page history: ")+T("Today (Now)")) {
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
	tt.Advance(900 * time.Millisecond)
	if a.tm.open {
		t.Fatal("still open after putting back")
	}
	if text, _ := a.page.Text(); !strings.Contains(text, "The black cat sat on the mat") {
		t.Fatalf("not put back: %q", text)
	}
}

// The history comes and goes at once, with no motion: its copies are whole pages, and moving them every frame cost too much. The
// first frame is the history, and a frame after Esc is the page.
func TestTimeMachineComesAndGoesAtOnce(t *testing.T) {
	a, tt := lapsePage(t)
	a.openTimeMachine(a.page)
	tt.Frame()
	if !tt.HasText(T("Page history: ")+T("Today (Now)")) || !tt.HasText(T("Restore")) {
		t.Fatalf("the first frame is not the history: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.tm.open || tt.HasText(T("Restore")) {
		t.Fatalf("a frame after Esc it is still there: open %v", a.tm.open)
	}
	if _, ok := tt.Find("Sayfa"); !ok {
		t.Fatal("the editor is not back")
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
