package main

import (
	"runtime"

	"github.com/egoist/mygo/ui"
)

// noAlignKeys takes Ctrl+L, Ctrl+E and Ctrl+R off macOS, where MyGo's editor aligns a paragraph with them, as Word
// does: the app offers no alignment, and they are pressed by mistake (Ctrl+R reloads a browser), leaving a paragraph
// at the right, kept so in the page. Taken here, before the editor, they do nothing (#280).
func (a *app) noAlignKeys(c *ui.Context) {
	if runtime.GOOS == "darwin" { // ⌘L and ⌘R align nothing there, and ⌘E is code
		return
	}
	for _, k := range []ui.Key{ui.KeyL, ui.KeyE, ui.KeyR} {
		c.Shortcut(ui.Cmd, k)
	}
}
