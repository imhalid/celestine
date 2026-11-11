package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
)

func TestSplitTitle(t *testing.T) {
	for _, c := range []struct{ md, title, rest, head string }{
		{"# Sayfa\n\nMetin\n", "Sayfa", "Metin\n", "# Sayfa\n\n"},
		{"\n# sayfa  \nMetin\n", "Sayfa", "Metin\n", "\n# sayfa  \n"},
		{"# Başka\n\nMetin\n", "Sayfa", "# Başka\n\nMetin\n", ""},
		{"Metin\n# Sayfa\n", "Sayfa", "Metin\n# Sayfa\n", ""},
		{"## Sayfa\n", "Sayfa", "## Sayfa\n", ""},
		{"", "Sayfa", "", ""},
	} {
		rest, head := SplitTitle(c.md, c.title)
		if rest != c.rest || head != c.head {
			t.Errorf("%q: rest %q head %q", c.md, rest, head)
		}
	}
}

// "[[Page]]" typed becomes a chip as the last bracket is typed.
func TestTypedLinkBecomesChip(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyDown)
	typeEach(tt, " [[Alfa]]")
	tt.Frame()
	tt.Frame()
	if got := mdOf(a.doc); got != "Önce [[Alfa]]" {
		t.Fatalf("written: %q", got)
	}
	n := 0
	for _, r := range a.doc.Text().Runs(0, a.doc.Text().Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.target == "Alfa" {
				n++
			}
		}
	}
	if n != 1 {
		t.Fatalf("chips: %d", n)
	}
	typeEach(tt, "x")
	if got := mdOf(a.doc); got != "Önce [[Alfa]]x" {
		t.Fatalf("after: %q", got)
	}
}

// A property is moved by its grip; the row open is the one outlined.
func TestMoveItem(t *testing.T) {
	l := []string{"a", "b", "c", "d"}
	if got := moveItem(append([]string{}, l...), 0, 3); strings.Join(got, "") != "bcad" {
		t.Fatalf("down: %v", got)
	}
	if got := moveItem(append([]string{}, l...), 3, 1); strings.Join(got, "") != "adbc" {
		t.Fatalf("up: %v", got)
	}
	if got := moveItem(append([]string{}, l...), 1, 2); strings.Join(got, "") != "abcd" {
		t.Fatalf("same place: %v", got)
	}
	if got := moveItem(append([]string{}, l...), 0, 4); strings.Join(got, "") != "bcda" {
		t.Fatalf("to the end: %v", got)
	}
}

func chipTargets(a *app) []string {
	var out []string
	for _, r := range a.doc.Text().Runs(0, a.doc.Text().Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok {
				out = append(out, ch.target)
			}
		}
	}
	return out
}

// "[[" closes itself with "]]" and lists the pages; what is typed narrows
// them, ↵ links the one chosen.
func TestWikiMenu(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	for _, n := range []string{"Alfa", "Beta"} {
		if _, err := a.page.Coll.NewPage(nil, n); err != nil {
			t.Fatal(err)
		}
	}
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyDown)
	typeEach(tt, " [[")
	tt.Frame()
	tt.Frame()
	if got := strings.TrimSpace(a.doc.Text().String()); got != "Önce [[]]" {
		t.Fatalf("not closed: %q", got)
	}
	if !a.slash.open || a.slash.sign != "[[" {
		t.Fatalf("no menu: %+v", a.slash)
	}
	if got := lineTitles(a.menuItems(a.slashQuery())); len(got) != 2 {
		t.Fatalf("the pages: %v", got)
	}
	typeEach(tt, "alf")
	tt.Frame()
	if got := lineTitles(a.menuItems(a.slashQuery())); got[0] != "Alfa" {
		t.Fatalf("not narrowed: %v", got)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if got := mdOf(a.doc); got != "Önce [[Alfa]]" || a.slash.open {
		t.Fatalf("not linked: %q open %v", got, a.slash.open)
	}
	if got := chipTargets(a); len(got) != 1 || got[0] != "Alfa" {
		t.Fatalf("chips %v", got)
	}
}

// Typing the "]]" oneself goes over the ones written, and makes the chip.
func TestWikiTypedClosing(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyDown)
	typeEach(tt, " [[Beta]]")
	tt.Frame()
	tt.Frame()
	if got := mdOf(a.doc); got != "Önce [[Beta]]" {
		t.Fatalf("written: %q", got)
	}
	if got := chipTargets(a); len(got) != 1 || got[0] != "Beta" {
		t.Fatalf("chips %v", got)
	}
	if a.slash.open {
		t.Fatal("menu left open")
	}
}

// A chip is found in the text, whatever copy of it the text keeps, so a date
// chip can be changed from its calendar.
func TestChipPosAndDateEdit(t *testing.T) {
	a, tt := newTestApp(t, "Oku [[2026-11-05]] ve\n")
	tt.Frame()
	ch := &chip{kind: "date", target: "2026-11-05"}
	if got := a.chipPos(ch); got != 4 {
		t.Fatalf("chip at %d", got)
	}
	d, _ := parseDay(ch.target)
	a.dateEdit = dateEdit{open: true, ch: ch, cal: d}
	tt.Frame()
	tt.Frame()
	if !tt.HasText("November 2026") {
		t.Fatalf("no calendar: %v", tt.Texts())
	}
}

// The settings reach the editor and the status bar.
func TestSettingsTakeEffect(t *testing.T) {
	a, tt := newTestApp(t, "bir iki üç\n")
	tt.Frame()
	if a.font.Size != 16.5 || a.font.LineHeight != 1.55 {
		t.Fatalf("defaults: %+v", a.font)
	}
	a.settings.FontSize, a.settings.LineHeight, a.settings.ShowChars = 19, 1.8, true
	tt.Frame()
	tt.Frame()
	if a.font.Size != 19 || a.font.LineHeight != 1.8 {
		t.Fatalf("not taken: %+v", a.font)
	}
	if !tt.HasText("10 harf") {
		t.Fatalf("no letter count: %v", tt.Texts())
	}
	if tt.HasText("Slime") {
		t.Fatal("the caret's name is shown though hidden")
	}
}

// Under a page, the pages that link to it.
func TestBacklinks(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	c := a.page.Coll
	b, _ := c.NewPage(nil, "Beta")
	g, _ := c.NewPage(nil, "Gama")
	b.SaveText("Bkz. [[Sayfa]] ve [[sayfa|takma ad]]\n")
	g.SaveText("Başka bir şey [[Beta]]\n")
	a.open(b)
	tt.Frame()
	tt.Frame()
	if got := a.linksTo(b, a.clock(), true); len(got) != 1 || got[0] != g {
		t.Fatalf("beta's backlinks: %v", got)
	}
	a.open(a.lib.Collections[0].Pages[0]) // Sayfa
	tt.Frame()
	tt.Frame()
	if !tt.HasText("1 sayfa bu sayfaya bağlanıyor") || !tt.HasText("Bağlantı: Beta") {
		t.Fatalf("no backlinks shown: %v", tt.Texts())
	}
	if err := tt.Click("Bağlantı: Beta"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page != b {
		t.Fatal("did not open the page that links")
	}
}

// ⌘J opens today's page in the journal, once made it is the same page.
func TestJournalToday(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	a.clock = func() time.Time { return time.Date(2026, 11, 6, 9, 0, 0, 0, time.Local) }
	tt.Key(ui.Cmd, ui.KeyJ)
	tt.Frame()
	if a.page == nil || a.page.Title != "2026-11-06" || a.page.Coll.Name != journalName {
		t.Fatalf("not today's page: %+v", a.page)
	}
	first := a.page
	a.open(a.lib.Collections[0].Pages[0])
	a.openToday()
	if a.page != first || len(first.Coll.All()) != 1 {
		t.Fatalf("a second page for the day: %d", len(first.Coll.All()))
	}
}

// The journal is in every library, and is not thrown away: not from the
// menus, not from its settings, not by the library.
func TestJournalStays(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	j, err := a.journal()
	if err != nil || !j.Journal {
		t.Fatalf("no journal: %v", err)
	}
	if again, _ := a.journal(); again != j {
		t.Fatal("a second journal")
	}
	for _, it := range a.collectionMenu(j) {
		if it.title == "Çöpe taşı" {
			t.Fatal("the menu offers to throw the journal away")
		}
	}
	a.trashCollection(j)
	if len(a.lib.Collections) != 2 || a.notice == "" {
		t.Fatalf("thrown away: %d collections, notice %q", len(a.lib.Collections), a.notice)
	}
	if _, err := a.lib.TrashCollection(j); err == nil {
		t.Fatal("the library threw it away")
	}
	// a renamed journal is still the journal
	if err := j.Rename("Günlüğüm"); err != nil {
		t.Fatal(err)
	}
	if again, _ := a.journal(); again != j {
		t.Fatal("a new journal after a rename")
	}
	// and in a library reopened it is the one marked
	lib2, err := OpenLibrary(a.lib.Root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range lib2.Collections {
		found = found || c.Journal
	}
	if !found {
		t.Fatal("the mark is not kept")
	}
	_ = tt
}

func TestFormatDay(t *testing.T) {
	withLang(t, "tr")
	d := time.Date(2026, 11, 6, 9, 0, 0, 0, time.Local) // a Friday
	for layout, want := range map[string]string{
		"":                  "2026-11-06",
		"YYYY-MM-DD":        "2026-11-06",
		"DD.MM.YYYY":        "06.11.2026",
		"D MMMM YYYY":       "6 Kasım 2026",
		"D MMMM YYYY, dddd": "6 Kasım 2026, Cuma",
		"ddd D MMM":         "Cum 6 Kas",
		"[YY]":              "[26]",
	} {
		if got := formatDay(layout, d); got != want {
			t.Errorf("%q: %q, want %q", layout, got, want)
		}
	}
}

// The journal takes today's page and no other; its days are titled by its
// format, which a change in its settings applies to every day.
func TestJournalRules(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	today := time.Date(2026, 11, 6, 9, 0, 0, 0, time.Local)
	a.clock = func() time.Time { return today }
	a.openToday()
	j := a.page.Coll
	if !j.Journal || a.page.Title != "2026-11-06" {
		t.Fatalf("today's page: %+v", a.page)
	}
	if _, err := j.NewPage(nil, "başka"); err == nil {
		t.Fatal("the library made another page in the journal")
	}
	a.newPage(j, nil) // any new page is today's, which is there
	if len(j.All()) != 1 {
		t.Fatalf("a second page for today: %d", len(j.All()))
	}
	// the past: a page there is opened, none is made
	old := today.AddDate(0, 0, -3)
	a.openDay(old)
	if a.notice == "" || len(j.All()) != 1 {
		t.Fatalf("a page for a day gone: %q, %d", a.notice, len(j.All()))
	}
	// a day gone that has its page: made by an earlier clock
	a.clock = func() time.Time { return old }
	a.notice = ""
	a.openDay(old)
	if a.notice != "" || len(j.All()) != 2 {
		t.Fatalf("that day's page: %q, %d", a.notice, len(j.All()))
	}
	a.clock = func() time.Time { return today }
	// a random day is another page
	a.openToday()
	a.openRandomDay()
	if a.page == nil || a.page.Title == "2026-11-06" {
		t.Fatalf("random: %+v", a.page)
	}
	// the format: set, saved, every title follows
	a.openCollSettings(j, 0)
	a.cs.draft.JournalFormat = "D MMMM YYYY, dddd"
	a.saveCollSettings()
	titles := map[string]bool{}
	for _, p := range j.All() {
		titles[p.Title] = true
	}
	if !titles["6 Kasım 2026, Cuma"] || !titles["3 Kasım 2026, Salı"] {
		t.Fatalf("not retitled: %v", titles)
	}
	// the title is not typed in the page
	a.openToday()
	tt.Frame()
	tt.Frame()
	a.startRename(a.page, a.page.Title)
	if a.renaming != nil {
		t.Fatal("a day is renamed")
	}
}

// The sidebar's button closes every branch.
func TestCollapseAll(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	child, _ := a.page.Coll.NewPage(a.page, "Çocuk")
	_ = child
	a.page.Open, a.page.Coll.Open = true, true
	tt.Frame()
	if err := tt.Click("Hepsini kapat"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page.Open || a.page.Coll.Open {
		t.Fatal("not closed")
	}
}

// The calendar view shows a collection's pages on their days.
func TestCalendarView(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	if err := coll.SetView("calendar"); err != nil {
		t.Fatal(err)
	}
	a.calMonth = time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	a.openCollection(coll)
	a.calMonth = time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Kasım 2026") || !tt.HasText("Pzt") {
		t.Fatalf("no month: %v", tt.Texts())
	}
	if err := tt.Click("Önceki ay"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	// Anna Karenina was made ten days before the 4th of November: a dot on
	// that day, and its pages when the day is opened
	if !tt.HasText("Ekim 2026") || !tt.HasText("Gün: 2026-10-25") || tt.HasText(locales.ViewsOpenLabel("Anna Karenina")) {
		t.Fatalf("a quiet day with a dot: %v", tt.Texts())
	}
	if err := tt.Click("Gün: 2026-10-25"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if !a.calOpen || !tt.HasText(locales.ViewsOpenLabel("Anna Karenina")) || !tt.HasText("25 Ekim 2026, Pazar") {
		t.Fatalf("the day did not open: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(600 * time.Millisecond)
	if a.calOpen {
		t.Fatal("Esc did not close the day")
	}
	_ = a
}

// A pinned collection comes first; collections move up and down among
// those pinned as they are; the places are kept in the files.
func TestPinAndMoveCollections(t *testing.T) {
	a, _ := newTestApp(t, "Metin\n")
	for _, n := range []string{"Beta", "Alfa", "Gama"} {
		if _, err := a.lib.NewCollection(n); err != nil {
			t.Fatal(err)
		}
	}
	names := func() string {
		var out []string
		for _, c := range a.lib.Collections {
			out = append(out, c.Name)
		}
		return strings.Join(out, ",")
	}
	find := func(n string) *Collection {
		for _, c := range a.lib.Collections {
			if c.Name == n {
				return c
			}
		}
		return nil
	}
	SortCollections(a.lib.Collections)
	if got := names(); got != "Alfa,Beta,Gama,Test" {
		t.Fatalf("by name: %s", got)
	}
	a.pinCollection(find("Gama"), true)
	if got := names(); got != "Gama,Alfa,Beta,Test" {
		t.Fatalf("pinned first: %s", got)
	}
	a.moveCollection(find("Beta"), -1)
	if got := names(); got != "Gama,Beta,Alfa,Test" {
		t.Fatalf("moved up: %s", got)
	}
	a.moveCollection(find("Gama"), -1) // already first of the pinned: stays
	a.pinCollection(find("Test"), true)
	if got := names(); got != "Gama,Test,Beta,Alfa" {
		t.Fatalf("a second pinned: %s", got)
	}
	lib2, err := OpenLibrary(a.lib.Root)
	if err != nil {
		t.Fatal(err)
	}
	var again []string
	for _, c := range lib2.Collections {
		again = append(again, c.Name)
	}
	if strings.Join(again, ",") != "Gama,Test,Beta,Alfa" && strings.Join(again, ",") != "Test,Gama,Beta,Alfa" {
		t.Fatalf("not kept: %v", again)
	}
}

// A day with many pages shows a dot for each, and opens into the list.
func TestCalendarDayList(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	for _, n := range []string{"Bir", "İki", "Üç", "Dört", "Beş"} {
		p, _ := coll.NewPage(nil, n)
		p.Created = time.Date(2026, 11, 4, 9, 0, 0, 0, time.Local)
		p.SaveText("")
	}
	coll.SetView("calendar")
	a.openCollection(coll)
	a.calMonth = time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Gün: 2026-11-04"); err != nil {
		t.Fatalf("no day: %v %v", err, tt.Texts())
	}
	tt.Advance(600 * time.Millisecond)
	if !a.calOpen || !tt.HasText("Beş") || !tt.HasText("4 Kasım 2026, Çarşamba") {
		t.Fatalf("no list: %v", tt.Texts())
	}
	if err := tt.Click("Günü kapat"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if a.calOpen {
		t.Fatal("not closed")
	}
}

// A collection with more pages than the window holds scrolls, as one page.
func TestCollectionScrolls(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	for i := 0; i < 40; i++ {
		coll.NewPage(nil, fmt.Sprintf("Sayfa %02d", i))
	}
	coll.SetView("list")
	a.openCollection(coll)
	tt.SetSize(1100, 600)
	tt.Advance(500 * time.Millisecond)
	before, _ := tt.Find("Görünüm")
	tt.Wheel(700, 300, 0, 500)
	tt.Advance(400 * time.Millisecond)
	after, ok := tt.Find("Görünüm")
	if ok && after.W > 0 && after.Y >= before.Y-100 {
		t.Fatalf("the collection did not scroll: %+v then %+v", before, after)
	}
}

// The arrow keys go through the tree from the row focused, opening what they
// come to.
func TestTreeArrowKeys(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	c := a.page.Coll
	b, _ := c.NewPage(nil, "Beta")
	g, _ := c.NewPage(nil, "Gama")
	first := a.page
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Sayfa: Sayfa"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Advance(200 * time.Millisecond)
	if a.page != b {
		t.Fatalf("↓ did not go to the page after: %v", a.page.Title)
	}
	tt.Key(0, ui.KeyDown)
	tt.Advance(200 * time.Millisecond)
	if a.page != g {
		t.Fatalf("↓ again: %v", a.page.Title)
	}
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyUp)
	tt.Advance(200 * time.Millisecond)
	if a.page != first {
		t.Fatalf("↑↑ did not go back: %v", a.page.Title)
	}
}

// A collection pinned is unpinned from its own pin; collections are sorted
// by hand within those pinned as they are; the sidebar is kept as it was left.
func TestPinSortAndKeepSidebar(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	for _, n := range []string{"Beta", "Gama"} {
		a.lib.NewCollection(n)
	}
	SortCollections(a.lib.Collections)
	byName := func(n string) *Collection {
		for _, c := range a.lib.Collections {
			if c.Name == n {
				return c
			}
		}
		return nil
	}
	tt.Frame()
	if r, ok := tt.Find("Koleksiyon: Gama"); ok { // the pointer on the row, as a hand would be
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Advance(300 * time.Millisecond)
	}
	if err := tt.Click("Sabitle: Gama"); err != nil {
		t.Fatalf("no pin button: %v", err)
	}
	tt.Frame()
	if !byName("Gama").Pinned || a.lib.Collections[0].Name != "Gama" {
		t.Fatal("not pinned to the top")
	}
	if err := tt.Click("Sabitlemeyi kaldır: Gama"); err != nil {
		t.Fatalf("no way to unpin: %v", err)
	}
	tt.Frame()
	if byName("Gama").Pinned {
		t.Fatal("still pinned")
	}
	// by hand: Gama before Beta
	if err := a.lib.MoveCollectionBefore(byName("Gama"), byName("Beta")); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range a.lib.Collections {
		names = append(names, c.Name)
	}
	if got := strings.Join(names, ","); got != "Gama,Beta,Test" {
		t.Fatalf("by hand: %s", got)
	}
	// the sidebar kept
	a.settingsFile = filepath.Join(t.TempDir(), "settings.json")
	a.toggleSidebar()
	if !a.settings.SidebarClosed {
		t.Fatal("the sidebar's state is not kept")
	}
	a.sidebarClosed = false
	a.applyTree()
	if !a.sidebarClosed {
		t.Fatal("not applied")
	}
}

// A link copied from the page's menu is "[[Page]]" as text; pasted, it is a
// chip, as when it is typed.
func TestPastedLinkBecomesChip(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyDown)
	_, caret := a.doc.Selection()
	paste := " [[Logical Replication & Write-Ahead]] ve [[Ikinci]] son"
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(caret, caret, richtext.NewText(paste))
		ed.Select(caret+len([]rune(paste)), caret+len([]rune(paste)))
	})
	tt.Frame()
	tt.Frame()
	if got := chipTargets(a); len(got) != 2 || got[0] != "Logical Replication & Write-Ahead" || got[1] != "Ikinci" {
		t.Fatalf("chips: %v in %q", got, a.doc.Text().String())
	}
	if got := mdOf(a.doc); got != "Önce [[Logical Replication & Write-Ahead]] ve [[Ikinci]] son" {
		t.Fatalf("written: %q", got)
	}
	_, c := a.doc.Selection()
	if c != a.doc.Text().Len() {
		t.Fatalf("the caret is not at the end: %d of %d", c, a.doc.Text().Len())
	}
}

// The settings are a screen of the window, like the trash: the sections listed at the left, a
// section chosen at the right; it is in the history, and Esc and ⌘, go back to the page.
func TestSettingsScreen(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1300, 800)
	tt.Frame()
	page := a.page
	a.openSettings()
	tt.Frame()
	tt.Advance(900 * time.Millisecond)
	if !a.settingsOpen {
		t.Fatal("the settings did not open")
	}
	for _, s := range settingsSections {
		if !tt.HasText(msg(s.name)) {
			t.Fatalf("no %q in the list: %v", s.name, tt.Texts())
		}
	}
	if a.modal() {
		t.Fatal("the settings are a modal: they must not be")
	}
	if a.here().kind != "settings" {
		t.Fatalf("not in the history: %+v", a.here())
	}
	// a section chosen
	if err := tt.Click("MCP"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	if a.settingsTab != tabMCP {
		t.Fatalf("tab %d", a.settingsTab)
	}
	// Esc goes back to the page
	tt.Key(0, ui.KeyEscape)
	tt.Advance(500 * time.Millisecond)
	if a.settingsOpen || a.page != page {
		t.Fatalf("Esc did not go back: settings %v, page %v", a.settingsOpen, a.page != nil)
	}
	// ⌘, opens, and shuts
	a.toggleSettings()
	if !a.settingsOpen {
		t.Fatal("toggle did not open")
	}
	a.toggleSettings()
	if a.settingsOpen || a.page != page {
		t.Fatal("toggle did not go back to the page")
	}
	// another view takes it away
	a.openSettings()
	a.openTrash()
	if a.settingsOpen {
		t.Fatal("the settings stayed under the trash")
	}
}
