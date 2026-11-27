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

// Launching the app opens directly onto Today's single editable sheet with zero taps and creates no empty file
// until content is actually written.
func TestOpensOnlyOnToday(t *testing.T) {
	a, tt := newTestApp(t)
	j := a.journalColl()
	if j == nil || !j.Journal || j.Icon != "calendar" {
		t.Fatalf("journal %+v", j)
	}
	if !a.ed.isTodayDraft || a.todayPage() != nil {
		t.Fatal("expected clean unwritten Today draft on launch")
	}

	a.ed.doc.SetText(textOf(library.Doc{{R: []library.Run{{T: "Bugün "}, {T: "harika", W: 700}}}}))
	a.typed(tt.Now())
	tt.Advance(saveQuiet + 200*time.Millisecond)
	tt.Frame()

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
	dawnBg, _ := paperToneAt(time.Date(2026, 11, 28, 6, 0, 0, 0, time.Local), false)
	sunsetBg, _ := paperToneAt(time.Date(2026, 11, 28, 18, 30, 0, 0, time.Local), false)
	nightBg, _ := paperToneAt(time.Date(2026, 11, 28, 0, 0, 0, 0, time.Local), true)
	if dawnBg == sunsetBg || nightBg == dawnBg {
		t.Fatalf("expected distinct time-of-day paper tones: dawn=%v sunset=%v night=%v", dawnBg, sunsetBg, nightBg)
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
