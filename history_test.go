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

	if err := tt.Click("Sayfa menüsü"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if err := tt.Click("Sürüm geçmişi…"); err != nil {
		t.Fatal(err, tt.Texts())
	}
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

// TestTimeLapse: the page's button shows the time-lapse in place of the
// editor - a day a version, named in the status bar - and Esc brings the
// editor back.
func TestTimeLapse(t *testing.T) {
	a, tt := lapsePage(t)
	if err := tt.Click(T("Time-lapse")); err != nil {
		t.Fatal(err, tt.Texts())
	}
	for range 50 {
		tt.Advance(20 * time.Millisecond)
		if a.lapse.words != nil {
			break
		}
	}
	tt.Advance(100 * time.Millisecond)
	l := &a.lapse
	if len(l.days) != 4 || l.days[0].Day() != 6 || l.days[3].Day() != 10 {
		t.Fatalf("days: %v", l.days)
	}
	if !equal([]string{strconv.Itoa(l.sizes[0]), strconv.Itoa(l.sizes[3])}, []string{"6", "7"}) {
		t.Fatalf("sizes: %v", l.sizes)
	}
	for _, s := range []string{"v1", "v4", "black", "rug"} {
		if !tt.HasText(s) {
			t.Fatalf("no %q: %v", s, tt.Texts())
		}
	}
	if tt.HasText("mat") || tt.HasText("sat") {
		t.Fatalf("the cut words: %v", tt.Texts())
	}
	if _, ok := tt.Find("Sayfa"); ok {
		t.Fatal("the editor is there too")
	}
	// a click lights the day, a double click leaves its words alone
	if err := tt.Click(Tf("Version %d", 2)); err != nil {
		t.Fatal(err)
	}
	tt.Advance(50 * time.Millisecond)
	if l.lit != 2 || !tt.HasText("The") {
		t.Fatalf("lit %d", l.lit)
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
	tt.Key(0, ui.KeyEscape)
	tt.Advance(100 * time.Millisecond)
	if a.lapse.on {
		t.Fatal("Esc did not leave it")
	}
	if _, ok := tt.Find("Sayfa"); !ok || a.kb.normal {
		t.Fatalf("the editor is not back (normal mode %v)", a.kb.normal)
	}
}
