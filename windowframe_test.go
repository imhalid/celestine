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

// TestPaneBar (#468): split, the frame's top edge has no tabs and the cards stay at the window's top; each pane has a
// bar with its page's menu and a close button, and its title only once the page's own big title has scrolled away.
// The close button closes its pane; alone, a pane's close button closes its page, and its collection is shown.
func TestPaneBar(t *testing.T) {
	a, tt := newTestApp(t, longPage())
	tt.SetSize(1200, 700)
	tt.Advance(300 * time.Millisecond)
	second, err := a.pane().page.Coll.NewPage(nil, "İkinci")
	if err != nil {
		t.Fatal(err)
	}
	second.SaveText(longPage())
	a.openInSide(place{"page", second.Dir})
	tt.Advance(900 * time.Millisecond)
	if a.paneCount() != 2 {
		t.Fatalf("panes %d", a.paneCount())
	}
	if _, ok := tt.Find("Maximize"); ok {
		t.Fatal("an area has the Dock's own header")
	}
	bars := 0
	for _, s := range tt.Texts() {
		if s == locales.PanesBar() {
			bars++
		}
	}
	if bars != 2 {
		t.Fatalf("%d pane bars, want 2: %v", bars, tt.Texts())
	}
	// the title is in the bar once the page has scrolled under it
	titles := func() int { // the sidebar's row has it too, and the page's own title
		n := 0
		for _, s := range tt.Texts() {
			if s == "İkinci" {
				n++
			}
		}
		return n
	}
	unscrolled := titles()
	r := a.panesR
	tt.Wheel(r.X+r.W*3/4, r.Y+300, 0, 600)
	tt.Advance(600 * time.Millisecond)
	if a.pane().pageScroll.Y < a.pane().headH {
		t.Fatalf("the page did not scroll past its title: %v < %v", a.pane().pageScroll.Y, a.pane().headH)
	}
	if titles() != unscrolled+1 {
		t.Fatalf("scrolled, the bar does not say the page's title: %d of it, %d before", titles(), unscrolled)
	}
	// closing the second pane by its bar
	if err := tt.Click(locales.CloseThePane()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(900 * time.Millisecond)
	if a.paneCount() != 1 {
		t.Fatalf("after its X: panes %d", a.paneCount())
	}
	// alone, the X closes the page: its collection is shown
	if err := tt.Click(locales.PanesClosePage()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	if !a.noPage() || a.pane().coll == nil || a.pane().coll != second.Coll {
		t.Fatalf("the page is still shown (%v), or not its collection (%v)", !a.noPage(), a.pane().coll)
	}
}

func rectsOverlap(a, b ui.Rect) bool {
	return a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y
}

// With no tabs and the sidebar folded, the pane card rises to frameW on macOS while keeping initial page
// content and the Settings nav list clear of traffic lights and top-left buttons, and a second pane opened
// keeps the cards at the top.
func TestNoTabsCardClearTopLeftButtons(t *testing.T) {
	a, tt := newTestApp(t, "Birinci paragraf\n")
	tt.SetTitleBar(ui.TitleBar{Left: 84, Height: frameTopH})
	tt.SetSize(680, 620)
	a.toggleSidebar()
	tt.Advance(1200 * time.Millisecond)
	if a.sidebarShown > 0.1 {
		t.Fatalf("sidebar did not close: %v", a.sidebarShown)
	}
	if frameW != 8 || frameR != 8 {
		t.Fatalf("frame constants = (%d, %d), want (8, 8)", frameW, frameR)
	}
	if !ownWindowControls && (a.panesR.Y != frameW || a.pane().scrollR.Y != frameTopH+1) {
		t.Fatalf("single-pane card = (%v, %v), want (%d, %d)", a.panesR.Y, a.pane().scrollR.Y, frameW, frameTopH+1)
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
	if len(a.pane().boxes) == 0 {
		t.Fatal("missing editor paragraph box")
	}
	firstPara := ui.Rect{X: a.pane().editorR.X, Y: a.pane().editorR.Y + a.pane().boxes[0].Top, W: a.pane().editorR.W, H: a.pane().boxes[0].Height}
	if rectsOverlap(topLeft, firstPara) {
		t.Fatalf("top-left chrome %+v overlaps first paragraph %+v", topLeft, firstPara)
	}
	if titleR, ok := tt.Find(locales.CommonTitle()); !ok {
		t.Fatal("page title input not found")
	} else if rectsOverlap(topLeft, titleR) {
		t.Fatalf("top-left chrome %+v overlaps page title %+v", topLeft, titleR)
	}

	a.pane().settingsOpen = true
	tt.Frame() // set from outside a frame: a window at rest draws none of its own
	tt.Advance(200 * time.Millisecond)
	navApp, ok := tt.Find(locales.CommonAppearance())
	if !ok {
		t.Fatal("Settings Appearance nav item not found")
	}
	if rectsOverlap(topLeft, navApp) {
		t.Fatalf("top-left chrome %+v overlaps Settings nav item %+v", topLeft, navApp)
	}
	a.pane().settingsOpen = false
	tt.Advance(200 * time.Millisecond)

	// Open the sidebar again: both cards must share the exact same top edge and keep sidebar rows below buttons.
	a.toggleSidebar()
	tt.Advance(1200 * time.Millisecond)
	if !ownWindowControls && a.sidebarR.Y != a.panesR.Y {
		t.Fatalf("sidebar card top (%v) != pane card top (%v)", a.sidebarR.Y, a.panesR.Y)
	}
	if collHdr, ok := tt.Find(locales.SidebarCollapseAll()); ok && rectsOverlap(topLeft, collHdr) {
		t.Fatalf("top-left chrome %+v overlaps the sidebar's head %+v", topLeft, collHdr)
	}
	a.toggleSidebar()
	tt.Advance(1200 * time.Millisecond)

	if !ownWindowControls {
		second, err := a.pane().page.Coll.NewPage(nil, "İkinci")
		if err != nil {
			t.Fatal(err)
		}
		a.openInSide(place{"page", second.Dir})
		tt.Advance(600 * time.Millisecond)
		if a.panesR.Y != frameW { // split, the cards stay at the window's top: the panes have bars, not tabs (#468)
			t.Fatalf("split, the panes start at %v, not at the window's top (%v)", a.panesR.Y, frameW)
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
	tt.Wheel(a.pane().editorR.X+100, 300, 0, 500)
	tt.Advance(300 * time.Millisecond)
	if a.pane().pageScroll.Y < 200 {
		t.Fatalf("page did not scroll enough: %v", a.pane().pageScroll.Y)
	}
	if a.pane().scrollR.Y < frameTopH {
		t.Fatalf("scroll viewport enters top-left chrome: scrollR.Y=%v < frameTopH=%d", a.pane().scrollR.Y, frameTopH)
	}
	afterPage := tt.Image()
	if diff := regionDiffPixels(beforePage, afterPage, trafficZone); diff != 0 {
		t.Fatalf("scrolled page painted %d pixels inside traffic-light zone %v", diff, trafficZone)
	}
	checkNoTextOverlap("scrolled page")

	// Settings also clears topLeft, keeps a non-scrolling top DragWindow strip, and stays clean when scrolled.
	a.pane().settingsOpen = true
	tt.Advance(300 * time.Millisecond)
	if !ownWindowControls && (a.pane().headDragR.Y != frameW+1 || a.pane().headDragR.H != frameTopH-frameW || a.pane().headDragR.W < 300) {
		t.Fatalf("Settings top DragWindow strip = %+v, want Y=%d H=%d W>300", a.pane().headDragR, frameW+1, frameTopH-frameW)
	}
	beforeSet := tt.Image()
	tt.Wheel(420, 300, 0, 400)
	tt.Advance(300 * time.Millisecond)
	if !ownWindowControls && (a.pane().headDragR.Y != frameW+1 || a.pane().headDragR.H != frameTopH-frameW) {
		t.Fatalf("Settings top DragWindow strip moved while scrolling: %+v", a.pane().headDragR)
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

	// Settings shows the Liquid Glass restart badge only on darwin, Glass turned on since the window was made (#497).
	a.settings.Glass, a.glassAtLaunch = true, false
	a.pane().settingsTab = 0
	a.openSettings()
	tt.SetSize(900, 1400)
	tt.Advance(200 * time.Millisecond)
	if hasNote := tt.HasText(locales.SettingsNeedsRestart()); runtime.GOOS == "darwin" && !hasNote {
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
	first := a.pane().page
	if err := first.SetTitle(longTitle); err != nil {
		t.Fatal(err)
	}
	a.pane().title = longTitle
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

// TestSplitKeepsThePaneStill: while the window splits, the new pane's page keeps its place and its height every
// frame, so nothing in it moves and its scroll bar does not show (#450): the cards stay at the window's top, each
// with its bar (#468), and nothing moves on a spring the Dock would follow a frame late.
func TestSplitKeepsThePaneStill(t *testing.T) {
	if ownWindowControls { // the cards rise only beside macOS's own window buttons: elsewhere the frame is as it was
		t.Skip("the window draws its own controls: the cards do not rise")
	}
	a, tt := newTestApp(t, longPage())
	tt.SetSize(900, 600)
	a.wframe.on = true
	tt.Advance(300 * time.Millisecond)
	p, err := a.pane().page.Coll.NewPage(nil, "İkinci")
	if err != nil {
		t.Fatal(err)
	}
	p.SaveText(longPage())
	one := a.pane().pageR // the window with one pane
	a.openAtPane(a.paneCount(), place{"page", p.Dir})
	var first ui.Rect
	for i := 0; i < 40; i++ {
		tt.Advance(16 * time.Millisecond)
		r := a.pane().pageR
		if r.W == 0 || r == one { // the Dock is not laid out yet
			continue
		}
		if first.W == 0 {
			first = r
			continue
		}
		if r.Y != first.Y || r.H != first.H {
			t.Fatalf("frame %d: the pane's page is at %v, it was at %v", i, r, first)
		}
	}
}

// TestPaneBarDrag (#468): a pane's bar held and carried over another pane's compass moves the pane there: let go
// on the left arm of the first pane, the second goes before it.
func TestPaneBarDrag(t *testing.T) {
	a, tt := newTestApp(t, "Birinci\n")
	tt.SetSize(1400, 800)
	tt.Advance(300 * time.Millisecond)
	second, err := a.pane().page.Coll.NewPage(nil, "İkinci")
	if err != nil {
		t.Fatal(err)
	}
	second.SaveText("Öteki\n")
	a.openInSide(place{"page", second.Dir})
	tt.Advance(900 * time.Millisecond)
	before := a.ps.layout.PaneIDs()
	if len(before) != 2 {
		t.Fatalf("panes %v", before)
	}
	r0, r1 := a.ps.rects[before[0]], a.ps.rects[before[1]]
	x, y := r0.X+r0.W/2-(compassTileOf()+4), r0.Y+r0.H/2 // the compass's left arm, over the first pane
	holdTo(tt, r1.X+r1.W/2, r1.Y+12, x, y)               // from the middle of the second pane's bar
	if !a.paneDrag.active || a.carry.drop == nil {
		t.Fatal("the bar held and moved does not carry its pane")
	}
	tt.Release(x, y)
	tt.Advance(600 * time.Millisecond)
	if after := a.ps.layout.PaneIDs(); len(after) != 2 || after[0] != before[1] || after[1] != before[0] {
		t.Fatalf("panes %v, then %v: the second did not go before the first", before, after)
	}
	if a.carry.drop != nil || a.paneDrag.active {
		t.Fatal("still carried after it was let go")
	}
}

// compassTileOf is the side of one of the Dock's compass targets (Ryugo's compassTile): the middle of the left arm is
// that and a gap left of the compass's middle.
func compassTileOf() float32 { return 30 }

// The cards' corners follow the window's: its radius less the frame's width, concentric with it, or frameR at the
// Corners measure where its corners are square or not told (#517). The window opens with the card's corner at that
// measure and the frame around it, so the two stay concentric at every measure (#600).
func TestCardRadiusFollowsTheWindow(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.wframe.on = true
	defer func(s float32) { radiusScale = s }(radiusScale)
	radiusScale = 1
	for _, c := range []struct{ win, want float64 }{{26, 18}, {16, 8}, {0, frameR}, {6, frameR}} {
		a.winRadius = c.win
		if got := a.cardR(); float64(got) != c.want {
			t.Errorf("a window of %v: cards of %v, want %v", c.win, got, c.want)
		}
	}
	for _, p := range []int{0, 50, 100, 150, 200} {
		radiusScale = float32(p) / 100
		a.winRadius = float64(winCorner(p))
		if got, want := a.cardR(), rad(frameR); a.winRadius > frameW && got != want {
			t.Errorf("Corners at %d%%: a window of %v, cards of %v, want %v", p, a.winRadius, got, want)
		}
		if got := float32(a.winRadius) - a.cardR(); a.winRadius > frameW && got != frameW {
			t.Errorf("Corners at %d%%: window %v and cards %v are not concentric", p, a.winRadius, a.cardR())
		}
	}
	if winCorner(100) != 16 {
		t.Errorf("the window's corner at 100%% is %d, want 16", winCorner(100))
	}
	radiusScale = 1
	a.winRadius = 26
	tt.Frame() // drawn with it: the sidebar's card, the page's, and the pane's (cardRadius) take cardR
}
