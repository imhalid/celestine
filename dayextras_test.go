package main

import (
	"strings"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

// TestDayExtras: under today's page of the journal, the tasks of other pages due today and those overdue, which
// tick where they are, and the journal's pages of the same day in earlier years, which open; a section folds by its
// heading, and the settings turn both off; a page not of the journal shows none (#552).
func TestDayExtras(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1100, 900)
	notes := a.page
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	journalDays(t, a, now, "2024-11-09", "2025-11-09", "2025-11-10", "2026-11-09")
	if err := notes.SaveText("- [ ] due today [[2026-11-09]]\n- [ ] late [[2026-11-01]]\n- [ ] later [[2026-12-01]]\n- [x] done late [[2026-11-02]]\n"); err != nil {
		t.Fatal(err)
	}
	a.linksStale()
	a.links.built = time.Time{}
	a.allTasks(true)
	a.openToday()
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	for _, want := range []string{"due today", "late", "2025", "2024"} {
		if !tt.HasText(want) {
			t.Fatalf("no %q under today: %v", want, tt.Texts())
		}
	}
	if tt.HasText("later") || tt.HasText("done late") || tt.HasText(locales.CommonLinkLabel("2025-11-10")) {
		t.Fatalf("a task of another day, a done one, or a page of another day: %v", tt.Texts())
	}
	// it ticks where it is
	if err := tt.Click(locales.TasksTaskLabel("due today")); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if body, _ := notes.Text(); !strings.Contains(body, "[x] due today") {
		t.Fatalf("not ticked in its page: %q", body)
	}
	// a year opens its page
	if err := tt.Click(locales.CommonLinkLabel("2025-11-09")); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if a.page == nil || a.page.Title != "2025-11-09" {
		t.Fatalf("the year opened %v", a.page)
	}
	// that day's: no tasks due then, and 2024 in its earlier years
	if tt.HasText("due today") || !tt.HasText("2024") {
		t.Fatalf("under 2025-11-09: %v", tt.Texts())
	}
	// folded by its heading, kept
	if err := tt.Click(msg("journal_onThisDay")); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if tt.HasText(locales.CommonLinkLabel("2024-11-09")) || len(a.settings.DayFold) != 1 {
		t.Fatalf("folded: %v, %v", a.settings.DayFold, tt.Texts())
	}
	// off in the settings, and none under a page not of the journal
	a.settings.NoDayExtras = true
	tt.Frame()
	if tt.HasText(msg("journal_onThisDay")) {
		t.Fatal("shown with the setting off")
	}
	a.settings.NoDayExtras = false
	a.open(notes)
	tt.Frame()
	if tt.HasText(msg("journal_onThisDay")) || tt.HasText(locales.CommonLinkLabel("2024-11-09")) {
		t.Fatalf("under a page not of the journal: %v", tt.Texts())
	}
}
