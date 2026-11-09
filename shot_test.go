package main

import (
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestShot draws the welcome document: run with SHOT=file.png.
func TestShot(t *testing.T) {
	out := os.Getenv("SHOT")
	if out == "" {
		t.Skip()
	}
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := seed(lib); err != nil {
		t.Fatal(err)
	}
	c := lib.Collections[0]
	for _, title := range []string{"Proje planı", "Okuma listesi"} {
		p, _ := c.NewPage(nil, title)
		c.NewPage(p, "Alt sayfa: "+title)
	}
	a := newApp(lib)
	a.doc.SetSelection(40, 40)
	tt := ui.NewTester(a.view, 1180, 820)
	tt.Advance(400 * time.Millisecond)
	f, _ := os.Create(out)
	png.Encode(f, tt.Image())
	f.Close()
}

// TestViewShots draws a collection's list and table: run with VIEWS=dir.
func TestViewShots(t *testing.T) {
	dir := os.Getenv("VIEWS")
	if dir == "" {
		t.Skip()
	}
	_, tt, coll := viewsLibrary(t)
	coll.AddProperty("Durum", PropChoice, []string{"Yapılacak", "Sürüyor", "Bitti"})
	coll.AddProperty("Yazar", PropText, nil)
	for _, p := range coll.All() {
		switch p.Title {
		case "Dune":
			p.SetValue("yazar", "Frank Herbert")
			p.SetValue("durum", "Yapılacak")
		case "Anna Karenina":
			p.SetValue("yazar", "Lev Tolstoy")
			p.SetValue("durum", "Bitti")
		}
	}
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Advance(100 * time.Millisecond)
	save("list")
	tt.Click("Tablo")
	tt.Advance(100 * time.Millisecond)
	save("table")
	tt.Click("Pano")
	tt.Advance(time.Second)
	save("board")
	// a card held over the next column
	from, _ := tt.Find("Kart: Dune")
	to, _ := tt.Find("Sürüyor")
	x0, y0 := from.X+from.W/2, from.Y+from.H/2
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0+(to.X+60-x0)*float32(k)/10, y0+(to.Y+50-y0)*float32(k)/10)
		tt.Frame()
	}
	tt.Advance(400 * time.Millisecond)
	save("board-drag")
	tt.Release(to.X+60, to.Y+50)
	tt.Advance(time.Second)
	save("board-dropped")
}

// TestFoldShots draws the sidebar open, folding and folded: run with FOLD=dir.
func TestFoldShots(t *testing.T) {
	dir := os.Getenv("FOLD")
	if dir == "" {
		t.Skip()
	}
	lib, _ := OpenLibrary(t.TempDir())
	seed(lib)
	c := lib.Collections[0]
	for _, title := range []string{"Proje planı", "Okuma listesi"} {
		p, _ := c.NewPage(nil, title)
		c.NewPage(p, "Alt sayfa")
	}
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1180, 760)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Advance(time.Second)
	save("fold-0-open")
	tt.Key(ui.Cmd, ui.KeyBackslash)
	tt.Advance(110 * time.Millisecond)
	save("fold-1-mid")
	tt.Advance(time.Second)
	save("fold-2-closed")
}

// TestPolishShots draws the tabs' pill on its way and a row being renamed: run with POLISH=dir.
func TestPolishShots(t *testing.T) {
	dir := os.Getenv("POLISH")
	if dir == "" {
		t.Skip()
	}
	a, tt, _ := viewsLibrary(t)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Advance(time.Second)
	tt.Click("Tablo")
	tt.Advance(90 * time.Millisecond)
	save("pill-mid")
	tt.Advance(time.Second)
	tt.RightClick("Sayfa: Dune")
	tt.Click("Yeniden adlandır")
	tt.Advance(300 * time.Millisecond)
	_ = a
	save("rename")
}

// TestDragShots draws a drag on its way: a line between rows, then a row lit
// to take the page under it. Run with DRAG=dir.
func TestDragShots(t *testing.T) {
	dir := os.Getenv("DRAG")
	if dir == "" {
		t.Skip()
	}
	lib, ps, _, _ := dragLibrary(t)
	a := newApp(lib)
	tt := ui.NewTester(a.view, 1180, 700)
	tt.Advance(time.Second)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	r := a.treeR
	x := r.X + 70
	y0 := r.Y + float32(rowOf(a, ps["C"]))*rowH + rowH/2
	tt.Press(x, y0)
	for _, to := range []struct {
		row  int
		frac float32
		name string
	}{{rowOf(a, ps["A"]), 0.12, "drag-line"}, {rowOf(a, ps["A"]), 0.5, "drag-into"}} {
		y := r.Y + (float32(to.row)+to.frac)*rowH
		for k := 1; k <= 6; k++ {
			tt.Move(x+float32(k)*6, y0+(y-y0)*float32(k)/6)
			tt.Frame()
		}
		y0 = y
		tt.Advance(400 * time.Millisecond)
		save(to.name)
	}
	tt.Release(x+36, y0)
	tt.Advance(120 * time.Millisecond)
	save("drag-land")
	tt.Advance(300 * time.Millisecond)
	save("drag-rise")
}

// TestTrashShots draws the toast after a page is thrown away and the
// trash: run with TRASH=dir.
func TestTrashShots(t *testing.T) {
	dir := os.Getenv("TRASH")
	if dir == "" {
		t.Skip()
	}
	a, tt, coll := viewsLibrary(t)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	a.trashPage(coll.Pages[1])
	tt.Advance(700 * time.Millisecond)
	save("toast")
	a.lib.TrashCollection(coll)
	a.lib.NewCollection("Notlar")
	tt.Click("Çöp")
	tt.Advance(time.Second)
	save("trash")
}

// TestSettingsShot draws the settings with the MCP server on: run with SETTINGS=file.png.
func TestSettingsShot(t *testing.T) {
	out := os.Getenv("SETTINGS")
	if out == "" {
		t.Skip()
	}
	a, tt := newTestApp(t, "metin")
	a.settings.Port = 0
	tt.SetSize(1180, 760)
	tt.Click("Ayarlar")
	tt.Advance(300 * time.Millisecond)
	tt.Click("Tüm ayarlar…")
	tt.Advance(500 * time.Millisecond)
	tt.Click("MCP sunucusunu aç")
	tt.Advance(time.Second)
	a.settings.Port = defaultPort // as it shows by default
	tt.Frame()
	f, _ := os.Create(out)
	png.Encode(f, tt.Image())
	f.Close()
	a.setMCP(false)
}

// TestPropsShots draws a page with properties and the dialog adding one: run with PROPS=dir.
func TestPropsShots(t *testing.T) {
	dir := os.Getenv("PROPS")
	if dir == "" {
		t.Skip()
	}
	a, tt := newTestApp(t, "Çöl gezegeni Arrakis.\n\n---\n\nBaharat her şeydir.\n")
	tt.SetSize(1180, 640)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Advance(300 * time.Millisecond)
	save("page-empty")
	c := a.page.Coll
	c.AddProperty("Durum", PropChoice, []string{"Yapılacak", "Sürüyor", "Bitti"})
	c.AddProperty("Yazar", PropText, nil)
	c.AddProperty("Yıl", PropNumber, nil)
	a.page.SetValue("durum", "Sürüyor")
	a.page.SetValue("yazar", "Frank Herbert")
	a.page.SetValue("yil", "1965")
	tt.Advance(300 * time.Millisecond)
	save("page-props")
	tt.Click("Özellik ekle")
	tt.Advance(500 * time.Millisecond)
	tt.Type("Tür")
	tt.Frame()
	save("prop-dialog")
}

// TestKeptShot draws a page of Markdown from elsewhere: run with KEPT=file.png.
func TestKeptShot(t *testing.T) {
	out := os.Getenv("KEPT")
	if out == "" {
		t.Skip()
	}
	_, tt := newTestApp(t, "Obsidian'dan gelen bir sayfa, [[Dune]] bağlantısıyla.\n\n| Kitap | Yıl |\n|---|---:|\n| Dune | 1965 |\n| Anna Karenina | 1878 |\n\n```go\nfmt.Println(\"merhaba!\")\n```\n\n> [!note] Not\n> Obsidian not kutusu.\n\nDipnot[^1].\n\n[^1]: Kaynak.\n")
	tt.SetSize(1000, 700)
	tt.Advance(300 * time.Millisecond)
	f, _ := os.Create(out)
	png.Encode(f, tt.Image())
	f.Close()
}

// TestChromeShots draws the window's chrome in the dark, the sidebar open
// and folded: run with CHROME=dir.
func TestChromeShots(t *testing.T) {
	dir := os.Getenv("CHROME")
	if dir == "" {
		t.Skip()
	}
	a, tt := newTestApp(t, "Satır başında caret.\n\n- liste\n")
	tt.SetDark(true)
	tt.SetTitleBar(ui.TitleBar{Left: 78, Height: 38})
	tt.SetSize(1000, 520)
	a.settings.Port = 0
	a.setMCP(true)
	defer a.setMCP(false)
	a.doc.SetSelection(0, 0)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Advance(500 * time.Millisecond)
	save("chrome-open")
	a.toggleSidebar()
	tt.Advance(time.Second)
	save("chrome-folded")
}

// TestPaletteShots draws the palette, empty and typed, and finding in the
// page: run with PALETTE=dir.
func TestPaletteShots(t *testing.T) {
	dir := os.Getenv("PALETTE")
	if dir == "" {
		t.Skip()
	}
	a, tt, _ := viewsLibrary(t)
	tt.SetSize(1100, 700)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	a.openPalette()
	tt.Advance(500 * time.Millisecond)
	save("palette-empty")
	tt.Type("dun")
	tt.Advance(200 * time.Millisecond)
	save("palette-typed")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(500 * time.Millisecond)
	a.openFind()
	tt.Frame()
	tt.Type("a")
	tt.Advance(300 * time.Millisecond)
	save("find")
}

// TestCollSettingsShots draws a collection's settings, each tab, and its
// page with an icon, a color and a folded board column: run with CSET=dir.
func TestCollSettingsShots(t *testing.T) {
	dir := os.Getenv("CSET")
	if dir == "" {
		t.Skip()
	}
	a, tt, coll := viewsLibrary(t)
	tt.SetSize(1180, 820)
	coll.AddProperty("Durum", PropChoice, []string{"Okunacak", "Okunuyor", "Okundu"})
	coll.AddProperty("Yazar", PropText, nil)
	coll.Icon, coll.Color, coll.Description = "book", "c3", "Okuduğum ve okuyacağım kitaplar."
	for _, p := range coll.All() {
		switch p.Title {
		case "Dune":
			p.SetValue("durum", "Okunuyor")
			p.SetValue("yazar", "Frank Herbert")
		case "Anna Karenina":
			p.SetValue("durum", "Okundu")
			p.SetValue("yazar", "Lev Tolstoy")
		}
	}
	for i := range coll.Views {
		if coll.Views[i].Type == "board" {
			coll.Views[i].Fields = []string{"yazar"}
			coll.Views[i].Collapsed = []string{"Okundu"}
		}
	}
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Frame()
	tt.Click("Pano")
	tt.Frame()
	tt.Advance(time.Second)
	save("cset-board")
	for i, name := range []string{"general", "props", "views", "pages"} {
		a.openCollSettings(coll, i)
		if i == 2 {
			a.cs.view = "board"
		}
		if i == 1 {
			a.cs.newName = "ya"
		}
		tt.Frame()
		tt.Advance(1500 * time.Millisecond)
		save("cset-" + name)
		a.cs.open = false
		tt.Frame()
		tt.Advance(time.Second)
	}
}

// TestBlockShots draws the grip and its menu, a block dragged, and the /
// menu: run with BLOCKS=dir.
func TestBlockShots(t *testing.T) {
	dir := os.Getenv("BLOCKS")
	if dir == "" {
		t.Skip()
	}
	a, tt := newTestApp(t, "# Okuma listesi\n\nBu ay okuyacaklarım.\n\n- Dune\n    - Notlar\n- Anna Karenina\n\nSon paragraf.\n")
	tt.SetSize(1000, 640)
	save := func(name string) {
		f, _ := os.Create(dir + "/" + name + ".png")
		png.Encode(f, tt.Image())
		f.Close()
	}
	tt.Advance(300 * time.Millisecond)
	r := a.editorR
	box := func(s string) (float32, float32) {
		start := at(a.doc, s)
		for _, b := range a.boxes {
			if b.Start == start {
				return r.Y + b.Top, b.Height
			}
		}
		return 0, 0
	}
	y, h := box("Dune")
	tt.Move(r.X+60, y+h/2)
	tt.Frame()
	tt.Frame()
	save("grip")
	g, _ := tt.Find("Blok tutamağı")
	tt.Move(g.X+g.W/2, g.Y+g.H/2)
	tt.Frame()
	tt.Click("Blok tutamağı")
	tt.Advance(300 * time.Millisecond)
	save("grip-menu")
	tt.Key(0, ui.KeyEscape)
	tt.Advance(300 * time.Millisecond)
	tt.Move(g.X+g.W/2, g.Y+g.H/2)
	tt.Frame()
	tt.Frame()
	x0, y0 := g.X+g.W/2, g.Y+g.H/2
	ys, hs := box("Son paragraf.")
	tt.Press(x0, y0)
	for k := 1; k <= 10; k++ {
		tt.Move(x0+float32(k)*4, y0+(ys+hs/2-y0)*float32(k)/10)
		tt.Frame()
	}
	tt.Advance(200 * time.Millisecond)
	save("grip-drag")
	tt.Release(x0+40, ys+hs/2)
	tt.Advance(300 * time.Millisecond)
	a.doc.SetSelection(a.doc.Text().Len(), a.doc.Text().Len())
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	typeEach(tt, "/")
	tt.Advance(200 * time.Millisecond)
	save("slash")
}

// TestChipShots draws chips in a page and the @ menu: run with CHIPS=dir.
func TestChipShots(t *testing.T) {
	dir := os.Getenv("CHIPS")
	if dir == "" {
		t.Skip()
	}
	a, tt := newTestApp(t, "Bu hafta [[Dune]] ve [[Anna Karenina|Anna]] okunacak, [[2026-11-07]] günü bitmeli. [[Olmayan sayfa]] da var.\n")
	tt.SetSize(1000, 560)
	coll := a.page.Coll
	coll.NewPage(nil, "Dune")
	coll.NewPage(nil, "Anna Karenina")
	coll.AddProperty("Durum", PropChoice, []string{"Okunacak", "Okunuyor", "Okundu"})
	a.doc.SetText(a.parse("Bu hafta [[Dune]] ve [[Anna Karenina|Anna]] okunacak, [[2026-11-07]] günü bitmeli. [[Olmayan sayfa]] da var.\n"))
	a.doc.SetSelection(a.doc.Text().Len(), a.doc.Text().Len())
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	typeEach(tt, "Sonra @")
	tt.Advance(300 * time.Millisecond)
	f, _ := os.Create(dir + "/chips-at.png")
	png.Encode(f, tt.Image())
	f.Close()
}

// TestWelcomeShots draws the welcome in the app's window, as Linux and Windows have it, for each ground: the ground
// when it is all there, and the opening halfway. Run with WELSHOT=dir.
func TestWelcomeShots(t *testing.T) {
	dir := os.Getenv("WELSHOT")
	if dir == "" {
		t.Skip()
	}
	for _, g := range welcomeGrounds {
		a := welcomeApp(t)
		welcomeShaders = true
		a.settings.WelcomeGround, a.settings.Theme = g.id, 2
		tt := ui.NewTester(a.view, 472, 304)
		tt.Frame()
		now := a.wel.start
		a.wel.chosen, a.wel.chosenAt, a.wel.start = true, now.Add(-5*time.Second), now.Add(-5*time.Second)
		tt.Frame()
		save := func(name string) {
			f, _ := os.Create(dir + "/" + g.id + "-" + name + ".png")
			png.Encode(f, tt.Image())
			f.Close()
		}
		save("ground")
		o := a.welcomeOpeningLook()
		a.wel.origin, a.wel.goAt = ui.Point{X: 236, Y: 200}, now.Add(-time.Duration(o.length()*0.2*float64(time.Second)))
		tt.Frame()
		save("opening")
	}
}
