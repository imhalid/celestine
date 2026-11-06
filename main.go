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
	"unicode/utf8"

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

> Her sayfa kitaplık klasöründe bir klasör ve içinde page.json: uygulamanın kendi biçimi.

⌘⇧M caret'i değiştirir: Slime ya da Mizan.
`

const (
	columnMax = 720
	pageTop   = 48
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

// titleCaretSpec is the caret in the page's title, as it was found: where it
// is, how wide, and what was just written.
type titleCaretSpec struct {
	on     bool
	r      ui.Rect
	cw     float32
	box    ui.Rect
	sel    bool
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
	version  uint64
	length   int
	caretAt  int
	wrote    int
	edited   bool
	erased   int // letters erased since the caret was drawn, which way (-1 Backspace, 1 Delete) and what they were (caretfx.go)
	eraseDir int
	gone     string
	prevText richtext.Text // the text before the last edit, for what was erased
	pacman   *pacman
	comet    *comet
	editAt   time.Time
	dirty    bool
	saveErr  string
	notice   string     // a toast to show
	trashed  *TrashItem // just thrown away: a toast offers it back

	trashOpen     bool       // the trash is shown, instead of a page
	quick         *quickNote // the panel for a note, over everything (quick.go)
	tasksOpen     bool       // the task view is shown (tasks.go)
	searchOpen    bool       // the search over the whole library is shown (search.go)
	searchQ       string
	searchFocus   bool
	quickKeyDraft string // the key being typed in the settings
	quickKeyErr   string
	tagRen        tagRename      // the dialog to rename a tag (tags.go)
	vhist         histView       // the dialog of a page's versions (historyui.go)
	noticePage    *Page          // the page the notice is about, to open from it
	taskDue       int            // 0 any day, 1 today, 2 this week, 3 late
	taskFilter    int            // 0 open, 1 done, 2 all
	taskList      ui.ListState   // the list of tasks, a row for each in view (tasks.go)
	taskQuery     string         // what is typed in the tasks' search
	taskColl      string         // the collection the tasks are of (its Dir), "" all of them
	taskFolded    map[*Page]bool // the pages whose tasks are folded away
	tag           string         // the tag (its key) whose pages are shown, instead of a page (tags.go)
	tagHits       []tagHit       // the tags drawn this frame; a click is against the last's
	tagSeen       []tagHit
	linkHits      []linkHit // the links drawn this frame, and the last frame's (links.go)
	linkSeen      []linkHit
	hoverTag      string
	coverHover    bool     // the pointer is in the cover band
	picHits       []picHit // the pictures drawn this frame, and the last frame's (images.go)
	picSeen       []picHit
	tableHits     []tableHit // the cells of the tables drawn this frame, and the last frame's (tables.go)
	tableSeen     []tableHit
	borderHits    []colBorder // the borders between columns, drawn and seen
	borderSeen    []colBorder
	hoverBorder   borderKey
	colDrag       colDragState
	cellEd        cellEdit
	cellW         map[string]float32 // how wide a cell's text is drawn, by its text (tables.go)
	win           *mygo.Window       // the window, for its own controls (windowcontrols.go)
	titleCaret    titleCaretSpec     // where the caret in the title is, to draw it (the title is an input)
	titleSeen     bool
	titleLen      int
	selPrevA      int // the selection before a pointer went down in the editor
	selPrevC      int
	lightbox      *picHit  // the picture shown large
	ptags         []string // the open page's tags, for the side panel
	ptagsVer      uint64
	ptagsDoc      *richtext.Document
	trashItems    []*TrashItem // what is in it, read when it opens and changes
	boxes         []richtext.ParagraphBox
	focused       bool
	clock         func() time.Time
	lastSave      string                  // the body as last written
	look          string                  // the light or dark and the color theme the window was last drawn in
	lastWords     int                     // the words it had, or -1 when not known
	style         richtext.ParagraphStyle // the caret paragraph's, last frame
	refocus       bool                    // the editor takes the keyboard focus in the next frame
	pageR         ui.Rect                 // the area the page is in, last frame: the editor reaches its left edge
	caretPara     int                     // where the caret was, last frame: its paragraph, its place in it, the paragraph's top
	caretRel      float32
	caretTop      float32
	sel           selBar          // the bar over a selection (selbar.go)
	edPressed     bool            // the pointer is down in the editor
	afterView     []func()        // what waits for the view to be built
	boardFocus    string          // the board column asked for, wide
	boardOpened   map[string]bool // empty columns the user opened
	boardScroll   ui.ScrollState  // the board scrolled sideways: where its edges fade (edgeFades)
	rowsLast      []treeEntry     // the tree's rows, last frame
	treeFocus     any             // the row the arrow keys moved to, to take the focus
	treeNav       bool            // the focus is kept in the tree as the page opens: the arrow keys are in use
	treeHeld      bool            // a row has the focus in this frame
	treeSigSeen   uint64
	cdrag         collDragState
	collRects     map[*Collection]ui.Rect
	ctx           ctxMenu        // the context menu (ctxmenu.go)
	pageScroll    ui.ScrollState // the page scrolled as a whole (pageView)
	scrollR       ui.Rect        // its room in the window, last frame
	barInset      float32        // the part of the top of the page the title bar lies over (the page scrolls under it)
	topSpace      float32        // what the page leaves above its title, in its scroll
	winScrolled   bool           // a huge page is scrolled in its window: its title and properties give way (trackWindow)
	winTop        float32        // where its first paragraph is, scrolled to the top
	winTrim       float32        // the room around the window of a huge page that the page does not have (it starts with a guess)
	winFlip       time.Time      // when its title last went or came back
	headH         float32        // the title's and properties' height, last frame
	titleFocused  bool
	edH           float32 // the editor's height, last frame
	frame         func()  // asks for a frame that builds the view
	revealWant    bool    // the caret moved: bring it into view
	statsVer      uint64  // the version of the text the counts are of
	statWords     int
	statChars     int
	settingsTab   int       // the settings' tab
	links         linkIndex // who links to whom (backlinks.go)
	linksH        float32   // the backlinks' height, last frame
	calDay        time.Time // the day open on the calendar, and whether it is
	calOpen       bool
	calGrid       ui.Rect
	calMonth      time.Time // the month the calendar view shows
	hoverCh       *chip     // the chip the pointer is over
	dateEdit      dateEdit  // the calendar a date chip opens
	editorR       ui.Rect   // the editor's box, from its last frame

	// The settings, and the MCP server they turn on (mcp.go). A frame and
	// an agent's tool each hold mu: the library is theirs one at a time.
	mu           sync.Mutex
	settings     Settings
	settingsFile string // "" in tests: not kept
	// settingsStore, when the library is a file of its own, keeps the settings
	// in it; settingsFile then is only where they were before.
	settingsStore *Store
	importNote    string                // what the last import said, in the settings
	embedCache    map[string][]string   // the first lines of the pages embedded (embed.go)
	ctxCache      map[string][]seg      // the lines that link to a page (backlinks.go)
	bulk          bulkState             // pages being chosen in a collection, to be done to all (bulk.go)
	chipW         map[string]float32    // how wide the text of each chip is (chips.go)
	autoCovers    map[uint64]*autoCover // the patterns made for the covers of the pages shown lately (autocover.go)
	tables        map[string]*tableUI   // the state of each collection table, by collection and view (views.go)
	statsOpen     bool                  // the dialog of what is written (writing.go)
	writingSaved  time.Time
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
	a := &app{lib: lib, docs: map[*Page]*richtext.Document{}, jelly: newSlime(), mizan: &mizanCaret{}, pacman: newPacman(), comet: newComet(), clock: time.Now}
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
	return p.SaveText(welcome)
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
			TitleBarStyle: mygo.TitleBarHiddenInset, TrafficLightPosition: &mygo.Point{X: 18, Y: 16},
			HideWindowButtons: ownWindowControls, Content: ui.View(a.view)})
		if ownWindowControls {
			a.win = w
		}
		a.invalidate = w.Invalidate
		w.SetIcon(appIcon)             // the title bar and taskbar (Linux, Windows)
		mygo.App.Dock.SetIcon(appIcon) // the Dock (macOS)
		a.quick = newQuick(a)
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
	a.coll, a.trashOpen, a.tag, a.tasksOpen, a.searchOpen = nil, false, "", false, false
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
		a.sweepPictures(p, body)
		doc = richtext.NewDocument(a.parsePage(p, body))
		if hugePage(doc.Text()) {
			doc.SetSelection(0, 0) // a document begins with its caret at the end: a huge page opens at its top
		}
		a.docs[p] = doc
	}
	a.page, a.doc, a.title = p, doc, p.Title
	a.find.ver = 0 // another text to find in
	_, caret := doc.Selection()
	a.version, a.length, a.caretAt = doc.Version(), doc.Text().Len(), caret
	a.prevText = doc.Text()
	a.dirty, a.saveErr, a.lastSave, a.lastWords = false, "", "", -1
	a.boxes = nil
	a.winScrolled, a.winTop, a.winFlip = false, 0, a.clock()
	a.pageScroll = ui.ScrollState{}
	// a page opened starts clean: the caret is put in its place, not flown
	// there from where it was in the last page, and what was laid out is gone
	a.jelly.placed, a.mizan.seen, a.mizan.glide = false, false, nil
	a.pacman.reset()
	a.comet.reset()
	a.editorR, a.caretPara, a.edH = ui.Rect{}, -1, 0
	a.linksStale()
	a.links.built = time.Time{} // asked for at once for the page opened
}

// newPage makes a page in c (under parent when not nil) and opens it, its title ready to be typed.
func (a *app) newPage(c *Collection, parent *Page) {
	if c.Dynamic() {
		a.notice = "Arama koleksiyonuna sayfa eklenmez: sayfalarını sorgusu bulur"
		return
	}
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
		if err := p.SaveText(expandTemplate(c.Template, p.Title, a.clock())); err != nil {
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
	a.chipSeen, a.chipHits = a.chipHits, a.chipSeen[:0] // the last frame's, for a click now
	mode := []ryugo.Mode{ryugo.ModeInherit, ryugo.Light, ryugo.Dark}[a.settings.Theme%3]
	radiusScale = float32(a.cornerPercent()) / 100
	tok := ryugo.Root(c, ryugo.Options{Mode: mode, Size: ryugo.Sm, Corners: a.ryugoCorners()}) // Ryugo's theme, its components' tokens, for the appearance
	if a.appTheme().colors(tok.Dark).bg != "" {                                                // and in the app's theme: the same appearance, in its colors
		tok = ryugo.Root(c, ryugo.Options{Mode: map[bool]ryugo.Mode{false: ryugo.Light, true: ryugo.Dark}[tok.Dark], Accent: a.accentOf(tok.Dark), Size: ryugo.Sm, Corners: a.ryugoCorners()})
		a.skin(tok, c.Theme())
	}
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
	// A new look (light or dark, or another color theme) is a new tree: what was there
	// would take each color over a moment of its own - the sidebar, the inputs, the
	// boxes of a checkbox, each at its own pace - and the change would show in pieces.
	look := strconv.FormatBool(tok.Dark) + ":" + strconv.Itoa(a.settings.AppTheme)
	if a.look != "" && a.look != look {
		a.refocus = true // the editor was rebuilt, and has the focus again
	}
	a.look = look
	c.Key("look:" + look)
	root := ui.Row(c).Fill().AlignItems(ui.Stretch).Background(a.pageBg)
	for _, f := range root.DroppedFiles() { // and those dropped elsewhere in the window
		a.dropFile(f, false)
	}
	if !a.menuBar { // in the app, the menu bar has these (menu.go)
		if c.Shortcut(ui.Cmd, ui.KeyBackslash) {
			a.toggleSidebar()
		}
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyBackslash) {
			a.togglePanel()
		}
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyM) {
			a.setCaret((a.caretKind + 1) % caretKinds)
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
		if c.Shortcut(ui.Cmd, ui.KeyK) {
			a.openPalette()
		}
		if c.Shortcut(ui.Cmd, ui.KeyP) {
			a.openCommands()
		}
		if c.Shortcut(ui.Cmd, ui.KeyJ) {
			a.openToday()
		}
		if c.Shortcut(ui.Cmd, ui.KeyF) {
			a.openFind()
		}
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyF) {
			a.openSearch(a.searchQ)
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
			// a page and a collection scroll as a whole, from the top of the window, under the title bar
			overBar := !a.trashOpen && a.tag == "" && !a.tasksOpen && !a.searchOpen && (a.coll != nil || a.page != nil)
			if !overBar {
				a.titleBar(c)
			}
			// the page scrolls over the whole width, its bar at the window's edge:
			// the margins are inside it
			padX := float32(32)
			if !a.trashOpen && a.tag == "" && !a.tasksOpen && !a.searchOpen && (a.coll != nil || a.page != nil) {
				padX = 0
			}
			topPad := float32(pageTop - barH + 8)
			if a.coverKind(a.page) != coverKindNone && !a.noPage() { // the cover starts under the bar
				topPad = 0
			}
			a.barInset, a.topSpace = 0, 0
			if overBar { // it scrolls from the top of the window, under the bar, and starts below it
				a.barInset, a.topSpace, topPad = barH, barH+topPad, 0
				if a.coverKind(a.page) != coverKindNone && !a.noPage() { // a cover begins at the very top, behind the bar
					a.topSpace = 0
				}
			}
			pageRow := ui.Row(c).Grow(1).Justify(ui.Center).Padding(topPad, padX, 0, padX)
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
				if a.searchOpen {
					a.focused = false
					a.searchView(c)
					return
				}
				if a.coll != nil {
					a.focused = false
					a.collectionView(c)
					return
				}
				if a.page == nil {
					a.emptyState(c, iconPage, "Bir sayfa seç", "Soldaki ağaçtan bir sayfa aç, koleksiyon satırındaki + ile de yenisini yaz.", "", nil)
					return
				}
				a.pageView(c, th)
			})
			a.status(c)
			a.keepTree()
			if overBar {
				a.topBar(c)
			}
		})
		a.sidePanel(c)
		a.sidebarButton(c)
		a.windowControls(c)
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
	a.tagRenameView(c)
	a.historyView(c)
	a.writingView(c)
	if a.modal() || a.ctx.open { // under a dialog or a menu the cell is let go
		a.cellEd.open = false
	}
	a.cellEditView(c)
	a.lightboxView(c)
	if !a.treeHeld { // the focus is not in the tree: no more arrow keys
		a.treeNav = false
	}
	a.treeHeld = false
	a.runLater()
	ryugo.Toaster(c)
	if a.notice != "" {
		if pg := a.noticePage; pg != nil { // a note put away: the page it went to is one click away
			c.ToastAction(a.notice, "Aç", func() { a.open(pg) })
		} else {
			c.Toast(a.notice)
		}
		a.notice, a.noticePage = "", nil
	}
	if item := a.trashed; item != nil {
		a.trashed = nil
		c.ToastAction("“"+item.Title+"” çöpe taşındı", "Geri al", func() { a.restore(item) })
		a.loadTrash()
	}
	if a.doc != nil && a.coll == nil {
		a.boxes = a.doc.VisibleParagraphs()
		a.trackWindow()
		a.autosave(c)
	}
}

// bg is the page's background: Ryugo's, but in the dark not its black -
// its card's dark gray, a step below the sidebar.
func (a *app) bg(c *ui.Context) ui.Color {
	if t := ryugo.From(c); t.Dark && a.appTheme().colors(true).bg == "" { // Ryugo's own dark: the page is its card
		return t.Colors.Card
	}
	return c.Theme().Background // a theme's page is its own color
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
			if a.barInset > 0 { // the bar lies over the top: the find bar is below it
				ui.Box(c.Key("find-top")).FillWidth().Height(a.barInset)
			}
			ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, 32).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(a.pageWidth()).Children(func() { a.findBar(c) })
			})
		}
		sc := ui.Scroll(c.Key("page-scroll:" + p.Dir)).TrackScroll(&a.pageScroll).Grow(1)
		sc.Children(func() {
			if a.topSpace > 0 {
				ui.Box(c.Key("page-top")).FillWidth().Height(a.topSpace)
			}
			// a cover is a band behind the top of the page, over its whole width
			wrap := ui.Column(c.Key("page-wrap")).FillWidth()
			if _, y, over := wrap.PointerPosition(); over && a.coverKind(p) != coverKindNone {
				a.coverHover = y < a.coverBandOf(p)
			} else {
				a.coverHover = false
			}
			a.paintCover(c, p, wrap)
			wrap.Children(func() {
				a.coverButtons(c, p)
				ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, 32).Children(func() {
					ui.Column(c).FillWidth().MaxWidth(a.pageWidth()).Gap(4).Children(func() { a.pageBody(c, p, th) })
				})
			})
		})
		a.scrollR = sc.LayoutBounds()
	})
}

// pageBody is the title, the properties and the text.
func (a *app) pageBody(c *ui.Context, p *Page, th *richtext.Theme) {
	window := hugePage(a.doc.Text())
	if window && a.winScrolled { // scrolled in its window: the title and the properties are gone with what scrolled
		a.headH = 0
	} else {
		head := ui.Column(c.Key("page-head")).FillWidth()
		if window {
			head.Transition(branch).Clip()
		}
		head.Children(func() { a.pageHead(c, p) })
		a.headH = head.LayoutBounds().H
	}
	c.Key("body:" + p.Dir)
	ed := richtext.EditorBase(c, a.doc, richtext.EditorOptions{
		Label: "Sayfa", Font: a.font, Theme: th,
		Placeholder: "Yaz…  # başlık · - liste · [] görev · > alıntı",
	}).FillWidth().Height(a.editorH(c))
	if window {
		ed.Transition(windowEditor)
	}
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
	a.linkSeen, a.linkHits = a.linkHits, a.linkSeen[:0]
	a.picSeen, a.picHits = a.picHits, a.picSeen[:0]
	if x, y, over := ed.PointerPosition(); over {
		if key, ok := a.tagAt(a.editorR.X+x, a.editorR.Y+y); ok {
			a.hoverTag = key
			ed.Cursor(ui.CursorPointer)
		}
		if a.tableCellAt(a.editorR.X+x, a.editorR.Y+y) != nil {
			ed.Cursor(ui.CursorPointer)
		}
		if _, ok := a.linkAt(a.editorR.X+x, a.editorR.Y+y); ok {
			ed.Cursor(ui.CursorPointer)
		}
		if a.pictureAt(a.editorR.X+x, a.editorR.Y+y) != nil {
			ed.Cursor(ui.CursorPointer)
		}
		if h := a.chipAt(a.editorR.X+x, a.editorR.Y+y); h != nil {
			a.hoverCh = h.ch
			ed.Cursor(ui.CursorPointer)
		}
	}
	for _, f := range ed.DroppedFiles() { // files dropped on the text
		a.dropFile(f, true)
	}
	a.dragColumns(ed)
	if ed.Clicked() { // on a tag: its pages; on a chip: its page, or its day
		x, y, _ := ed.PointerPosition()
		if h := a.tableCellAt(a.editorR.X+x, a.editorR.Y+y); h != nil {
			a.doc.SetSelection(a.selPrevA, a.selPrevC)
			a.clickCell(h)
		} else if key, ok := a.tagAt(a.editorR.X+x, a.editorR.Y+y); ok {
			a.openTag(key)
		} else if url, ok := a.linkAt(a.editorR.X+x, a.editorR.Y+y); ok {
			a.openLink(url)
		} else if pic := a.pictureAt(a.editorR.X+x, a.editorR.Y+y); pic != nil {
			a.lightbox = pic
		} else {
			a.clickChip(a.editorR.X+x, a.editorR.Y+y)
		}
	}
	if !a.edPressed { // where the caret is when the pointer is not down: the click's own move is not kept
		a.selPrevA, a.selPrevC = a.doc.Selection()
	}
	ed.DrawOver(func(p *ui.Painter, r ui.Rect) {
		a.editorR = r
		a.drawMatches(p, r)
		a.drawBlockDrop(p, r)
		a.drawDividers(p, r)
		a.drawTags(p, r)
		a.sealTables()
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
			if tc := a.titleCaret; tc.on {
				a.paintCaret(p, tc.r, tc.cw, tc.box, kick{wrote: tc.wrote, edited: tc.edited}, tc.sel)
			}
		})
	})
}

// pageHead is the page's title and its properties.
func (a *app) pageHead(c *ui.Context, p *Page) {
	ink := c.Theme().Text
	if a.coverKind(p) != coverKindNone { // the title lies on the cover, as the band's bottom is dark
		ui.Box(c.Key("cover-room")).FillWidth().Height(a.coverRoomOf(p))
		ink = ui.Hex("#ffffff")
		if a.coverKind(p) == coverKindAuto && !ryugo.From(c).Dark { // a pale mosaic: the text's own ink
			ink = c.Theme().Text
		}
	}
	c.Key("title:" + p.Dir)
	if p.Coll.Journal { // a day's title is the journal's format, not typed
		c.Key("title:" + p.Dir)
		ui.Text(c, p.Title).FontSize(32).FontWeight(720).Padding(6, 0, 10, 0).Label("Başlık").Selectable().TextColor(ink)
		a.titleFocused = false
		a.coverGap(c, p)
		a.propsSection(c, p)
		return
	}
	title := ui.TextInputBase(c, &a.title).Label("Başlık").FontSize(32).FontWeight(720).Padding(6, 0, 10, 0).TextColor(ink).CaretWidth(0)
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
	a.titleCaret = titleCaretSpec{}
	if b := title.LayoutBounds(); title.Focused() && b.W > 0 { // the caret is ours, as the page's
		anchor, caret := title.TextAnchorCaret()
		cr, cw := a.inputCaret(a.title, caret, 32, 720, b.X, b.Y+6, b.H-16)
		n := utf8.RuneCountInString(a.title)
		spec := titleCaretSpec{on: true, r: cr, cw: cw, box: b, sel: anchor != caret}
		if a.titleSeen && n != a.titleLen {
			spec.edited, spec.wrote = true, max(n-a.titleLen, 0)
		}
		a.titleSeen, a.titleLen = true, n
		a.titleCaret = spec
	} else if !title.Focused() {
		a.titleSeen = false
	}
	a.coverGap(c, p)
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
	least := max(a.scrollR.H-a.topSpace-a.headH-a.linksH-8, 160)
	if hugePage(a.doc.Text()) {
		// a window on the text, which scrolls in it (see hugePageRunes); the room
		// around it (gaps, paddings) is learnt from how far the page around it
		// still scrolls, so that it does not: a second bar
		if a.winTrim == 0 {
			a.winTrim = 40 // a guess, learnt below
		}
		if a.pageScroll.MaxY > 0.5 && a.clock().Sub(a.winFlip) > time.Second { // not while the title moves
			a.winTrim += a.pageScroll.MaxY
		}
		return max(least-a.winTrim, 160)
	}
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
	// pasted text with [[links]] in it: they are chips, as if typed (a link copied from
	// the page's menu is such text)
	if grew := n - a.length; grew > 1 && caret-a.caretAt == grew && a.chipsInPasted(caret-grew, caret) {
		_, caret = a.doc.Selection()
		text, v = a.doc.Text(), a.doc.Version()
		style = text.ParagraphStyleAt(caret)
		n = text.Len()
		a.length, a.caretAt = n, caret
	}
	switch grew := n - a.length; {
	case grew > 0 && caret-a.caretAt == grew && grew <= 8:
		a.wrote += grew
	case grew < 0 && grew >= -48 && caret == a.caretAt+grew: // Backspace: what was behind the caret
		a.eraseOf(a.prevText, caret, a.caretAt, -1)
		a.edited = true
	case grew < 0 && grew >= -48 && caret == a.caretAt: // Delete: what was ahead
		a.eraseOf(a.prevText, caret, caret-grew, 1)
		a.edited = true
	default:
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
	a.prevText = text
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
		a.pacman.shift(top - a.caretTop)
		a.comet.shift(top - a.caretTop)
	}
	a.caretPara, a.caretRel, a.caretTop = cb.para, cb.r.Y-cb.top, top
	cr := cb.r
	cr.X += r.X
	cr.Y += r.Y
	a.caretWin = cr
	a.revealCaret(cr)
	anchor, caret := a.doc.Selection()
	if anchor != caret { // over a selection, a plain caret: the jelly would hide what is selected
		a.paintCaret(p, cr, cb.cw, r, kick{}, true)
		return
	}
	k := kick{wrote: a.wrote, edited: a.edited, erased: a.erased, dir: a.eraseDir, gone: a.gone}
	a.wrote, a.edited, a.erased, a.gone = 0, false, 0, ""
	a.paintCaret(p, cr, cb.cw, r, k, false)
}

// paintCaret draws the caret cr, in the editor's box bounds (or an input's):
// the slime or the mizan, as chosen, or a plain bar over a selection. wrote
// is how many letters were just written, edited whether the text was changed.
func (a *app) paintCaret(p *ui.Painter, cr ui.Rect, cw float32, bounds ui.Rect, k kick, selecting bool) {
	dark := p.Theme().Dark
	if selecting {
		p.Fill(ui.Rect{X: cr.X - 1, Y: cr.Y, W: 2, H: cr.H}, p.Theme().Accent, rad(1))
		return
	}
	f := caretFrame{r: cr, cw: cw, dark: dark, bounds: bounds, wrote: k.wrote, edited: k.edited}
	// not clipped to the editor's box: at the start of a line the caret
	// goes past its left edge
	room := a.scrollR
	if room.H == 0 {
		room = ui.Rect{X: bounds.X - 24, Y: bounds.Y - 12, W: bounds.W + 48, H: bounds.H + 24}
	}
	p.Clip(room, 0, func() {
		switch a.caretKind {
		case caretMizan:
			a.mizan.draw(p, f)
			return
		case caretPacman:
			a.pacman.draw(p, f, k)
			return
		case caretComet:
			a.comet.draw(p, f, k)
			return
		}
		col := ui.Hex("#59f939")
		if !dark {
			col = ui.Hex("#2f9a1c")
		}
		a.jelly.draw(p, cr, cw, col, dark)
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
	ui.Box(c.Key("status-line")).FillWidth().Height(1).Background(ryugo.From(c).Colors.Hairline) // the page ends here
	ui.Row(c).Padding(3, 16, 3, 10).Justify(ui.End).AlignItems(ui.Center).Gap(12).Children(func() {
		if a.barButton(c, iconSettings, "Ayarlar") { // at the left end: the rest is pushed to the right
			a.settingsOpen = true
		}
		ui.Box(c.Key("status-gap")).Grow(1)
		// the MCP server on: a dot of the accent, which opens the settings
		if a.mcpServer != nil {
			m := ui.Row(c.Key("mcp-status")).Gap(5).Padding(2, 6).Radius(rad(6)).AlignItems(ui.Center).
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
		a.statusExtras(c)
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
		a.themeButton(c)
		a.pageButtons(c)
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
	a.rollRecurring()
	body := encodeBody(a.doc.Text())
	if body == a.lastSave {
		a.dirty = false
		return
	}
	if err := a.page.Save(body); err != nil {
		a.saveErr = err.Error()
		return
	}
	words := len(strings.Fields(a.doc.Text().PlainText()))
	a.countWriting(a.lastWords, words)
	a.lastSave, a.lastWords, a.dirty = body, words, false
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
