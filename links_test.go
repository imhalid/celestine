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
