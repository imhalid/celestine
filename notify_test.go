package main

import (
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/imhalid/celestine/locales"
)

var errDeniedForTest = mygo.ErrNotificationsDenied

// shownNotes stands in for the system's notifications in a test: what would have been shown, by ID.
type shownNotes struct {
	mu  sync.Mutex
	ids []string
	err error
}

func (s *shownNotes) take() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ids...)
}

func catchNotes(t *testing.T) *shownNotes {
	s := &shownNotes{}
	was := notifyShow
	notifyShow = func(id, _, _ string) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.ids = append(s.ids, id)
		return s.err
	}
	t.Cleanup(func() { notifyShow = was })
	return s
}

// waitNotes waits for the goroutine that shows a notification, and for its a.do.
func waitNotes(a *app, s *shownNotes, n int) []string {
	for i := 0; i < 200 && len(s.take()) < n; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	a.do(func() error { return nil })
	return s.take()
}

// TestNotifyAsksFirst: nothing is shown before the user said yes. The first notification that would have been is
// remembered and named in a toast as the app comes back; its button turns them on, and the next one is shown -
// only while the app is behind others.
func TestNotifyAsksFirst(t *testing.T) {
	notes := catchNotes(t)
	a, _ := newTestApp(t, "x\n")
	a.mu.Lock()
	a.nt.active.Store(false)
	a.notify(notifySync, locales.NotifySyncStopped(), "the server said no")
	a.mu.Unlock()
	if got := waitNotes(a, notes, 1); len(got) != 0 {
		t.Fatalf("shown before asking: %v", got)
	}
	a.mu.Lock()
	a.nt.active.Store(true)
	a.askNotify()
	var ask *toastItem
	for _, it := range a.toasts.items {
		if it.fn != nil {
			ask = it
		}
	}
	a.mu.Unlock()
	if ask == nil || ask.action != locales.NotifyAllow() {
		t.Fatalf("no ask: %+v", a.toasts.items)
	}
	a.mu.Lock()
	ask.fn() // the toast's button, under the lock as a frame runs it
	a.mu.Unlock()
	if got := waitNotes(a, notes, 1); len(got) != 1 || got[0] != "celestine-on" {
		t.Fatalf("turning them on shows the first: %v", got)
	}
	if a.settings.Notify != notifyOn {
		t.Fatalf("Notify = %q", a.settings.Notify)
	}
	a.mu.Lock()
	a.notify(notifyUpdate, locales.NotifyUpdateTitle(), "") // in front: a toast's work, not a notification's
	a.nt.active.Store(false)
	a.notify(notifyUpdate, locales.NotifyUpdateTitle(), "")
	a.mu.Unlock()
	if got := waitNotes(a, notes, 2); len(got) != 2 || got[1] != notifyUpdate {
		t.Fatalf("behind other apps it is shown, in front not: %v", got)
	}
}

// TestNotifyDenied: the system's no turns the switch off and says so; nothing more is shown.
func TestNotifyDenied(t *testing.T) {
	notes := catchNotes(t)
	notes.err = errDeniedForTest
	a, _ := newTestApp(t, "x\n")
	a.mu.Lock()
	a.setNotify(true)
	a.mu.Unlock()
	waitNotes(a, notes, 1)
	for i := 0; i < 100; i++ {
		a.mu.Lock()
		off := a.settings.Notify == notifyOff
		a.mu.Unlock()
		if off {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("still %q after the system said no", a.settings.Notify)
}

// TestNotifyClickOpens: a click on the sync's notification opens its settings.
func TestNotifyClickOpens(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	a.mu.Lock()
	defer a.mu.Unlock()
	a.notifyClicked(notifySync)
	if !a.pane().settingsOpen || a.pane().settingsTab != tabSync {
		t.Fatalf("open %v tab %d", a.pane().settingsOpen, a.pane().settingsTab)
	}
}
