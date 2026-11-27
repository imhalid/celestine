package main

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
)

// app is the phone app's state: the library, where the user is, the page being written, the settings.
type app struct {
	dir     string // the app's own folder: the library, the settings
	lib     *library.Library
	win     *mygo.Window
	router  *ui.Router
	openErr string // why the library could not be opened, shown in place of it

	settings settings
	ed       editorState // the page being written
	fresh    string      // the ID of a page just made, whose title is asked for first
	query    string      // what the search asks
	notice   string      // a short word at the bottom, gone after a moment
	noticeAt time.Time   // when it was said

	sy   syncState // the account and the sync (sync.go)
	home homeState // the home screen's scrolling and the system bars

	cs     collSettings    // a collection's settings being changed (collsettings.go)
	ticked map[string]bool // the tasks ticked off on the Tasks screen, shown done until it is left

	menuOpen bool // the menu covers the screen
	plusOpen bool // the + has opened its list of what to make
	moreOpen bool // a page's sheet of what to do with it
	menuDim  bool // the screen is dimmed for the menu, rather than the + or a sheet
}

// settings are the phone's own: kept in the app's folder, not the library's, as they are this device's.
type settings struct {
	Theme       string `json:"theme,omitempty"`       // "", "light" or "dark": "" follows the system
	AccountUser string `json:"accountUser,omitempty"` // the account signed in to, its token and key in the secret store
	SyncURL     string `json:"syncURL,omitempty"`     // the server, when not the account's own (a test's)
}

func newApp(dir string) *app {
	a := &app{dir: dir, router: ui.NewRouter("/"), ticked: map[string]bool{}}
	if b, err := os.ReadFile(a.settingsFile()); err == nil {
		json.Unmarshal(b, &a.settings)
	}
	return a
}

func (a *app) settingsFile() string { return filepath.Join(a.dir, "settings.json") }

// saveSettings writes the settings.
func (a *app) saveSettings() {
	if b, err := json.Marshal(a.settings); err == nil {
		library.WriteFilePrivate(a.settingsFile(), b)
	}
}

// applyTheme has the app light or dark as the settings say, or as the system is: the system's bars follow, their
// icons dark on the light look and light on the dark.
func (a *app) applyTheme() {
	src := mygo.ThemeSystem
	switch a.settings.Theme {
	case "light":
		src = mygo.ThemeLight
	case "dark":
		src = mygo.ThemeDark
	}
	mygo.Theme.SetSource(src)
}

// say shows a short notice at the bottom of the screen.
func (a *app) say(s string) { a.notice, a.noticeAt = s, time.Now() }

// noticeFor is how long a notice shows.
const noticeFor = 2500 * time.Millisecond

// The screens at the top of the app, which the menu goes to, by their paths.
const (
	pathHome     = "/"
	pathLibrary  = "/library"
	pathTasks    = "/tasks"
	pathSearch   = "/search"
	pathSettings = "/settings"
	pathSignIn   = "/signin"
)

// view is a frame of the window: the theme, the screen the router shows, and over it the + and what it and the menu
// open, the screen dimmed behind them: dimmed, not blurred, as a blur of the whole screen is drawn again each frame of
// the motion, which a phone does slowly (#637).
func (a *app) view(c *ui.Context) {
	c.SetTheme(themeOf(c.Theme().Dark)) // light or dark as the system says, or the settings (applyTheme)
	t := c.Theme()
	c.Root().Background(t.Background)
	if a.router.Path() == pathHome { // the home's summary goes on under the system's bar
		c.Root().Background(nightBg)
	}
	if a.lib == nil {
		ui.Column(c).Fill().Justify(ui.Center).Padding(24).Children(func() {
			ui.Text(c, a.openErr).TextColor(t.TextMuted).Wrap()
		})
		return
	}
	a.lockDays() // a day turns past at midnight
	a.syncTick(c.Now())
	over := a.menuOpen || a.plusOpen || a.moreOpen
	a.setBars(a.router.Path() == pathHome && !over) // the home's summary is dark under them
	screen := ui.Box(c.Key("screen")).Fill()
	veil := screen.AnimateTo("veil", b2f(over), motionIn(0))
	screen.Children(func() {
		a.router.View(c, func(r *ui.Route) {
			switch {
			case r.Match(pathHome):
				a.homeScreen(c)
			case r.Match(pathLibrary):
				a.libraryScreen(c)
			case r.Match(pathTasks):
				a.tasksScreen(c)
			case r.Match(pathSearch):
				a.searchScreen(c)
			case r.Match(pathSettings):
				a.settingsScreen(c)
			case r.Match(pathSignIn):
				a.signInScreen(c)
			case r.Match("/c/{dir}/settings"):
				if coll := a.collAt(r.Param("dir")); coll != nil {
					a.collSettingsScreen(c, coll)
				}
			case r.Match("/c/{dir}"):
				if coll := a.collAt(r.Param("dir")); coll != nil {
					r.Title(coll.Name)
					a.collectionScreen(c, coll)
				}
			case r.Match("/p/{id}"):
				if p := a.lib.PageByID(r.Param("id")); p != nil {
					r.Title(p.Title)
					a.pageScreen(c, p)
				}
			}
		})
	})
	a.leaveEditor()
	if len(a.ticked) > 0 && a.router.Path() != pathTasks {
		clear(a.ticked)
	}
	if veil > 0.001 {
		if over {
			a.menuDim = a.menuOpen // kept as the dim fades, so that it fades from where it was
		}
		dim := float32(0.8) // the menu's words are large: the screen is all but hidden under them
		if a.menuDim {
			dim = 0.96
		}
		ui.Overlay(c, func() {
			scrim := ui.Box(c.Key("scrim")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
				Background(pal.bg.Alpha(dim * min(veil, 1)))
			if scrim.Clicked() {
				a.closeAll()
			}
		})
	}
	if !strings.HasPrefix(a.router.Path(), "/p/") && a.router.Path() != pathSignIn && !a.menuOpen {
		a.plusButton(c)
	}
	a.menuView(c)
	a.moreSheet(c)
	if over && c.Shortcut(0, ui.KeyEscape) { // Android's back closes what is open first
		a.closeAll()
	}
	a.noticeView(c)
}

// closeAll closes the menu, the +'s list and a page's sheet.
func (a *app) closeAll() { a.menuOpen, a.plusOpen, a.moreOpen = false, false, false }

// b2f is 1 for true.
func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// openPage shows page p to be written in.
func (a *app) openPage(p *library.Page) {
	a.closeAll()
	a.router.Transition = ui.TransitionAuto
	a.router.Push(pagePath(p))
}

// openCollection shows the pages of collection c.
func (a *app) openCollection(c *library.Collection) {
	a.closeAll()
	a.router.Transition = ui.TransitionAuto
	a.router.Push("/c/" + url.PathEscape(c.Dir))
}

// goTo shows a screen of the menu: home is the bottom of the history, the others one screen over it. The menu's
// places fade into each other, as places side by side, where a page or a collection slides in over the screen it
// was opened from.
func (a *app) goTo(path string) {
	a.closeAll()
	a.router.Transition = ui.TransitionFade
	if a.router.Path() == path {
		return
	}
	for a.router.CanGoBack() {
		a.router.Back()
	}
	if path != pathHome {
		a.router.Push(path)
	}
}

// collAt is the collection in folder dir, or nil.
func (a *app) collAt(dir string) *library.Collection {
	if d, err := url.PathUnescape(dir); err == nil {
		dir = d
	}
	for _, c := range a.lib.Collections {
		if c.Dir == dir {
			return c
		}
	}
	return nil
}

// noticeView is the notice, a pill over the bottom of the screen, while it shows.
func (a *app) noticeView(c *ui.Context) {
	if a.notice == "" {
		return
	}
	if time.Since(a.noticeAt) > noticeFor {
		a.notice = ""
		return
	}
	c.AnimationFrame()
	ui.Overlay(c, func() {
		ui.Row(c.Key("notice")).Absolute().Left(0).Right(0).Bottom(112).Justify(ui.Center).Children(func() {
			shadow(ui.Text(c, a.notice).Padding(12, 18).Radius(22).Background(pal.text).TextColor(pal.bg).FontSize(15).
				MaxWidth(340).Wrap())
		})
	})
}
