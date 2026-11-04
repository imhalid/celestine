// ryudit: a block editor on MyGo's rich text editor, over a library of
// collections and pages kept as plain Markdown files (library.go). The
// Markdown you type turns into structure as you type it and is never seen.
// The caret is ours: Slime or Mizan.
//
// Run with: go run . [library folder]   (default: ./kitaplik)
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

const welcome = `Burada **Markdown işareti görmezsin**: yazdığın işaretler anında yapıya dönüşür.

## Dene

- Yeni bir satırda \# ve boşluk: başlık. \- liste, 1. numaralı liste, \[\] görev, \> alıntı.
- Sözcükleri iki yıldız arasına al: **kalın**. Tek yıldız _italik_, ters tırnak ` + "`kod`" + `, iki tilde ~~üstü çizili~~.
- ⌘B, ⌘I, ⌘U biçimlendirir; ⌘Z bir dönüşümü geri alır.
  - Tab içeri, ⇧Tab dışarı.

## Sayfalar

- Soldaki bir koleksiyonun yanındaki + yeni sayfa açar; bir sayfanın yanındaki + onun altına alt sayfa.
- ⌃N bu koleksiyonda yeni sayfa açar.
- [ ] Kutuya tıkla
- [x] Bitmiş bir görev

> Her sayfa kitaplık klasöründe bir klasör ve içinde index.md: başka her araç okuyabilir.

⌘⇧M caret'i değiştirir: Slime ya da Mizan.
`

const (
	columnMax = 720
	pageTop   = 48
	sidebarW  = 252
)

type caretKind int

const (
	caretSlime caretKind = iota
	caretMizan
)

// caretFrame is what a caret is told each frame (see mizan.go).
type caretFrame struct {
	r      ui.Rect
	cw     float32
	dark   bool
	bounds ui.Rect
	wrote  int
	edited bool
}

type app struct {
	lib   *Library
	page  *Page              // the page open
	doc   *richtext.Document // its text
	docs  map[*Page]*richtext.Document
	title string // the title field's text

	coll     *Collection // the collection whose page is shown, instead of a page
	collName string      // its name field's text
	search   string      // its search field's text
	fresh    bool        // the page was just made: its title takes the focus

	theme *richtext.Theme // the editor's, resolved: what locate lays paragraphs out with
	font  ui.Font

	caretKind caretKind
	jelly     *slime
	mizan     *mizanCaret

	// What changed since the last frame, for the caret and for saving.
	version  uint64
	length   int
	caretAt  int
	wrote    int
	edited   bool
	editAt   time.Time
	dirty    bool
	saveErr  string
	boxes    []richtext.ParagraphBox
	focused  bool
	clock    func() time.Time
	lastSave string
	style    richtext.ParagraphStyle // the caret paragraph's, last frame
	editorR  ui.Rect                 // the editor's box, from its last frame
}

func newApp(lib *Library) *app {
	a := &app{lib: lib, docs: map[*Page]*richtext.Document{}, jelly: newSlime(), mizan: &mizanCaret{}, clock: time.Now}
	for _, c := range lib.Collections {
		if len(c.Pages) > 0 {
			a.open(c.Pages[0])
			break
		}
	}
	return a
}

// seed gives an empty library a collection and a page to start from.
func seed(lib *Library) error {
	if len(lib.Collections) > 0 {
		return nil
	}
	c, err := lib.NewCollection("Notlar")
	if err != nil {
		return err
	}
	p, err := c.NewPage(nil, "Hoş geldin")
	if err != nil {
		return err
	}
	return p.Save(welcome)
}

func main() {
	root := "kitaplik"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	root, _ = filepath.Abs(root)
	lib, err := OpenLibrary(root)
	if err != nil {
		log.Fatal(err)
	}
	if err := seed(lib); err != nil {
		log.Fatal(err)
	}
	a := newApp(lib)
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu}}))
		mygo.NewWindow(mygo.WindowOptions{Title: "ryudit", Width: 1180, Height: 760, Content: ui.View(a.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// open shows page p, saving the page that was open first.
func (a *app) open(p *Page) {
	a.coll = nil
	if p == a.page {
		return
	}
	a.flush()
	doc := a.docs[p]
	if doc == nil {
		body, err := p.Body()
		if err != nil {
			a.saveErr = err.Error()
		}
		doc = richtext.NewDocument(richtext.ParseMarkdown(body))
		a.docs[p] = doc
	}
	a.page, a.doc, a.title = p, doc, p.Title
	_, caret := doc.Selection()
	a.version, a.length, a.caretAt = doc.Version(), doc.Text().Len(), caret
	a.dirty, a.saveErr, a.lastSave = false, "", ""
	a.boxes = nil
}

// newPage makes a page in c (under parent when not nil) and opens it, its title ready to be typed.
func (a *app) newPage(c *Collection, parent *Page) {
	p, err := c.NewPage(parent, "Adsız")
	if err != nil {
		a.saveErr = err.Error()
		return
	}
	a.open(p)
	a.fresh = true
}

// flush saves the open page now, if it changed.
func (a *app) flush() {
	if a.page != nil && a.dirty {
		a.save()
	}
}

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	th := richtext.DefaultTheme(t)
	th.Caret = invisible // ours is drawn over it
	th.ParagraphSpacing = 0.55
	for i := range th.Headings {
		th.Headings[i].Top, th.Headings[i].Bottom = 0.9, 0.3
	}
	a.theme = th
	a.font = ui.Font{Family: t.Font, Size: 16.5, LineHeight: 1.55}
	if a.doc != nil {
		a.track(c)
	}

	root := ui.Row(c).Fill().AlignItems(ui.Stretch).Background(t.Background)
	if root.Shortcut(ui.Cmd|ui.Shift, ui.KeyM) {
		a.caretKind = (a.caretKind + 1) % 2
	}
	if root.Shortcut(ui.Ctrl, ui.KeyN) && a.page != nil {
		a.newPage(a.page.Coll, nil)
	}
	root.Children(func() {
		a.sidebar(c)
		ui.Column(c).Grow(1).Children(func() {
			ui.Row(c).Grow(1).Justify(ui.Center).Padding(pageTop, 32, 0, 32).Children(func() {
				if a.coll != nil {
					a.focused = false
					a.collectionView(c)
					return
				}
				if a.page == nil {
					ui.Text(c, "Soldan bir sayfa seç ya da + ile yeni bir sayfa aç.").TextColor(t.TextMuted)
					return
				}
				ui.Column(c).Fill().MaxWidth(columnMax).Gap(4).Children(func() { a.pageView(c, th) })
			})
			a.status(c)
		})
	})
	if a.doc != nil && a.coll == nil {
		a.boxes = a.doc.VisibleParagraphs()
		a.autosave(c)
	}
}

// pageView is the open page: its collection, its title, its text.
func (a *app) pageView(c *ui.Context, th *richtext.Theme) {
	t := c.Theme()
	p := a.page
	ui.Row(c).Children(func() {
		ui.Box(c).Padding(3, 10).Radius(999).Background(t.Surface).Children(func() {
			ui.Text(c, a.crumbs()).FontSize(12).TextColor(t.TextMuted)
		})
	})
	c.Key("title:" + p.Dir)
	title := ui.TextInputBase(c, &a.title).Label("Başlık").FontSize(32).FontWeight(720).Padding(6, 0, 10, 0)
	if a.fresh {
		title.AutoFocus()
		a.fresh = false
	}
	if title.Changed() {
		name := strings.TrimSpace(a.title)
		if name == "" {
			name = "Adsız"
		}
		if err := p.SetTitle(name); err != nil {
			a.saveErr = err.Error()
		}
	}
	c.Key("body:" + p.Dir)
	ed := richtext.EditorBase(c, a.doc, richtext.EditorOptions{
		Label: "Sayfa", Font: a.font, Theme: th,
		Placeholder: "Yaz…  # başlık · - liste · [] görev · > alıntı",
	}).Grow(1)
	if !title.Focused() && !a.fresh {
		ed.AutoFocus()
	}
	a.focused = ed.Focused()
	ed.DrawOver(a.drawCaret)
}

// crumbs is where the page is: its collection and the pages above it.
func (a *app) crumbs() string {
	var parts []string
	for q := a.page.Parent; q != nil; q = q.Parent {
		parts = append([]string{q.Title}, parts...)
	}
	return strings.Join(append([]string{a.page.Coll.Name}, parts...), "  ›  ")
}

// sidebar lists the collections and their pages as a tree: a collection's
// + makes a page in it, a page's + a page under it.
func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(sidebarW).Background(t.Surface).Padding(18, 10, 10, 10).Gap(1).Children(func() {
		ui.Row(c).Padding(0, 8, 8, 8).Children(func() {
			ui.Text(c, "KOLEKSİYONLAR").FontSize(11).FontWeight(650).TextColor(t.TextMuted).Grow(1)
			if a.plus(c, "Yeni koleksiyon", true) {
				if coll, err := a.lib.NewCollection("Yeni koleksiyon"); err == nil {
					a.newPage(coll, nil)
				} else {
					a.saveErr = err.Error()
				}
			}
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				for _, coll := range a.lib.Collections {
					a.collectionRow(c, coll)
					if coll.Open {
						for _, p := range coll.Pages {
							a.pageRow(c, p)
						}
					}
				}
			})
		})
	})
}

func (a *app) collectionRow(c *ui.Context, coll *Collection) {
	t := c.Theme()
	c.Key("coll:" + coll.Dir)
	row := ui.Row(c).Height(30).Padding(0, 6, 0, 4).Gap(4).Radius(6).AlignItems(ui.Center).Label("Koleksiyon: " + coll.Name).Role(ui.RoleButton)
	hover := row.Hovered()
	switch {
	case coll == a.coll:
		row.Background(t.Selection)
	case hover:
		row.Background(t.SurfaceHover)
	}
	clicked := false
	row.Children(func() {
		if a.chevron(c, len(coll.Pages) > 0, coll.Open) {
			coll.Open = !coll.Open
			clicked = true
		}
		ui.Text(c, coll.Name).FontSize(13.5).FontWeight(600).MaxLines(1).Grow(1)
		if a.plus(c, "Yeni sayfa: "+coll.Name, hover) {
			a.newPage(coll, nil)
			clicked = true
		}
	})
	if row.Clicked() && !clicked {
		a.openCollection(coll)
	}
}

func (a *app) pageRow(c *ui.Context, p *Page) {
	t := c.Theme()
	c.Key("page:" + p.Dir)
	row := ui.Row(c).Height(30).Padding(0, 6, 0, 4+float32(p.Depth()+1)*14).Gap(4).Radius(6).AlignItems(ui.Center)
	hover := row.Hovered()
	switch {
	case p == a.page && a.coll == nil:
		row.Background(t.Selection)
	case hover:
		row.Background(t.SurfaceHover)
	}
	handled := false
	row.Children(func() {
		if a.chevron(c, len(p.Children) > 0, p.Open) {
			p.Open = !p.Open
			handled = true
		}
		ui.Text(c, p.Title).FontSize(13.5).MaxLines(1).Grow(1)
		if p.Coll.SubPages && a.plus(c, "Alt sayfa: "+p.Title, hover) {
			a.newPage(p.Coll, p)
			handled = true
		}
	})
	if row.Clicked() && !handled {
		a.open(p)
	}
	if p.Open {
		for _, ch := range p.Children {
			a.pageRow(c, ch)
		}
	}
}

// chevron opens and closes a row; it holds its room when there is nothing below.
func (a *app) chevron(c *ui.Context, has, open bool) bool {
	t := c.Theme()
	glyph := "▸"
	if open {
		glyph = "▾"
	}
	b := ui.Box(c).Width(16).Height(20).AlignItems(ui.Center).Justify(ui.Center)
	b.Children(func() {
		if has {
			ui.Text(c, glyph).FontSize(11).TextColor(t.TextMuted)
		}
	})
	return has && b.Clicked()
}

// plus is a small + button; shown only when the row is hovered (or always).
func (a *app) plus(c *ui.Context, label string, shown bool) bool {
	t := c.Theme()
	b := ui.Box(c).Width(22).Height(22).Radius(5).AlignItems(ui.Center).Justify(ui.Center).Label(label).Role(ui.RoleButton)
	if b.Hovered() {
		b.Background(t.SurfacePressed)
	}
	if !shown {
		b.Opacity(0)
	}
	b.Children(func() { ui.Text(c, "+").FontSize(15).TextColor(t.TextMuted) })
	return b.Clicked()
}

// track notes what the user did since the last frame: letters written (for
// Mizan's pace), any other edit, and that the file needs saving.
func (a *app) track(c *ui.Context) {
	v := a.doc.Version()
	_, caret := a.doc.Selection()
	text := a.doc.Text()
	style := text.ParagraphStyleAt(caret)
	if v == a.version {
		a.caretAt, a.style = caret, style
		return
	}
	n := text.Len()
	if grew := n - a.length; grew > 0 && caret-a.caretAt == grew && grew <= 8 {
		a.wrote += grew
	} else {
		a.edited = true
	}
	// A Markdown shortcut ("[] ", "# "…) just turned an empty paragraph into
	// another kind: what is typed there starts plain. (richtext v0.7.4 takes
	// the empty paragraph's attributes from its end mark, which a bold word
	// closed at the end of the paragraph above left bold.)
	if s, e := text.ParagraphRange(caret); n < a.length && style != a.style && s == e {
		a.doc.SetTypingAttributes(richtext.Attributes{})
	}
	a.version, a.length, a.caretAt, a.style = v, n, caret, style
	a.dirty, a.editAt, a.saveErr = true, a.clock(), ""
}

func (a *app) drawCaret(p *ui.Painter, r ui.Rect) {
	a.editorR = r
	if !a.focused {
		return
	}
	cb, ok := locate(a.doc, a.boxes, r.W, a.font, a.theme)
	if !ok {
		return
	}
	cr := cb.r
	cr.X += r.X
	cr.Y += r.Y
	dark := p.Theme().Dark
	anchor, caret := a.doc.Selection()
	if anchor != caret { // over a selection, a plain caret: the jelly would hide what is selected
		p.Fill(ui.Rect{X: cr.X - 1, Y: cr.Y, W: 2, H: cr.H}, p.Theme().Accent, 1)
		return
	}
	f := caretFrame{r: cr, cw: cb.cw, dark: dark, bounds: r, wrote: a.wrote, edited: a.edited}
	a.wrote, a.edited = 0, false
	p.Clip(r, 0, func() {
		if a.caretKind == caretMizan {
			a.mizan.draw(p, f)
			return
		}
		col := ui.Hex("#59f939")
		if !dark {
			col = ui.Hex("#2f9a1c")
		}
		a.jelly.draw(p, cr, cb.cw, col, dark)
	})
}

// status is the line at the bottom: the file, whether it is saved, the caret.
func (a *app) status(c *ui.Context) {
	t := c.Theme()
	state := "kaydedildi"
	switch {
	case a.saveErr != "":
		state = "kaydedilemedi: " + a.saveErr
	case a.dirty:
		state = "kaydediliyor…"
	}
	caret := map[caretKind]string{caretSlime: "Slime", caretMizan: "Mizan"}[a.caretKind]
	ui.Row(c).Padding(10, 18).Justify(ui.End).Children(func() {
		where := "ryudit"
		if a.coll != nil {
			if rel, err := filepath.Rel(a.lib.Root, filepath.Join(a.coll.Dir, collectionFile)); err == nil {
				where = rel
			}
		} else if a.page != nil {
			if rel, err := filepath.Rel(a.lib.Root, a.page.file()); err == nil {
				where = rel
			}
		}
		ui.Text(c, where+" · "+state+"   ·   caret: "+caret+" (⌘⇧M)").FontSize(11.5).TextColor(t.TextMuted)
	})
}

// autosave writes the file a moment after the last edit.
func (a *app) autosave(c *ui.Context) {
	if !a.dirty || a.page == nil || a.saveErr != "" {
		return
	}
	if wait := 700*time.Millisecond - a.clock().Sub(a.editAt); wait > 0 {
		c.After(wait)
		return
	}
	a.save()
}

func (a *app) save() {
	md := a.doc.Text().Markdown()
	if !strings.HasSuffix(md, "\n") {
		md += "\n"
	}
	if md == a.lastSave {
		a.dirty = false
		return
	}
	if err := a.page.Save(md); err != nil {
		a.saveErr = err.Error()
		return
	}
	a.lastSave, a.dirty = md, false
}
