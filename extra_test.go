package main

import (
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

func TestHideStatusBar(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.Frame()
	if !tt.HasText("Kaydedildi") {
		t.Fatalf("no status bar to hide: %v", tt.Texts())
	}
	a.settings.HideStatus = true
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if tt.HasText("Kaydedildi") {
		t.Fatal("the status bar is still drawn")
	}
	if readingTime(0) != "" || readingTime(150) != "1 dk'dan az" || readingTime(1000) != "5 dk" {
		t.Fatalf("%q %q %q", readingTime(0), readingTime(150), readingTime(1000))
	}
}

// What is shown puts its commands first, and a selection brings its formats.
func TestPaletteContext(t *testing.T) {
	a, tt := newTestApp(t, "bir iki üç\n")
	tt.Frame()
	for _, it := range a.commands() {
		if strings.HasPrefix(it.title, "Seçimi") {
			t.Fatalf("a format without a selection: %q", it.title)
		}
	}
	a.doc.SetSelection(0, 3)
	var bold *palItem
	items := a.commands()
	for i := range items {
		if items[i].title == "Seçimi kalın yap" {
			bold = &items[i]
		}
	}
	if bold == nil || bold.boost == 0 {
		t.Fatalf("no format commands for a selection: %+v", bold)
	}
	a.runItem(*bold)
	if f := a.doc.Text().Slice(0, 3); !strings.Contains(f.String(), "bir") {
		t.Fatal("text changed")
	}
	// page commands beat the same words elsewhere
	if got := a.paletteItems(">sayfa"); len(got) == 0 {
		t.Fatal("nothing")
	}
}

func TestSettingsSearchAndReset(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	res := searchDefs("yazi boyutu")
	if len(res) == 0 || res[0].name != "common_fontSize" {
		t.Fatalf("search: %v", res)
	}
	if len(searchDefs("autosave")) == 0 || searchDefs("autosave")[0].name != "common_autosave" {
		t.Fatal("the other language does not find it")
	}
	if len(a.changedDefs()) != 0 {
		t.Fatalf("changed from the start: %v", a.changedDefs())
	}
	a.setFontSize(20)
	a.settings.ShowSpaces = true
	a.settings.Corner = new(int)
	ch := a.changedDefs()
	names := map[string]bool{}
	for _, d := range ch {
		names[d.name] = true
	}
	if !names["common_fontSize"] || !names["showSpaces"] || !names["common_cornerRoundness"] {
		t.Fatalf("changed: %v", names)
	}
	for _, d := range ch {
		if d.name == "common_fontSize" {
			d.reset(a)
		}
	}
	if a.fontSize() != 16.5 {
		t.Fatalf("not back: %v", a.fontSize())
	}
	if n := a.resetAll(); n != 3 || len(a.changedDefs()) != 0 || a.settings.ShowSpaces {
		t.Fatalf("resetAll %d: %v", n, a.changedDefs())
	}
	// the screen
	a.openSettings()
	tt.Frame()
	a.setSearch = "kayıt"
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Otomatik kayıt") || !tt.HasText("⌘P › Otomatik kayıt") && !strings.Contains(strings.Join(tt.Texts(), "|"), "⌘P › Otomatik kayıt") {
		t.Fatalf("results: %v", tt.Texts())
	}
	a.setSearch, a.setMode = "", "changed"
	tt.Frame()
	if !tt.HasText(locales.SettingsNothingHasChangedAll()) {
		t.Fatalf("%v", tt.Texts())
	}
	a.setFontSize(21)
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Hepsini varsayılana döndür (1)") {
		t.Fatalf("%v", tt.Texts())
	}
}

func TestProfiles(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	a.settings.Theme = 2
	if !a.applyProfile("code") {
		t.Fatal("no built-in profile")
	}
	if a.fontSize() != 14.5 || !a.settings.ShowSpaces || !a.settings.NoLigatures {
		t.Fatalf("not applied: %v %v", a.fontSize(), a.settings.ShowSpaces)
	}
	if a.settings.Theme != 0 { // a profile is what it says: the theme it does not hold is the default
		t.Fatalf("theme kept: %d", a.settings.Theme)
	}
	a.settings.MCP, a.settings.Token = true, "gizli"
	a.setFontSize(23)
	a.settings.Theme = 1
	a.saveProfile("Benimki")
	a.applyProfile("settings_profileWriter")
	if a.fontSize() != 18 || a.settings.ShowSpaces {
		t.Fatal("Yazar not applied")
	}
	if !a.applyProfile("Benimki") || a.fontSize() != 23 || a.settings.Theme != 1 {
		t.Fatalf("saved profile: %v theme %d", a.fontSize(), a.settings.Theme)
	}
	if !a.settings.MCP || a.settings.Token != "gizli" {
		t.Fatal("a profile touched what is not a look")
	}
	found := false
	for _, it := range a.commands() {
		if it.title == "Profil: Benimki" {
			found = true
		}
	}
	if !found || a.applyProfile("yok böyle") {
		t.Fatal("palette or unknown profile")
	}
	a.deleteProfile("Benimki")
	if a.applyProfile("Benimki") {
		t.Fatal("deleted, and applied")
	}
}

func TestSettingsFile(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	a.settingsFile = filepath.Join(t.TempDir(), "settings.json")
	a.setFontSize(19)
	a.settings.Token, a.settings.SyncKey, a.settings.QuickKey = "gizli-jeton", "gizli-anahtar", "cmd+x"
	a.settings.Writing = map[string]int{"2026-11-07": 5}
	p := filepath.Join(t.TempDir(), "ayarlar.json")
	if err := a.exportSettingsTo(p); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	for _, secret := range []string{"gizli-jeton", "gizli-anahtar", "writing", "quickKey"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("%q was exported", secret)
		}
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	if m["fontSize"] != 19.0 {
		t.Fatalf("%v", m)
	}
	c, _ := newTestApp(t, "x\n")
	c.settings.Token = "başkasının"
	n, err := c.importSettingsFrom(p)
	if err != nil || n == 0 || c.fontSize() != 19 {
		t.Fatalf("import: %d %v %v", n, err, c.fontSize())
	}
	if c.settings.Token != "başkasının" {
		t.Fatal("a secret was imported")
	}
	// a file with a secret in it is not trusted
	os.WriteFile(p, []byte(`{"token":"çalındı","fontSize":15}`), 0o644)
	c.importSettingsFrom(p)
	if c.settings.Token != "başkasının" || c.fontSize() != 15 {
		t.Fatal("secret taken from a file")
	}
	if _, err := c.importSettingsFrom(filepath.Join(t.TempDir(), "yok.json")); err == nil {
		t.Fatal("a missing file imported")
	}
	os.WriteFile(p, []byte("kırık"), 0o644)
	if _, err := c.importSettingsFrom(p); err == nil {
		t.Fatal("junk imported")
	}
}

func TestFocusVeilAndTabs(t *testing.T) {
	a, tt := newTestApp(t, "bir\n\niki\n\nüç\n\ndört\n")
	a.doc.SetSelection(5, 5)
	tt.Advance(100 * time.Millisecond)
	snap := func() *image.RGBA {
		tt.Frame()
		tt.Advance(60 * time.Millisecond)
		im := *tt.Image()
		im.Pix = append([]byte(nil), im.Pix...)
		return &im
	}
	off := snap()
	a.settings.Focus = "paragraph"
	on := snap()
	r := image.Rect(int(a.editorR.X), int(a.editorR.Y), int(a.editorR.X+a.editorR.W), int(a.editorR.Y)+300)
	if d := pixelsDiffer(off, on, r, image.Rectangle{}); d < 100 {
		t.Fatalf("nothing veiled: %d", d)
	}
	a.settings.Focus = ""
	// a Tab
	a.settings.TabSpaces = 4
	a.doc.SetSelection(0, 0)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Advance(50 * time.Millisecond)
	if !strings.HasPrefix(a.doc.Text().String(), "    bir") {
		t.Fatalf("tab as spaces: %q", a.doc.Text().String())
	}
	tt.Key(ui.Shift, ui.KeyTab)
	tt.Advance(50 * time.Millisecond)
	_ = a
	if a.tabText() != "    " {
		t.Fatal("tabText")
	}
	a.settings.TabSpaces = 0
	if a.tabText() != "\t" {
		t.Fatal("tabText default")
	}
	if a.caretSleep() != 5*time.Second {
		t.Fatal("sleep default")
	}
	a.settings.SleepSecs = 12
	if a.caretSleep() != 12*time.Second {
		t.Fatal("sleep set")
	}
}

func TestRevertToBeforeSave(t *testing.T) {
	st := testStore(t)
	lib, _ := OpenLibraryIn(st, StoreRoot)
	a := newApp(lib)
	p := lib.Collections[0].All()[0]
	a.open(p)
	p.SaveText("ilk metin\n")
	h, _ := historyOf(p)
	h.SnapshotNow(p.File())
	p.SaveText("ikinci metin\n")
	a.revertToBeforeSave()
	got, _ := p.Text()
	if !strings.Contains(got, "ilk metin") {
		t.Fatalf("not reverted: %q (%s)", got, a.notice)
	}
}

func TestStatsTagsAndPageTime(t *testing.T) {
	a, _ := statsApp(t, "iki kelime #meyve\n")
	a.linksStale()
	a.linksTo(a.page, a.clock(), true)
	a.noteWords(a.page, 25)
	tags := a.sumBy(func(u *dayUse) map[string]int { return u.Tags })
	if tags["meyve"] != 25 {
		t.Fatalf("tags %v", tags)
	}
	a.statLast = a.clock()
	a.stirred()
	for i := 0; i < 20; i++ {
		a.setClock(2 * time.Second)
		a.statTick()
	}
	secs := a.sumBy(func(u *dayUse) map[string]int { return u.PageSecs })
	if secs[a.page.Dir] < 30 {
		t.Fatalf("page time %v", secs)
	}
}

// What dragging does in the sidebar, the palette does: a page up or down its row, out of its parent, into another collection.
func TestMoveCommands(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	c := a.page.Coll
	p2, _ := c.NewPage(nil, "İkinci")
	p3, _ := c.NewPage(nil, "Üçüncü")
	other, _ := a.lib.NewCollection("Başka")
	a.open(p3)
	tt.Frame()
	order := func() string {
		var s []string
		for _, p := range c.Pages {
			s = append(s, p.Title)
		}
		return strings.Join(s, ",")
	}
	runCmd(t, a, ">yukarı taşı", locales.MoveThePageUpInIts())
	if got := order(); got != "Sayfa,Üçüncü,İkinci" {
		t.Fatalf("up: %s", got)
	}
	runCmd(t, a, ">aşağı taşı", locales.MoveThePageDownInIts())
	if got := order(); got != "Sayfa,İkinci,Üçüncü" {
		t.Fatalf("down: %s", got)
	}
	// the end of the row says so
	for _, it := range a.commands() {
		if it.title == locales.MoveThePageDownInIts() && it.off == "" {
			t.Fatal("down at the end is allowed")
		}
	}
	runCmd(t, a, ">taşı başka", "Sayfayı taşı: Başka")
	if p3.Coll != other || len(other.Pages) != 1 {
		t.Fatalf("not moved: %v", p3.Coll.Name)
	}
	_ = p2
	a.openCollection(other)
	tt.Frame()
	runCmd(t, a, ">koleksiyonu aşağı", "Koleksiyonu aşağı taşı")
}

// A line of at most n characters: the column narrows to n zeros of the text, and off it is the page's.
func TestLineChars(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.Frame()
	wide := a.pageWidth()
	a.settings.LineChars = 40
	narrow := a.pageWidth()
	if narrow >= wide || narrow < 200 {
		t.Fatalf("40 characters: %v of %v", narrow, wide)
	}
	a.settings.LineChars = 80
	if a.pageWidth() <= narrow {
		t.Fatal("more characters, not wider")
	}
	a.settings.LineChars = 200
	if a.pageWidth() != wide {
		t.Fatalf("200 characters is wider than the page: %v vs %v", a.pageWidth(), wide)
	}
	a.settings.LineChars = 0
	if a.pageWidth() != wide {
		t.Fatal("off, and not the page's")
	}
	a.settings.LineChars = 40
	a.setFontSize(24)
	if a.pageWidth() <= narrow {
		t.Fatal("a bigger font, the same column")
	}
}

// TestProfileNoticeSaysItsName (#464): putting on a built-in profile from the settings says its name, not its message's id,
// in one notice, the one that can take it back (it said both, once each, with "settings_profileCompact" in one).
func TestProfileNoticeSaysItsName(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.openSettings()
	a.setMode = "profiles"
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	before := len(a.toasts.items)
	a.notice = ""
	// Compact's Apply, on its card
	if err := tt.Click(locales.SettingsApplyProfile(msg("settings_profileCompact"))); err != nil {
		t.Fatalf("the profiles are not listed: %v %v", err, tt.Texts())
	}
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	added := a.toasts.items[before:]
	if len(added) != 1 || a.notice != "" {
		t.Fatalf("%d notices, and %q besides: want the one with Undo", len(added), a.notice)
	}
	if text := added[0].text; strings.Contains(text, "settings_") || added[0].action == "" {
		t.Fatalf("the notice says %q, with %q", text, added[0].action)
	}
}

// A built-in profile is called by its message, in the window's language; one of the user's own by the name it was
// given, even one that is a message's id ("code") (#545).
func TestProfileLabel(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	if got := a.profileLabel("code"); got != msg("code") || got == "code" {
		t.Fatalf("the built-in code: %q", got)
	}
	if got := a.profileLabel("list"); got != "list" {
		t.Fatalf("a name that is no profile: %q", got)
	}
	a.settings.Profiles = map[string]map[string]any{"code": {"fontSize": 13.0}, "list": {"fontSize": 15.0}}
	for _, n := range []string{"code", "list"} {
		if got := a.profileLabel(n); got != n {
			t.Fatalf("the user's own %q: %q", n, got)
		}
	}
}
