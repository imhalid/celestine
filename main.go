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
	"strconv"
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

	trashOpen    bool         // the trash is shown, instead of a page
	trashItems   []*TrashItem // what is in it, read when it opens and changes
	boxes        []richtext.ParagraphBox
	focused      bool
	clock        func() time.Time
	lastSave     string
	style        richtext.ParagraphStyle // the caret paragraph's, last frame
	refocus      bool                    // the editor takes the keyboard focus in the next frame
	pageR        ui.Rect                 // the area the page is in, last frame: the editor reaches its left edge
	heads        map[*Page]string        // the heading of each page that is its title, kept aside (hidetitle.go)
	caretPara    int                     // where the caret was, last frame: its paragraph, its place in it, the paragraph's top
	caretRel     float32
	caretTop     float32
	sel          selBar         // the bar over a selection (selbar.go)
	edPressed    bool           // the pointer is down in the editor
	ctx          ctxMenu        // the context menu (ctxmenu.go)
	pageScroll   ui.ScrollState // the page scrolled as a whole (pageView)
	scrollR      ui.Rect        // its room in the window, last frame
	headH        float32        // the title's and properties' height, last frame
	titleFocused bool
	edH          float32 // the editor's height, last frame
	frame        func()  // asks for a frame that builds the view
	revealWant   bool    // the caret moved: bring it into view
	statsVer     uint64  // the version of the text the counts are of
	statWords    int
	statChars    int
	settingsTab  int      // the settings' tab
	hoverCh      *chip    // the chip the pointer is over
	dateEdit     dateEdit // the calendar a date chip opens
	editorR      ui.Rect  // the editor's box, from its last frame

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
	pal         palette            // the command palette (palette.go)
	chipHits    []chipHit          // the chips drawn this frame (chips.go)
	chipSeen    []chipHit          // and last frame, for a click
	slash       slashMenu          // the / menu (slash.go)
	caretWin    ui.Rect            // the caret in the window, last drawn
	grip        gripState          // the grip of the block under the pointer (blockui.go)
	cs          collSettings       // a collection\'s settings (collsettings.go)
	find        finder             // finding in the page (find.go)
	hist        []place            // where the app went, and where it is in that (nav.go)
	histAt      int
	going       bool     // going back or forward: no new place in the history
	menuBar     bool     // the menu bar has the shortcuts, not the window
	pageBg      ui.Color // the page's background this frame (bg)
}

func newApp(lib *Library) *app {
	a := &app{lib: lib, docs: map[*Page]*richtext.Document{}, jelly: newSlime(), mizan: &mizanCaret{}, clock: time.Now}
	a.histAt, a.grip.para = -1, -1
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
		mygo.App.SetMenu(a.menu())
		a.menuBar = true
		w := mygo.NewWindow(mygo.WindowOptions{Title: "ryudit", Width: 1180, Height: 760, MinWidth: 640, MinHeight: 420,
			TitleBarStyle: mygo.TitleBarHiddenInset, TrafficLightPosition: &mygo.Point{X: 18, Y: 16}, Content: ui.View(a.view)})
		a.invalidate = w.Invalidate
		go a.watch()
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
	defer a.visit()
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
		doc = richtext.NewDocument(a.parsePage(p, body))
		a.docs[p] = doc
	}
	a.page, a.doc, a.title = p, doc, p.Title
	a.find.ver = 0 // another text to find in
	_, caret := doc.Selection()
	a.version, a.length, a.caretAt = doc.Version(), doc.Text().Len(), caret
	a.dirty, a.saveErr, a.lastSave = false, "", ""
	a.boxes = nil
	a.pageScroll = ui.ScrollState{}
}

// newPage makes a page in c (under parent when not nil) and opens it, its title ready to be typed.
func (a *app) newPage(c *Collection, parent *Page) {
	p, err := c.NewPage(parent, "Adsız")
	if err != nil {
		a.saveErr = err.Error()
		return
	}
	if c.Template != "" { // it starts as the collection says
		if err := p.Save(c.Template); err != nil {
			a.saveErr = err.Error()
		}
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
	a.frame = c.Invalidate
	a.chipSeen, a.chipHits = a.chipHits, a.chipSeen[:0]                                                              // the last frame's, for a click now
	ryugo.Root(c, ryugo.Options{Mode: []ryugo.Mode{ryugo.ModeInherit, ryugo.Light, ryugo.Dark}[a.settings.Theme%3]}) // Ryugo's theme, its components' tokens
	t := c.Theme()
	th := richtext.DefaultTheme(t)
	th.Caret = invisible // ours is drawn over it
	th.ParagraphSpacing = 0.55
	// A heading's room above stays under half its line: the editor moves ↑ by
	// the height of the line the caret is on, and from a heading that must
	// land in the paragraph above, not in the heading's own room.
	for i := range th.Headings {
		th.Headings[i].Top, th.Headings[i].Bottom = 0.6, 0.3
	}
	a.theme = th
	a.font = ui.Font{Family: t.Font, Size: a.fontSize(), LineHeight: a.lineHeight()}
	if a.doc != nil {
		a.track(c)
	}

	a.pageBg = a.bg(c)
	root := ui.Row(c).Fill().AlignItems(ui.Stretch).Background(a.pageBg)
	if !a.menuBar { // in the app, the menu bar has these (menu.go)
		if c.Shortcut(ui.Cmd, ui.KeyBackslash) {
			a.toggleSidebar()
		}
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyM) {
			a.setCaret((a.caretKind + 1) % 2)
		}
		if c.Shortcut(ui.Cmd, ui.KeyBracketLeft) {
			a.step(-1)
		}
		if c.Shortcut(ui.Cmd, ui.KeyBracketRight) {
			a.step(1)
		}
		if c.Shortcut(ui.Cmd, ui.KeyComma) {
			a.settingsOpen = true
		}
		if c.Shortcut(ui.Cmd, ui.KeyK) || c.Shortcut(ui.Cmd, ui.KeyP) {
			a.openPalette()
		}
		if c.Shortcut(ui.Cmd, ui.KeyF) {
			a.openFind()
		}
		if c.Shortcut(ui.Cmd, ui.KeySlash) && a.doc != nil && a.page != nil {
			_, caret := a.doc.Selection()
			a.toggleFold(caret)
		}
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyUp) && a.doc != nil {
			moveBlock(a.doc, -1)
		}
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyDown) && a.doc != nil {
			moveBlock(a.doc, 1)
		}
	}
	if c.Shortcut(ui.Ctrl, ui.KeyN) && a.page != nil {
		a.newPage(a.page.Coll, nil)
	}
	root.Children(func() {
		shown := a.sidebar(c)
		a.sidebarShown = shown
		ui.Column(c).Grow(1).Children(func() {
			a.titleBar(c)
			// the page scrolls over the whole width, its bar at the window's edge:
			// the margins are inside it
			padX := float32(32)
			if !a.trashOpen && a.coll == nil && a.page != nil {
				padX = 0
			}
			pageRow := ui.Row(c).Grow(1).Justify(ui.Center).Padding(pageTop-barH+8, padX, 0, padX)
			defer func() { a.pageR = pageRow.LayoutBounds() }()
			pageRow.Children(func() {
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
				a.pageView(c, th)
			})
			a.status(c)
		})
		a.sidebarButton(c)
	})
	a.caretView(c)
	a.selectionBar(c)
	a.gutterView(c)
	a.gripView(c)
	a.slashView(c)
	a.dateEditView(c)
	a.contextView(c)
	a.paletteView(c)
	a.collSettingsView(c)
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

// bg is the page's background: Ryugo's, but in the dark not its black -
// its card's dark gray, a step below the sidebar.
func (a *app) bg(c *ui.Context) ui.Color {
	if t := ryugo.From(c); t.Dark {
		return t.Colors.Card
	}
	return c.Theme().Background
}

// pageView is the open page: its title, its properties and its text, one
// page that scrolls as a whole. The editor is as tall as its text (and some
// more, to lay out what is coming), so it has nothing to scroll in itself,
// and the wheel passes to the scroll around it; where the caret is is kept
// in view here (drawCaret asks for it).
func (a *app) pageView(c *ui.Context, th *richtext.Theme) {
	p := a.page // where it is shows in the title bar
	// the page is as wide as the window and scrolls in it, the bar at its edge;
	// the text is a column in the middle, with margins either side
	ui.Column(c).Fill().Children(func() {
		if a.find.open {
			ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, 32).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(a.pageWidth()).Children(func() { a.findBar(c) })
			})
		}
		sc := ui.Scroll(c.Key("page-scroll:" + p.Dir)).TrackScroll(&a.pageScroll).Grow(1)
		sc.Children(func() {
			ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, 32).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(a.pageWidth()).Gap(4).Children(func() { a.pageBody(c, p, th) })
			})
		})
		a.scrollR = sc.LayoutBounds()
	})
}

// pageBody is the title, the properties and the text.
func (a *app) pageBody(c *ui.Context, p *Page, th *richtext.Theme) {
	head := ui.Column(c.Key("page-head")).FillWidth()
	head.Children(func() { a.pageHead(c, p) })
	a.headH = head.LayoutBounds().H
	c.Key("body:" + p.Dir)
	ed := richtext.EditorBase(c, a.doc, richtext.EditorOptions{
		Label: "Sayfa", Font: a.font, Theme: th,
		Placeholder: "Yaz…  # başlık · - liste · [] görev · > alıntı",
	}).FillWidth().Height(a.editorH(c))
	if !a.titleFocused && !a.fresh {
		ed.AutoFocus()
	}
	if a.refocus {
		ed.Focus()
		a.refocus = false
	}
	a.focused = ed.Focused()
	a.edPressed = ed.Pressed()
	a.trackGrip(ed)
	a.hoverCh = nil
	if x, y, over := ed.PointerPosition(); over {
		if h := a.chipAt(a.editorR.X+x, a.editorR.Y+y); h != nil {
			a.hoverCh = h.ch
			ed.Cursor(ui.CursorPointer)
		}
	}
	if ed.Clicked() { // on a chip: its page, or its day
		x, y, _ := ed.PointerPosition()
		a.clickChip(a.editorR.X+x, a.editorR.Y+y)
	}
	ed.DrawOver(func(p *ui.Painter, r ui.Rect) {
		a.editorR = r
		a.drawMatches(p, r)
		a.drawBlockDrop(p, r)
		a.drawDividers(p, r)
	})
}

// caretView draws the caret in a layer over the window - the editor clips
// what it draws at its own edge, which cut the caret at the start of a
// line - itself clipped to the page's room.
func (a *app) caretView(c *ui.Context) {
	if a.doc == nil || a.coll != nil || a.trashOpen || a.page == nil {
		return
	}
	ww, wh := c.Size()
	ui.Overlay(c, func() {
		ui.Box(c.Key("caret-layer")).Absolute().Left(0).Top(0).Size(ww, wh).PassThrough().Draw(func(p *ui.Painter, _ ui.Rect) {
			a.drawCaret(p, a.editorR)
		})
	})
}

// pageHead is the page's title and its properties.
func (a *app) pageHead(c *ui.Context, p *Page) {
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
	a.titleFocused = title.Focused()
	// ↓ or ↵ in the title goes into the text
	if title.Focused() && (title.Shortcut(0, ui.KeyDown) || title.Submitted()) {
		a.doc.SetSelection(0, 0)
		a.refocus = true
	}
	a.propsSection(c, p)
}

// editorH is editorHeight, asking for another frame while it grows.
func (a *app) editorH(c *ui.Context) float32 {
	h := a.editorHeight()
	if h != a.edH {
		a.edH = h
		c.Invalidate()
	}
	return h
}

// editorHeight is the editor's: its text, and more while there is text
// still to lay out; at least what is left of the page's room.
func (a *app) editorHeight() float32 {
	least := max(a.scrollR.H-a.headH-8, 160)
	n := len(a.boxes)
	if n == 0 {
		return least
	}
	last := a.boxes[n-1]
	_, pe := a.doc.Text().ParagraphRange(last.Start)
	slack := max(a.scrollR.H*2, 1600)
	if pe >= a.doc.Text().Len() {
		slack = 96 // the last paragraph is in
	}
	return max(last.Top+last.Height+slack, least)
}

// track notes what the user did since the last frame: letters written (for
// Mizan's pace), any other edit, and that the file needs saving.
func (a *app) track(c *ui.Context) {
	v := a.doc.Version()
	_, caret := a.doc.Selection()
	text := a.doc.Text()
	style := text.ParagraphStyleAt(caret)
	if v == a.version {
		if a.slash.open && caret != a.caretAt { // the caret moved: ↑ ↓ in the / menu
			a.trackSlash(text, caret, false, false)
			_, caret = a.doc.Selection()
		}
		if caret != a.caretAt {
			a.revealWant = true
		}
		a.caretAt, a.style = caret, style
		return
	}
	a.revealWant = true
	n := text.Len()
	// the / menu (slash.go)
	if a.trackSlash(text, caret, true, n-a.length == 1 && caret-a.caretAt == 1) {
		_, caret = a.doc.Selection()
		text, v = a.doc.Text(), a.doc.Version()
		style = text.ParagraphStyleAt(caret)
		n = text.Len()
		a.length, a.caretAt = n, caret
	}
	// "---" and "```" typed: a divider, a block of code (blocks.go)
	if a.blockShortcuts(text, caret, n-a.length == 1 && caret-a.caretAt == 1) {
		_, caret = a.doc.Selection()
		text, v = a.doc.Text(), a.doc.Version()
		style = text.ParagraphStyleAt(caret)
		a.length, a.caretAt = text.Len()-1, caret-1 // still the letter typed, to the caret
		n = text.Len()
	}
	// "[[Page]]": the "]" typed over the one the "[[" wrote, then a chip (chips.go)
	single := n-a.length == 1 && caret-a.caretAt == 1
	if a.overtype(text, caret, single) {
		text, v = a.doc.Text(), a.doc.Version()
		n = text.Len()
	}
	if a.chipShortcut(text, caret, single) {
		_, caret = a.doc.Selection()
		text, v = a.doc.Text(), a.doc.Version()
		style = text.ParagraphStyleAt(caret)
		n = text.Len()
		a.length, a.caretAt = n, caret
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

// revealCaret scrolls the page, when the caret was moved, to bring it into
// view with some room around it.
func (a *app) revealCaret(cr ui.Rect) {
	if !a.revealWant || a.scrollR.H == 0 {
		return
	}
	a.revealWant = false
	const room = 56
	vp := a.scrollR
	var d float32
	switch {
	case cr.Y < vp.Y+room:
		d = cr.Y - (vp.Y + room)
	case cr.Y+cr.H > vp.Y+vp.H-room:
		d = cr.Y + cr.H - (vp.Y + vp.H - room)
	}
	if d != 0 {
		a.pageScroll.Y = max(0, a.pageScroll.Y+d)
		if d < 0 && a.pageScroll.Y < a.headH+room { // near the top: the title and the properties too
			a.pageScroll.Y = 0
		}
		if a.frame != nil {
			a.frame()
		}
	}
}

func (a *app) drawCaret(p *ui.Painter, r ui.Rect) {
	if !a.focused {
		return
	}
	cb, ok := locate(a.doc, a.boxes, r.W, a.font, a.theme)
	if !ok {
		return
	}
	// scrolled: the caret went with its paragraph, the text under it the
	// same - the jelly goes with it and is not kicked
	top := r.Y + cb.top
	if cb.para == a.caretPara && cb.r.Y-cb.top == a.caretRel && top != a.caretTop {
		a.jelly.shift(0, top-a.caretTop)
		a.mizan.shift(top - a.caretTop)
	}
	a.caretPara, a.caretRel, a.caretTop = cb.para, cb.r.Y-cb.top, top
	cr := cb.r
	cr.X += r.X
	cr.Y += r.Y
	a.caretWin = cr
	a.revealCaret(cr)
	dark := p.Theme().Dark
	anchor, caret := a.doc.Selection()
	if anchor != caret { // over a selection, a plain caret: the jelly would hide what is selected
		p.Fill(ui.Rect{X: cr.X - 1, Y: cr.Y, W: 2, H: cr.H}, p.Theme().Accent, 1)
		return
	}
	f := caretFrame{r: cr, cw: cb.cw, dark: dark, bounds: r, wrote: a.wrote, edited: a.edited}
	a.wrote, a.edited = 0, false
	// not clipped to the editor's box: at the start of a line the caret
	// goes past its left edge
	room := a.scrollR
	if room.H == 0 {
		room = ui.Rect{X: r.X - 24, Y: r.Y - 12, W: r.W + 48, H: r.H + 24}
	}
	p.Clip(room, 0, func() {
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
	ui.Row(c).Padding(10, 18).Justify(ui.End).AlignItems(ui.Center).Gap(12).Children(func() {
		// the MCP server on: a dot of the accent, which opens the settings
		if a.mcpServer != nil {
			m := ui.Row(c.Key("mcp-status")).Gap(5).Padding(2, 6).Radius(6).AlignItems(ui.Center).
				Label("MCP açık: ayarlar").Role(ui.RoleButton)
			ryugo.Tooltip(c, m, "MCP sunucusu açık · "+a.mcpURL())
			if m.Hovered() {
				m.Background(t.SurfaceHover)
			}
			m.Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(ryugo.From(c).Colors.Down)
				ui.Text(c, "MCP").FontSize(11.5).FontWeight(600).TextColor(t.TextMuted)
			})
			if m.Clicked() {
				a.settingsOpen, a.settingsTab = true, 3 // the MCP tab
			}
		}
		// no file paths: what the settings ask for - the words, how it is saved, the caret
		if a.settings.ShowWords || a.settings.ShowChars {
			words, chars := a.docStats()
			var parts []string
			if a.settings.ShowWords {
				parts = append(parts, strconv.Itoa(words)+" kelime")
			}
			if a.settings.ShowChars {
				parts = append(parts, strconv.Itoa(chars)+" harf")
			}
			ui.Text(c, strings.Join(parts, " · ")).FontSize(11.5).TextColor(t.TextMuted)
		}
		if !a.settings.HideSaved {
			st := ui.Text(c, strings.ToUpper(state[:1])+state[1:]).FontSize(11.5).TextColor(t.TextMuted)
			if a.saveErr != "" {
				st.TextColor(ryugo.From(c).Colors.Up)
			}
		}
		if !a.settings.HideCaret {
			ui.Text(c, caret).FontSize(11.5).TextColor(t.TextMuted).Label("Caret: " + caret + " (⌘⇧M)")
		}
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
	md := a.pageMarkdown(a.page, a.doc.Text())
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

// docStats are the open page's words and letters, counted again when the text changed.
func (a *app) docStats() (words, chars int) {
	if a.doc == nil {
		return 0, 0
	}
	if v := a.doc.Version(); v != a.statsVer || a.statChars == 0 {
		s := a.doc.Text().String()
		a.statWords, a.statChars, a.statsVer = len(strings.Fields(s)), len([]rune(strings.ReplaceAll(strings.ReplaceAll(s, "\n", ""), string(richtext.ObjectReplacement), ""))), v
	}
	return a.statWords, a.statChars
}
