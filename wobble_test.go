package main

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeWin is a window that can be moved (or one that does not obey), and keeps where it has been.
type fakeWin struct {
	mu    sync.Mutex
	x, y  int
	obey  bool
	maxed bool
	seen  [][2]int
}

func (w *fakeWin) Position() (int, int) { w.mu.Lock(); defer w.mu.Unlock(); return w.x, w.y }
func (w *fakeWin) SetPosition(x, y int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seen = append(w.seen, [2]int{x, y})
	if w.obey {
		w.x, w.y = x, y
	}
}
func (w *fakeWin) IsMaximized() bool  { return w.maxed }
func (w *fakeWin) IsFullScreen() bool { return false }
func (w *fakeWin) IsMinimized() bool  { return false }

func waitShaken(a *app) {
	time.Sleep(wobbleLen + 300*time.Millisecond)
	for a.winBusy.Load() {
		time.Sleep(10 * time.Millisecond)
	}
}

// An agent's writing shakes the window, harder for more text, and it settles.
func TestJellyWobble(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	if _, _, _, _, _, on := swingOf(a.wobbleState(), time.Now()); on {
		t.Fatal("shaking before anything was written")
	}
	a.jiggle(0)
	if a.wobbleState().amp != 0 {
		t.Fatal("shaken by nothing")
	}
	a.jiggle(20)
	small := a.wobbleState().amp
	a.jiggle(100000)
	w := a.wobbleState()
	if w.amp <= small || w.amp > wobbleMax {
		t.Fatalf("amplitudes: %v then %v", small, w.amp)
	}
	moved, hopped := false, false
	for i := 0; i < 40; i++ {
		dx, _, _, _, hop, on := swingOf(w, w.at.Add(time.Duration(i)*20*time.Millisecond))
		moved = moved || (on && dx != 0)
		hopped = hopped || (on && hop > 1)
	}
	if !moved || !hopped {
		t.Fatalf("it never moved (%v) or hopped (%v)", moved, hopped)
	}
	if _, _, _, _, _, on := swingOf(w, w.at.Add(wobbleLen+time.Millisecond)); on {
		t.Fatal("it never settles")
	}
	tt.Advance(100 * time.Millisecond)
	tt.Advance(2 * time.Second)
	a.settings.NoWobble = true
	a.wob = wobble{}
	a.jiggle(500)
	if a.wobbleState().amp != 0 {
		t.Fatal("shook with it off")
	}
}

// The window itself is moved, side to side and up and down, and is put back where it was.
func TestWindowShake(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	win := &fakeWin{x: 300, y: 200, obey: true}
	a.mover = win
	a.jiggle(5000)
	waitShaken(a)
	left, right, up := false, false, false
	for _, p := range win.seen {
		left = left || p[0] < 300
		right = right || p[0] > 300
		up = up || p[1] < 200
		if p[1] > 200 {
			t.Fatalf("it went down past where it was: %v", p)
		}
	}
	if !left || !right || !up {
		t.Fatalf("left %v right %v hop %v of %d steps", left, right, up, len(win.seen))
	}
	if x, y := win.Position(); x != 300 || y != 200 {
		t.Fatalf("not put back: %d,%d", x, y)
	}
	if a.winShaking.Load() {
		t.Fatal("still says the window shakes")
	}
}

// A window the user takes hold of is let go: it stays where they put it.
func TestWindowShakeUserMoves(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	win := &fakeWin{x: 300, y: 200, obey: true}
	a.mover = win
	a.jiggle(5000)
	time.Sleep(150 * time.Millisecond)
	win.mu.Lock()
	win.x, win.y = 700, 500
	win.mu.Unlock()
	waitShaken(a)
	if x, y := win.Position(); x != 700 || y != 500 {
		t.Fatalf("the shake took the window back: %d,%d", x, y)
	}
}

// A window that cannot be moved (or is maximized) is not fought: what is drawn in it shakes instead.
func TestWindowShakeFallsBack(t *testing.T) {
	for name, win := range map[string]*fakeWin{"does not obey": {x: 300, y: 200}, "maximized": {x: 0, y: 0, obey: true, maxed: true}} {
		a, _ := newTestApp(t, "x\n")
		a.mover = win
		a.jiggle(5000)
		time.Sleep(200 * time.Millisecond)
		if a.winShaking.Load() && name == "maximized" {
			t.Fatalf("%s: says the window shakes", name)
		}
		waitShaken(a)
		if x, y := win.Position(); name == "does not obey" && (x != 300 || y != 200) || name == "maximized" && len(win.seen) != 0 {
			t.Fatalf("%s: moved to %d,%d (%d steps)", name, x, y, len(win.seen))
		}
		if a.winShaking.Load() {
			t.Fatalf("%s: still says the window shakes", name)
		}
	}
}

// A page written by an agent is what shakes it.
func TestJellyOnWrite(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	if err := a.writeBody(a.page, strings.Repeat("kelime ", 200)); err != nil {
		t.Fatal(err)
	}
	if a.wobbleState().amp == 0 {
		t.Fatal("writing did not shake the window")
	}
}
