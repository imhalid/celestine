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
