package main

import (
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const zoomPage = "Before.\n\n# Notes\n\n```go\nx := 1\n```\n\n- [!] urgent\n\n## Deeper\n\ninside\n\n# Next\n\nafter\n"

// Zooming into a heading shows its section alone, with the rest behind a chip above and one below; what is
// written while zoomed lands in its place, the file is always the whole page, and zooming out brings the page
// back with its code language and task kind.
func TestZoomIntoBlock(t *testing.T) {
	a, tt := newTestApp(t, zoomPage)
	tt.Advance(100 * time.Millisecond)
	at := strings.Index(a.doc.Text().String(), "Notes")
	a.zoomIn(at)
	tt.Frame()
	shown := a.doc.Text().String()
	if strings.Contains(shown, "Before.") || strings.Contains(shown, "after") || !strings.Contains(shown, "inside") {
		t.Fatalf("zoomed: %q", shown)
	}
	if top, end, _, _ := zoomChips(a.doc.Text()); top == nil || end == nil {
		t.Fatalf("no chip above or below: %q", shown)
	}
	// write in the zoomed block
	i := utf8.RuneCountInString(shown[:strings.Index(shown, "inside")+len("inside")])
	a.doc.SetSelection(i, i)
	a.doc.InsertText(" too")
	tt.Frame()
	a.save()
	whole := strings.Replace(zoomPage, "inside", "inside too", 1)
	if md := a.currentMarkdown(a.page); md != whole {
		t.Fatalf("the file while zoomed:\n%q\nwant\n%q", md, whole)
	}
	// a level deeper, and back up twice
	at = strings.Index(a.doc.Text().String(), "Deeper")
	a.zoomIn(at)
	if s := a.doc.Text().String(); strings.Contains(s, "urgent") || !strings.Contains(s, "inside too") {
		t.Fatalf("deeper: %q", s)
	}
	a.zoomOut()
	if !a.zoomed() || !strings.Contains(a.doc.Text().String(), "urgent") {
		t.Fatalf("one level up: %q", a.doc.Text().String())
	}
	a.zoomOut()
	tt.Frame()
	if a.zoomed() {
		t.Fatal("still zoomed")
	}
	a.save()
	if md := bodyMarkdown(a.doc.Text()); md != strings.TrimSuffix(whole, "\n") {
		t.Fatalf("zoomed out:\n%q\nwant\n%q", md, whole)
	}
}

// A fold keeps what the blocks it hides are: a code block's language and a task's kind come back with them.
func TestFoldKeepsBlocks(t *testing.T) {
	a, tt := newTestApp(t, zoomPage)
	tt.Advance(100 * time.Millisecond)
	at := strings.Index(a.doc.Text().String(), "Notes")
	a.fold(at)
	tt.Frame()
	a.save()
	if md := a.currentMarkdown(a.page); md != zoomPage {
		t.Fatalf("the file while folded:\n%q", md)
	}
	a.toggleFold(at)
	if md := bodyMarkdown(a.doc.Text()); md != strings.TrimSuffix(zoomPage, "\n") {
		t.Fatalf("unfolded:\n%q", md)
	}
}

// The zoom's keys reach it from the text: Alt+→ in and Alt+← out off macOS, where Option+←/→ moves by words (#269).
func TestZoomKeys(t *testing.T) {
	if runtime.GOOS != "darwin" && (zoomInKey != "alt+right" || zoomOutKey != "alt+left") {
		t.Fatalf("the zoom's keys: %q %q", zoomInKey, zoomOutKey)
	}
	a, tt := newTestApp(t, zoomPage)
	tt.Advance(100 * time.Millisecond)
	at := strings.Index(a.doc.Text().String(), "Notes")
	a.doc.SetSelection(at, at)
	a.refocus = true
	tt.Frame()
	press := func(accel string) {
		mods, key, _, ok := parseAccel(accel)
		if !ok {
			t.Fatalf("not a key: %q", accel)
		}
		tt.Key(mods, key)
		tt.Advance(50 * time.Millisecond)
	}
	press(zoomInKey)
	if !a.zoomed() || strings.Contains(a.doc.Text().String(), "Before.") {
		t.Fatalf("%s did not zoom in: %q", zoomInKey, a.doc.Text().String())
	}
	press(zoomOutKey)
	if a.zoomed() || !strings.Contains(a.doc.Text().String(), "Before.") {
		t.Fatalf("%s did not zoom out: %q", zoomOutKey, a.doc.Text().String())
	}
}
