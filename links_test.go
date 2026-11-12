package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A link written as page://collection/page opens that page when it is clicked.
func TestPageLinkClick(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	coll := a.page.Coll
	target, err := coll.NewPage(nil, "PostgreSQL Internals & Query Optimizer")
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.Base(coll.Dir) + "/" + filepath.Base(target.Dir)
	body := `[{"r":[{"t":"Alt modül: "},{"t":"PostgreSQL Internals & Query Optimizer","l":"page://` + rel + `"}]}]`
	a.doc.SetText(decodeBody(body))
	a.doc.SetSelection(0, 0)
	tt.SetSize(900, 500)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if len(a.linkSeen) == 0 {
		t.Fatal("no link was drawn")
	}
	r := a.linkSeen[0].r
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(300 * time.Millisecond)
	if a.page != target {
		t.Fatalf("the link did not open the page: %v", a.here())
	}
	// a path nobody has says so
	a.openLink("page://yok/boyle-bir-sayfa")
	if a.notice == "" {
		t.Fatal("not told")
	}
	_ = ui.Cmd
}

// With two panes, a link in the first opens on a click: each pane tests a click against the links it drew,
// not against those the other pane drew last (#390).
func TestPageLinkClickInFirstPane(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1400, 800)
	coll := a.page.Coll
	target, err := coll.NewPage(nil, "Hedef")
	if err != nil {
		t.Fatal(err)
	}
	other, err := coll.NewPage(nil, "Öteki")
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.Base(coll.Dir) + "/" + filepath.Base(target.Dir)
	a.doc.SetText(decodeBody(`[{"r":[{"t":"Hedef","l":"page://` + rel + `"},{"t":" sayfa"}]}]`))
	a.openInSide(place{"page", other.Dir}) // the second pane, in use, with no link
	tt.Advance(300 * time.Millisecond)
	first := a.ps.states[0]
	if a.paneCount() != 2 || a.ps.active != 1 || len(first.boxes) == 0 {
		t.Fatalf("panes %d, in use %d", a.paneCount(), a.ps.active)
	}
	b := first.boxes[0]
	tt.ClickAt(first.editorR.X+8, first.editorR.Y+b.Top+b.Height/2)
	tt.Advance(300 * time.Millisecond)
	if a.ps.active != 0 || a.page != target {
		t.Fatalf("the link in the first pane did not open: pane %d, %v", a.ps.active, a.here())
	}
}

// Text selected with the pointer, ending on a link, does not follow the link:
// only a click does.
func TestLinkSelectDoesNotOpen(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	coll := a.page.Coll
	target, err := coll.NewPage(nil, "Hedef")
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.Base(coll.Dir) + "/" + filepath.Base(target.Dir)
	body := `[{"r":[{"t":"Önce yazı "},{"t":"Hedef sayfa","l":"page://` + rel + `"}]}]`
	a.doc.SetText(decodeBody(body))
	a.doc.SetSelection(0, 0)
	tt.SetSize(900, 500)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	here := a.page
	r := a.linkSeen[0].r
	y := r.Y + r.H/2
	tt.Press(r.X-60, y)
	for k := 1; k <= 6; k++ {
		tt.Move(r.X-60+float32(k)*(r.W/2+60)/6, y)
		tt.Frame()
	}
	tt.Release(r.X+r.W/2, y)
	tt.Advance(300 * time.Millisecond)
	if a.page != here {
		t.Fatal("selecting over a link followed it")
	}
	if sa, sc := a.doc.Selection(); sa == sc {
		t.Fatal("nothing was selected")
	}
}
