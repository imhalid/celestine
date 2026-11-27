package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A past day of the journal is locked: what is typed in it, a tick of its task, a new title and the trash change
// nothing of it, and say so; today's page is not locked; at midnight today turns past and locks (#622).
func TestJournalPastDaysLocked(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.Local)
	a.clock = func() time.Time { return now }
	tt.SetSize(1000, 700)
	j, err := a.journal()
	if err != nil {
		t.Fatal(err)
	}
	past, err := a.makeDay(j, now.AddDate(0, 0, -1))
	if err != nil {
		t.Fatal(err)
	}
	if err := past.SaveText("- [ ] dünkü iş\n"); err != nil {
		t.Fatal(err)
	}
	today, err := a.makeDay(j, now)
	if err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.pageLocked(past) || a.pageLocked(today) {
		t.Fatalf("locked: past %v, today %v", a.pageLocked(past), a.pageLocked(today))
	}
	before, _ := past.Text()
	a.open(past)
	tt.Advance(300 * time.Millisecond)
	pn := a.pane()
	pn.doc.SetSelection(pn.doc.Text().Len(), pn.doc.Text().Len())
	tt.Frame()
	typeEach(tt, " eklenen")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	a.flush()
	if got, _ := past.Text(); got != before || strings.Contains(pn.doc.Text().String(), "eklenen") {
		t.Fatalf("typed into a past day:\n%s", got)
	}
	pn.doc.ToggleChecked(0) // an edit that reaches its text another way: put back, said
	tt.Frame()
	a.flush()
	tt.Frame()
	if got, _ := past.Text(); got != before || pn.doc.Text().ParagraphStyleAt(0).Checked {
		t.Fatalf("a past day's task ticked:\n%s", got)
	}
	if err := past.SetTitle("başka"); err == nil {
		t.Fatal("a past day renamed")
	}
	if _, err := a.lib.TrashPage(past); err == nil {
		t.Fatal("a past day sent to the trash")
	}
	if _, ok := tt.Find(msg("journal_lockedBadge")); !ok {
		t.Fatal("the page does not say it is locked")
	}
	if err := today.SaveText("bugün\n"); err != nil {
		t.Fatalf("today's page: %v", err)
	}
	now = now.AddDate(0, 0, 1) // midnight: today is past
	tt.Frame()
	if !a.pageLocked(today) {
		t.Fatal("yesterday's page did not lock at midnight")
	}
}

// A day's link, [[2026-11-25]], opens its day's page when clicked, not the calendar; ⌥-click opens the calendar
// (#621).
func TestDateLinkOpensItsDay(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.Local)
	a.clock = func() time.Time { return now }
	tt.SetSize(1000, 700)
	j, _ := a.journal()
	day, err := a.makeDay(j, now.AddDate(0, 0, -1))
	if err != nil {
		t.Fatal(err)
	}
	page := a.pane().page
	if err := page.SaveText("bak: [[2026-11-25]]\n"); err != nil {
		t.Fatal(err)
	}
	a.takeOutside(page)
	tt.Advance(300 * time.Millisecond)
	pn := a.pane()
	var hit *chipHit
	tt.Frame() // the chips drawn, seen for a click
	for i := range a.chipSeen {
		if a.chipSeen[i].ch.kind == "date" {
			hit = &a.chipSeen[i]
		}
	}
	if hit == nil {
		t.Fatal("no date chip drawn")
	}
	x, y := hit.r.X+hit.r.W/2, hit.r.Y+hit.r.H/2
	a.clickChip(pn, x, y, true)
	if !a.dateEdit.open || a.pane().page != page {
		t.Fatal("⌥-click did not open the calendar")
	}
	a.dateEdit.open = false
	a.clickChip(pn, x, y, false)
	tt.Frame()
	if a.dateEdit.open || a.pane().page != day {
		t.Fatalf("a click did not open the day: %v", a.pane().page.Title)
	}
}

// A day of the journal, today's too, is not moved to the trash: its menu has no Move to trash, a trash asked for some
// other way is refused and said, and a bulk trash leaves it; a page that is not a day goes (#631).
func TestJournalDayNotTrashed(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.Local)
	a.clock = func() time.Time { return now }
	j, err := a.journal()
	if err != nil {
		t.Fatal(err)
	}
	today, err := a.makeDay(j, now)
	if err != nil {
		t.Fatal(err)
	}
	tomorrow, err := a.makeDay(j, now.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.journalDay(today) || !a.journalDay(tomorrow) || a.journalDay(a.lib.Collections[0].Pages[0]) {
		t.Fatal("which pages are days")
	}
	for _, it := range a.pageMenu(today, false) {
		if it.title == "moveToTrash" {
			t.Fatal("today's menu moves it to the trash")
		}
	}
	a.trashPage(today)
	if !a.inLibrary(today) || a.trashed != nil {
		t.Fatal("today's page went to the trash")
	}
	if !strings.Contains(a.notice, msg("journal_dayCannotBeTrashed")) {
		t.Fatalf("not said: %q", a.notice)
	}
	a.bulkTrash(a.pane(), []*Page{tomorrow})
	if !a.inLibrary(tomorrow) {
		t.Fatal("a bulk trash took a coming day")
	}
	other := a.lib.Collections[0].Pages[0]
	if other.Coll == j {
		t.Fatal("the first collection is the journal")
	}
	a.trashPage(other)
	if a.inLibrary(other) {
		t.Fatal("a page that is not a day stayed")
	}
}

func TestJournalLockBypassesClosed(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.Local)
	a.clock = func() time.Time { return now }
	tt.SetSize(1000, 700)
	j, err := a.journal()
	if err != nil {
		t.Fatal(err)
	}
	day, err := a.makeDay(j, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := day.SaveText("- [ ] task one\n\nAl [ ] süt\n"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// Cross midnight WITHOUT rendering a frame: MCP must still lock and reject writes & set_task.
	now = now.AddDate(0, 0, 1)
	cs := agent(t, a)
	ctx := context.Background()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "update_page",
		Arguments: map[string]any{"page": a.idOf(day.Dir), "append": "late edit"},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Fatal("MCP update_page mutated past journal day after midnight without a frame")
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "set_task",
		Arguments: map[string]any{"page": a.idOf(day.Dir), "line": 0, "done": true},
	})
	if err == nil && (res == nil || !res.IsError) {
		t.Fatal("MCP set_task succeeded on locked past journal day")
	}

	// Open the locked day and verify Tab, putCover, and extractToPage all refuse cleanly.
	a.open(day)
	tt.Frame()
	beforeText := a.pane().doc.Text().String()
	a.pane().doc.SetSelection(len([]rune(beforeText)), len([]rune(beforeText)))
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if got := a.pane().doc.Text().String(); got != beforeText {
		t.Fatalf("Tab mutated locked page: %q", got)
	}
	if err := a.putCover(day, []byte("fake"), ".png"); err == nil || day.Cover != "" {
		t.Fatalf("putCover mutated locked page: err=%v cover=%q", err, day.Cover)
	}
	pagesBefore := len(a.lib.Collections[0].All())
	a.extractToPage()
	if len(a.lib.Collections[0].All()) != pagesBefore {
		t.Fatal("extractToPage created an orphan page from a locked day")
	}
}
