package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func welcomeApp(t *testing.T) *app {
	welcomeShaders = false
	t.Cleanup(func() { welcomeShaders = true })
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := seed(lib); err != nil {
		t.Fatal(err)
	}
	a := newApp(lib)
	a.wel.on = true
	return a
}

// TestWelcome: the words rise, Başla opens the app, and the welcome is done.
func TestWelcome(t *testing.T) {
	a := welcomeApp(t)
	tt := ui.NewTester(a.view, 560, 660)
	tt.Advance(3500 * time.Millisecond)
	if !tt.HasText("Başla") || !tt.HasText("Yaz. Gerisi kendiliğinden.") {
		t.Fatalf("the welcome is not up: %v", tt.Texts())
	}
	if !a.modal() {
		t.Fatal("the app takes keys under the welcome")
	}
	if err := tt.Click("Başla"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(2 * time.Second)
	if a.wel.on {
		t.Fatalf("the welcome did not end: %v", tt.Texts())
	}
	if !a.settings.WelcomeDone {
		t.Fatal("it will be shown again")
	}
}
