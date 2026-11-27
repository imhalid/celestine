package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
)

// sheetKind identifies the single discreet sheet that can open over Today's paper (Settings or Sign-In).
type sheetKind int

const (
	sheetNone sheetKind = iota
	sheetSettings
	sheetSignIn
)

// app holds the One Day mobile app's complete state: the library, Today's living paper, sync state, and settings.
type app struct {
	dir     string
	lib     *library.Library
	win     *mygo.Window
	openErr string
	clock   func() time.Time // injectable clock for tests; nil uses time.Now

	settings settings

	// Today's living paper state.
	activeDay    time.Time
	activePageID string
	ed           editorState

	// Discreet Settings / Sign-In sheet.
	sheet sheetKind

	// Sync state (sync.go).
	sy syncState

	// Brief ephemeral toast.
	notice   string
	noticeAt time.Time
}

type settings struct {
	Theme       string `json:"theme,omitempty"`       // "", "light", or "dark"
	AccountUser string `json:"accountUser,omitempty"` // signed-in email/account
	SyncURL     string `json:"syncURL,omitempty"`     // custom sync server (tests)
}

func newApp(dir string) *app {
	a := &app{dir: dir}
	if b, err := os.ReadFile(a.settingsFile()); err == nil {
		json.Unmarshal(b, &a.settings)
	}
	return a
}

func (a *app) now() time.Time {
	if a.clock != nil {
		return a.clock()
	}
	return time.Now()
}

func (a *app) settingsFile() string { return filepath.Join(a.dir, "settings.json") }

func (a *app) saveSettings() {
	if b, err := json.Marshal(a.settings); err == nil {
		library.WriteFilePrivate(a.settingsFile(), b)
	}
}

func (a *app) applyTheme() {
	if a.win == nil {
		return
	}
	src := mygo.ThemeSystem
	switch a.settings.Theme {
	case "light":
		src = mygo.ThemeLight
	case "dark":
		src = mygo.ThemeDark
	}
	mygo.Theme.SetSource(src)
}

func (a *app) say(s string) { a.notice, a.noticeAt = s, a.now() }

const noticeFor = 2400 * time.Millisecond

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// showToday binds the Living Paper to today's journal page (or an unwritten draft ready to type immediately).
func (a *app) showToday() {
	today := startOfDay(a.now())
	a.flush()
	a.activeDay = today
	if p := a.dayPage(today); p != nil {
		a.activePageID = p.ID
		a.ed = a.stateOf(p)
		return
	}
	a.activePageID = ""
	a.ed = a.todayDraftState()
}

// checkMidnightRollover automatically turns the page to the new Today once midnight passes.
func (a *app) checkMidnightRollover() {
	today := startOfDay(a.now())
	if !a.activeDay.IsZero() && !a.activeDay.Equal(today) {
		a.flush()
		a.lockDays()
		a.showToday()
	}
}

// handleBack steps back through Sign-In -> Settings -> Today.
func (a *app) handleBack() bool {
	switch a.sheet {
	case sheetSignIn:
		a.sheet = sheetSettings
		return true
	case sheetSettings:
		a.sheet = sheetNone
		return true
	}
	return false
}

// view renders one frame: Today's Living Paper, optional dimmed scrim, minimal Settings/Sign-In sheet, and toast.
func (a *app) view(c *ui.Context) {
	isDark := c.Theme().Dark
	switch a.settings.Theme {
	case "dark":
		isDark = true
	case "light":
		isDark = false
	}
	c.SetTheme(themeOf(a.now(), isDark))
	t := c.Theme()
	c.Root().Background(t.Background)
	if a.lib == nil {
		ui.Column(c).Fill().Justify(ui.Center).Padding(24).Children(func() {
			ui.Text(c, a.openErr).TextColor(t.TextMuted).Wrap()
		})
		return
	}

	a.checkMidnightRollover()
	a.lockDays()
	a.syncTick(c.Now())

	// Today's Living Paper stays completely stationary behind any sheet for locked 60 FPS.
	ui.Box(c.Key("today-root")).Fill().Children(func() {
		a.todayView(c)
	})

	// Full-bleed paper grain overlay: rendered in ui.Overlay so it extends edge-to-edge under the top status/battery
	// bar and the bottom gesture navigation bar without blocking any touch input.
	ui.Overlay(c, func() {
		ui.Box(c.Key("edge-grain")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
			HitTest(func(x, y float32, r ui.Rect) bool { return false }).
			Draw(drawPaperGrain)
	})

	over := a.sheet != sheetNone
	scrimHost := ui.Box(c.Key("scrim-anim")).Absolute().Size(0, 0)
	veil := scrimHost.AnimateTo("veil", b2f(over), fadeAnim())
	if veil > 0.005 {
		dim := float32(0.38)
		if dark {
			dim = 0.6
		}
		ui.Overlay(c, func() {
			scrim := ui.Box(c.Key("scrim")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
				Background(pal.scrim.Alpha(dim * min(veil, 1)))
			if scrim.Clicked() {
				a.sheet = sheetNone
			}
		})
	}

	a.settingsSheet(c)
	a.signInSheet(c)

	if c.Shortcut(0, ui.KeyEscape) {
		a.handleBack()
	}

	a.noticeView(c)
}

func (a *app) noticeView(c *ui.Context) {
	if a.notice == "" {
		return
	}
	if a.now().Sub(a.noticeAt) > noticeFor {
		a.notice = ""
		return
	}
	c.AnimationFrame()
	ui.Overlay(c, func() {
		_, _, bottom, _ := c.SafeArea()
		ui.Row(c.Key("notice")).Absolute().Left(0).Right(0).Bottom(24 + bottom).Justify(ui.Center).
			HitTest(func(x, y float32, r ui.Rect) bool { return false }).Children(func() {
			shadow(ui.Text(c, a.notice).Padding(10, 18).Radius(20).
				Background(pal.toastBg).TextColor(pal.toastText).
				FontSize(14).FontWeight(550).MaxWidth(320).Wrap())
		})
	})
}
