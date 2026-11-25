package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui/richtext"
)

// The watcher scans the disk outside the lock (#193): a scan made before the app saved a page, applied after, does
// not take the page for one changed outside and leaves no conflict copy beside it.
func TestWatchLateScan(t *testing.T) {
	a, tt := newTestApp(t, "ilk\n")
	p := a.pane().page
	disk := scanLibrary(a.lib.FS, a.lib.Root) // the scan, before the save
	time.Sleep(20 * time.Millisecond)         // the file's time moves
	a.pane().doc.SetText(richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{{Text: "kaydedildi"}}}))
	a.pane().dirty = true
	a.save()
	a.pane().doc.SetText(richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{{Text: "kaydedildi, sonra yazıldı"}}}))
	a.pane().dirty = true
	a.applyDisk(disk, true)
	tt.Frame()
	ents, _ := os.ReadDir(p.Dir)
	for _, e := range ents {
		if strings.Contains(e.Name(), "çakışma") {
			t.Fatalf("a conflict copy from a late scan: %s", filepath.Join(p.Dir, e.Name()))
		}
	}
	if !strings.Contains(a.pane().doc.Text().String(), "sonra yazıldı") {
		t.Fatalf("what was typed is gone: %q", a.pane().doc.Text().String())
	}
}
