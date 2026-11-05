// ryudit: a block editor on MyGo's rich text editor, over a library of
// collections and pages kept as plain Markdown files (library.go). The
// Markdown you type turns into structure as you type it and is never seen.
// The caret is ours: Slime or Mizan.
//
// Run with: go run . [library folder]   (default: ./kitaplik)
package main

import (
	_ "embed"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

	trashOpen    bool     // the trash is shown, instead of a page
	tasksOpen    bool     // the task view is shown (tasks.go)
	taskFilter   int      // 0 open, 1 done, 2 all
	tag          string   // the tag (its key) whose pages are shown, instead of a page (tags.go)
	tagHits      []tagHit // the tags drawn this frame; a click is against the last's
	tagSeen      []tagHit
	hoverTag     string
	ptags        []string // the open page's tags, for the side panel
	ptagsVer     uint64
	ptagsDoc     *richtext.Document
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
	sel          selBar          // the bar over a selection (selbar.go)
	edPressed    bool            // the pointer is down in the editor
	afterView    []func()        // what waits for the view to be built
	boardFocus   string          // the board column asked for, wide
	boardOpened  map[string]bool // empty columns the user opened
	rowsLast     []treeEntry     // the tree's rows, last frame
	treeFocus    any             // the row the arrow keys moved to, to take the focus
	treeNav      bool            // the focus is kept in the tree as the page opens: the arrow keys are in use
	treeHeld     bool            // a row has the focus in this frame
	treeSigSeen  uint64
	cdrag        collDragState
	collRects    map[*Collection]ui.Rect
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
	settingsTab  int       // the settings' tab
	links        linkIndex // who links to whom (backlinks.go)
	linksH       float32   // the backlinks' height, last frame
	calDay       time.Time // the day open on the calendar, and whether it is
	calOpen      bool
	calGrid      ui.Rect
	calMonth     time.Time // the month the calendar view shows
	hoverCh      *chip     // the chip the pointer is over
	dateEdit     dateEdit  // the calendar a date chip opens
	editorR      ui.Rect   // the editor's box, from its last frame

	// The settings, and the MCP server they turn on (mcp.go). A frame and
	// an agent's tool each hold mu: the library is theirs one at a time.
	mu           sync.Mutex
	settings     Settings
	settingsFile string // "" in tests: not kept
	// settingsStore, when the library is a file of its own, keeps the settings
	// in it; settingsFile then is only where they were before.
	settingsStore *Store
	exportNote    string                    // what the last export said, in the settings
	images        map[string]ui.ImageSource // the pictures read, by their file (images.go)
	clipText      func(string)              // the clipboard to write; the system's when nil
	clipImage     func() []byte             // the clipboard's picture; the system's when nil
	outlineOf     []heading                 // the page's headings, for the side panel
	outlineVer    uint64
	outlineDoc    *richtext.Document
	imgOnClip     atomic.Bool // a picture, and no text, is on the clipboard
	settingsOpen  bool
	mcpServer     *http.Server
	mcpErr        string

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
	a.boardOpened = map[string]bool{}
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

// appIcon is the app's icon, for its window and its Dock (assets/).
//
//go:embed assets/icon-512.png
var appIcon []byte

func main() {
	// The library is a file of its own, next to the program: nothing else
	// opens it, and the two move together. A folder named on the command line
	// is a library of plain files, as before; the first time, a folder
	// library found here is taken into the file.
	var (
		lib   *Library
		store *Store
		err   error
	)
	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}
	if info, serr := os.Stat(arg); arg != "" && serr == nil && info.IsDir() {
		root, _ := filepath.Abs(arg)
		lib, err = OpenLibrary(root)
	} else {
		file := libraryFile()
		if arg != "" {
			file, _ = filepath.Abs(arg)
		}
		_, existed := os.Stat(file)
		if store, err = OpenStore(file); err == nil {
			if existed != nil {
				for _, old := range []string{filepath.Join(filepath.Dir(file), "kitaplik"), "kitaplik"} {
					if isLibraryFolder(old) {
						store.db.Exec(`PRAGMA synchronous = OFF`) // a copy of what is still there: no wait for each file
						err = copyTree(osFS{}, old, store, storeRoot)
						store.db.Exec(`PRAGMA synchronous = FULL`)
						if err != nil {
							log.Fatal(err)
						}
						break
					}
				}
			}
			lib, err = OpenLibraryIn(store, storeRoot)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
	if err := seed(lib); err != nil {
		log.Fatal(err)
	}
	a := newApp(lib)
	if _, err := a.journal(); err != nil { // every library has its journal
		log.Fatal(err)
	}
	if dir, err := os.UserConfigDir(); err == nil { // the settings kept, not a test's
		a.settingsFile, a.settings = filepath.Join(dir, "ryudit", "settings.json"), Settings{}
	}
	if store != nil {
		a.settingsStore = store
	}
	if a.settingsFile != "" || a.settingsStore != nil {
		a.loadSettings()
		a.applyTree()
	}
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(a.menu())
		a.menuBar = true
		w := mygo.NewWindow(mygo.WindowOptions{Title: "ryudit", Width: 1180, Height: 760, MinWidth: 640, MinHeight: 420,
			TitleBarStyle: mygo.TitleBarHiddenInset, TrafficLightPosition: &mygo.Point{X: 18, Y: 16}, Content: ui.View(a.view)})
		a.invalidate = w.Invalidate
		w.SetIcon(appIcon)             // the title bar and taskbar (Linux, Windows)
		mygo.App.Dock.SetIcon(appIcon) // the Dock (macOS)
		go a.watch()
		go a.watchClipboard()
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
	a.coll, a.trashOpen, a.tag, a.tasksOpen = nil, false, "", false
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
	// a page opened starts clean: the caret is put in its place, not flown
	// there from where it was in the last page, and what was laid out is gone
	a.jelly.placed, a.mizan.seen, a.mizan.glide = false, false, nil
	a.editorR, a.caretPara, a.edH = ui.Rect{}, -1, 0
	a.linksStale()
	a.links.built = time.Time{} // asked for at once for the page opened
}

// newPage makes a page in c (under parent when not nil) and opens it, its title ready to be typed.
func (a *app) newPage(c *Collection, parent *Page) {
	if c.Journal { // the journal gets today's page and no other
		a.openToday()
		return
	}
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
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyBackslash) {
			a.togglePanel()
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
		if c.Shortcut(ui.Cmd, ui.KeyJ) {
			a.openToday()
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
	// ⌘V with a picture alone on the clipboard (watchClipboard): the editor
	// pastes text, and nothing of a picture, so the key is ours then
	if a.imgOnClip.Load() && c.Shortcut(ui.Cmd, ui.KeyV) && a.page != nil && a.doc != nil {
		a.pasteImage()
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
			if !a.trashOpen && a.tag == "" && !a.tasksOpen && (a.coll != nil || a.page != nil) {
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
				if a.tag != "" {
					a.focused = false
					a.tagView(c)
					return
				}
				if a.tasksOpen {
					a.focused = false
					a.tasksView(c)
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
			a.keepTree()
		})
		a.sidePanel(c)
		a.sidebarButton(c)
		a.pageMoreButton(c)
	})
	a.collDragView(c)
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
	if !a.treeHeld { // the focus is not in the tree: no more arrow keys
		a.treeNav = false
	}
	a.treeHeld = false
	a.runLater()
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
	if !a.titleFocused && !a.fresh && !a.treeNav {
		ed.AutoFocus()
	}
	if a.refocus {
		ed.Focus()
		a.refocus = false
	}
	a.focused = ed.Focused()
	a.edPressed = ed.Pressed()
	a.trackGrip(ed)
	a.hoverCh, a.hoverTag = nil, ""
	a.tagSeen, a.tagHits = a.tagHits, a.tagSeen[:0] // this frame's are drawn after
	if x, y, over := ed.PointerPosition(); over {
		if key, ok := a.tagAt(a.editorR.X+x, a.editorR.Y+y); ok {
			a.hoverTag = key
			ed.Cursor(ui.CursorPointer)
		}
		if h := a.chipAt(a.editorR.X+x, a.editorR.Y+y); h != nil {
			a.hoverCh = h.ch
			ed.Cursor(ui.CursorPointer)
		}
	}
	for _, f := range ed.DroppedFiles() { // pictures dropped on the text
		a.addImageFile(f)
	}
	if ed.Clicked() { // on a tag: its pages; on a chip: its page, or its day
		x, y, _ := ed.PointerPosition()
		if key, ok := a.tagAt(a.editorR.X+x, a.editorR.Y+y); ok {
			a.openTag(key)
		} else {
			a.clickChip(a.editorR.X+x, a.editorR.Y+y)
		}
	}
	ed.DrawOver(func(p *ui.Painter, r ui.Rect) {
		a.editorR = r
		a.drawMatches(p, r)
		a.drawBlockDrop(p, r)
		a.drawDividers(p, r)
		a.drawTags(p, r)
	})
	a.backlinksView(c, p)
}

// caretView draws the caret in a layer over the window - the editor clips
// what it draws at its own edge, which cut the caret at the start of a
// line - itself clipped to the page's room.
func (a *app) caretView(c *ui.Context) {
	if a.doc == nil || a.noPage() || a.page == nil {
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
	if p.Coll.Journal { // a day's title is the journal's format, not typed
		c.Key("title:" + p.Dir)
		ui.Text(c, p.Title).FontSize(32).FontWeight(720).Padding(6, 0, 10, 0).Label("Başlık").Selectable()
		a.titleFocused = false
		a.propsSection(c, p)
		return
	}
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
	least := max(a.scrollR.H-a.headH-a.linksH-8, 160)
	n := len(a.boxes)
	if n == 0 {
		// nothing laid out yet (a page just opened): room for what the text
		// should take, more than it will, so the editor does not scroll
		// itself to the caret and then slide as it grows
		t := a.doc.Text()
		paras := strings.Count(t.String(), "\n") + 1
		line := a.fontSize() * a.lineHeight()
		return min(max(least, (float32(paras)+float32(t.Len())/55)*line*1.5+240), 80000)
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
	ui.Row(c).Padding(4, 16).Justify(ui.End).AlignItems(ui.Center).Gap(12).Children(func() {
		// the MCP server on: a dot of the accent, which opens the settings
		if a.mcpServer != nil {
			m := ui.Row(c.Key("mcp-status")).Gap(5).Padding(2, 6).Radius(6).AlignItems(ui.Center).
				Label("MCP açık: ayarlar").Role(ui.RoleButton)
			ryugo.Tooltip(c, m, "MCP sunucusu açık · "+a.mcpURL())
			if m.Hovered() {
				m.Background(t.SurfaceHover)
			}
			m.Children(func() {
				a.pulseDot(c)
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

// pulseDot is the light that says the MCP server is up: a green drop - a
// gradient, lit from above, ringed in a thin line of another green - with a
// wave going out of it and fading, again and again, as a heartbeat.
func (a *app) pulseDot(c *ui.Context) {
	cl := ryugo.From(c).Colors
	base := cl.Down
	host := ui.Box(c.Key("mcp-pulse")).Size(18, 18).Shrink(0)
	beat := host.Keyframes("beat", []ui.Keyframe{{At: 0, Value: 0}, {At: 1800 * time.Millisecond, Value: 1}},
		ui.KeyframeOptions{Repeat: ui.RepeatForever})
	// a second wave, half a beat behind the first
	late := host.Keyframes("beat2", []ui.Keyframe{{At: 0, Value: 0}, {At: 900 * time.Millisecond, Value: 0}, {At: 2700 * time.Millisecond, Value: 1}},
		ui.KeyframeOptions{Repeat: ui.RepeatForever})
	host.Center().Children(func() {
		for i, v := range []float32{beat, late} {
			ui.Box(c.Key("wave"+strconv.Itoa(i))).Absolute().Top(4).Left(4).Size(10, 10).Radius(5).
				Background(base.Alpha(0.5*(1-v)*(1-v))).Scale(1+1.4*v, 1+1.4*v)
		}
		breath := 1 + 0.07*float32(math.Sin(float64(beat)*2*math.Pi))
		ui.Box(c.Key("drop")).Size(10, 10).Radius(5).
			LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: ryugo.Mix(base, ui.Hex("#ffffff"), 0.55), To: ryugo.Mix(base, ui.Hex("#000000"), 0.1)}).
			Border(1, ryugo.Mix(base, ui.Hex("#d6ffe0"), 0.75)).
			InsetShadow(0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.35)).
			Scale(breath, breath)
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
	a.linksStale()
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
