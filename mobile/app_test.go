package main

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

var registerOnce sync.Once

func newTestApp(t *testing.T) (*app, *ui.Tester) {
	t.Helper()
	registerOnce.Do(registerFonts)
	a := newApp(t.TempDir())
	if err := a.open(); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 412, 860)
	tt.Frame()
	return a, tt
}

// Launching the app opens directly onto Today's single editable sheet with zero taps and creates no empty journal
// collection or page on disk until content is actually written (preventing duplicate "Journal" + "Günlük" on sync).
func TestOpensOnlyOnToday(t *testing.T) {
	a, tt := newTestApp(t)
	if len(a.lib.Collections) != 0 {
		t.Fatalf("fresh open should not create placeholder collections before writing or sync: %+v", a.lib.Collections)
	}
	if !a.ed.isTodayDraft || a.todayPage() != nil {
		t.Fatal("expected clean unwritten Today draft on launch")
	}

	a.ed.doc.SetText(textOf(library.Doc{{R: []library.Run{{T: "Bugün "}, {T: "harika", W: 700}}}}))
	a.typed(tt.Now())
	tt.Advance(saveQuiet + 200*time.Millisecond)
	tt.Frame()

	j := a.journalColl()
	if j == nil || !j.Journal || j.Icon != "calendar" {
		t.Fatalf("journal not created on first write: %+v", j)
	}
	p := a.todayPage()
	if p == nil || p.Props[journalDayKey] != startOfDay(a.now()).Format("2006-01-02") {
		t.Fatalf("today page not saved: %+v", p)
	}
	if a.ed.id != p.ID || a.ed.isTodayDraft {
		t.Fatalf("editor not bound to Today's saved page: id=%q draft=%v", a.ed.id, a.ed.isTodayDraft)
	}
	md, _ := p.Text()
	if !strings.Contains(md, "Bugün **harika**") {
		t.Fatalf("saved markdown = %q", md)
	}
	if !tt.HasText(locales.PageWords(2)) {
		t.Fatalf("expected live word count, got: %v", tt.Texts())
	}
}

// When desktop sync brings a localized journal (e.g. "Günlük"), the phone picks it over any empty placeholder
// "Journal" and removes the empty duplicate cleanly.
func TestJournalPrefersLocalizedAndCleansEmptyDuplicate(t *testing.T) {
	a, _ := newTestApp(t)
	// Simulate an empty English "Journal" and a synced Turkish "Günlük" with pages.
	en, err := a.journal()
	if err != nil {
		t.Fatal(err)
	}
	tr, err := a.lib.NewCollection("Günlük")
	if err != nil {
		t.Fatal(err)
	}
	tr.Journal = true
	if err := tr.Save(); err != nil {
		t.Fatal(err)
	}
	tr.Creating = true
	p, err := tr.NewPage(nil, "2026-11-27")
	tr.Creating = false
	if err != nil {
		t.Fatal(err)
	}
	p.Props = map[string]string{journalDayKey: "2026-11-27"}
	p.SaveText("Masaüstünden")

	if got := a.journalColl(); got != tr {
		t.Fatalf("journalColl = %q, want Günlük (en=%q)", got.Name, en.Name)
	}
	a.cleanEmptyDuplicateJournals()
	if len(a.lib.Collections) != 1 || a.lib.Collections[0].Name != "Günlük" {
		t.Fatalf("empty duplicate Journal was not cleaned: %+v", a.lib.Collections)
	}
}

// When midnight passes, yesterday's unsaved edits are flushed onto yesterday's page, yesterday is sealed, and the
// app automatically turns to a clean blank sheet for the new Today.
func TestMidnightTurnsThePage(t *testing.T) {
	registerOnce.Do(registerFonts)
	now := time.Date(2026, 11, 28, 23, 59, 59, 0, time.Local)
	a := newApp(t.TempDir())
	a.clock = func() time.Time { return now }
	if err := a.open(); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 412, 860)
	tt.Frame()

	a.ed.doc.SetText(textOf(library.Doc{{R: []library.Run{{T: "Gece notu"}}}}))
	a.typed(tt.Now())

	// Cross midnight before debounce timer fires.
	now = time.Date(2026, 11, 29, 0, 0, 5, 0, time.Local)
	tt.Frame()

	yesterday := a.dayPage(time.Date(2026, 11, 28, 0, 0, 0, 0, time.Local))
	if yesterday == nil {
		t.Fatal("yesterday's page was not flushed before midnight rollover")
	}
	if yesterday.Locked == "" {
		t.Fatal("yesterday was not sealed after midnight")
	}
	if md, _ := yesterday.Text(); !strings.Contains(md, "Gece notu") {
		t.Fatalf("yesterday markdown = %q", md)
	}
	if !a.activeDay.Equal(time.Date(2026, 11, 29, 0, 0, 0, 0, time.Local)) || !a.ed.isTodayDraft {
		t.Fatalf("did not turn cleanly to new Today: activeDay=%v draft=%v", a.activeDay, a.ed.isTodayDraft)
	}
	if got := strings.TrimSpace(a.ed.doc.Text().PlainText()); got != "" {
		t.Fatalf("new Today should be blank, got %q", got)
	}
}

// While writing on Today, the keyboard accessory bar toggles tasks & releases focus via Done; direct checkbox taps
// tick items without focusing the editor, and the header tally updates live.
func TestWritingBarTaskTallyAndTapCheckbox(t *testing.T) {
	a, tt := newTestApp(t)
	p, err := a.makeToday()
	if err != nil {
		t.Fatal(err)
	}
	p.SaveText("süt al")
	a.showToday()
	tt.Frame()

	// Tap inside Today's paper body below the masthead & daylight hairline.
	tt.Press(120, 260)
	tt.Release(120, 260)
	tt.Advance(100 * time.Millisecond)

	if err := tt.Click(locales.PageTask()); err != nil {
		t.Fatalf("task toggle missing while writing: %v (texts: %v)", err, tt.Texts())
	}
	tt.Advance(saveQuiet + 200*time.Millisecond)
	tt.Frame()
	if md, _ := p.Text(); !strings.Contains(md, "- [ ] süt al") {
		t.Fatalf("saved markdown = %q", md)
	}
	if !tt.HasText("0/1") {
		t.Fatalf("expected 0/1 task tally, got: %v", tt.Texts())
	}

	if err := tt.Click(locales.PageDone()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if a.ed.writing {
		t.Fatal("Done did not release editor focus")
	}

	// Tap directly on the task checkbox without focusing the editor.
	boxes := a.ed.doc.VisibleParagraphs()
	if len(boxes) == 0 {
		t.Fatal("expected visible paragraphs")
	}
	var body ui.Element
	a.tapText(&a.ed, body, 1, boxes[0].Top+4)
	tt.Advance(saveQuiet + 200*time.Millisecond)
	tt.Frame()
	if md, _ := p.Text(); !strings.Contains(md, "- [x] süt al") {
		t.Fatalf("direct checkbox tap saved %q", md)
	}
	if !tt.HasText("1/1") {
		t.Fatalf("expected 1/1 task tally, got: %v", tt.Texts())
	}
	if a.ed.writing {
		t.Fatal("checkbox tap unexpectedly focused editor")
	}
}

// The discreet corner icon opens the minimal Settings & Sync sheet; theme selection persists and Android Back walks
// cleanly from SignIn -> Settings -> Today.
func TestSettingsSheetAndBackNavigation(t *testing.T) {
	a, tt := newTestApp(t)
	if err := tt.Click(locales.SettingsSync()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	if a.sheet != sheetSettings {
		t.Fatalf("sheet = %v, want settings", a.sheet)
	}
	if err := tt.Click(locales.SettingsDark()); err != nil {
		t.Fatal(err)
	}
	if a.settings.Theme != "dark" || !dark {
		t.Fatalf("theme = %q dark=%v", a.settings.Theme, dark)
	}
	if err := tt.Click(locales.SyncSignIn()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	if a.sheet != sheetSignIn {
		t.Fatalf("sheet = %v, want signIn", a.sheet)
	}
	if !a.handleBack() || a.sheet != sheetSettings {
		t.Fatalf("back from SignIn -> %v", a.sheet)
	}
	if !a.handleBack() || a.sheet != sheetNone {
		t.Fatalf("back from Settings -> %v", a.sheet)
	}
}

func TestDayProgressAndFormatDay(t *testing.T) {
	noon := time.Date(2026, 11, 28, 12, 0, 0, 0, time.Local)
	if p := dayProgress(noon); p < 0.49 || p > 0.51 {
		t.Fatalf("noon progress = %v, want ~0.5", p)
	}

	// Verify continuous minute-by-minute solar gradient progression (not blocky jumps).
	clearWx := weatherInfo{HasWeather: true, Kind: weatherClear, SunriseHr: 7.0, SunsetHr: 18.0, SolarDay: "2026-11-28"}
	t1700 := time.Date(2026, 11, 28, 17, 0, 0, 0, time.Local)
	t1720 := time.Date(2026, 11, 28, 17, 20, 0, 0, time.Local)
	top1, bot1, _, dark1 := paperGradientAt(t1700, "", clearWx)
	top2, bot2, _, _ := paperGradientAt(t1720, "", clearWx)
	if top1 == top2 || bot1 == bot2 {
		t.Fatalf("expected continuous minute-by-minute gradient shift between 17:00 and 17:20")
	}
	if top1 == bot1 {
		t.Fatalf("golden hour should render a vertical gradient, got flat %v", top1)
	}
	if dark1 {
		t.Fatal("17:00 golden hour should still use daylight polarity")
	}

	// Bright midday vs shadowed dark-blue evening vs deep night.
	mTop, mBot, _, mDark := paperGradientAt(time.Date(2026, 11, 28, 12, 0, 0, 0, time.Local), "", clearWx)
	eTop, eBot, _, eDark := paperGradientAt(time.Date(2026, 11, 28, 20, 0, 0, 0, time.Local), "", clearWx)
	nTop, _, _, nDark := paperGradientAt(time.Date(2026, 11, 28, 0, 30, 0, 0, time.Local), "", clearWx)
	if mDark || luminance(mTop.MixOklab(mBot, 0.5)) < 0.9 {
		t.Fatalf("midday should be bright daylight, dark=%v lum=%v", mDark, luminance(mTop))
	}
	if !eDark || eBot.B <= eBot.R {
		t.Fatalf("evening should be dark shadowed blue, dark=%v bot=%v", eDark, eBot)
	}
	if !nDark || luminance(nTop) >= luminance(eTop) {
		t.Fatalf("midnight should be darker than evening: night=%v evening=%v", nTop, eTop)
	}

	// Live weather modulates the sky-paper gradient cleanly.
	rainWx := clearWx
	rainWx.Kind = weatherRain
	rTop, _, _, _ := paperGradientAt(noon, "", rainWx)
	if rTop == mTop {
		t.Fatal("rainy weather did not modulate midday sky-paper color")
	}
	if wmoToWeatherKind(61, 90) != weatherRain || wmoToWeatherKind(73, 100) != weatherSnow || wmoToWeatherKind(95, 100) != weatherStorm {
		t.Fatal("WMO weather classification mismatch")
	}
	if hr, day, ok := parseLocalISOHour("2026-11-28T17:38"); !ok || day != "2026-11-28" || hr < 17.6 || hr > 17.7 {
		t.Fatalf("parseLocalISOHour = %v %q %v", hr, day, ok)
	}

	if paperGrainBitmap() == nil {
		t.Fatal("paper grain bitmap not created")
	}
	d := time.Date(2026, 11, 27, 0, 0, 0, 0, time.Local)
	for layout, want := range map[string]string{"": "2026-11-27", "DD.MM.YYYY": "27.11.2026", `D \de MMMM`: "27 de November"} {
		if got := formatDay(layout, d); got != want {
			t.Errorf("%q: %q, want %q", layout, got, want)
		}
	}
}
