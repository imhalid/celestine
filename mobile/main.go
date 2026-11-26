// Command mobile is Celestine for phones: an app of its own, made for a hand and a small screen, on the same library
// as the desktop's (internal/library), so that its pages are the desktop's pages and sync as they do (#633).
//
// Its screens are a phone's: today's page and what was written lately, the collections and their pages, a search,
// the settings; a page fills the screen to be written in. It shares nothing with the desktop's interface.
//
//	cd mobile && go run github.com/egoist/mygo/cmd/mygo dev -platform android
//	cd mobile && go run github.com/egoist/mygo/cmd/mygo build -platform android
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// version is the app's version, put in at the build (-ldflags "-X main.version=v1.2.3").
var version = "dev"

func main() {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	registerFonts()
	a := newApp(dir)
	a.applyTheme() // the look the settings chose, from the first frame
	if err := a.open(); err != nil {
		a.openErr = locales.AppCannotOpen(err.Error())
		log.Print(a.openErr)
	}
	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Celestine", Width: 412, Height: 860, Content: ui.View(a.view)})
	})
	mygo.Power.OnSuspend(func() { a.flush() }) // to the background: what was typed is saved before the system may end it
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// open opens the library in the app's folder, made and given its journal when it is new.
func (a *app) open() error {
	root := filepath.Join(a.dir, "library")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	lib, err := library.OpenLibraryIn(library.OSFS{}, root)
	if err != nil {
		return err
	}
	a.lib = lib
	if _, err := a.journal(); err != nil {
		return err
	}
	a.lockDays()
	return nil
}
