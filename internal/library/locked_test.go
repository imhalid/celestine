package library

import (
	"errors"
	"os"
	"testing"
)

// A locked page (a past day of the journal, #622) is not written, renamed, given a value, sent to the trash nor moved
// out of its collection, and nothing of it changes in memory either; its own stamp of its meta still reaches its file.
func TestLockedPage(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lib.NewCollection("Journal")
	other, _ := lib.NewCollection("Notes")
	p, err := c.NewPage(nil, "2026-11-05")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SaveText("what was written that day\n"); err != nil {
		t.Fatal(err)
	}
	pr, err := c.AddProperty("Mood", PropText, nil)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p.File())
	p.Locked = "a past day"
	if err := p.SaveText("over it\n"); !errors.Is(err, ErrLocked) {
		t.Fatalf("saved: %v", err)
	}
	if err := p.SetTitle("renamed"); !errors.Is(err, ErrLocked) || p.Title != "2026-11-05" {
		t.Fatalf("renamed: %v, %q", err, p.Title)
	}
	if err := p.SetValue(pr.ID, "glad"); !errors.Is(err, ErrLocked) || p.Props[pr.ID] != "" {
		t.Fatalf("a value: %v, %q", err, p.Props[pr.ID])
	}
	if _, err := lib.TrashPage(p); !errors.Is(err, ErrLocked) {
		t.Fatalf("trashed: %v", err)
	}
	if err := lib.MovePage(p, other, nil, 0); !errors.Is(err, ErrLocked) || p.Coll != c {
		t.Fatalf("moved out: %v", err)
	}
	if after, _ := os.ReadFile(p.File()); string(after) != string(before) {
		t.Fatal("the file changed")
	}
	if ok, err := StampMeta(lib.FS, p.File(), "celestine:day", "2026-11-05"); !ok || err != nil {
		t.Fatalf("its stamp: %v %v", ok, err)
	}
	parent, _ := c.NewPage(nil, "a page with a past day under it")
	q, _ := c.NewPage(parent, "under")
	q.Locked = "a past day"
	if _, err := lib.TrashPage(parent); !errors.Is(err, ErrLocked) {
		t.Fatalf("a page with a locked one under it trashed: %v", err)
	}
}

func TestLockedPagePropertyAndBoardSafety(t *testing.T) {
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	j, _ := lib.NewCollection("Journal")
	j.Journal = true
	other, _ := lib.NewCollection("Notes")
	pr, err := j.AddProperty("Status", PropChoice, []string{"Open", "Done"})
	if err != nil {
		t.Fatal(err)
	}
	j.Creating = true
	past, err := j.NewPage(nil, "2026-11-25")
	if err != nil {
		t.Fatal(err)
	}
	past.SaveText("past\n")
	if err := past.SetValue(pr.ID, "Open"); err != nil {
		t.Fatal(err)
	}
	own, err := past.AddProperty("Note", PropText, nil)
	if err != nil {
		t.Fatal(err)
	}
	today, err := j.NewPage(nil, "2026-11-26")
	j.Creating = false
	if err != nil {
		t.Fatal(err)
	}
	today.SaveText("today\n")

	beforePast, _ := os.ReadFile(past.File())
	past.Locked = "a past day"

	// Own-property mutations on a locked page must fail without changing in-memory slices.
	if _, err := past.AddProperty("Extra", PropText, nil); !errors.Is(err, ErrLocked) || len(past.Own) != 1 {
		t.Fatalf("AddProperty: %v, own=%d", err, len(past.Own))
	}
	if err := past.EditProperty(own.ID, "Changed", nil); !errors.Is(err, ErrLocked) || past.Own[0].Name != "Note" {
		t.Fatalf("EditProperty: %v, name=%q", err, past.Own[0].Name)
	}
	if err := past.RemoveProperty(own.ID); !errors.Is(err, ErrLocked) || len(past.Own) != 1 {
		t.Fatalf("RemoveProperty: %v, own=%d", err, len(past.Own))
	}

	// Setting a choice property on today's unlocked page must succeed even when a locked page is in the column.
	if err := today.SetValue(pr.ID, "Open"); err != nil {
		t.Fatalf("today SetValue failed with locked sibling: %v", err)
	}
	if today.Props[pr.ID] != "Open" {
		t.Fatalf("today prop: %q", today.Props[pr.ID])
	}
	if afterPast, _ := os.ReadFile(past.File()); string(afterPast) != string(beforePast) {
		t.Fatal("locked past day file changed during MoveOnBoard")
	}

	// Even today's unlocked journal day cannot be moved out of the journal collection.
	if err := lib.MovePage(today, other, nil, 0); err == nil || today.Coll != j {
		t.Fatalf("today moved out of journal: err=%v coll=%q", err, today.Coll.Name)
	}
}
