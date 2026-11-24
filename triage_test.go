package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// TestTriage: a collection's pages one at a time, each sorted with one key - moved by M and a digit, tagged by T, a
// tag and ↵, linked from today by L, trashed by D, left by → - and the triage ends after the last (#551).
func TestTriage(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1100, 800)
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	a.clock = func() time.Time { return now }
	inbox, _ := a.lib.NewCollection("Inbox")
	books, _ := a.lib.NewCollection("Books")
	var ps []*Page
	for _, title := range []string{"Dune", "Idea", "Call Ada", "Junk", "Later"} {
		p, _ := inbox.NewPage(nil, title)
		p.SaveText("about " + title + "\n")
		ps = append(ps, p)
	}
	tt.Frame()
	a.openTriage(inbox)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !a.triage.open || !tt.HasText("Dune") {
		t.Fatalf("not open on the first page: %v", tt.Texts())
	}
	step := func(keys ...ui.Key) {
		t.Helper()
		for _, k := range keys {
			tt.Key(0, k)
			tt.Frame()
		}
		tt.Advance(200 * time.Millisecond)
	}
	// M, then the number of Books among the collections
	step(ui.KeyM)
	n := -1
	for i, c := range a.triageTargets() {
		if c == books {
			n = i
		}
	}
	step(triageDigits[n])
	if ps[0].Coll != books {
		t.Fatalf("Dune is in %s", ps[0].Coll.Name)
	}
	// T, a tag, ↵
	step(ui.KeyT)
	tt.Type("someday")
	step(ui.KeyEnter)
	if body, _ := ps[1].Text(); !strings.Contains(body, "#someday") {
		t.Fatalf("tagged: %q", body)
	}
	a.linksStale()
	a.links.built = time.Time{}
	if got := a.linksTo(nil, a.clock(), true); got != nil || len(a.links.tags["someday"]) != 1 {
		t.Fatalf("the tag is not one: %v", a.links.tags)
	}
	// L: a link from today's page
	step(ui.KeyL)
	today, _ := a.todayPage()
	if body, _ := today.Text(); !strings.Contains(body, "[[Call Ada]]") {
		t.Fatalf("today's page: %q", body)
	}
	// D: the trash; →: left as it is, and the end
	step(ui.KeyD)
	if a.inLibrary(ps[3]) {
		t.Fatal("Junk is not in the trash")
	}
	step(ui.KeyRight)
	if a.triage.open || ps[4].Coll != inbox {
		t.Fatalf("after the last: open %v, Later in %s", a.triage.open, ps[4].Coll.Name)
	}
	if !strings.Contains(strings.Join(tt.Texts(), " "), locales.TriageDone(4)) {
		t.Fatalf("no word of the end: %v", tt.Texts())
	}
}
