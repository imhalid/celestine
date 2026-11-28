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
	"time"

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
	if err := a.open(); err != nil {
		a.openErr = locales.AppCannotOpen(err.Error())
		log.Print(a.openErr)
	}
	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Celestine", Width: 412, Height: 860, Content: ui.View(a.view)})
		a.applyTheme()     // sync Android system bars to the chosen appearance once the host is ready
		a.refreshWeather() // fetch local weather & sunrise/sunset in background
		a.syncNow()        // what the other devices wrote, as the app opens
		go func() {        // a frame every few minutes, whose syncTick makes the pass that is due
			for range time.Tick(time.Minute) {
				a.update(func() {})
			}
		}()
	})
	mygo.Power.OnSuspend(func() { a.flush() }) // to the background: what was typed is saved before the system may end it
	mygo.App.OnActivate(func(bool) {           // back in the foreground: what was written elsewhere meanwhile comes
		a.update(func() {
			if time.Since(a.sy.tried) >= syncOnActivate {
				a.syncNow()
			}
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// open opens the library in the app's folder without creating a placeholder Journal on disk: if the phone signs in to
// an existing account whose journal is named in another language (e.g. "Günlük"), first sync pulls it cleanly with no
// duplicate "Journal" collection.
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
	a.lockDays()
	a.showToday()
	return nil
}
