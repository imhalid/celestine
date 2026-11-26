package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSlashMenu(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "/")
	tt.Advance(100 * time.Millisecond)
	if !a.slash.open || !tt.HasText("Öneriler") {
		t.Fatalf("not open: %+v", a.slash)
	}
	typeEach(tt, "bas")
	tt.Frame()
	if items := slashItems(a.slashQuery()); len(items) < 3 || items[0].kind != "h1" || items[2].kind != "h3" {
		t.Fatalf("narrowed: %+v", items)
	}
	tt.Key(0, ui.KeyDown) // the second: Başlık 2
	tt.Frame()
	if a.slash.sel != 1 {
		t.Fatalf("sel %d", a.slash.sel)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if a.slash.open {
		t.Fatal("still open")
	}
	typeEach(tt, "Başlık")
	if got := mdOf(a.pane().doc); got != "Önce\n\n## Başlık" {
		t.Fatalf("chosen: %q", got)
	}
	// Esc closes it; the "/" stays as typed
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "/")
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.slash.open {
		t.Fatal("Esc did not close it")
	}
	// a / inside a line opens nothing
	typeEach(tt, "a/b")
	if a.slash.open {
		t.Fatal("open inside a line")
	}
}

func lineTitles(items []menuItem) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.title)
	}
	return out
}

// The first menu: the four kinds used last, then the menus; one menu
// whichever sign opens it, a / at a word's start opens it as @ does.
func TestMenuRoot(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "Oku /")
	tt.Frame()
	if !a.slash.open || a.slash.sign != "/" {
		t.Fatalf("a / at a word's start did not open it: %+v", a.slash)
	}
	items := a.menuItems("")
	got := lineTitles(items)
	want := []string{"common_task", "blocks_heading1", "blocks_heading2", "blocks_bulletList", "slash_format", "slash_insert", "slash_dateTime", "slash_link"}
	if len(got) != len(want) {
		t.Fatalf("lines %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lines %v, want %v", got, want)
		}
	}
	for _, it := range items[4:] {
		if !it.nav {
			t.Fatalf("%q does not go into a menu", it.title)
		}
	}
	a.slash.sign = "@"
	if len(a.menuItems("")) != len(items) {
		t.Fatal("@ and / differ")
	}
	// typed: all at once, grouped
	groups := map[string]bool{}
	for _, it := range a.menuItems("b") {
		groups[it.group] = true
	}
	if !groups["slash_groupFormat"] || !groups["slash_groupDate"] {
		t.Fatalf("not searched together: %v", groups)
	}
}

// ↵ on a menu goes into it; its lines are its own, typing narrows them, and
// Esc goes back before it closes.
func TestMenuInto(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "@")
	tt.Frame()
	a.slash.sel = 7 // Bağlantı…
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if !a.slash.open || a.slash.view != "link" {
		t.Fatalf("not in the link menu: %+v", a.slash)
	}
	if !tt.HasText(msg("findAPage4")) || !tt.HasText(msg("common_back")) {
		t.Fatal("no field or way back")
	}
	if got := mdOf(a.pane().doc); got != "Önce\n\n@" {
		t.Fatalf("the text changed: %q", got)
	}
	n := len(a.menuItems("")) - 1 // without ← Geri, the first line
	typeEach(tt, "zzzz")
	tt.Frame()
	if got := len(a.menuItems(a.slashQuery())) - 1; got >= n && n > 0 {
		t.Fatalf("typing did not narrow: %d", got)
	}
	for range 4 {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !a.slash.open || a.slash.view != "" {
		t.Fatalf("Esc did not go back: %+v", a.slash)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.slash.open {
		t.Fatal("Esc did not close it")
	}
}

// A date picked on the calendar becomes a chip; a kind used moves to the top
// of the suggestions.
func TestMenuDateAndRecent(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "/")
	tt.Frame()
	a.slash.sel = 6 // Tarih ya da saat
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if a.slash.view != "date" {
		t.Fatalf("not in the date menu: %+v", a.slash)
	}
	r, ok := tt.Find("Bugün")
	if !ok {
		t.Fatal("no days named")
	}
	_ = r
	tt.Key(0, ui.KeyEnter) // the first: today
	tt.Frame()
	tt.Frame()
	if a.slash.open || !strings.Contains(mdOf(a.pane().doc), "[[20") {
		t.Fatalf("no date chip: %q", mdOf(a.pane().doc))
	}
	// a kind used is the first suggestion next time
	typeEach(tt, "/")
	tt.Frame()
	a.menuItems("")[3].run() // Madde listesi
	tt.Frame()
	typeEach(tt, "x")
	tt.Key(0, ui.KeyEnter)
	typeEach(tt, "/")
	tt.Frame()
	if got := lineTitles(a.menuItems(""))[0]; got != "blocks_bulletList" {
		t.Fatalf("first suggestion %q", got)
	}
}

// The menu stays inside the window: above the caret when there is no room
// below.
func TestMenuInWindow(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.SetSize(900, 300)
	tt.Frame()
	typeEach(tt, "/")
	tt.Frame()
	tt.Frame()
	if !a.slash.open {
		t.Fatal("not open")
	}
	r, ok := tt.Find("Satırı biçimlendir…")
	if !ok && !tt.HasText("Öneriler") {
		t.Fatal("no menu")
	}
	_ = r
	if r, ok := tt.Find("Öneriler"); ok && (r.Y < 0 || r.Y+r.H > 300) {
		t.Fatalf("outside the window: %+v", r)
	}
}

// The pointer left of the text, in the margin, is over the editor: the grip
// shows there.
func TestGripInMargin(t *testing.T) {
	a, tt := newTestApp(t, "Bir\n\nİki\n")
	tt.Frame()
	if a.pane().editorR.W == 0 {
		t.Fatal("no editor")
	}
	tt.Move(a.pane().editorR.X-12, a.pane().editorR.Y+8)
	tt.Frame()
	if a.pane().grip.para < 0 {
		t.Fatalf("no grip with the pointer in the margin: %+v", a.pane().grip)
	}
}

// The link and date menus have a field of their own with the focus in it:
// what is typed narrows the list, ↵ chooses, and the text is left alone.
func TestMenuField(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	for _, n := range []string{"Alfa", "Beta"} {
		if _, err := a.pane().page.Coll.NewPage(nil, n); err != nil {
			t.Fatal(err)
		}
	}
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "@")
	tt.Frame()
	a.slash.sel = 7 // Bağlantı…
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if got := len(a.menuItems("")) - 1; got != 2 { // ← Geri is the first line
		t.Fatalf("pages in the menu: %d", got)
	}
	typeEach(tt, "alf")
	tt.Frame()
	if a.slash.field != "alf" {
		t.Fatalf("not typed in the field: %q", a.slash.field)
	}
	if got := mdOf(a.pane().doc); got != "Önce\n\n@" {
		t.Fatalf("the text changed: %q", got)
	}
	if items := a.menuItems(a.slash.field); len(items) != 3 || items[1].title != "Alfa" { // ← Geri, Alfa, and a new page of that name
		t.Fatalf("not narrowed: %v", lineTitles(items))
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if a.slash.open || !strings.Contains(mdOf(a.pane().doc), "[[Alfa]]") {
		t.Fatalf("not linked: open %v, %q", a.slash.open, mdOf(a.pane().doc))
	}
	typeEach(tt, "@")
	tt.Frame()
	a.slash.sel = 7
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	// date: a day by name
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	a.slash.sel = 6
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	typeEach(tt, "yar")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tomorrow := a.clock().AddDate(0, 0, 1).Format("2006-01-02")
	if !strings.Contains(mdOf(a.pane().doc), "[["+tomorrow+"]]") {
		t.Fatalf("no date: %q", mdOf(a.pane().doc))
	}
}

// A menu with nothing in it shows a picture and a few words.
func TestMenuEmpty(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "@")
	tt.Frame()
	a.slash.sel = 7
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if !tt.HasText(msg("common_noPageMatches")) {
		t.Fatal("no empty state")
	}
}

// The whole margin left of the text, from the page area's edge, is the
// editor's: the grip shows there, and a click puts the caret on the line.
func TestGutterReachesPageEdge(t *testing.T) {
	a, tt := newTestApp(t, "Bir\n\nİki\n")
	tt.SetSize(1800, 1000) // a wide window: the text is centered, the margin is wide
	tt.Frame()
	tt.Frame()
	if a.pane().pageR.W == 0 || a.pane().pageR.X >= a.pane().editorR.X-gutterW {
		t.Fatalf("the page area %+v is no wider than the text %+v", a.pane().pageR, a.pane().editorR)
	}
	y := a.pane().editorR.Y + a.pane().boxes[len(a.pane().boxes)-1].Top + 6
	tt.Move(a.pane().pageR.X+6, y)
	tt.Frame()
	tt.Frame()
	if a.pane().grip.para < 0 {
		t.Fatalf("no grip far left: %+v", a.pane().grip)
	}
	tt.ClickAt(a.pane().pageR.X+6, y)
	tt.Frame()
	tt.Frame()
	if _, caret := a.pane().doc.Selection(); caret != a.pane().grip.para {
		t.Fatalf("the caret %d, not at the start of the line %d", caret, a.pane().grip.para)
	}
}

// A chip answers the pointer: it is the one hovered, and a date's click
// opens a calendar.
func TestChipHover(t *testing.T) {
	a, tt := newTestApp(t, "Oku [[2026-11-05]] ve [[Alfa]]\n")
	tt.Frame()
	tt.Frame()
	var hits []chipHit // each chip once: a frame can draw them twice
	for _, h := range a.chipSeen {
		if len(hits) == 0 || hits[len(hits)-1].ch != h.ch {
			hits = append(hits, h)
		}
	}
	if len(hits) < 2 {
		t.Fatalf("chips seen: %d", len(hits))
	}
	for _, h := range hits[:2] {
		tt.Move(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
		tt.Frame()
		tt.Frame()
		if a.hoverCh != h.ch {
			t.Fatalf("%q not hovered", h.ch.label())
		}
	}
	tt.Move(a.pane().editorR.X+a.pane().editorR.W-10, a.pane().editorR.Y+300)
	tt.Frame()
	if a.hoverCh != nil {
		t.Fatal("hovered away from any chip")
	}
	h := hits[0]
	tt.ClickAtWith(ui.Alt, h.r.X+h.r.W/2, h.r.Y+h.r.H/2) // ⌥-click: its calendar; a click opens its day (#621)
	tt.Frame()
	tt.Frame()
	if !a.dateEdit.open {
		t.Fatal("⌥-click on a date did not open its calendar")
	}
}

// In a menu inside the / menu, ↑ from the first line reaches ← Geri, and ↵ goes back.
func TestSlashMenuKeysReachBack(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.SetSize(900, 600)
	typeEach(tt, "/")
	tt.Advance(200 * time.Millisecond)
	a.enterView("insert")
	tt.Advance(200 * time.Millisecond)
	if a.slash.view != "insert" || a.slash.sel != 1 {
		t.Fatalf("not in the menu, on its first line: view %q sel %d", a.slash.view, a.slash.sel)
	}
	if items := a.menuItems(""); len(items) < 2 || !items[0].back || items[0].title != "common_back" {
		t.Fatalf("← Geri is not the first line: %+v", items)
	}
	tt.Key(0, ui.KeyUp)
	tt.Advance(100 * time.Millisecond)
	if a.slash.sel != 0 {
		t.Fatalf("↑ did not reach the way back: sel %d", a.slash.sel)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	if a.slash.view != "" || !a.slash.open {
		t.Fatalf("↵ on ← Geri: view %q open %v", a.slash.view, a.slash.open)
	}
}

// TestSlashMenuArrows: → goes into the submenu chosen and ← back out of it while nothing is typed; on a line that
// opens no menu → is the caret's (#492).
func TestSlashMenuArrows(t *testing.T) {
	a, tt := newTestApp(t, "")
	tt.SetSize(900, 600)
	typeEach(tt, "/")
	tt.Advance(200 * time.Millisecond)
	items := a.menuItems("")
	sub, plain := -1, -1
	for i, it := range items {
		if it.nav && sub < 0 {
			sub = i
		}
		if !it.nav && !it.back && plain < 0 {
			plain = i
		}
	}
	if sub < 0 || plain < 0 {
		t.Fatalf("no submenu and plain line in %d lines", len(items))
	}
	a.slash.sel = plain
	tt.Key(0, ui.KeyRight)
	tt.Advance(100 * time.Millisecond)
	if a.slash.view != "" {
		t.Fatalf("→ on a plain line opened %q", a.slash.view)
	}
	a.slash.sel = sub
	tt.Key(0, ui.KeyRight)
	tt.Advance(200 * time.Millisecond)
	if a.slash.view == "" || !a.slash.open {
		t.Fatalf("→ did not open the submenu: view %q open %v", a.slash.view, a.slash.open)
	}
	tt.Key(0, ui.KeyLeft)
	tt.Advance(200 * time.Millisecond)
	if a.slash.view != "" || !a.slash.open {
		t.Fatalf("← did not come back: view %q open %v", a.slash.view, a.slash.open)
	}
}

// A line made by into is a submenu of MyGo's menu, and a line chosen in it runs (#305).
func TestContextSubmenu(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(900, 600)
	ran := ""
	inner := []menuItem{{title: "İç bir", run: func() { ran = "bir" }}, {title: "İç iki", run: func() { ran = "iki" }}}
	a.showContext(300, 200, []menuItem{a.into("Daha…", iconPlus, func() []menuItem { return inner })})
	tt.Frame()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if err := tt.ChooseMenuItem("Daha…", "İç iki"); err != nil {
		t.Fatal(err, tt.Menu())
	}
	tt.Advance(200 * time.Millisecond)
	if ran != "iki" || a.ctx.open || a.ctx.items != nil {
		t.Fatalf("ran %q, open %v", ran, a.ctx.open)
	}
}

// TestGearMenuThemeSubmenuCheck verifies that choosing a theme in the quick-settings
// submenu moves the ✓ trailing mark to the newly active item when the menu is reopened.
func TestGearMenuThemeSubmenuCheck(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(900, 600)

	// Open the gear menu, select Dark from the Theme submenu, then reopen and open Theme.
	a.gearMenu(40, 60)
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if err := tt.ChooseMenuItem(msg("common_theme"), msg("common_dark")); err != nil {
		t.Fatal(err, tt.Menu())
	}
	if a.settings.Theme != 2 {
		t.Fatalf("Theme = %d, want 2", a.settings.Theme)
	}
	a.gearMenu(40, 60)
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	tt.Click(msg("common_theme"))
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	darkR, okDark := tt.Find(msg("common_dark"))
	sysR, okSys := tt.Find(msg("common_system"))
	checkR, okChk := tt.Find("✓")
	if !okDark || !okSys || !okChk {
		t.Fatalf("missing submenu items: dark=%v sys=%v check=%v texts=%v", okDark, okSys, okChk, tt.Texts())
	}
	if checkR.Y < darkR.Y-4 || checkR.Y > darkR.Y+darkR.H+4 {
		t.Fatalf("✓ bounds Y=%v not aligned with Dark row=%v (System row=%v)", checkR.Y, darkR, sysR)
	}
	sub := a.ctx.items[0].items()
	var darkHint, sysHint string
	for _, it := range sub {
		if it.title == "common_dark" {
			darkHint = it.hint
		}
		if it.title == "common_system" {
			sysHint = it.hint
		}
	}
	if darkHint != "✓" || sysHint != "" {
		t.Fatalf("reopened ticks: dark=%q sys=%q", darkHint, sysHint)
	}
}

// In English, the slash menu's heading is English both before and after a query is typed (#474).
func TestSlashHeadingInEnglish(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	withLang(t, "en")
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "/")
	tt.Advance(100 * time.Millisecond)
	if !a.slash.open || !tt.HasText("Suggestions") || tt.HasText("Öneriler") {
		t.Fatalf("heading before a query is not English: %+v", a.slash)
	}
	typeEach(tt, "ta")
	tt.Frame()
	if !tt.HasText("Results") || tt.HasText("Sonuçlar") {
		t.Fatal("heading after a query is not English")
	}
}

// The / menu finds the lines whose name has what is typed in it, in either language, the letters together: "ta" is not
// found in "Bulleted list" by the t of one name and the a of the other; those whose name starts with it come first
// (#609).
func TestSlashFindsWhatNamesContain(t *testing.T) {
	for _, l := range []string{"en", "tr"} {
		setLang(l)
		for _, q := range []string{"t", "ta", "task", "gör", "list"} {
			for _, k := range slashItems(q) {
				if !strings.Contains(foldText(names(k.name)), foldText(q)) && !strings.HasPrefix(k.md, q) {
					t.Errorf("%s: %q found %q, which has no %q", l, q, msg(k.name), q)
				}
			}
		}
		for _, k := range slashItems("ta") {
			if k.kind == "bullets" {
				t.Errorf("%s: \"ta\" found the bulleted list", l)
			}
		}
	}
	setLang("en")
	got := slashItems("task")
	if len(got) < 2 || got[0].kind != "tasks" {
		t.Fatalf("task: %v", got)
	}
	for i, k := range got {
		if i > 0 && menuRank("task", names(k.name)) < menuRank("task", names(got[i-1].name)) {
			t.Fatalf("%q before %q", msg(got[i-1].name), msg(k.name))
		}
	}
	setLang("tr")
}

// Task chosen from the / menu in a line, after its words, is a task in the running text where it was typed; at the
// start of a line it makes the line a task, as before (#609).
func TestSlashTaskInLine(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1000, 800)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "Süt al /görev")
	tt.Advance(200 * time.Millisecond)
	if !a.slash.open {
		t.Fatal("the menu is not open")
	}
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	if got := mdOf(a.pane().doc); got != "x\n\nSüt al [ ]" {
		t.Fatalf("in a line: %q", got)
	}
	if boxes := checksIn(a); len(boxes) != 1 {
		t.Fatalf("%d tasks in the running text", len(boxes))
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "/görev")
	tt.Advance(200 * time.Millisecond)
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	typeEach(tt, "yap")
	if got := mdOf(a.pane().doc); got != "x\n\nSüt al [ ] \n\n- [ ] yap" { // a space after the box, as after every chip put in
		t.Fatalf("at a line's start: %q", got)
	}
}
