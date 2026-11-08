package main

import (
	"path/filepath"
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
// is never pushed out by the limit, and has its name in the list.
func TestNamedVersion(t *testing.T) {
	st, now := storeAt(t)
	st.Mkdir("/a")
	st.WriteFile("/a/"+PageFileName, JoinPage(map[string]string{"title": "A"}, encodeBody(importMarkdown("ilk metin"))))
	if !st.SaveNamed("/a/"+PageFileName, "Yayına giden") {
		t.Fatal("not kept")
	}
	for i := 0; i < HistoryKeep+20; i++ {
		*now = now.Add(10 * time.Minute)
		st.WriteFile("/a/"+PageFileName, JoinPage(map[string]string{"title": "A"}, encodeBody(importMarkdown("metin "+strings.Repeat("x", i+1)))))
	}
	vs := st.Versions("/a/" + PageFileName)
	named := 0
	for _, v := range vs {
		if v.Label == "Yayına giden" {
			named++
		}
	}
	if named != 1 || len(vs) != HistoryKeep+1 {
		t.Fatalf("named %d, copies %d", named, len(vs))
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
