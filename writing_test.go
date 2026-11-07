package main

import (
	"testing"
	"time"
)

// TestWriting: the words a save adds are counted into the day, what is
// deleted is not taken away, and the figures follow.
func TestWriting(t *testing.T) {
	a, tt := newTestApp(t, "bir iki\n")
	now := time.Date(2026, 11, 5, 14, 0, 0, 0, time.UTC) // a Thursday
	a.clock = func() time.Time { return now }
	tt.Frame()
	a.lastWords = 2
	a.countWriting(2, 5)
	a.countWriting(5, 1) // deleting counts nothing
	if got := a.writtenOn(now); got != 3 {
		t.Fatalf("today %d", got)
	}
	for _, d := range []struct {
		back, n int
	}{{1, 10}, {2, 10}, {3, 0}, {4, 7}} {
		a.settings.Writing[now.AddDate(0, 0, -d.back).Format("2006-01-02")] = d.n
	}
	s := a.stats()
	if s.today != 3 || s.streak != 3 || s.best != 10 || s.total != 30 {
		t.Fatalf("%+v", s)
	}
	if s.week != 23 { // Monday the 2nd to today: 3 + 10 + 10
		t.Fatalf("week %d", s.week)
	}
	// a goal: the day counts only when it is reached
	a.settings.WriteGoal = 10
	if s = a.stats(); s.streak != 2 {
		t.Fatalf("streak with a goal: %d", s.streak)
	}
	// the save does it
	a.settings.Writing = nil
	a.settings.WriteGoal = 0
	a.doc.SetSelection(7, 7)
	tt.Type(" yeni kelimeler geldi")
	tt.Advance(time.Second)
	a.save()
	if got := a.writtenOn(now); got != 3 {
		t.Fatalf("by a save: %d", got)
	}
	// the page of figures
	a.openStats()
	tt.Frame()
	tt.Advance(600 * time.Millisecond)
	if !tt.HasText("İstatistikler") || !tt.HasText("gün üst üste") {
		t.Fatalf("page: %v", tt.Texts())
	}
}
