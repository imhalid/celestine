// celestine: a block editor on MyGo's rich text editor, over a library of
// collections and pages kept as plain Markdown files (library.go). The
// Markdown you type turns into structure as you type it and is never seen.
// The caret is ours: Slime or Mizan.
//
// Run with: go run . [library folder]   (default: ./kitaplik)
package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
	"github.com/imhalid/ryu-go/dockwin"
)

const welcome = `Here you **don't see Markdown's marks**: what you type turns into structure as you type it.

## Try it

- At the start of a line, \# and a space: a heading. \- a list, 1. a numbered list, \[\] a task, \> a quote.
- Put words between two stars: **bold**. One star _italic_, backticks ` + "`code`" + `, two tildes ~~struck out~~.
- Three dashes on an empty line: a divider. Three backticks: a code block.
- ⌘B, ⌘I, ⌘U format; ⌘Z undoes a change.
  - Tab in, ⇧Tab out.

---

## Pages

- The + beside a collection on the left makes a page; the + beside a page makes a sub-page under it.
- ⌃N makes a page in this collection.
- [ ] Click the box
- [x] A task that is done

> Each page is a folder in the library, with page.json in it: the app's own format.

⌘⇧M changes the caret: Slime or Mizan.
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
	bg     ui.Color // the page's color, behind the caret
	bounds ui.Rect
	wrote  int
	edited bool
	jelly  *ui.Path                // the caret's body when the jelly stretches it; nil: the plain box
	size   float32                 // the size of the letters on the caret's line; 0: not known
	style  caretStyle              // the text under the caret
	near   func(n int) []nearGlyph // the letters of its line within n of it; nil where there are none (a field, a demo)
	words  caretText               // the text round it: the word just written, its paragraph's words, where a word is; nil as near
	rest   time.Duration           // how long it has stood still, nothing written (idleFrame)
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
	lib *Library

	paneState         // what the pane in use shows and keeps (panes.go): its page or screen, its editor, its scroll
	ps        paneSet // the panes of the window (panes.go)

	bars       map[string]*funBar      // the scroll bars of the scroll areas, by name (funbar.go)
	titles     titleIdx                // the pages by their titles, for the chips (chips.go)
	nearFn     func(n int) []nearGlyph // the letters near the text's caret, while it is being painted (caret.go)
	wordsFn    caretText               // and the words round it
	caretTry   caretTry                // the caret's playground in the settings (caretplay.go)
	census     census                  // the sidebar's tree, counted once a frame (sidebar.go)
	frameSeq   uint64                  // the frames built, counted
	meter      frameMeter              // what CELESTINE_FRAMES counts (framemeter.go)
	caretDraws int                     // the caret's paints, counted: a window at rest has none (TestIdleAsksNoFrames)

	forcePhone bool  // a phone's layout in any window (the tests); a narrow browser window has it anyway (mobile.go)
	drawerOpen bool  // on a phone, the sidebar is open over the page
	drawerFrom place // what was shown when the drawer was looked at last: something else opened closes it

	treeVirtual       bool          // the sidebar's tree builds only the rows near what it shows (sidebar.go)
	treeTop, treeView float32       // where the tree is scrolled, and how tall its view is, last frame
	dockHost          *dockwin.Host // the windows of the panes torn off (panewin.go)

	docs map[*Page]*richtext.Document

	sidebarClosed bool

	drag  dragState   // a page being dragged in the tree
	rows  []treeEntry // the tree's rows this frame, top to bottom
	treeR ui.Rect     // the tree's box, from its last frame

	renaming      any // the page or collection whose row is a field, being renamed
	renameText    string
	renameFocused bool    // the field has had the focus: losing it keeps the name
	sidebarShown  float32 // its width this frame, on the way to open or closed

	theme *richtext.Theme // the editor's, resolved: what locate lays paragraphs out with
	font  ui.Font

	caretKind caretKind
	jelly     *slime
	mizan     *mizanCaret

	// Updates (update.go), shortcuts and macros of one's own, and the settings' search.
	updKey      string      // a test's key for releases
	updBusy     atomic.Bool // a check or a download goes on
	updErr      string      // what the last check said, when it failed
	updState    updateState // where an update the user asked for is
	updRel      *Release    // the newer version found
	updAuto     bool        // a check found a release and the settings say to put it in place
	updNotes    bool        // the dialog shows the notes of the version installed, not a new one
	updOpen     bool        // the dialog of it
	keyCmd      string      // the command and the key being typed for a shortcut of one's own
	keyAccel    string
	macroName   string
	macroList   string
	setSearch   string             // the search of the settings
	setMode     string             // the settings' special page: "changed", "profiles", "file"
	profName    string             // the name being typed for a profile
	charW       map[string]float32 // the width of n zeros in a font, for the line length (editing.go)
	sy          syncUI             // the state of synchronization (syncapp.go)
	ac          acctUI             // the account: signing in, its devices (account.go)
	vimReg      richtext.Text      // what dd and yy took, for p
	kb          kbState            // the keyboard's normal mode, the letters on what can be pressed (kb.go)
	winW, winH  float32            // the window's size, last frame
	tourAt      int                // the step of the tour
	tourOff     bool               // the tour is put off for this session
	wel         welcomeState       // the first opening (welcome.go)
	winOnce     sync.Once          // the window coming forward and going behind, in order (windowEvent)
	winEv       chan bool
	codeTheme   *editor.Theme // the colors of code, in the window's theme (codecolors.go)
	codeLangs   codeLangs     // the languages of the page's lines of code, by paragraph (codecolors.go)
	blink       caretBlink    // when the caret last moved, for it to blink once still (caretBlinkOff)
	bsel        blockSel      // blocks chosen from the keyboard (blocksel.go)
	toasts      toastState    // the toasts up (toast.go)
	statCache   libStats      // the library's figures, read at most every ten seconds
	statCacheAt time.Time
	sx          *searchIndex        // the text of every page, for the palette and the search (searchindex.go)
	px          placeIdx            // the palette's places, folded once (palette.go)
	edge        edgeDrag            // the sidebar or the side panel being dragged wider or narrower
	statLoading bool                // the first frame of the stats screen is drawn before the figures are read
	setClock    func(time.Duration) // a test moves the clock
	statReset   bool                // the reset asked once: the next click does it
	usage       *usageData          // what is used, by the day (stats.go)
	usageDirty  bool
	usageSaved  time.Time
	activeAt    time.Time     // the last thing done: a minute after it, the time is not counted
	pageFrac    time.Duration // the open page's time gathered, not yet a whole second (statTick)
	pageFracOf  string        // the page it is of
	activeFrac  time.Duration
	statLast    time.Time
	statVer     uint64
	sess        session // the writing session that goes on
	sessLast    time.Time
	backupDir   string // where the backups go, when not the config folder's (a test's)
	backupBusy  atomic.Bool
	crashDir    string // where the logs go, when not the config folder's (a test's)
	crashMsg    string // the panic last written, and when: the same one is not written over and over
	crashAt     time.Time
	crashPath   string                   // its log
	crashText   string                   // what it said
	testPanic   string                   // a test sets it to have a frame panic
	crashOpen   bool                     // the dialog of a crash is shown
	asking      askState                 // a question before an action that cannot be taken back (confirm.go)
	crashPrev   string                   // the last session ended badly: its log, or ""
	fonts       fontState                // the typefaces installed, and the lists of them (fonts.go)
	fx          []caretFX                // the carets of the presets, made as they are used (caretfx.go)
	jellied     smear                    // the jelly of the settings, put on a caret (caretfx_look.go)
	sleep       sleeper                  // the caret asleep after five seconds (caretextras.go)
	demos       map[caretKind]*caretDemo // the tiles of the carets in the settings, each at its own game
	cursor      cursorState              // the effect of the mouse pointer (cursorfx.go)
	winRect     ui.Rect                  // the window, as the caret layer was last made: the torch darkens all of it
	saveErr     string
	saveTried   time.Time      // when the open page's save last failed: autosave tries again a while after
	unsaved     map[*Page]bool // pages left with a save that failed: their texts are kept, and saved again (retryUnsaved)
	unsavedAt   time.Time
	daysStamped bool       // the journal's pages have their days kept (stampDays)
	storeErr    string     // the library store's last write error, as shown
	notice      string     // a toast to show
	trashed     *TrashItem // just thrown away: a toast offers it back

	quick         *quickNote // the panel for a note, over everything (quick.go)
	quickKeyDraft string     // the key being typed in the settings
	quickKeyErr   string
	tagRen        tagRename   // the dialog to rename a tag (tags.go)
	vhist         histView    // the dialog of a page's versions (historyui.go)
	tm            timeMachine // the page's history as a stack and a timeline (timemachine.go)
	tmSide        tmSideState // what the timeline in the sidebar shows, while it is there and as it fades (tmside.go)
	lapse         lapseState  // the time-lapse of the page, in place of its editor (lapseui.go)
	noticePage    *Page       // the page the notice is about, to open from it
	noticeUndo    func()      // takes back what the notice says was done: the notice has a Geri al
	noticeTone    *toastKind  // the notice's kind, when its words do not tell it (noticeKind)
	tagHits       []tagHit    // the tags drawn this frame; a click is against the last's
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
	cellW         map[string]float32 // how wide a cell's text is drawn, by its text (tables.go)
	win           *mygo.Window       // the window, for its own controls (windowcontrols.go)
	lightbox      *picHit            // the picture shown large
	ptags         []string           // the open page's tags, for the side panel
	ptagsVer      uint64
	ptagsDoc      *richtext.Document
	trashItems    []*TrashItem // what is in it, read when it opens and changes
	clock         func() time.Time
	look          string          // the light or dark and the color theme the window was last drawn in
	afterView     []func()        // what waits for the view to be built
	boardOpened   map[string]bool // empty columns the user opened
	rowsLast      []treeEntry     // the tree's rows, last frame
	treeFocus     any             // the row the arrow keys moved to, to take the focus
	treeNav       bool            // the focus is kept in the tree as the page opens: the arrow keys are in use
	treeHeld      bool            // a row has the focus in this frame
	treeSigSeen   uint64
	cdrag         collDragState
	collRects     map[*Collection]ui.Rect
	ctx           ctxMenu   // the context menu (ctxmenu.go)
	frame         func()    // asks for a frame that builds the view
	links         linkIndex // who links to whom (backlinks.go)
	hoverCh       *chip     // the chip the pointer is over
	dateEdit      dateEdit  // the calendar a date chip opens

	// The settings, and the MCP server they turn on (mcp.go). A frame and
	// an agent's tool each hold mu: the library is theirs one at a time.
	mu           sync.Mutex
	settings     Settings
	settingsFile string // "" in tests: not kept
	// settingsStore, when the library is a file of its own, keeps the settings
	// in it; settingsFile then is only where they were before.
	settingsStore *Store
	// inKeychain is each secret of the settings the keychain holds, as it was put there (keychain.go)
	inKeychain    map[string]string
	importNote    string                  // what the last import said, in the settings
	embedCache    map[string][]string     // the first lines of the pages embedded (embed.go)
	queryCache    map[string]queryResult  // what the query blocks found (queryblock.go)
	ctxCache      map[string][]seg        // the lines that link to a page (backlinks.go)
	chipW         map[string]float32      // how wide the text of each chip is (chips.go)
	autoCovers    map[uint64]*autoCover   // the patterns made for the covers of the pages shown lately (autocover.go)
	tables        map[string]*tableUI     // the state of each collection table, by collection and view (views.go)
	sortLists     map[string]*manualState // each list in its own order, by collection and view (views.go)
	writingSaved  time.Time
	exportNote    string                    // what the last export said, in the settings
	docUse        []*Page                   // the pages whose texts are kept, the longest unused first (touchDoc)
	images        map[string]ui.ImageSource // the pictures read, by their file (images.go)
	imageMiss     map[string]time.Time      // the pictures that could not be read, and when: they are tried again after imageRetry (images.go)
	clipText      func(string)              // the clipboard to write; the system's when nil
	clipRead      func() string             // and to read
	ownClip       ownClip                   // the blocks of the last copy in the editor (clipmodes.go)
	carry         dockCarry                 // a page or a collection carried from the sidebar over the panes (dockdrop.go)
	panesR        ui.Rect                   // where the panes were, last frame
	prewarming    bool                      // the pages are being read away from the window (prewarm.go)
	fieldSels     map[*string]fieldSel      // a selection the menu of a field left, to set on it (fieldmenu.go)
	edPtr         ui.Point                  // the pointer over the page's text, last frame
	cellBar       ui.Rect                   // the bar of the cell being written, as last laid out: the text under it has no pointer there (tables.go)
	tagDrafts     map[string]*string        // what is typed in the fields of tags and numbers (tagfield.go)
	countEditing  string                    // the number field being typed in
	tagPop        map[string]bool           // a field of tags whose last was taken away with ⌫
	clipImage     func() []byte             // the clipboard's picture; the system's when nil
	outlineOf     []heading                 // the page's headings, for the side panel
	outlineVer    uint64
	outlineDoc    *richtext.Document
	imgOnClip     atomic.Bool // a picture, and no text, is on the clipboard
	settingsDirty bool        // changed, not yet written (saveSettingsSoon)
	wob           wobble      // the shimmer, when an agent writes (wobble.go)
	wobMu         sync.Mutex  // guards it: an agent's goroutine sends it
	settingsAt    time.Time   // when
	savedAt       time.Time   // the last save of the page, shown in the status bar
	mcpServer     *http.Server
	agent         agentEdits // what agents changed, to put back
	mcpErr        string

	prop        propEdit                  // the property dialog
	propBuf     map[string]*string        // the text of the properties' fields, by page and property
	propEditing string                    // the field being typed in
	propErr     string                    // the field whose value did not fit
	invalidate  func()                    // asks the window for a frame, from any goroutine
	pal         palette                   // the command palette (palette.go)
	searchCache map[searchKey][]searchHit // the last searches' hits (searchKept)
	viewCache   struct {                  // the pages of the view shown (viewPages)
		key   viewKey
		pages []*Page
		have  bool
		made  int // how many times, counted for a test
	}
	searchMade int          // the searches made, not taken from searchCache: counted for a test
	chipHits   []chipHit    // the chips drawn this frame (chips.go)
	chipSeen   []chipHit    // and last frame, for a click
	slash      slashMenu    // the / menu (slash.go)
	grip       gripState    // the grip of the block under the pointer (blockui.go)
	tasks      taskFx       // the task boxes: what turns, and the confetti (taskkinds.go)
	cs         collSettings // a collection\'s settings (collsettings.go)
	hist       []place      // where the app went, and where it is in that (nav.go)
	histAt     int
	going      bool     // going back or forward: no new place in the history
	menuBar    bool     // the menu bar has the shortcuts, not the window
	pageBg     ui.Color // the page's background this frame (bg)
}

func newApp(lib *Library) *app {
	a := &app{lib: lib, docs: map[*Page]*richtext.Document{}, jelly: newSlime(), mizan: &mizanCaret{}, clock: time.Now, sx: newSearchIndex()}
	a.paneState = newPaneState()
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
	a.restorePaneLayout()
	return a
}

// seed gives an empty library a collection and a page to start from.
func seed(lib *Library) error {
	if len(lib.Collections) > 0 {
		return nil
	}
	_, err := seedWelcome(lib)
	return err
}

// bareLibrary is whether lib has nothing in it but its journal, if that.
func bareLibrary(lib *Library) bool {
	for _, c := range lib.Collections {
		if !c.Journal {
			return false
		}
	}
	return true
}

// seedWelcome makes the Notes collection with its Welcome page.
func seedWelcome(lib *Library) (*Page, error) {
	c, err := lib.NewCollection("Notes")
	if err != nil {
		return nil, err
	}
	p, err := c.NewPage(nil, "Welcome")
	if err != nil {
		return nil, err
	}
	return p, p.SaveText(welcome)
}

// mcpFlag is the command line's word for the app to turn its MCP server on (`celestine mcp` starts it, or the
// running app is handed it).
const mcpFlag = "--mcp"

// withoutMCPFlag is the command line without --mcp, and whether it was there.
func withoutMCPFlag(args []string) ([]string, bool) {
	if !slices.Contains(args, mcpFlag) {
		return args, false
	}
	return slices.DeleteFunc(slices.Clone(args), func(s string) bool { return s == mcpFlag }), true
}

// appIcon is the app's icon, for its window and its Dock (assets/): in the Dock's shape, a rounded
// square with room around it. winIcon is the one Windows has, the picture filling its square.
//
//go:embed assets/icon-512.png
var appIcon []byte

//go:embed assets/icon-win.png
var winIcon []byte

func main() {
	// `celestine mcp`: the app's MCP server for an agent over stdio, by way of the app itself (mcpbridge.go)
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		os.Exit(runMCP(os.Args[2:]))
	}
	// MyGo's own words, menus and text engine speak the window's language, not the system's (i18n.go).
	showLocale = func(tag string) { mygo.App.SetLocale(tag) }
	// One copy of the app at a time: a second one hands its command line to the first, which comes to the front,
	// and goes, before it opens the library - two would write the same file, and each would take the other's mark
	// of running for a crash (crash.go). --mcp goes with it, for the first to turn its MCP server on.
	// A restart waits for the copy before it to be gone first (restart.go).
	var leaving int
	if len(os.Args) > 1 {
		var rest []string
		rest, leaving = withoutRestartFlag(os.Args[1:])
		os.Args = append(os.Args[:1:1], rest...)
	}
	waitForExit(leaving, 15*time.Second)
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	// An update's first start is stamped before anything can fail, and a second start that finds the stamp
	// unproved puts the old program back and starts it (update.go).
	if note := rollbackIfBroken(updateMark(), version); note != "" {
		if exe, err := os.Executable(); err == nil {
			exec.Command(exe, restartArgs()...).Start()
		}
		log.Println(note)
		os.Exit(0)
	}
	var mcpOn bool
	os.Args, mcpOn = withoutMCPFlag(os.Args)
	// The library is a file of its own, next to the program: nothing else
	// opens it, and the two move together. A folder named on the command line
	// is a library of plain files, as before; the first time, a folder
	// library found here is taken into the file.
	var (
		lib   *Library
		store *Store
		err   error
	)
	restoreNote := ""
	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}
	if info, serr := os.Stat(arg); arg != "" && serr == nil && info.IsDir() {
		root, _ := filepath.Abs(arg)
		lib, err = OpenLibraryIn(&countFS{FS: OSFS{}}, root) // its writes counted: the sync goes a moment after them
	} else {
		file := LibraryFile()
		if arg != "" {
			file, _ = filepath.Abs(arg)
		}
		restored, rerr := applyRestore(backupDir(), file, time.Now()) // a backup chosen before the last quit is put in place first
		if rerr != nil {
			restored = "Geri yükleme olmadı: " + rerr.Error()
		}
		restoreNote = restored
		if _, serr := os.Stat(file); serr != nil {
			if err := importFolderLibrary(file); err != nil {
				cannotOpen(nil, err)
				return
			}
		}
		if store, err = OpenStore(file); err == nil {
			store.BeginSnapshot() // the table read at once: not a query a folder and a page
			lib, err = OpenLibraryIn(store, StoreRoot)
			store.EndSnapshot()
		}
	}
	var newer *NewerFormatError
	if errors.As(err, &newer) {
		newerLibrary(store)
		return
	}
	if err != nil {
		cannotOpen(store, err)
		return
	}
	// A library with nothing in it yet but a journal (a new file, or one made and left empty): what it holds after
	// this is only what it makes for itself, and its first sync takes the account's copies of that (markPristine).
	// A folder's has its welcome page at once; one in a file has it once the settings say whether it is signed in
	// to an account (below).
	empty := bareLibrary(lib)
	if store == nil {
		if err := seed(lib); err != nil {
			cannotOpen(store, err)
			return
		}
	}
	a := newApp(lib)
	a.notice = restoreNote
	if _, err := a.journal(); err != nil { // every library has its journal
		cannotOpen(store, err)
		return
	}
	keychain = systemKeychain()                     // the secrets of the settings, out of the file (keychain.go)
	if dir, err := os.UserConfigDir(); err == nil { // the settings kept, not a test's
		a.settingsFile, a.settings = filepath.Join(dir, "celestine", "settings.json"), Settings{}
	}
	if store != nil {
		a.settingsStore = store
	}
	if a.settingsFile != "" || a.settingsStore != nil {
		a.loadSettings()
		a.seedNew(empty && store != nil)
		a.applyTree()
		setLang(a.settings.Lang) // the window speaks the language that was chosen, or the system's
	}
	if len(lib.Damaged) > 0 { // left out, and left as they are on disk
		a.notice = locales.AppSomeCollectionsCouldNot() + strings.Join(lib.Damaged, "; ")
	}
	if empty {
		a.markPristine()
	}
	a.wel.on = a.wantWelcome()                              // the first opening, if it is one
	a.purgeTrash()                                          // what has been in the trash trashDays is deleted for good
	a.prewarm()                                             // the pages' text, read away from the window
	unclean, prevLog := markRunning(crashDir(), time.Now()) // the last session that did not end as it should is told
	if unclean {
		a.crashPrev, a.crashOpen = prevLog, true
		if prevLog == "" {
			a.crashText = "Son oturum bir hata günlüğü bırakmadan kapandı (uygulama zorla kapatılmış ya da bilgisayar kapanmış olabilir)."
		} else if b, err := os.ReadFile(prevLog); err == nil {
			a.crashText, a.crashPath = string(b), prevLog
		}
	}
	time.AfterFunc(20*time.Second, func() { updateProved(updateMark(), version) }) // twenty seconds in, it has proved itself
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {                                  // what is written and not saved goes to its file before the app goes
		if a.quick != nil {
			a.quick.leaving = true
		}
		a.mu.Lock()
		a.flush()
		if a.settingsDirty { // what saveSettingsSoon left
			a.saveSettings()
		}
		a.endSession()
		a.saveUsage()
		a.mu.Unlock()
		a.syncAtQuit(6 * time.Second) // what was written since the last pass goes up before the app goes
		markClean(crashDir())
		updateProved(updateMark(), version) // a first start that ended as it should has proved itself too
	})
	defer func() { // a panic of the program itself is written, too, and then goes on
		if r := recover(); r != nil {
			crashLog(crashDir(), r, debug.Stack(), time.Now())
			panic(r)
		}
	}()
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(a.menu())
		a.menuBar = true
		opts := mygo.WindowOptions{Title: "celestine", Width: 1180, Height: 760, MinWidth: 640, MinHeight: 420,
			TitleBarStyle: mygo.TitleBarHiddenInset, TrafficLightPosition: &mygo.Point{X: 18, Y: (headerH - 14) / 2},
			HideWindowButtons: ownWindowControls, Content: ui.View(a.view)}
		if a.wel.on && runtime.GOOS == "darwin" { // the first opening: the window waits, hidden, under the stage (welcome.go)
			opts.Hidden, opts.DisableShadow = true, true
			a.wel.clear = true
		}
		fadeIn := !a.wel.clear && runtime.GOOS != "js" // a browser's tab is there already
		if fadeIn {
			opts.Hidden = true
		}
		w := mygo.NewWindow(opts)
		a.wel.win = w
		if a.wel.clear {
			a.openStage()
		}
		if fadeIn { // shown once its first frame is drawn, and faded in: no blank window, no flash of the system's ground
			var once sync.Once
			show := func() {
				once.Do(func() {
					w.SetOpacity(0)
					w.Show()
					a.fadeWindow(w, 0, 1, 0, 0.22)
				})
			}
			w.OnReadyToShow(show)
			time.AfterFunc(1500*time.Millisecond, show) // a platform that never says the frame is drawn still gets its window
		}
		if ownWindowControls {
			a.win = w
		}
		a.invalidate = w.Invalidate
		if runtime.GOOS != "js" { // a browser has one window: the panes stay in it
			a.newDockHost(w)
			a.dockHost.Sync() // the windows of the panes, as they were left
		}
		mygo.App.OnSecondInstance(func(args []string, _ string) {
			if slices.Contains(args, mcpFlag) { // `celestine mcp` found the server off: it is turned on
				a.do(func() error {
					if a.mcpServer == nil {
						a.setMCP(true)
					}
					return nil
				})
			}
			a.bringToFront()
		})
		w.OnClosed(mygo.App.Quit) // the window closed is the app closed: the hidden quick note panel does not keep it on
		w.OnFocus(a.windowIn)     // back from another device: what it wrote comes
		w.OnBlur(a.windowOut)     // the server is not asked for changes behind other windows
		if runtime.GOOS == "windows" {
			w.SetIcon(winIcon) // the title bar and taskbar
		} else {
			w.SetIcon(appIcon) // the title bar and taskbar (Linux)
		}
		mygo.App.Dock.SetIcon(appIcon) // the Dock (macOS)
		if runtime.GOOS != "js" {      // a browser has no global key, and the panel's window would name the tab
			a.quick = newQuick(a)
		}
		go func() { // the day's backup, a moment after the start, when the window is up
			defer a.guard()
			time.Sleep(4 * time.Second)
			a.do(func() error { a.backupDaily(); return nil })
			time.Sleep(4 * time.Second)
			a.do(func() error { a.checkUpdate(false); return nil })
		}()
		go func() { defer a.guard(); a.watch() }()
		go func() { defer a.guard(); a.watchClipboard() }()
		if a.settings.MCP || mcpOn { // on as it was left, or asked for by `celestine mcp`
			a.setMCP(true)
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// importFolderLibrary takes a folder library found beside file (from before the one-file library) into a new
// library file, the first time: into file.importing, which becomes file only when it is whole. A copy that failed
// left a file that was there, so the next start skipped the import and opened part of the library (#141).
func importFolderLibrary(file string) error {
	for _, old := range []string{filepath.Join(filepath.Dir(file), "kitaplik"), "kitaplik"} {
		if !IsLibraryFolder(old) {
			continue
		}
		tmp := file + ".importing"
		for _, ext := range []string{"", "-wal", "-shm"} {
			os.Remove(tmp + ext) // what a failed import left
		}
		st, err := OpenStore(tmp)
		if err != nil {
			return err
		}
		st.Fast(true) // a copy of what is still there: no wait for each file
		err = CopyTree(OSFS{}, old, st, StoreRoot)
		st.Fast(false)
		if cerr := st.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Rename(tmp, file)
		}
		for _, ext := range []string{"-wal", "-shm"} {
			os.Remove(tmp + ext)
		}
		if err != nil {
			os.Remove(tmp)
		}
		return err
	}
	return nil
}

// cannotOpen says, in a window, that the library could not be opened and why, and quits when it is closed: a
// library's content is not a reason for the app to vanish with a line in a log no one reads (#134).
func cannotOpen(store *Store, err error) {
	setLang(savedLang(store))
	mygo.App.WhenReady(func() {
		mygo.Dialog.Message(mygo.MessageOptions{
			Type:    mygo.MessageError,
			Title:   "Celestine",
			Message: locales.AppTheLibraryCouldNot(),
			Detail:  err.Error() + "\n\n" + locales.AppNothingWasChangedBackup(),
		})
		mygo.App.Quit()
	})
	if rerr := mygo.App.Run(); rerr != nil {
		log.Fatal(err)
	}
}

// newerLibrary says, in a window, that the library was written by a newer Celestine, and quits when it is closed: the
// app does not open the library, so that it does not write what it does not know (internal/library/format.go).
// The settings are not read yet: the window speaks the language they have chosen all the same.
func newerLibrary(store *Store) {
	setLang(savedLang(store))
	mygo.App.WhenReady(func() {
		mygo.Dialog.Message(mygo.MessageOptions{
			Type:    mygo.MessageWarning,
			Title:   "Celestine",
			Message: locales.AppThisLibraryWasWritten(),
			Detail:  locales.AppUpdateCelestineOpenNothing(),
		})
		mygo.App.Quit()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// savedLang is the language the settings have chosen, read as loadSettings reads them: from the library's file,
// or from the settings file beside it.
func savedLang(store *Store) string {
	var s struct {
		Lang string `json:"lang"`
	}
	var data []byte
	if store != nil {
		data = store.Meta(SettingsKey)
	}
	if dir, err := os.UserConfigDir(); data == nil && err == nil {
		data, _ = os.ReadFile(filepath.Join(dir, "celestine", "settings.json"))
	}
	json.Unmarshal(data, &s)
	return s.Lang
}

// frameTheme gives the window of the frame the app's look: Ryugo's tokens, in the app's color theme, its
// typeface, and the text's theme. Every window of the app has it, the torn-off panes' too (dockwin.go).
func (a *app) frameTheme(c *ui.Context) (*ryugo.Tokens, *richtext.Theme) {
	mode := []ryugo.Mode{ryugo.ModeInherit, ryugo.Light, ryugo.Dark}[a.settings.Theme%3]
	radiusScale = float32(a.cornerPercent()) / 100
	// Ryugo's theme, its components' tokens, for the appearance; a pressed button's wave as the settings say
	opts := ryugo.Options{Mode: mode, Size: ryugo.Sm, Corners: a.ryugoCorners(), CornerScale: a.ryugoScale(), Waves: !a.settings.NoWaves}
	tok := ryugo.Root(c, opts)
	if a.appTheme().colors(tok.Dark).bg != "" { // and in the app's theme: the same appearance, in its colors
		opts.Mode, opts.Accent = map[bool]ryugo.Mode{false: ryugo.Light, true: ryugo.Dark}[tok.Dark], a.accentOf(tok.Dark)
		tok = ryugo.Root(c, opts)
		a.skin(tok, c.Theme())
	}
	t := c.Theme()
	// the typeface of the window, from the first one that is installed: put in before anything reads the theme,
	// and given to the root again (SetTheme copies the font into it), or the first frame is drawn in Ryugo's own
	// font and the next in this one, and the whole window moves
	if f := a.appFont(); t.Font != f {
		t.Font = f
		c.SetTheme(t)
	}
	// the scroll areas have the app's own bar (funbar.go); what MyGo still paints (a table that scrolls
	// sideways, the board, the Dock) is slim, in the muted color of the theme
	t.Scrollbar, t.ScrollbarWidth = tok.Colors.Muted.Alpha(0.45), 7
	th := richtext.DefaultTheme(t)
	th.Caret = invisible // ours is drawn over it
	th.ParagraphSpacing = a.paraGap()
	th.FirstLineIndent = a.firstIndent()
	// A heading's room above stays under half its line: the editor moves ↑ by
	// the height of the line the caret is on, and from a heading that must
	// land in the paragraph above, not in the heading's own room.
	for i := range th.Headings {
		th.Headings[i].Top, th.Headings[i].Bottom = 0.6, 0.3
	}
	a.theme = th
	th.CodeFont = a.monoFont()
	a.font = ui.Font{Family: a.editorFont(), Size: a.fontSize(), LineHeight: a.lineHeight(), Weight: a.fontWeight(), Features: a.fontFeatures()}
	return tok, th
}

// windowIn and windowOut are the window's coming forward and going behind. They do their work off the window's thread, not in the call
// that raised them: showing or focusing a window is done by a frame that holds the app's lock (the welcome's Başla), and the system
// answers with the focus at once, in that very call. Taking the lock there waited for the frame that was waiting for it, and the
// window froze (TestWindowFocusDoesNotWaitForTheLock). One worker takes them in the order they came, so that a window that came
// forward and went behind at once is behind, not forward.
func (a *app) windowIn()  { a.windowEvent(true) }
func (a *app) windowOut() { a.windowEvent(false) }

func (a *app) windowEvent(forward bool) {
	a.winOnce.Do(func() {
		a.winEv = make(chan bool, 64)
		go func() {
			defer a.guard()
			for fwd := range a.winEv {
				a.do(func() error {
					if fwd {
						a.windowFocused()
					} else {
						a.sy.behind = true
					}
					return nil
				})
			}
		}()
	})
	select {
	case a.winEv <- forward:
	default: // a flood of them: the next one comes
	}
}

// open shows page p, saving the page that was open first.
func (a *app) open(p *Page) {
	defer a.visit()
	a.coll, a.trashOpen, a.tag, a.tasksOpen, a.searchOpen, a.settingsOpen = nil, false, "", false, false, false
	a.statsScreen, a.profileScreen = false, false
	if p == a.page {
		return
	}
	a.flush()
	if a.page != nil && a.dirty { // its save failed: its text is kept and saved when it can be, not dropped (#129)
		if a.unsaved == nil {
			a.unsaved = map[*Page]bool{}
		}
		a.unsaved[a.page] = true
	}
	a.noteOpened(p)
	doc := a.docFor(p)
	a.page, a.doc, a.title = p, doc, p.Title
	a.find.ver = 0 // another text to find in
	_, caret := doc.Selection()
	a.version, a.length, a.caretAt = doc.Version(), doc.Text().Len(), caret
	a.prevText = doc.Text()
	a.dirty, a.saveErr, a.lastSave, a.lastWords = false, "", "", -1
	if a.unsaved[p] { // back to a page whose save failed: what it shows is still to be saved
		delete(a.unsaved, p)
		a.dirty, a.editAt = true, a.clock()
	}
	a.boxes = nil
	*a.pageScroll = ui.ScrollState{}
	// a page opened starts clean: the caret is put in its place, not flown
	// there from where it was in the last page, and what was laid out is gone
	a.jelly.placed, a.mizan.seen, a.mizan.glide = false, false, nil
	a.eachFX(func(c caretFX) { c.reset() })
	a.jellied.reset()
	a.sleep.wake()
	a.editorR, a.caretPara, a.edH = ui.Rect{}, -1, 0
	a.linksStale()
	a.links.built = time.Time{} // asked for at once for the page opened
}

// docFor is the text of page p, read from its file the first time and kept: a page shown in two panes is the one text.
func (a *app) docFor(p *Page) *richtext.Document {
	doc := a.docs[p]
	if doc == nil {
		body, err := p.Body()
		if err == nil {
			err = bodyReadable(body)
		}
		p.Unreadable = "" // read again: what it says now
		if err != nil {   // shown read-only and never saved over, and its pictures kept (#125, #126)
			p.Unreadable = err.Error()
		} else {
			a.sweepPictures(p, body)
		}
		doc = richtext.NewDocument(a.parsePage(p, body))
		a.docs[p] = doc
	}
	a.touchDoc(p)
	return doc
}

// maxDocs is how many pages' texts are kept: a text in the editor is some twenty times the size of its page, and a
// session that goes through hundreds of pages kept them all. The pages shown, in any pane, are never let go.
const maxDocs = 40

// touchDoc notes that page p's text was used, and lets go of the longest unused ones past maxDocs. A page
// written is saved when it is left (open), so what is let go is on disk; it is read again when it is opened.
func (a *app) touchDoc(p *Page) {
	if i := slices.Index(a.docUse, p); i >= 0 {
		a.docUse = slices.Delete(a.docUse, i, i+1)
	}
	a.docUse = append(a.docUse, p)
	if len(a.docs) <= maxDocs {
		return
	}
	shown := map[*Page]bool{p: true, a.page: true}
	for q := range a.unsaved { // a text not yet saved is not let go
		shown[q] = true
	}
	for _, st := range a.ps.states {
		shown[st.page] = true
	}
	for i := 0; len(a.docs) > maxDocs && i < len(a.docUse); {
		if q := a.docUse[i]; shown[q] {
			i++
		} else {
			delete(a.docs, q)
			a.docUse = slices.Delete(a.docUse, i, i+1)
		}
	}
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
		a.fail(err)
		return
	}
	a.noteCreated()
	if c.Template != "" { // it starts as the collection says
		if err := p.SaveText(expandTemplate(c.Template, p.Title, a.clock())); err != nil {
			a.fail(err)
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
	if len(a.unsaved) > 0 {
		a.retryUnsaved()
	}
	a.flushPanes()
}

// view is a frame of the window. A panic in it is written to the crash log, and the window shows what
// it was, with a way on, instead of the app falling.
func (a *app) view(c *ui.Context) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			a.mu.Lock() // the frame's own lock was let go as it unwound
			a.noteCrash(r, stack)
			a.mu.Unlock()
			func() {
				defer func() { recover() }() // the tree may be half made: if this fails too, the next frame tries again
				a.crashFallback(c)
			}()
		}
	}()
	if frameMeterOn {
		t0 := time.Now()
		defer func() { a.meter.mu.Lock(); a.meterView(time.Since(t0)); a.meter.mu.Unlock() }()
	}
	a.viewFrame(c)
}

func (a *app) viewFrame(c *ui.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.testPanic != "" {
		panic(a.testPanic)
	}
	a.frame = c.Invalidate
	a.frameSeq++
	a.updatePhone(c)
	a.keepSettings(c)
	a.statTick()
	a.syncTick(c)
	a.tmKeys(c) // the page's history has the keys while it is open (timemachine.go)
	a.kbBegin()
	if !a.prewarming && !a.sx.warm(a, sxBudget) { // the pages are taken into the search index a little a frame, as the window opens
		c.Invalidate()
	}
	sleepAfter = a.caretSleep()
	a.chipSeen, a.chipHits = a.chipHits, a.chipSeen[:0] // the last frame's, for a click now
	tok, th := a.frameTheme(c)
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
	a.welcomeBackdrop(c, false) // the first opening's circles, under the app
	root := ui.Row(c).Fill().AlignItems(ui.Stretch).Background(a.pageBg)
	a.welcomeRoot(c, root) // and the app hidden, then opened by a shader
	a.shake(c, root)
	// the sync's asks rest while nobody is here
	a.activity(root)
	for _, f := range root.DroppedFiles() { // and those dropped elsewhere in the window
		a.dropFile(f, false)
	}
	if !a.menuBar { // in the app, the menu bar has these (menu.go)
		a.appKeys(c)
	}
	// Esc leaves the settings, back to where they were opened from
	if a.settingsOpen && !a.modal() && c.Shortcut(0, ui.KeyEscape) {
		a.closeSettings()
	}
	if a.statsScreen && !a.modal() && c.Shortcut(0, ui.KeyEscape) {
		a.closeStats()
	}
	if a.profileScreen && !a.modal() && c.Shortcut(0, ui.KeyEscape) {
		a.closeProfile()
	}
	// Esc takes back what was lit or left alone in the time-lapse; with none of that it is the editor's
	lapseEsc := a.lapse.on && (a.lapse.solo != 0 || a.lapse.lit != 0) && !a.modal() && c.Shortcut(0, ui.KeyEscape)
	if lapseEsc {
		if a.lapse.solo != 0 {
			a.lapse.solo, a.lapse.ver = 0, a.lapse.ver+1
		} else {
			a.lapse.lit, a.lapse.ver = 0, a.lapse.ver+1
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
		a.kbAnchor(c)
		if !phone { // on a phone the sidebar is a drawer over everything, built last (mobile.go)
			a.sidebarShown = a.sidebar(c)
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(func() { // the header, the content, the status bar: each a region of its own
			a.header(c)
			a.panesView(c, th)
			if !a.settings.HideStatus {
				a.status(c)
			}
			a.keepTree()
		})
		if phone {
			a.drawer(c)
		} else {
			a.sidePanel(c)
			a.sidebarButton(c)
		}
	})
	a.collDragView(c)
	if !a.activeElsewhere() { // the pane in use is in a window of its own, which draws these (panewin.go)
		a.textOverlays(c)
	}
	a.contextView(c)
	a.paletteView(c)
	if !a.kb.normal && !a.modal() && !a.pal.open && !a.find.open && !a.slash.open {
		// Esc, in the text or on a screen that has nothing to close: the keys are commands. The settings
		// and the figures have their own Esc, which goes back.
		if !a.settingsOpen && !a.statsScreen && !a.profileScreen && !lapseEsc && c.Shortcut(0, ui.KeyEscape) {
			a.kbEnter()
		}
	}
	if c.Shortcut(ui.Cmd, ui.Key1) { // the regions: the tree, the side panel, the text
		a.focusTree()
	}
	if c.Shortcut(ui.Cmd, ui.Key2) && !a.settings.PanelOpen {
		a.togglePanel()
	}
	if c.Shortcut(ui.Cmd, ui.Key3) {
		a.kbLeave()
	}
	a.kbKeys(c)
	a.boundKeys(c)
	a.noAlignKeys(c) // after the user's own keys, which may take Ctrl+E
	a.kbView(c)
	a.welcomeView(c, false)
	a.tourView(c)
	a.updateView(c)
	a.collSettingsView(c)
	a.propDialog(c)
	a.tagRenameView(c)
	a.historyView(c)
	a.crashView(c)
	a.frameMeterView(c)
	a.askView(c)
	if a.modal() || a.ctx.open { // under a dialog or a menu the cell is let go
		a.cellEd.open = false
	}
	a.lightboxView(c)
	a.timeMachineView(c) // the page's history, over everything (timemachine.go)
	if !a.treeHeld {     // the focus is not in the tree: no more arrow keys
		a.treeNav = false
	}
	a.treeHeld = false
	a.runLater()
	if a.notice != "" {
		kind := noticeKind(a.notice) // from the Turkish, as it was written
		if a.noticeTone != nil {
			kind = *a.noticeTone
		}
		if undo := a.noticeUndo; undo != nil { // a setting changed: it is one click from taken back
			a.toastOf(kind, T(a.notice), locales.CommonUndo2(), undo)
		} else if pg := a.noticePage; pg != nil { // a note put away: the page it went to is one click away
			a.toastOf(kind, T(a.notice), locales.CommonOpen(), func() { a.open(pg) })
		} else {
			a.toastOf(kind, T(a.notice), "", nil)
		}
		a.notice, a.noticePage, a.noticeUndo, a.noticeTone = "", nil, nil, nil
	}
	if item := a.trashed; item != nil {
		a.trashed = nil
		a.toastAction(Tf("“%s” çöpe taşındı", item.Title), locales.CommonUndo2(), func() { a.restore(item) })
		a.loadTrash()
	}
	a.toastView(c)
	a.cursorFXView(c, root)            // over everything, the pointer's effect when one is chosen (cursorfx.go)
	if a.doc != nil && a.coll == nil { // the boxes were read as the page's editor was built (pageBody)
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
		sc := ui.Scroll(c.Key("page-scroll:" + p.Dir)).Grow(1)
		a.scrolled(c, "page-scroll:"+p.Dir, a.pageScroll, sc, func() {
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
				ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, pageMargin()).Children(func() {
					ui.Column(c).FillWidth().MaxWidth(a.pageWidth()).Gap(4).Children(func() { a.pageBody(c, p, th) })
				})
			})
		})
		a.scrollR = sc.LayoutBounds()
		a.findBar(c) // over the page, at its top right: nothing under it moves
	})
}

// pageBody is the title, the properties and the text.
func (a *app) pageBody(c *ui.Context, p *Page, th *richtext.Theme) {
	head := ui.Column(c.Key("page-head")).FillWidth()
	head.Children(func() { a.pageHead(c, p) })
	a.headH = head.LayoutBounds().H
	a.lapseAuto(c, p)                      // the time-lapse, as the settings say (lapseui.go)
	if a.lapseOn(p) && a.lapse.solo != 0 { // one day's words alone, to look at: nothing to type in
		a.focused = false
		a.lapseView(c)
		a.backlinksView(c, p)
		return
	}
	if p.Unreadable != "" { // shown, not edited: the editor would save what it shows over the file
		ui.Text(c, locales.AppThisPageCouldNot()+"\n"+p.Unreadable).FontSize(13).TextColor(ryugo.From(c).Colors.Warn).Wrap().FillWidth().Padding(0, 0, 8, 0)
	}
	c.Key("body:" + p.Dir)
	textSpacing = spacingOf(a.settings.LetterSpacing, a.font.Size) // the editor's spans and the caret's layout (codecolors.go)
	a.codeTheme = editor.DefaultTheme(c.Theme())
	ed := richtext.EditorBase(c, a.doc, richtext.EditorOptions{
		Label: "Sayfa", Font: a.font, Theme: th,
		Placeholder:  locales.AppWriteHeadingListTask(),
		Menu:         a.editorMenu,  // the app's menu, not the system's (fieldmenu.go)
		OnKey:        a.editorKeys,  // the / menu's keys, and copy and paste, before the editor's (slash.go, clipmodes.go)
		Spans:        a.editorSpans, // code colored, letters spaced (codecolors.go)
		SpansVersion: a.spansVersion(),
		Space:        a.blockRoom, // the room a block dragged opens where it would go (blockui.go)
		ReadOnly:     p.Unreadable != "",
		// the page scrolls as a whole, the editor as tall as its text, however long: no bar of its own beside the
		// page's, which showed when a split pane narrowed it and the height worked out was a frame behind (#190), and
		// no window on a long text scrolling in itself (#314)
		NoScrollbar: true,
	}).FillWidth().Height(a.editorH(c)).FocusRing(false) // the caret shows where the text has the focus
	// this editor's paragraphs, read right after it is built: with the page open in two panes, each pane's are its
	// own, laid out at its width, and its height is worked out from them (#190)
	a.boxes = a.doc.VisibleParagraphs()
	if !a.titleFocused && !a.fresh && !a.treeNav && !a.inactivePane() && !a.kb.normal {
		ed.AutoFocus()
	}
	if a.refocus {
		ed.Focus()
		a.refocus = false
	}
	a.focused = ed.Focused()
	if a.focused && a.cellEd.open && a.cellEd.was { // the text took the keyboard back from a cell: it is written and let go
		a.commitCell()
		a.cellEd.open = false
	}
	a.tabKey(c)
	a.tableKeys(c)
	a.edPressed = ed.Pressed()
	a.trackGrip(ed)
	a.hoverCh, a.hoverTag = nil, ""
	a.tagSeen, a.tagHits = a.tagHits, a.tagSeen[:0] // this frame's are drawn after
	a.linkSeen, a.linkHits = a.linkHits, a.linkSeen[:0]
	a.picSeen, a.picHits = a.picHits, a.picSeen[:0]
	if x, y, over := a.textPointer(ed); over {
		a.edPtr = ui.Point{X: x, Y: y} // where a right click opens the menu
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
		sa, sc := a.doc.Selection()
		selecting := sa != sc
		if h := a.tableCellAt(a.editorR.X+x, a.editorR.Y+y); h != nil {
			a.doc.SetSelection(a.selPrevA, a.selPrevC)
			a.clickCell(h)
		} else if selecting { // the pointer selected text, it did not click what it ended on
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
		a.drawBlockSel(p, r)
		a.drawMatches(p, r)
		a.drawBlockDrop(p, r)
		a.drawDividers(p, r)
		a.drawTasks(p, r)
		a.drawWhite(p, r)
		a.drawFocusVeil(p, r)
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
	a.winRect = ui.Rect{W: ww, H: wh}
	ui.Overlay(c, func() {
		ui.Box(c.Key("caret-layer")).Absolute().Left(0).Top(0).Size(ww, wh).PassThrough().Draw(func(p *ui.Painter, _ ui.Rect) {
			a.drawConfetti(p)
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
		ttl := ui.Text(c, p.Title).FontSize(a.titleSize(p.Title, a.titleW)).FontWeight(titleWeight).Padding(6, 0, 10, 0).Label(locales.CommonTitle()).Selectable().Wrap().FillWidth().TextColor(ink)
		a.titleMenu(ttl, p, false)
		a.titleRoom(c, ttl)
		a.titleFocused = false
		a.coverGap(c, p)
		a.propsSection(c, p)
		return
	}
	// the title wraps, at a size its length gives it, and is as high as its lines (titlesize.go); it is one line of
	// text all the same: a line break typed or pasted is not kept
	size := a.titleSize(a.title, a.titleW)
	_, textH := a.titleLayout(a.title, size, a.titleW).Size()
	before := a.title
	title := ui.TextAreaBase(c, &a.title).Label(locales.CommonTitle()).FontSize(size).FontWeight(titleWeight).Padding(6, 0, 10, 0).
		FillWidth().Height(textH + 16).TextColor(ink).CaretWidth(0)
	a.titleMenu(title, p, true)
	a.titleRoom(c, title)
	enter := false
	if strings.ContainsAny(a.title, "\r\n") {
		// the area takes ↵ as a break: one break typed into the title as it was is ↵, which goes into the text below,
		// as from the one-line input; a break pasted is a space
		if enter = strings.Count(a.title, "\n") == 1 && strings.Replace(a.title, "\n", "", 1) == before; enter {
			a.title = before
		} else {
			a.title = strings.Join(strings.Fields(a.title), " ")
		}
	}
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
			a.fail(err)
		}
	}
	a.titleFocused = title.Focused()
	a.titleCaret = titleCaretSpec{}
	if b := title.LayoutBounds(); title.Focused() && b.W > 0 { // the caret is ours, as the page's
		anchor, caret := title.TextAnchorCaret()
		cr, cw := a.inputCaretIn(a.title, caret, size, titleWeight, a.titleW, b.X, b.Y+6, b.H-16)
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
	if title.Focused() && (enter || title.Shortcut(0, ui.KeyDown) && a.titleLastLine(size, titleCaretOf(title))) {
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
	n := len(a.boxes)
	if n == 0 {
		// nothing laid out yet (a page just opened): room for what the text
		// should take, more than it will, so the editor does not scroll
		// itself to the caret and then slide as it grows. However long the
		// text: a long page is laid out whole in its first frame, once, rather
		// than its height growing frame after frame, the page's bar with it (#314)
		t := a.doc.Text()
		line := a.fontSize() * a.lineHeight()
		return min(max(least, (float32(t.ParagraphCount())+float32(t.Len())/55)*line*1.5+240), 4e6)
	}
	last := a.boxes[n-1]
	slack := max(a.scrollR.H*2, 1600)
	if last.Index >= a.doc.Text().ParagraphCount()-1 {
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
	if a.keepHidden(a.prevText, text, caret) { // a fold's or a zoom's chip taken away: what it held, back
		_, caret = a.doc.Selection()
		text, v = a.doc.Text(), a.doc.Version()
		style = text.ParagraphStyleAt(caret)
	}
	n := text.Len()
	// an edit that moves nothing - a box ticked, a mark changed - does not take the page to the caret:
	// the caret may be far from what was clicked
	if caret != a.caretAt || n != a.length {
		a.revealWant = true
	}
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
	if grew := n - a.length; grew > 1 && caret-a.caretAt == grew && (a.tableInPasted(caret-grew, caret) || a.chipsInPasted(caret-grew, caret)) {
		_, caret = a.doc.Selection()
		text, v = a.doc.Text(), a.doc.Version()
		style = text.ParagraphStyleAt(caret)
		n = text.Len()
		a.length, a.caretAt = n, caret
	}
	switch grew := n - a.length; {
	case grew > 0 && caret-a.caretAt == grew && grew <= 8:
		a.wrote += grew
		a.typed += strings.ReplaceAll(text.Slice(caret-grew, caret).String(), string(richtext.ObjectReplacement), "")
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
	case a.settings.Typewriter: // the line stays in the middle
		d = cr.Y + cr.H/2 - (vp.Y + vp.H/2)
		if d > -2 && d < 2 {
			d = 0
		}
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
	a.caretDraws++
	if !a.focused {
		// the find bar has the focus, and the match it steps to is the selection: the page goes to it all the same.
		// Only the caret's place is needed, so it is the editor's own (CaretRect), not laid out again (#306).
		if a.revealWant && a.find.open {
			if cr, ok := a.doc.CaretRect(); ok {
				cr.X += r.X
				cr.Y += r.Y
				a.revealCaret(cr)
			}
		}
		return
	}
	_, caret := a.doc.Selection()
	cb, ok := a.locateRune(caret, r.W)
	if !ok {
		return
	}
	// scrolled: the caret went with its paragraph, the text under it the
	// same - the jelly goes with it and is not kicked
	top := r.Y + cb.top
	if cb.para == a.caretPara && cb.r.Y-cb.top == a.caretRel && top != a.caretTop {
		a.jelly.shift(0, top-a.caretTop)
		a.mizan.shift(top - a.caretTop)
		a.eachFX(func(c caretFX) { c.shift(top - a.caretTop) })
	}
	a.caretPara, a.caretRel, a.caretTop = cb.para, cb.r.Y-cb.top, top
	cr := cb.r
	cr.X += r.X
	cr.Y += r.Y
	a.caretWin = cr
	a.revealCaret(cr)
	anchor, caret := a.doc.Selection()
	if a.caretBlinkOff(p, anchor, caret) { // the off half of a blink
		return
	}
	if anchor != caret || a.settings.NoCaretFX { // over a selection, or with the effects off, a plain caret: the jelly would hide what is selected
		a.paintCaret(p, cr, cb.cw, r, kick{}, true)
		return
	}
	k := kick{wrote: a.wrote, edited: a.edited, erased: a.erased, dir: a.eraseDir, gone: a.gone, typed: a.typed, style: a.styleAt(caret)}
	a.wrote, a.edited, a.erased, a.gone, a.typed = 0, false, 0, "", ""
	doc, boxes, font, th := a.doc, a.boxes, a.font, a.theme
	a.nearFn = func(n int) []nearGlyph { return nearLetters(doc, boxes, r.X, r.Y, r.W, font, th, n) }
	a.wordsFn = &caretWords{doc: doc, boxes: boxes, x: r.X, y: r.Y, w: r.W, font: font, th: th}
	a.paintCaret(p, cr, cb.cw, r, k, false)
	a.nearFn, a.wordsFn = nil, nil
}

// caretBlink is where the caret last was, and since when: it blinks only once it has been still.
type caretBlink struct {
	anchor, caret int
	since         time.Time
}

// caretBlinkOff reports whether the caret is in the off half of its blink: with the blink set, once it has stood
// still a moment, it is hidden and shown again every CaretBlink ms; typing or moving it shows it at once.
func (a *app) caretBlinkOff(p *ui.Painter, anchor, caret int) bool {
	rate := time.Duration(a.settings.CaretBlink) * time.Millisecond
	now := p.Now()
	b := &a.blink
	if b.anchor != anchor || b.caret != caret || a.wrote > 0 || a.edited || b.since.IsZero() {
		*b = caretBlink{anchor: anchor, caret: caret, since: now}
	}
	if rate <= 0 || anchor != caret {
		return false
	}
	still := now.Sub(b.since)
	if still < rate { // shown, and still while it is moved about
		p.After(rate - still + time.Millisecond)
		return false
	}
	phase := (still - rate) / rate
	p.After(rate - (still-rate)%rate + time.Millisecond)
	return phase%2 == 0
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
	f := caretFrame{r: cr, cw: cw, dark: dark, bg: a.pageBg, bounds: bounds, wrote: k.wrote, edited: k.edited, style: k.style, near: a.nearFn, words: a.wordsFn}
	// the jelly, when it is on, is the shape of the caret's own body (the slime is jelly already)
	if a.settings.Jelly && a.caretKind != caretSlime {
		f.jelly = a.jellied.body(p, f)
	}
	// the torch lights the whole window, not just the text: what it darkens is the window
	if i := int(a.caretKind) - int(caretFirstFX); i >= 0 && i < len(caretPresets) {
		if t, ok := a.fxOf(i).(*torch); ok && a.winRect.W > 0 {
			f.bounds = a.winRect
			a.sleep.observe(p, f, k)
			f.rest = a.sleep.rested(p.Now())
			t.draw(p, f, k)
			a.sleep.draw(p, f, a.caretTint(p))
			return
		}
	}
	// not clipped to the editor's box: at the start of a line the caret
	// goes past its left edge
	room := a.scrollR
	if room.H == 0 {
		room = ui.Rect{X: bounds.X - 24, Y: bounds.Y - 12, W: bounds.W + 48, H: bounds.H + 24}
	}
	a.sleep.observe(p, f, k) // five seconds of rest, and it sleeps
	f.rest = a.sleep.rested(p.Now())
	body := func() {
		switch a.caretKind {
		case caretMizan:
			a.mizan.draw(p, f)
			return
		}
		if i := int(a.caretKind) - int(caretFirstFX); i >= 0 && i < len(caretPresets) {
			a.fxOf(i).draw(p, f, k)
			return
		}
		col := ui.Hex("#59f939")
		if !dark {
			col = ui.Hex("#2f9a1c")
		}
		styledDraw(p, f, boxOf(f), func() { a.jelly.draw(p, cr, cw, col, dark) })
	}
	p.Clip(room, 0, func() {
		// asleep, it breathes
		if tf, px, py, on := a.sleep.breathe(p.Now(), f); on {
			p.Transform(tf, px, py, body)
			return
		}
		body()
	})
	a.sleep.draw(p, f, a.caretTint(p)) // and the z's, out of it
}

// status is the line at the bottom: the file, whether it is saved, the caret.
func (a *app) status(c *ui.Context) {
	t := c.Theme()
	state := a.savedText()
	ui.Box(c.Key("status-line")).FillWidth().Height(1).Background(ryugo.From(c).Colors.Hairline) // the page ends here
	ui.Row(c).Padding(3, 16, 3, 10).Justify(ui.End).AlignItems(ui.Center).Gap(12).Children(func() {
		a.statusHints(c)
		// the MCP server on: a dot of the accent, which opens the settings
		if a.mcpServer != nil {
			m := ui.Row(c.Key("mcp-status")).Gap(5).Padding(2, 6).Radius(rad(6)).AlignItems(ui.Center).
				Label(locales.AppMcpSettings()).Role(ui.RoleButton)
			ryugo.Tooltip(c, m, locales.AppMcpServer()+a.mcpURL())
			if m.Hovered() {
				m.Background(t.SurfaceHover)
			}
			m.Children(func() {
				a.pulseDot(c)
				ui.Text(c, "MCP").FontSize(11.5).FontWeight(600).TextColor(t.TextMuted)
			})
			if m.Clicked() {
				a.openSettings()
				a.settingsTab = tabMCP
			}
		}
		a.agentStatus(c)
		a.syncBadge(c)
		a.statusExtras(c)
		// no file paths: what the settings ask for - the letters, how it is saved, the caret
		if a.settings.ShowChars {
			_, chars := a.docStats()
			ui.Text(c, strconv.Itoa(chars)+" harf").FontSize(11.5).TextColor(t.TextMuted)
		}
		if !a.settings.HideSaved {
			st := ui.Text(c, strings.ToUpper(state[:1])+state[1:]).FontSize(11.5).TextColor(t.TextMuted)
			if a.saveErr != "" {
				st.TextColor(ryugo.From(c).Colors.Up)
			}
		}
		a.lapseChips(c)
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
	if a.settingsStore != nil { // the browser's store writes later: what it said is shown once (#238)
		if err := a.settingsStore.WriteError(); err == nil {
			a.storeErr = ""
		} else if err.Error() != a.storeErr {
			a.storeErr = err.Error()
			a.fail(err)
		}
	}
	if len(a.unsaved) > 0 {
		if wait := retrySave - a.clock().Sub(a.unsavedAt); wait > 0 {
			c.After(wait)
		} else {
			a.retryUnsaved()
			c.After(retrySave)
		}
	}
	if !a.dirty || a.page == nil || a.settings.NoAutoSave {
		return
	}
	if a.saveErr != "" { // it failed: again a while after, not at every frame
		if wait := retrySave - a.clock().Sub(a.saveTried); wait > 0 {
			c.After(wait)
			return
		}
	}
	if wait := a.saveAfter() - a.clock().Sub(a.editAt); wait > 0 {
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
		a.saveErr, a.saveTried = err.Error(), a.clock()
		return
	}
	a.saveErr = ""
	words := len(strings.Fields(unfold(a.doc.Text()).PlainText())) // a fold's lines too: folding is not writing (#128)
	a.countWriting(a.lastWords, words)
	a.lastSave, a.lastWords, a.dirty = body, words, false
	a.savedAt = a.clock()
	a.linksStale()
}

// retrySave is how long after a failed save it is tried again.
const retrySave = 5 * time.Second

// retryUnsaved saves again the pages left with a save that failed, and forgets those it could save.
func (a *app) retryUnsaved() {
	a.unsavedAt = a.clock()
	for p := range a.unsaved {
		doc := a.docs[p]
		if doc == nil {
			delete(a.unsaved, p)
			continue
		}
		if err := p.Save(encodeBody(doc.Text())); err == nil {
			delete(a.unsaved, p)
			a.linksStale()
		}
	}
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

// bringToFront is the app started again while it runs: its window comes to the front, out of the Dock if it was
// there - or, while the welcome plays, the welcome's.
func (a *app) bringToFront() {
	a.do(func() error {
		if a.wel.clear && a.wel.stage != nil {
			a.wel.stage.Focus()
			return nil
		}
		if w := a.wel.win; w != nil {
			if w.IsMinimized() {
				w.Restore()
			}
			w.Show()
			w.Focus()
		}
		return nil
	})
}
