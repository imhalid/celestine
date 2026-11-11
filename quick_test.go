package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

func TestNoteLines(t *testing.T) {
	got := noteLines("  bir\n\n[] iki\n- zaten\n3. üç\n[ ] dört  ")
	want := []string{"- bir", "- [ ] iki", "- zaten", "3. üç", "- [ ] dört"}
	if !equal(got, want) {
		t.Fatalf("%q", got)
	}
}

// TestCaptureNote: a note goes to the end of today's page of the journal,
// made when there is none, in the editor too when that page is open; or it is
// a page of its own.
func TestCaptureNote(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	if _, err := a.captureNote(quickDay, "  "); err == nil {
		t.Fatal("an empty note")
	}
	msg, err := a.captureNote(quickDay, "ilk not\n[] bir iş")
	if err != nil || msg != "Günlüğe eklendi" {
		t.Fatalf("%q %v", msg, err)
	}
	today, _ := a.todayPage()
	if today == nil || !sameDay(dayOf(today), a.clock()) {
		t.Fatalf("today's page: %+v", today)
	}
	if _, err := a.captureNote(quickDay, "ikinci"); err != nil {
		t.Fatal(err)
	}
	body, _ := today.Text()
	if body != "- ilk not\n- [ ] bir iş\n- ikinci\n" {
		t.Fatalf("body %q", body)
	}
	// open, the editor has it as well, and a task of it is in the task view
	a.open(today)
	tt.Frame()
	if _, err := a.captureNote(quickDay, "üçüncü"); err != nil {
		t.Fatal(err)
	}
	if md := a.pageMarkdown(a.page, a.doc.Text()); !strings.Contains(md, "üçüncü") {
		t.Fatalf("the open page: %q", md)
	}
	a.links.built = time.Time{}
	found := false
	for _, k := range a.allTasks(true) {
		found = found || k.text == "bir iş"
	}
	if !found {
		t.Fatal("not among the tasks")
	}
	// a page of its own, in the collection shown, the first line its title
	c := a.lib.Collections[0]
	if c.Journal {
		c = a.lib.Collections[1]
	}
	before := len(c.Pages)
	if msg, err := a.captureNote(quickPage, "# Toplantı fikri\nbirinci\nikinci"); err != nil || !strings.Contains(msg, "Toplantı fikri") {
		t.Fatalf("%q %v", msg, err)
	}
	if len(c.Pages) != before+1 {
		t.Fatalf("pages %d", len(c.Pages))
	}
	p := c.Pages[len(c.Pages)-1]
	if b, _ := p.Text(); p.Title != "Toplantı fikri" || b != "birinci ikinci\n" {
		t.Fatalf("%q %q", p.Title, b)
	}
}

// TestQuickPanel: ⌘↵ in the panel adds what is typed; ↵ breaks the line.
func TestQuickPanel(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	q := &quickNote{a: a}
	pt := ui.NewTester(q.view, 580, 250)
	pt.Advance(50 * time.Millisecond)
	q.focusWanted = true
	pt.Frame()
	typeEach(pt, "bir")
	pt.Key(0, ui.KeyEnter)
	typeEach(pt, "iki")
	pt.Advance(50 * time.Millisecond)
	if q.text != "bir\niki" {
		t.Fatalf("typed %q", q.text)
	}
	pt.Key(ui.Cmd, ui.KeyEnter)
	pt.Advance(50 * time.Millisecond)
	if q.text != "" {
		t.Fatalf("not added: %q", q.text)
	}
	today, _ := a.todayPage()
	if b, _ := today.Text(); b != "- bir\n- iki\n" {
		t.Fatalf("body %q", b)
	}
}

// TestQuickNoticeOpens: the notice of a note put away has an Aç that goes to its page.
func TestQuickNoticeOpens(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	if _, err := a.captureNote(quickDay, "bir şey"); err != nil {
		t.Fatal(err)
	}
	a.notice = "Günlüğe eklendi"
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Aç"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(100 * time.Millisecond)
	if today, _ := a.todayPage(); a.page != today {
		t.Fatalf("open %q", a.page.Title)
	}
}

// TestQuickKeySetting: the editor's settings have the key of the quick note.
func TestQuickKeySetting(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.settingsOpen, a.settingsTab = true, 1
	tt.Frame()
	tt.Advance(600 * time.Millisecond)
	if !tt.HasText(locales.SettingsQuickNoteKey()) {
		t.Fatalf("no row: %v", tt.Texts())
	}
}
