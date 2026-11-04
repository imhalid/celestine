// ryudit: a block editor on MyGo's rich text editor, over a library of
// collections and pages kept as plain Markdown files (library.go). The
// Markdown you type turns into structure as you type it and is never seen.
// The caret is ours: Slime or Mizan.
//
// Run with: go run . [library folder]   (default: ./kitaplik)
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

const welcome = `Burada **Markdown işareti görmezsin**: yazdığın işaretler anında yapıya dönüşür.

## Dene

- Yeni bir satırda \# ve boşluk: başlık. \- liste, 1. numaralı liste, \[\] görev, \> alıntı.
- Sözcükleri iki yıldız arasına al: **kalın**. Tek yıldız _italik_, ters tırnak ` + "`kod`" + `, iki tilde ~~üstü çizili~~.
- Boş bir satırda üç tire: ayırıcı çizgi. Üç ters tırnak: kod bloğu.
- ⌘B, ⌘I, ⌘U biçimlendirir; ⌘Z bir dönüşümü geri alır.
  - Tab içeri, ⇧Tab dışarı.

---

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

	sidebarClosed bool

	drag  dragState   // a page being dragged in the tree
	rows  []treeEntry // the tree's rows this frame, top to bottom
	treeR ui.Rect     // the tree's box, from its last frame

	renaming      any // the page or collection whose row is a field, being renamed
	renameText    string
	renameFocused bool    // the field has had the focus: losing it keeps the name
	sidebarShown  float32 // its width this frame, on the way to open or closed
	fresh         bool    // the page was just made: its title takes the focus

	theme *richtext.Theme // the editor's, resolved: what locate lays paragraphs out with
	font  ui.Font

	caretKind caretKind
	jelly     *slime
	mizan     *mizanCaret

	// What changed since the last frame, for the caret and for saving.
	version uint64
	length  int
	caretAt int
	wrote   int
	edited  bool
	editAt  time.Time
	dirty   bool
	saveErr string
	notice  string     // a toast to show
	trashed *TrashItem // just thrown away: a toast offers it back

	trashOpen  bool         // the trash is shown, instead of a page
	trashItems []*TrashItem // what is in it, read when it opens and changes
	boxes      []richtext.ParagraphBox
	focused    bool
	clock      func() time.Time
	lastSave   string
	style      richtext.ParagraphStyle // the caret paragraph's, last frame
	editorR    ui.Rect                 // the editor's box, from its last frame

	// The settings, and the MCP server they turn on (mcp.go). A frame and
	// an agent's tool each hold mu: the library is theirs one at a time.
	mu           sync.Mutex
	settings     Settings
	settingsFile string // "" in tests: not kept
	settingsOpen bool
	mcpServer    *http.Server
	mcpErr       string

	prop        propEdit           // the property dialog
	propBuf     map[string]*string // the text of the properties' fields, by page and property
	propEditing string             // the field being typed in
	propErr     string             // the field whose value did not fit
	invalidate  func()             // asks the window for a frame, from any goroutine
}

func newApp(lib *Library) *app {
	a := &app{lib: lib, docs: map[*Page]*richtext.Document{}, jelly: newSlime(), mizan: &mizanCaret{}, clock: time.Now}
	a.loadSettings()
	a.propBuf = map[string]*string{}
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
	if dir, err := os.UserConfigDir(); err == nil { // the settings kept, not a test's
		a.settingsFile, a.settings = filepath.Join(dir, "ryudit", "settings.json"), Settings{}
		a.loadSettings()
	}
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu}}))
		w := mygo.NewWindow(mygo.WindowOptions{Title: "ryudit", Width: 1180, Height: 760, MinWidth: 640, MinHeight: 420,
			TitleBarStyle: mygo.TitleBarHiddenInset, TrafficLightPosition: &mygo.Point{X: 18, Y: 16}, Content: ui.View(a.view)})
		a.invalidate = w.Invalidate
		if a.settings.MCP {
			a.setMCP(true)
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// open shows page p, saving the page that was open first.
func (a *app) open(p *Page) {
	a.coll, a.trashOpen = nil, false
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
		doc = richtext.NewDocument(parseBody(body))
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
	a.mu.Lock()
	defer a.mu.Unlock()
	ryugo.Root(c, ryugo.Options{}) // Ryugo's theme, its components' tokens
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
	if root.Shortcut(ui.Cmd, ui.KeyBackslash) {
		a.toggleSidebar()
	}
	if root.Shortcut(ui.Cmd|ui.Shift, ui.KeyM) {
		a.caretKind = (a.caretKind + 1) % 2
	}
	if root.Shortcut(ui.Ctrl, ui.KeyN) && a.page != nil {
		a.newPage(a.page.Coll, nil)
	}
	root.Children(func() {
		shown := a.sidebar(c)
		a.sidebarShown = shown
		ui.Column(c).Grow(1).Children(func() {
			a.titleBar(c, shown)
			ui.Row(c).Grow(1).Justify(ui.Center).Padding(pageTop-barH+8, 32, 0, 32).Children(func() {
				if a.trashOpen {
					a.focused = false
					a.trashView(c)
					return
				}
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
	a.settingsDialog(c)
	a.propDialog(c)
	ryugo.Toaster(c)
	if a.notice != "" {
		c.Toast(a.notice)
		a.notice = ""
	}
	if item := a.trashed; item != nil {
		a.trashed = nil
		c.ToastAction("“"+item.Title+"” çöpe taşındı", "Geri al", func() { a.restore(item) })
		a.loadTrash()
	}
	if a.doc != nil && a.coll == nil {
		a.boxes = a.doc.VisibleParagraphs()
		a.autosave(c)
	}
}

// pageView is the open page: its collection, its title, its text.
func (a *app) pageView(c *ui.Context, th *richtext.Theme) {
	p := a.page // where it is shows in the title bar
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
	a.propsSection(c, p)
	c.Key("body:" + p.Dir)
	ed := richtext.EditorBase(c, a.doc, richtext.EditorOptions{
		Label: "Sayfa", Font: a.font, Theme: th,
		Placeholder: "Yaz…  # başlık · - liste · [] görev · > alıntı",
	}).Grow(1)
	if !title.Focused() && !a.fresh {
		ed.AutoFocus()
	}
	a.focused = ed.Focused()
	ed.DrawOver(func(p *ui.Painter, r ui.Rect) {
		a.drawDividers(p, r)
		a.drawCaret(p, r)
	})
}

// crumbs is where the page is: its collection and the pages above it.
func (a *app) crumbs() string {
	var parts []string
	for q := a.page.Parent; q != nil; q = q.Parent {
		parts = append([]string{q.Title}, parts...)
	}
	return strings.Join(append([]string{a.page.Coll.Name}, parts...), "  ›  ")
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
	// "---" and "```" typed: a divider, a block of code (blocks.go)
	if a.blockShortcuts(text, caret, n-a.length == 1 && caret-a.caretAt == 1) {
		_, caret = a.doc.Selection()
		text, v = a.doc.Text(), a.doc.Version()
		style = text.ParagraphStyleAt(caret)
		a.length, a.caretAt = text.Len()-1, caret-1 // still the letter typed, to the caret
		n = text.Len()
	}
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
		if a.trashOpen {
			where = trashDir
		} else if a.coll != nil {
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
	md := bodyMarkdown(a.doc.Text())
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
