package main

import (
	"image"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// Split, the window's frame lists the panes as tabs in its top edge, and the Dock's areas have no headers of
// their own; a tab puts its pane in use, its close button closes it, and with one pane left the strip folds.
func TestFrameTabs(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1200, 700)
	tt.Advance(300 * time.Millisecond)
	if a.wframe.strip != 0 {
		t.Fatalf("one pane, and the strip is open: %v", a.wframe.strip)
	}
	second, err := a.page.Coll.NewPage(nil, "İkinci")
	if err != nil {
		t.Fatal(err)
	}
	a.openInSide(place{"page", second.Dir})
	tt.Advance(900 * time.Millisecond)
	if a.paneCount() != 2 || a.wframe.strip < 0.99 {
		t.Fatalf("panes %d, strip %v", a.paneCount(), a.wframe.strip)
	}
	if _, ok := tt.Find("Maximize"); ok {
		t.Fatal("an area has its header under the strip")
	}
	if err := tt.Click(a.ps.states[0].page.Title); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	if a.ps.active != 0 {
		t.Fatalf("the first tab did not put its pane in use: %d", a.ps.active)
	}
	if err := tt.Click(locales.CloseThePane() + ": İkinci"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(900 * time.Millisecond)
	if a.paneCount() != 1 || a.wframe.strip > 0.01 {
		t.Fatalf("after closing: panes %d, strip %v", a.paneCount(), a.wframe.strip)
	}
}

func rectsOverlap(a, b ui.Rect) bool {
	return a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y
}

// With no tabs and the sidebar folded, the pane card rises to frameW on macOS while keeping initial page
// content and the Settings nav list clear of traffic lights and top-left buttons, and opening a second pane
// animates smoothly instead of jumping frameTopH pixels.
func TestNoTabsCardClearTopLeftButtons(t *testing.T) {
	a, tt := newTestApp(t, "Birinci paragraf\n")
	tt.SetTitleBar(ui.TitleBar{Left: 84, Height: frameTopH})
	tt.SetSize(680, 620)
	a.toggleSidebar()
	tt.Advance(1200 * time.Millisecond)
	if a.sidebarShown > 0.1 {
		t.Fatalf("sidebar did not close: %v", a.sidebarShown)
	}
	if frameW != 8 || frameR != 10 {
		t.Fatalf("frame constants = (%d, %d), want (8, 10)", frameW, frameR)
	}
	if !ownWindowControls && (a.panesR.Y != frameW || a.scrollR.Y != frameTopH+1) {
		t.Fatalf("single-pane card = (%v, %v), want (%d, %d)", a.panesR.Y, a.scrollR.Y, frameW, frameTopH+1)
	}
	sbBtn, ok := tt.Find(locales.SidebarSidebar())
	if !ok {
		t.Fatal("sidebar button in top-left chrome not found")
	}
	if !ownWindowControls && sbBtn.X != 84+barLead { // room after the window's buttons, as macOS's own apps leave
		t.Fatalf("the first of the top-left buttons is at %v, want %v: barLead after the window's buttons", sbBtn.X, 84+barLead)
	}
	setBtn, ok := tt.Find(locales.CommonSettings())
	if !ok {
		t.Fatal("settings button in top-left chrome not found")
	}
	topLeft := ui.Rect{X: 0, Y: 0, W: max(84, setBtn.X+setBtn.W), H: frameTopH}
	if len(a.boxes) == 0 {
		t.Fatal("missing editor paragraph box")
	}
	firstPara := ui.Rect{X: a.editorR.X, Y: a.editorR.Y + a.boxes[0].Top, W: a.editorR.W, H: a.boxes[0].Height}
	if rectsOverlap(topLeft, firstPara) {
		t.Fatalf("top-left chrome %+v overlaps first paragraph %+v", topLeft, firstPara)
	}
	if titleR, ok := tt.Find(a.page.Title); ok && rectsOverlap(topLeft, titleR) {
		t.Fatalf("top-left chrome %+v overlaps page title %+v", topLeft, titleR)
	}

	a.settingsOpen = true
	tt.Advance(200 * time.Millisecond)
	navApp, ok := tt.Find(locales.CommonAppearance())
	if !ok {
		t.Fatal("Settings Appearance nav item not found")
	}
	if rectsOverlap(topLeft, navApp) {
		t.Fatalf("top-left chrome %+v overlaps Settings nav item %+v", topLeft, navApp)
	}
	a.settingsOpen = false
	tt.Advance(200 * time.Millisecond)

	// Open the sidebar again: both cards must share the exact same top edge and keep sidebar rows below buttons.
	a.toggleSidebar()
	tt.Advance(1200 * time.Millisecond)
	if !ownWindowControls && a.sidebarR.Y != a.panesR.Y {
		t.Fatalf("sidebar card top (%v) != pane card top (%v)", a.sidebarR.Y, a.panesR.Y)
	}
	if collHdr, ok := tt.Find(locales.SidebarCollections()); ok && rectsOverlap(topLeft, collHdr) {
		t.Fatalf("top-left chrome %+v overlaps sidebar Collections header %+v", topLeft, collHdr)
	}
	a.toggleSidebar()
	tt.Advance(1200 * time.Millisecond)

	if !ownWindowControls {
		second, err := a.page.Coll.NewPage(nil, "İkinci")
		if err != nil {
			t.Fatal(err)
		}
		a.openInSide(place{"page", second.Dir})
		tt.Advance(60 * time.Millisecond)
		if a.wframe.strip <= 0.01 || a.wframe.strip >= 0.99 {
			t.Fatalf("expected strip mid-animation, got %v", a.wframe.strip)
		}
		if a.panesR.Y != frameTopH { // the cards go under the top edge at once while the tabs come in (TestSplitKeepsThePaneStill)
			t.Fatalf("with the tabs coming in, the panes start at %v, not under the top edge (%v)", a.panesR.Y, frameTopH)
		}
	}
}

func regionDiffPixels(a, b *image.RGBA, r image.Rectangle) int {
	bnds := a.Bounds().Intersect(b.Bounds()).Intersect(r)
	diff := 0
	for y := bnds.Min.Y; y < bnds.Max.Y; y++ {
		for x := bnds.Min.X; x < bnds.Max.X; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				diff++
			}
		}
	}
	return diff
}

// Scrolled pages and Settings must never render text behind macOS traffic lights or top-left buttons (#450),
// and Settings keeps a non-scrolling DragWindow strip at the top while scrolled.
func TestScrolledContentClearsTopLeftButtons(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 60; i++ {
		b.WriteString(strings.Repeat("MMMMMM ", 14))
		b.WriteString("\n\n")
	}
	a, tt := newTestApp(t, b.String())
	tt.SetTitleBar(ui.TitleBar{Left: 84, Height: frameTopH})
	tt.SetSize(680, 620)
	a.toggleSidebar()
	tt.Advance(1200 * time.Millisecond)
	setBtn, ok := tt.Find(locales.CommonSettings())
	if !ok {
		t.Fatal("settings button in top-left chrome not found")
	}
	topLeft := ui.Rect{X: 0, Y: 0, W: max(84, setBtn.X+setBtn.W), H: frameTopH}
	trafficZone := image.Rect(int(frameW+4), int(frameW+2), int(setBtn.X-2), int(frameTopH-2))

	chrome := map[string]bool{
		locales.SidebarTopBar():  true,
		locales.SidebarSidebar(): true,
		msg("back2"):             true,
		msg("forward2"):          true,
		locales.CommonSettings(): true,
	}
	checkNoTextOverlap := func(phase string) {
		t.Helper()
		for _, acc := range tt.AccessAll() {
			if chrome[acc.Name] || acc.Offscreen || acc.Bounds.W == 0 || acc.Bounds.H == 0 {
				continue
			}
			if acc.Role == ui.RoleButton || acc.Role == ui.RoleText || acc.Role == ui.RoleHeading {
				if rectsOverlap(topLeft, acc.Bounds) && acc.Bounds.H < 200 {
					t.Fatalf("%s: element %q (%+v) overlaps topLeft %+v", phase, acc.Name, acc.Bounds, topLeft)
				}
			}
		}
	}

	beforePage := tt.Image()
	tt.Wheel(a.editorR.X+100, 300, 0, 500)
	tt.Advance(300 * time.Millisecond)
	if a.pageScroll.Y < 200 {
		t.Fatalf("page did not scroll enough: %v", a.pageScroll.Y)
	}
	if a.scrollR.Y < frameTopH {
		t.Fatalf("scroll viewport enters top-left chrome: scrollR.Y=%v < frameTopH=%d", a.scrollR.Y, frameTopH)
	}
	afterPage := tt.Image()
	if diff := regionDiffPixels(beforePage, afterPage, trafficZone); diff != 0 {
		t.Fatalf("scrolled page painted %d pixels inside traffic-light zone %v", diff, trafficZone)
	}
	checkNoTextOverlap("scrolled page")

	// Settings also clears topLeft, keeps a non-scrolling top DragWindow strip, and stays clean when scrolled.
	a.settingsOpen = true
	tt.Advance(300 * time.Millisecond)
	if !ownWindowControls && (a.headDragR.Y != frameW+1 || a.headDragR.H != frameTopH-frameW || a.headDragR.W < 300) {
		t.Fatalf("Settings top DragWindow strip = %+v, want Y=%d H=%d W>300", a.headDragR, frameW+1, frameTopH-frameW)
	}
	beforeSet := tt.Image()
	tt.Wheel(420, 300, 0, 400)
	tt.Advance(300 * time.Millisecond)
	if !ownWindowControls && (a.headDragR.Y != frameW+1 || a.headDragR.H != frameTopH-frameW) {
		t.Fatalf("Settings top DragWindow strip moved while scrolling: %+v", a.headDragR)
	}
	afterSet := tt.Image()
	if diff := regionDiffPixels(beforeSet, afterSet, trafficZone); diff != 0 {
		t.Fatalf("scrolled Settings painted %d pixels inside traffic-light zone %v", diff, trafficZone)
	}
	checkNoTextOverlap("scrolled Settings")
}

// Liquid Glass updates glassTint only when theme/dark changes, shows its restart note in Settings, applies
// glass.Glass to the palette only on darwin with Vibrancy enabled, and clears glassTint when turned off.
func TestGlassSyncAndToggle(t *testing.T) {
	a, tt := newTestApp(t, "hello\n")
	tt.SetSize(900, 620)
	tt.SetVibrancy(true)
	a.setGlass(true)
	tt.Frame()
	if runtime.GOOS == "darwin" {
		lightTint := a.glassTint
		if lightTint == "" {
			t.Fatal("expected glassTint to be set when Glass is enabled on darwin")
		}
		runsAfterInit := a.glassTintRuns
		if runsAfterInit != 1 {
			t.Fatalf("expected 1 initial glassTint computation, got %d", runsAfterInit)
		}
		// Static frames with unchanged theme/dark must not recompute glassTint.
		tt.Frame()
		tt.Frame()
		tt.Frame()
		if a.glassTintRuns != runsAfterInit {
			t.Fatalf("glassTint recomputed on unchanged frames: %d -> %d", runsAfterInit, a.glassTintRuns)
		}
		// Changing dark appearance recomputes glassTint once.
		tt.SetDark(true)
		tt.Frame()
		darkTint := a.glassTint
		if darkTint == "" || darkTint == lightTint || a.glassTintRuns != runsAfterInit+1 {
			t.Fatalf("expected single recompute on dark change: light=%q dark=%q runs=%d", lightTint, darkTint, a.glassTintRuns)
		}
		// Changing AppTheme recomputes glassTint once.
		a.settings.AppTheme = 1
		tt.Frame()
		themeTint := a.glassTint
		if themeTint == "" || themeTint == darkTint || a.glassTintRuns != runsAfterInit+2 {
			t.Fatalf("expected single recompute on theme change: dark=%q theme1=%q runs=%d", darkTint, themeTint, a.glassTintRuns)
		}
		tt.Frame()
		if a.glassTintRuns != runsAfterInit+2 {
			t.Fatalf("glassTint recomputed after theme change settled: %d != %d", a.glassTintRuns, runsAfterInit+2)
		}
		a.settings.AppTheme = 0
		tt.SetDark(false)
		tt.Frame()
	} else if a.glassTint != "" {
		t.Fatalf("expected empty glassTint outside darwin, got %q", a.glassTint)
	}

	// Settings shows the Liquid Glass restart note only on darwin.
	a.settingsTab = 0
	a.openSettings()
	tt.SetSize(900, 1400)
	tt.Advance(200 * time.Millisecond)
	if hasNote := tt.HasText(locales.SettingsGlassRestart()); runtime.GOOS == "darwin" && !hasNote {
		t.Fatalf("missing Liquid Glass restart note in Settings (has glass=%v)", tt.HasText(locales.SettingsGlass()))
	} else if runtime.GOOS != "darwin" && hasNote {
		t.Fatal("unexpected Liquid Glass restart note outside darwin")
	}
	tt.SetSize(900, 620)
	a.closeSettings()
	tt.Frame()

	// Palette uses glass.Glass only when darwin && Vibrancy && Glass.
	a.openPalette()
	tt.Frame()
	if !a.pal.open {
		t.Fatal("palette did not open under vibrancy")
	}
	palR, ok := tt.Find(a.palLabel())
	if !ok {
		t.Fatal("palette panel bounds not found")
	}
	palRect := image.Rect(int(palR.X)+4, int(palR.Y)+4, int(palR.X+palR.W)-4, int(palR.Y+palR.H)-4)
	imgGlassOnVibOn := tt.Image()

	tt.SetVibrancy(false)
	tt.Frame()
	imgGlassOnVibOff := tt.Image()

	tt.SetVibrancy(true)
	a.setGlass(false)
	tt.Frame()
	imgGlassOffVibOn := tt.Image()

	if diff := regionDiffPixels(imgGlassOnVibOff, imgGlassOffVibOn, palRect); diff != 0 {
		t.Fatalf("expected identical opaque palette when vibrancy=false or glass=false, got %d differing pixels", diff)
	}
	if runtime.GOOS == "darwin" {
		if diff := regionDiffPixels(imgGlassOnVibOn, imgGlassOnVibOff, palRect); diff == 0 {
			t.Fatal("expected palette glass.Glass material to alter rendered panel pixels on darwin with vibrancy=true")
		}
		if diff := regionDiffPixels(imgGlassOnVibOn, imgGlassOffVibOn, palRect); diff == 0 {
			t.Fatal("expected Glass=true vs Glass=false palette pixels to differ on darwin with vibrancy=true")
		}
	}
	if a.settings.Glass || a.glassTint != "" {
		t.Fatalf("expected Glass disabled and glassTint cleared, got glass=%v tint=%q", a.settings.Glass, a.glassTint)
	}
}

// TestFrameTabLongTitleMiddleTruncates checks that a long pane title stays constrained inside its
// capped frame tab while drawing both the beginning and the end (middle truncation) and keeping the full accessible label.
func TestFrameTabLongTitleMiddleTruncates(t *testing.T) {
	prefix := "Çok uzun sayfa başlığı ortadan kısaltılmalı ve sonu görünmeli "
	longTitle := prefix + "2026"
	a, tt := newTestApp(t, "içerik\n")
	first := a.page
	if err := first.SetTitle(longTitle); err != nil {
		t.Fatal(err)
	}
	a.title = longTitle
	second, err := first.Coll.NewPage(nil, "İkinci Sekme Başlığı Oldukça Uzun")
	if err != nil {
		t.Fatal(err)
	}
	tt.SetSize(760, 600)
	a.openInSide(place{"page", second.Dir})
	tt.Advance(900 * time.Millisecond)
	r, ok := tt.Find(longTitle)
	if !ok {
		t.Fatalf("long tab label %q not found", longTitle)
	}
	if r.W > 240.5 {
		t.Fatalf("tab width %v exceeded max 240", r.W)
	}
	tabRect := image.Rect(int(r.X)+2, int(r.Y)+2, int(r.X+r.W)-2, int(r.Y+r.H)-2)
	img2026 := tt.Image()

	// Change only the trailing "2026" -> "9999": middle truncation must change rendered tab pixels.
	altSuffix := prefix + "9999"
	if err := first.SetTitle(altSuffix); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find(altSuffix); !ok {
		t.Fatalf("updated tab label %q not found: %v", altSuffix, tt.Texts())
	}
	img9999 := tt.Image()
	if diff := regionDiffPixels(img2026, img9999, tabRect); diff == 0 {
		t.Fatal("rendered tab did not change when trailing \"2026\" changed to \"9999\"; tail is not shown (missing middle truncation)")
	}

	// Change only the leading prefix while keeping "2026": beginning must also be drawn.
	altPrefix := "Bir hayli uzun sayfa başlığı ortadan kısaltılmalı sonu görünsün 2026"
	if err := first.SetTitle(altPrefix); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find(altPrefix); !ok {
		t.Fatalf("updated prefix tab label %q not found", altPrefix)
	}
	imgHead := tt.Image()
	if diff := regionDiffPixels(img2026, imgHead, tabRect); diff == 0 {
		t.Fatal("rendered tab did not change when leading prefix changed; head is not shown")
	}
}

// TestSplitKeepsThePaneStill: while the window splits and the tabs come in, the new pane's page keeps its place and
// its height every frame, so nothing in it moves and its scroll bar does not show (#450): the cards' top edges change
// at once, not on the tabs' spring, which the Dock would follow a frame late.
func TestSplitKeepsThePaneStill(t *testing.T) {
	if ownWindowControls { // the cards rise only beside macOS's own window buttons: elsewhere the frame is as it was
		t.Skip("the window draws its own controls: the cards do not rise")
	}
	a, tt := newTestApp(t, longPage())
	tt.SetSize(900, 600)
	a.wframe.on = true
	tt.Advance(300 * time.Millisecond)
	p, err := a.page.Coll.NewPage(nil, "İkinci")
	if err != nil {
		t.Fatal(err)
	}
	p.SaveText(longPage())
	one := a.pageR // the window with one pane
	a.openAtPane(a.paneCount(), place{"page", p.Dir})
	var first ui.Rect
	for i := 0; i < 40; i++ {
		tt.Advance(16 * time.Millisecond)
		r := a.pageR
		if r.W == 0 || r == one { // the Dock is not laid out yet
			continue
		}
		if first.W == 0 {
			first = r
			continue
		}
		if r.Y != first.Y || r.H != first.H {
			t.Fatalf("frame %d (tabs %.2f): the pane's page is at %v, it was at %v", i, a.wframe.strip, r, first)
		}
	}
	if a.wframe.strip < 0.95 {
		t.Fatalf("the tabs did not come in: %.2f", a.wframe.strip)
	}
}
