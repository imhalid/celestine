// celestine: a block editor on MyGo's rich text editor, over a library of
// collections and pages kept as plain Markdown files (library.go). The
// Markdown you type turns into structure as you type it and is never seen.
// The caret is ours: Slime or Mizan.
//
// Run with: go run . [library folder]   (default: ./kitaplik)
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"log"
	"math"
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
	"testing"
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

	ps paneSet // the panes of the window (panes.go); a.pane() is the one in use

	// The state of the app grouped by what it is about (appstate.go, #596): their fields are the app's own,
	// a.links as before, and each says which lock holds it.
	indexState
	updateRuntime
	mcpRuntime
	syncRuntime
	chromeState

	bars         map[string]*funBar      // the scroll bars of the scroll areas, by name (funbar.go)
	nearFn       func(n int) []nearGlyph // the letters near the text's caret, while it is being painted (caret.go)
	wordsFn      caretText               // and the words round it
	caretTry     caretTry                // the caret's playground in the settings (caretplay.go)
	census       census                  // the sidebar's tree, counted once a frame (sidebar.go)
	frameSeq     uint64                  // the frames built, counted
	frameNow     time.Time               // the window's time this frame (c.Now): what the app's own animations are timed by
	paneDrag     paneDragState           // a pane carried by its bar (panebar.go)
	edBuilt      map[uint64]uint64       // the frame each editor (by its element) was last built in (editorIsNew)
	meter        frameMeter              // what CELESTINE_FRAMES counts and the debug mode shows (framemeter.go)
	wframe       frameState              // the window's frame (windowframe.go)
	caretDraws   int                     // the caret's paints, counted: a window at rest has none (TestIdleAsksNoFrames)
	caretFB      caretFallback           // cached editor CaretRect fallback for oversized paragraphs (#213)
	caretFBReads int                     // CaretRect fallback evaluations, for verifying no per-frame layout (#213)
	noSound      bool                    // no sound is heard, whatever the settings say: in tests (sound.go)
	soundAt      map[soundKind]time.Time // when each sound was last heard, so a burst of notices chimes once
	soundPlay    func(soundKind)         // a test hears the sounds here instead

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
	// ryugoOwn are Ryugo's own colors in the window's appearance, before the app's color theme is put on them:
	// what the Default theme's tile shows, whichever theme is chosen (#484)
	ryugoOwn ryugo.Colors
	font     ui.Font

	caretKind caretKind
	jelly     *slime
	mizan     *mizanCaret

	// Shortcuts and macros of one's own, and the settings' search.
	keyCmd       string // the command and the key being typed for a shortcut of one's own
	keyAccel     string
	macroName    string
	macroList    string
	setSearch    string             // the search of the settings
	setFocus     bool               // the settings' search takes the keys in the next frame (/)
	setSlash     bool               // the / that focused the settings' search is still to come as text
	setMode      string             // the settings' special page: "changed", "profiles", "file"
	profName     string             // the name being typed for a profile
	charW        map[string]float32 // the width of n zeros in a font, for the line length (editing.go)
	nt           notifyState        // the system's notifications (notify.go)
	libPath      string             // the library opened: its file, or the folder of a folder library (recent.go)
	subtasks     subtaskState       // the counts of tasks' subtasks (subtasks.go)
	vimReg       richtext.Text      // what dd and yy took, for p
	kb           kbState            // the keyboard's normal mode, the letters on what can be pressed (kb.go)
	wel          welcomeState       // the first opening (welcome.go)
	winOnce      sync.Once          // the window coming forward and going behind (windowEvent)
	winWake      chan struct{}      // the worker is woken to take winLast
	winLast      atomic.Bool        // the window's last state: in front
	codeTheme    *editor.Theme      // the colors of code, in the window's theme (codecolors.go)
	codeLangs    codeLangs          // the languages of the page's lines of code, by paragraph (codecolors.go)
	blink        caretBlink         // when the caret last moved, for it to blink once still (caretBlinkOff)
	bsel         blockSel           // blocks chosen from the keyboard (blocksel.go)
	statCache    libStats           // the library's figures, read at most every ten seconds
	statCacheAt  time.Time
	statCacheGen uint64   // the library's moves when they were read: they are read again only when it moved
	statUnused   []string // the commands never run, found with the figures (statsUse)
	statUnusedAt time.Time
	statLoading  bool                // the first frame of the stats screen is drawn before the figures are read
	setClock     func(time.Duration) // a test moves the clock
	statReset    bool                // the reset asked once: the next click does it
	usage        *usageData          // what is used, by the day (stats.go)
	usageDirty   bool
	usageSaved   time.Time
	activeAt     time.Time     // the last thing done: a minute after it, the time is not counted
	pageFrac     time.Duration // the open page's time gathered, not yet a whole second (statTick)
	pageFracOf   string        // the page it is of
	activeFrac   time.Duration
	statLast     time.Time
	statVer      uint64
	sess         session // the writing session that goes on
	sessLast     time.Time
	backupDir    string // where the backups go, when not the config folder's (a test's)
	backupBusy   atomic.Bool
	bk           backupUI         // the backups' settings: the last by hand, a delete asked (backup.go)
	crashDir     string           // where the logs go, when not the config folder's (a test's)
	crashRT      string           // the runtime's crash log a test has, with crashDir
	menuPanel    func(ui.Element) // the look of the menus' panels in the frame's theme (popmenu.go)
	crashMsg     string           // the panic last written, and when: the same one is not written over and over
	crashAt      time.Time
	crashPath    string                   // its log
	crashText    string                   // what it said
	testPanic    string                   // a test sets it to have a frame panic
	crashOpen    bool                     // the dialog of a crash is shown
	asking       askState                 // a question before an action that cannot be taken back (confirm.go)
	tplAsk       tplAsk                   // the values a template asks for, while they are typed (templates.go)
	mentions     mentionsState            // the open page's unlinked mentions, as last looked for (mentions.go)
	repl         replaceState             // the search's replace across the library (libreplace.go)
	triage       triageState              // a collection's pages sorted one at a time (triage.go)
	crashPrev    string                   // the last session ended badly: its log, or ""
	fonts        fontState                // the typefaces installed, and the lists of them (fonts.go)
	fx           []caretFX                // the carets of the presets, made as they are used (caretfx.go)
	jellied      smear                    // the jelly of the settings, put on a caret (caretfx_look.go)
	sleep        sleeper                  // the caret asleep after five seconds (caretextras.go)
	demos        map[caretKind]*caretDemo // the tiles of the carets in the settings, each at its own game
	cursor       cursorState              // the effect of the mouse pointer (cursorfx.go)
	saveErr      string
	saveTried    time.Time      // when the open page's save last failed: autosave tries again a while after
	savingAway   bool           // a big page's autosave is being made away from the frame (saveAway)
	unsaved      map[*Page]bool // pages left with a save that failed: their texts are kept, and saved again (retryUnsaved)
	unsavedAt    time.Time
	shellOpen    func(string, bool) // test seam for opening/revealing attached files (path, reveal)
	runOnUI      func(func())       // UI-thread dispatcher when the event loop is active (#173)
	daysStamped  bool               // the journal's pages have their days kept (stampDays)
	stamping     sync.WaitGroup     // the journal's days being written: a test waits for them
	stampHold    func()             // a test holds the writing of the journal's days here
	storeErr     string             // the library store's last write error, as shown
	trashed      *TrashItem         // just thrown away: a toast offers it back

	quick         *quickNote // the panel for a note, over everything (quick.go)
	quickKeyDraft string     // the key being typed in the settings
	quickKeyErr   string
	tagRen        tagRename          // the dialog to rename a tag (tags.go)
	vhist         histView           // the dialog of a page's versions (historyui.go)
	tm            timeMachine        // the page's history as a stack and a timeline (timemachine.go)
	tmSide        tmSideState        // what the timeline in the sidebar shows, while it is there and as it fades (tmside.go)
	lapse         lapseState         // the time-lapse of the page, in place of its editor (lapseui.go)
	coverW        float32            // the width the picture cover was last laid out at: its height follows it (cover.go)
	coverMenu     bool               // the cover's menu is open
	coverFix      *Page              // the page whose cover picture is being repositioned: dragged inside its band (cover.go)
	coverDrag     bool               // the picture is held, its position not yet written
	coverAt       float32            // its position while held, finer than the page's whole numbers
	bannerMenu    bool               // and the collection banner's (banner.go)
	cellW         map[string]float32 // how wide a cell's text is drawn, by its text (tables.go)
	win           *mygo.Window       // the window, for its own controls (windowcontrols.go)
	lightbox      *picHit            // the picture shown large
	ptags         []string           // the open page's tags, for the side panel
	ptagsVer      uint64
	ptagsDoc      *richtext.Document
	trashItems    []*TrashItem       // what is in it, read when it opens and changes
	trashPages    map[string][]*Page // the pages each collection in it holds, by its folder in the trash (trash.go)
	trashUnfold   map[string]bool    // the collections in it whose pages are shown
	clock         func() time.Time
	look          string          // the light or dark and the color theme the window was last drawn in
	afterView     []func()        // what waits for the view to be built
	boardOpened   map[string]bool // empty columns the user opened
	rowsLast      []treeEntry     // the tree's rows, last frame
	treeFocus     any             // the row the arrow keys moved to, to take the focus
	treeNav       bool            // the focus is kept in the tree as the page opens: the arrow keys are in use
	treeHeld      bool            // a row has the focus in this frame
	treeSigSeen   uint64
	treeKeptAt    time.Time // when what is open in the tree was last looked at (keepTree)
	cdrag         collDragState
	collRects     map[*Collection]ui.Rect
	ctx           ctxMenu        // the context menu (ctxmenu.go)
	frame         func()         // asks for a frame that builds the view
	web           webLinks       // the web pages links go to, as far as they are known (linkcards.go)
	wholeBlock    wholeBlock     // whether the block at the start of the page's text is all of it (blockhints.go)
	listMarks     *listMarks     // the lists of the page's text and their looks, read once for each text (listmarks.go)
	looks         *listMarks     // the lists that have a look, for the paint: none made for a text with no look (listLooks)
	toggleMu      sync.Mutex     // held by the toggle of a task read and written away from the window (toggleTaskSoon)
	toggling      sync.WaitGroup // the toggles going on: a test waits for them
	dateEdit      dateEdit       // the calendar a date chip opens

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
	importNote    string                             // what the last import said, in the settings
	embedCache    map[string][]string                // the first lines of the pages embedded (embed.go)
	queryCache    map[string]queryResult             // what the query blocks found (queryblock.go)
	queryDocs     map[*richtext.Document]*docQueries // query block attachments tracked per document (queryblock.go)
	queryVer      int                                // incremented when dynamic query block heights change (spansVersion)
	chipW         map[string]float32                 // how wide the text of each chip is (chips.go)
	autoCovers    map[uint64]*autoCover              // the patterns made for the covers of the pages shown lately (autocover.go)
	tables        map[string]*tableUI                // the state of each collection table, by collection and view (views.go)
	sortLists     map[string]*manualState            // each list in its own order, by collection and view (views.go)
	writingSaved  time.Time
	exportNote    string                    // what the last export said, in the settings
	docUse        []*Page                   // the pages whose texts are kept, the longest unused first (touchDoc)
	images        map[string]ui.ImageSource // the pictures read, by their file (images.go), the ones used last (limits.go)
	imageUsed     map[string]uint64         // when each was used last, by imageClock
	imageClock    uint64
	imageMiss     map[string]time.Time                      // the pictures that could not be read, and when: they are tried again after imageRetry (images.go)
	clipText      func(string)                              // the clipboard to write; the system's when nil
	clipWrite     func(string, mygo.ClipboardOptions) error // overrides mygo.Clipboard.Write in tests; nil uses system clipboard
	clipRead      func() string                             // and to read
	ownClip       ownClip                                   // the blocks or chips of the last copy in the editor (clipmodes.go)
	carry         dockCarry                                 // a page or a collection carried from the sidebar over the panes (dockdrop.go)
	rowsBuilt     int                                       // the rows of the tree built in the last frame: the others are gaps (#581)
	nextFrame     []func()                                  // what goroutines found that the paint reads: done as the next frame starts (doNextFrame)
	keychainErr   error                                     // the keychain did not answer as the settings were read: said, not taken for a sign-out
	fieldSels     map[*string]fieldSel                      // a selection the menu of a field left, to set on it (fieldmenu.go)
	edPtr         ui.Point                                  // the pointer over the page's text, last frame
	cellBar       ui.Rect                                   // the bar of the cell being written, as last laid out: the text under it has no pointer there (tables.go)
	tagDrafts     map[string]*string                        // what is typed in the fields of tags and numbers (tagfield.go)
	countEditing  string                                    // the number field being typed in
	tagPop        map[string]bool                           // a field of tags whose last was taken away with ⌫
	clipImage     func() []byte                             // the clipboard's picture; the system's when nil
	outlineOf     []heading                                 // the page's headings, for the side panel
	outlineVer    uint64
	outlineDoc    *richtext.Document
	imgOnClip     atomic.Bool // a picture, and no text, is on the clipboard
	settingsDirty bool        // changed, not yet written (saveSettingsSoon)
	settingsAt    time.Time   // when
	savedAt       time.Time   // the last save of the page, shown in the status bar

	prop        propEdit                                  // the property dialog
	propBuf     map[string]*string                        // the text of the properties' fields, by page and property
	propEditing string                                    // the field being typed in
	propErr     string                                    // the field whose value did not fit
	invalidate  func()                                    // asks the window for a frame, from any goroutine
	pal         palette                                   // the command palette (palette.go)
	sw          setWinState                               // the settings' window (setwin.go)
	newSetWin   func(view func(*ui.Context)) *mygo.Window // a test makes the settings' window here (nil: none, the test draws it)
	settingsGen uint64                                    // counts the changes of the settings, for the other window to follow (setwin.go)
	searchCache map[searchKey][]searchHit                 // the last searches' hits (searchKept)
	viewCache   struct {                                  // the pages of the view shown (viewPages)
		key   viewKey
		pages []*Page
		have  bool
		made  int // how many times, counted for a test
	}
	searchMade int          // the searches made, not taken from searchCache: counted for a test
	boards     boardKept    // the board's columns, as they were last made (boardColumns)
	more       viewMore     // how many pages the list or the gallery shows (viewLimit)
	calDays    calKept      // the calendar's pages by day, as they were last sorted (calendarDays)
	chipHits   []chipHit    // the chips drawn this frame (chips.go)
	chipSeen   []chipHit    // and last frame, for a click
	tableHits  []tableHit   // table cells drawn this paint pass (tables.go -> sealed onto pn)
	borderHits []colBorder  // column borders drawn this paint pass (tables.go -> sealed onto pn)
	picHits    []picHit     // pictures drawn this paint pass (images.go -> sealed onto pn)
	slash      slashMenu    // the / menu (slash.go)
	tasks      taskFx       // the task boxes: what turns, and the confetti (taskkinds.go)
	cs         collSettings // a collection\'s settings (collsettings.go)
	hist       []visited    // where the app went, and where it is in that (nav.go)
	histAt     int
	going      bool // going back or forward: no new place in the history
	menuBar    bool // the menu bar has the shortcuts, not the window
}

func newApp(lib *Library) *app {
	a := &app{lib: lib, docs: map[*Page]*richtext.Document{}, jelly: newSlime(), mizan: &mizanCaret{}, clock: time.Now, noSound: testing.Testing(),
		indexState: indexState{sx: newSearchIndex(), firstFrame: make(chan struct{})}}
	a.ps.states = []*paneState{newPaneState()}
	a.sw.view = a.settingsWindowView
	a.histAt = -1
	a.meter.hud.Store(debugOnStart)
	a.loadSettings()
	a.glassAtLaunch = a.settings.Glass
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
	keychain = systemKeychain() // the secrets of the settings, out of the file (keychain.go); also needed by `celestine mcp` (#165)
	// `celestine mcp`: the app's MCP server for an agent over stdio, by way of the app itself (mcpbridge.go)
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		os.Exit(runMCP(os.Args[2:]))
	}
	// MyGo's own words, menus and text engine speak the window's language, not the system's (i18n.go).
	showLocale = func(tag string) { mygo.App.SetLocale(tag) }
	// One copy of the app at a time: a second one hands its command line to the first, which comes to the front,
	// and goes, before it opens the library - two would write the same file, and each would take the other's mark
	// of running for a crash (crash.go). --mcp goes with it, for the first to turn its MCP server on.
	// A rollback start carries the pid of the copy that restored it and is exiting, takes the flag off its
	// command line, and waits for that copy to exit before taking the single-instance lock (#338).
	var leaving int
	if len(os.Args) > 1 {
		var rest []string
		rest, leaving = withoutRollbackFlag(os.Args[1:])
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
			exec.Command(exe, rollbackArgs()...).Start()
		}
		log.Println(note)
		os.Exit(0)
	}
	var mcpOn bool
	os.Args, mcpOn = withoutMCPFlag(os.Args)
	if len(os.Args) > 1 {
		os.Args = append(os.Args[:1:1], withoutUpdateRestartedFlag(os.Args[1:])...)
	}
	// The library is a file of its own, next to the program: nothing else
	// opens it, and the two move together. A folder named on the command line
	// is a library of plain files, as before; the first time, a folder
	// library found here is taken into the file.
	var (
		lib   *Library
		store *Store
		err   error
	)
	restoreNote, restoreKind := "", toastOK
	arg, libPath := "", "" // libPath: the library opened, its file or its folder, for the recent documents (recent.go)
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}
	if info, serr := os.Stat(arg); arg != "" && serr == nil && info.IsDir() {
		root, _ := filepath.Abs(arg)
		libPath = root
		lib, err = OpenLibraryIn(&countFS{FS: OSFS{}}, root) // its writes counted: the sync goes a moment after them
	} else {
		file := LibraryFile()
		if arg != "" {
			file, _ = filepath.Abs(arg)
		}
		libPath = file
		bdir := backupDir()
		hardenBackupDir(bdir)                                  // tighten existing backups once on startup (#179)
		restored, rerr := applyRestore(bdir, file, time.Now()) // a backup chosen before the last quit is put in place first
		if rerr != nil {
			restored, restoreKind = locales.AppRestoreFailed(rerr.Error()), toastErr
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
	a.libPath = libPath
	if restoreNote != "" {
		a.say(restoreKind, restoreNote)
	}
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
		a.restorePaneLayout()
		a.finishRenames()         // a title being typed when the app last went: its links follow now (#560)
		if a.keychainErr != nil { // its secrets not read: said, as no sign-out is (#163)
			a.say(toastWarn, locales.KeychainNoAnswer())
		}
		setLang(a.settings.Lang) // the window speaks the language that was chosen, or the system's
	}
	if len(lib.Damaged) > 0 { // left out, and left as they are on disk
		a.say(toastErr, locales.AppDamagedCollections(strings.Join(lib.Damaged, "; ")))
	}
	if empty {
		a.markPristine()
	}
	a.wel.on = a.wantWelcome()                           // the first opening, if it is one
	go func() { defer a.guard(); clearOpenedCopies() }() // the copies a session that ended badly left
	a.purgeTrash()                                       // what has been in the trash trashDays is deleted for good
	a.prewarm()                                          // the pages' text, read away from the window
	rt := runtimeCrashLog()
	unclean, prevLog, rtText := markRunning(crashDir(), rt, time.Now()) // the last session that did not end as it should is told
	if unclean {
		a.crashPrev, a.crashOpen = prevLog, true
		switch {
		case rtText != "": // the panic that ended it, as the runtime wrote it
			a.crashPrev, a.crashText, a.crashPath = rt, rtText, rt
		case prevLog == "":
			a.crashText = locales.AppClosedWithoutLog()
		default:
			if b, err := os.ReadFile(prevLog); err == nil {
				a.crashText, a.crashPath = string(b), prevLog
			}
		}
	}
	time.AfterFunc(20*time.Second, func() { updateProved(updateMark(), version) }) // twenty seconds in, it has proved itself
	mygo.App.OnQuit(relaunchAtQuit)                                                // an AppImage started again, as the app goes (relaunch.go)
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {                                  // what is written and not saved goes to its file before the app goes
		if a.quick != nil {
			a.quick.leaving = true
		}
		a.mu.Lock()
		if pn := a.pane(); pn.retitle.p != nil { // a title being typed: its links follow before the app goes
			r := pn.retitle
			pn.retitle = titleEdit{}
			a.relinkRenamed(r.p, r.from, false)
		}
		a.flush()
		a.keepTree(true)     // what was opened in the tree in its last second
		a.ps.kept.ok = false // made whatever it was made from: the last word on the panes
		a.keepPanes()
		a.keepPageIndexes(true) // the pages read again since the last write, for the next start (#594)
		if a.settingsDirty {    // what saveSettingsSoon left
			a.saveSettings()
		}
		a.endSession()
		a.saveUsage()
		a.mu.Unlock()
		a.syncAtQuit(6 * time.Second) // what was written since the last pass goes up before the app goes
		clearOpenedCopies()           // the copies of the one-file library's files that were opened
		markClean(crashDir())
		updateProved(updateMark(), version) // a first start that ended as it should has proved itself too
	})
	defer func() { // a panic of the program itself is written, too, and then goes on
		if r := recover(); r != nil {
			crashLog(crashDir(), r, debug.Stack(), time.Now())
			panic(r)
		}
	}()
	a.watchActive() // before Run: a notification's click that launched the app is told before WhenReady
	a.watchRecent() // and a library chosen from the system's recent documents (recent.go)
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(a.menu())
		a.menuBar = true
		a.watchSync()       // sync waits while offline, and runs at full speed out of sight (netsync.go)
		a.rememberLibrary() // the library opened, in the system's recent documents (recent.go)
		opts := mygo.WindowOptions{Title: "celestine", Width: 1180, Height: 760, MinWidth: 640, MinHeight: 420,
			TitleBarStyle: mygo.TitleBarHiddenInset, TrafficLightPosition: trafficLights(),
			HideWindowButtons: ownWindowControls, Content: ui.View(a.view)}
		if runtime.GOOS == "darwin" {
			// the window's corners concentric with its cards' (cardR) at the Corners setting's measure, where
			// macOS 26 would draw them at 26, the cards then at 18; drawn by the system, they still widen near a
			// display's rounded corners (#446, #600)
			opts.CornerRadius = winCorner(a.cornerPercent())
		}
		if a.settings.Glass && runtime.GOOS == "darwin" {
			opts.Transparent = true
			opts.Vibrancy = mygo.VibrancyGlass
		}
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
			show := onlyOnce(func() {
				w.SetOpacity(0)
				w.Show()
				a.fadeWindow(w, 0, 1, 0, 0.22)
				a.playStartupSound()
			})
			w.OnReadyToShow(show)
			// a platform that never says the frame is drawn still gets its window; the timer's call is made on the main
			// thread, as the window's are, and not under a lock the main thread could be waiting on (#567)
			time.AfterFunc(1500*time.Millisecond, func() { mygo.RunOnMain(show) })
		}
		if ownWindowControls {
			a.win = w
		}
		a.invalidate = w.Invalidate
		a.runOnUI = mygo.RunOnMain
		if runtime.GOOS != "js" { // a browser has one window: the panes stay in it
			a.newDockHost(w)
			a.dockHost.Sync() // the windows of the panes, as they were left
		}
		mygo.App.OnSecondInstance(func(args []string, _ string) {
			a.handleSecondInstance(args)
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
		a.iconShown = "paper"          // the first frame puts the look the settings choose on them (syncIcon)
		if runtime.GOOS != "js" {      // a browser has no global key, and the panel's window would name the tab
			a.quick = newQuick(a)
		}
		go func() { // the day's backup, a moment after the start, when the window is up, then periodic update checks (#327)
			defer a.guard()
			time.Sleep(4 * time.Second)
			a.do(func() error { a.backupDaily(); return nil })
			time.Sleep(4 * time.Second)
			a.do(func() error { a.checkUpdate(false); return nil })
			tk := time.NewTicker(time.Hour)
			defer tk.Stop()
			for range tk.C {
				a.do(func() error { a.checkUpdate(false); return nil })
			}
		}()
		go func() { defer a.guard(); a.watch(context.Background()) }()
		go func() { defer a.guard(); a.watchClipboard() }()
		if a.settings.MCP || mcpOn { // on as it was left, or asked for by `celestine mcp` (do not persist --mcp, #182)
			a.startMCP(true, a.settings.MCP)
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
	a.applyZoom(c) // the window's zoom, before anything is laid out (winzoom.go)
	mode := []ryugo.Mode{ryugo.ModeInherit, ryugo.Light, ryugo.Dark}[a.settings.Theme%3]
	radiusScale = float32(a.cornerPercent()) / 100
	// Ryugo's theme, its components' tokens, for the appearance; a pressed button's wave as the settings say
	opts := ryugo.Options{Mode: mode, Size: ryugo.Sm, Corners: a.ryugoCorners(), CornerScale: a.ryugoScale(), Waves: !a.settings.NoWaves && !fx.still}
	tok := ryugo.Root(c, opts)
	a.menuPanel = menuPanelLook(c) // the menus' look in this theme (popmenu.go)
	a.ryugoOwn = tok.Colors
	if a.appTheme().colors(tok.Dark).bg != "" { // and in the app's theme: the same appearance, in its colors
		opts.Mode, opts.Accent = map[bool]ryugo.Mode{false: ryugo.Light, true: ryugo.Dark}[tok.Dark], a.accentOf(tok.Dark)
		tok = ryugo.Root(c, opts)
		a.skin(tok, c.Theme())
	} else {
		lightLevels(tok) // Ryugo's own light, its steps set apart (#491)
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
	// A heading's lines are closer than the text's, as a type scale sets them, so that a heading that wraps
	// reads as one (#349). Its room above stays under half such a line: the editor moves ↑ by the height of
	// the line the caret is on, and from a heading that must land in the paragraph above, not in the heading's
	// own room. The room below is a little more (0.5 of the text), for the leading it gave up under its last line.
	for i := range th.Headings {
		th.Headings[i].Top, th.Headings[i].Bottom, th.Headings[i].LineHeight = 0.6, 0.5, headingLeading
	}
	a.theme = th
	th.CodeFont = a.monoFont()
	a.font = ui.Font{Family: a.editorFont(), Size: a.fontSize(), LineHeight: a.lineHeight(), Weight: a.fontWeight(), Features: a.fontFeatures()}
	return tok, th
}

// headingLeading is the height of a heading's lines, in ems of its size.
const headingLeading = 1.3

// windowIn and windowOut are the window's coming forward and going behind. They do their work off the window's thread, not in the call
// that raised them: showing or focusing a window is done by a frame that holds the app's lock (the welcome's Başla), and the system
// answers with the focus at once, in that very call. Taking the lock there waited for the frame that was waiting for it, and the
// window froze (TestWindowFocusDoesNotWaitForTheLock). One worker takes the window's last state each time it is woken, so that a window
// that came forward and went behind at once is behind, not forward; a flood of them is the last of them, never one dropped (#545).
func (a *app) windowIn()  { a.windowEvent(true) }
func (a *app) windowOut() { a.windowEvent(false) }

func (a *app) windowEvent(forward bool) {
	a.winOnce.Do(func() {
		a.winWake = make(chan struct{}, 1)
		go func() {
			defer a.guard()
			for range a.winWake {
				fwd := a.winLast.Load()
				a.do(func() error {
					if fwd {
						a.windowFocused()
					} else {
						a.sy.behind = true
					}
					a.winBehind = !fwd // the glass matte behind other windows (glassLive)
					return nil
				})
			}
		}()
	})
	a.winLast.Store(forward)
	select {
	case a.winWake <- struct{}{}:
	default: // woken already: it reads the state when it goes, this one's
	}
}

// open shows page p in the active pane, saving the page that was open first.
func (a *app) open(p *Page) { a.openIn(a.pane(), p) }

// openIn shows page p in pn, saving the page that was open first.
func (a *app) openIn(pn *paneState, p *Page) {
	defer a.visitIn(pn)
	pn.coll, pn.trashOpen, pn.tag, pn.tasksOpen, pn.searchOpen, pn.settingsOpen = nil, false, "", false, false, false
	pn.statsScreen, pn.profileScreen = false, false
	if p == pn.page {
		return
	}
	a.endTitleEditIn(pn)
	if pn == a.pane() {
		a.flush()
	} else if pn.page != nil && pn.dirty && pn.doc != nil {
		a.savePane(pn)
	}
	if pn.page != nil && pn.dirty { // its save failed: its text is kept and saved when it can be, not dropped (#129)
		if a.unsaved == nil {
			a.unsaved = map[*Page]bool{}
		}
		a.unsaved[pn.page] = true
	}
	a.noteOpened(p)
	doc := a.docFor(p)
	pn.page, pn.doc, pn.title, pn.blank = p, doc, p.Title, false
	pn.find.ver = 0 // another text to find in
	_, caret := doc.Selection()
	pn.version, pn.length, pn.caretAt = doc.Version(), doc.Text().Len(), caret
	pn.prevText = doc.Text()
	pn.dirty, a.saveErr, pn.lastSave, pn.lastWords = false, "", "", -1
	if a.unsaved[p] { // back to a page whose save failed: what it shows is still to be saved
		delete(a.unsaved, p)
		pn.dirty, pn.editAt = true, a.clock()
	}
	pn.boxes = nil
	*pn.pageScroll = ui.ScrollState{}
	// a page opened starts clean: the caret is put in its place, not flown
	// there from where it was in the last page, and what was laid out is gone
	a.jelly.placed, a.mizan.seen, a.mizan.glide = false, false, nil
	a.eachFX(func(c caretFX) { c.reset() })
	a.jellied.reset()
	a.sleep.wake()
	pn.editorR, pn.caretPara, pn.edH = ui.Rect{}, -1, 0
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
		doc.SetSelection(0, 0) // a page read starts where it is read from, not after its last letter (#454)
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
	shown := map[*Page]bool{p: true, a.pane().page: true}
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
			delete(a.queryDocs, a.docs[q])
			delete(a.docs, q)
			a.docUse = slices.Delete(a.docUse, i, i+1)
		}
	}
}

// newPage makes a page in c (under parent when not nil) and opens it, its title ready to be typed.
func (a *app) newPage(c *Collection, parent *Page) {
	if c.Dynamic() {
		a.say(toastWarn, locales.PagesAreNotAddedToA())
		return
	}
	if c.Journal { // the journal gets today's page and no other
		a.openToday()
		return
	}
	a.newPageFrom(c, parent, c.Template) // it starts as the collection says
}

// freshTitle readies the title of the page just made in pn to be typed: focused, and empty with "Untitled" as its
// placeholder rather than the word to be typed over. The page keeps "Untitled" as its title, in the file, the
// sidebar and the tabs, until another is typed, and the field shows it again if it is left with nothing typed (#598).
func (a *app) freshTitle(pn *paneState) {
	pn.fresh = true
	if pn.page != nil && untitled(pn.page.Title) {
		pn.title, pn.blank = "", true
	}
}

// flush saves the open page now, if it changed.
func (a *app) flush() {
	pn := a.pane()
	if pn.page != nil && pn.dirty {
		a.savePane(pn)
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
	if frameMeterOn || a.meter.hud.Load() {
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
	if a.frameSeq == 0 && a.firstFrame != nil {
		defer close(a.firstFrame) // the first frame, built: what waited for it goes on (prewarm.go)
	}
	a.frameSeq++
	a.frameNow = c.Now()
	if a.wel.win != nil { // it changes with full screen and back
		a.winRadius = a.wel.win.CornerRadius()
	}
	for _, fn := range a.nextFrame {
		fn()
	}
	a.nextFrame = nil
	a.motionFrame(c) // reduced motion, and the shader effects on or off (motion.go)
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
	pn := a.pane()
	if pn.doc != nil {
		a.track(c)
	}

	a.pageBg = a.bg(c)
	a.syncWallpaper(tok.Dark) // the wallpaper as the frame's ground, read again as it changes (wallpaper.go)
	a.syncGlassTint(c)
	a.syncIcon(tok.Dark)
	// A new look (light or dark, or another color theme) is a new tree: what was there
	// would take each color over a moment of its own - the sidebar, the inputs, the
	// boxes of a checkbox, each at its own pace - and the change would show in pieces.
	look := strconv.FormatBool(tok.Dark) + ":" + strconv.Itoa(a.settings.AppTheme)
	if a.look != "" && a.look != look {
		pn.refocus = true // the editor was rebuilt, and has the focus again
	}
	a.look = look
	c.Key("look:" + look)
	a.welcomeBackdrop(c, false) // the first opening's circles, under the app
	framed := a.framed()
	a.wframe.on = framed
	useGlass := a.glassLive(c)
	if useGlass {
		c.Root().Background(ui.Transparent)
	}
	var root ui.Element
	if framed { // the window's frame around the content, a card in it, and the status bar under (windowframe.go)
		root = ui.Column(c).Fill().AlignItems(ui.Stretch).Padding(0, frameW, 0, frameW)
		switch g, dark := frameGroundOf(c, a.pageBg), tok.Dark; {
		case a.wallActive() && a.wall.ok: // the wallpaper, the same whatever is behind the window (#606)
			root.Draw(func(p *ui.Painter, r ui.Rect) { a.paintWallGround(p, r, g, dark) })
		case !useGlass:
			root.Draw(func(p *ui.Painter, r ui.Rect) { g.paint(p, r) })
		}
	} else {
		bg := a.pageBg
		if useGlass {
			bg = ui.Transparent
		}
		root = ui.Row(c).Fill().AlignItems(ui.Stretch).Background(bg)
	}
	a.welcomeRoot(c, root) // and the app hidden, then opened by a shader
	a.helloRoot(c, root)   // the first start after an update: a wave of light up the window (updhello.go)
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
	if pn.settingsOpen && !a.modal() && c.Shortcut(0, ui.KeyEscape) {
		a.closeSettings()
	}
	if pn.statsScreen && !a.modal() && c.Shortcut(0, ui.KeyEscape) {
		a.closeStats()
	}
	if pn.profileScreen && !a.modal() && c.Shortcut(0, ui.KeyEscape) {
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
	if a.imgOnClip.Load() && c.Shortcut(ui.Cmd, ui.KeyV) && pn.page != nil && pn.doc != nil {
		a.pasteImage()
	}
	if c.Shortcut(ui.Ctrl, ui.KeyN) && pn.page != nil {
		a.newPage(pn.page.Coll, nil)
	}
	content := func() { // the sidebar, the header, the panes and the status bar
		a.kbAnchor(c)
		if !phone { // on a phone the sidebar is a drawer over everything, built last (mobile.go)
			a.sidebarShown = a.sidebar(c)
		}
		col := ui.Column(c).Grow(1).MinWidth(0)
		if framed { // in the window's frame, the gap after the sidebar's card, closing as it folds (windowframe.go)
			col.Margin(0, 0, 0, min(a.sidebarShown, frameGap))
		}
		col.Children(func() { // the header, the content, the status bar: each a region of its own
			if !framed { // in the frame, its top edge has the header's buttons (windowframe.go)
				a.header(c)
			}
			a.panesView(c, th)
			if !framed && !a.settings.HideStatus { // in the frame, the status bar is its bottom edge
				a.status(c)
			}
			a.keepTree(false)
		})
		if phone {
			a.drawer(c)
		} else {
			a.sidePanel(c)
			if !framed {
				a.sidebarButton(c)
			}
		}
	}
	root.Children(func() { // in the frame, the card; without it, the window
		if framed {
			a.frameTop(c)
			ui.Row(c.Key("frame-cards")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(content) // no ground: each its own card
			a.frameBottom(c)
		} else {
			content()
		}
	})
	a.collDragView(c)
	a.paneDragView(c)         // a pane carried by its bar (panebar.go)
	if !a.activeElsewhere() { // the pane in use is in a window of its own, which draws these (panewin.go)
		a.textOverlays(c)
	}
	a.taskPickerView(c) // the picker of a task's status, ⌥-click on its box (taskstyle.go)
	a.contextView(c)
	if !a.kb.normal && !a.modal() && !a.pal.open && !pn.find.open && !a.slash.open {
		// Esc, in the text or on a screen that has nothing to close: the keys are commands, when the settings say
		// so (Vim's motions, or Esc enters the keyboard mode): a writer's Esc by habit, then letters typed, made a
		// new page and an undo (#492, #207). The settings and the figures have their own Esc, which goes back.
		escNormal := a.settings.EscNormal || a.settings.VimKeys
		if escNormal && !pn.settingsOpen && !pn.statsScreen && !pn.profileScreen && !lapseEsc && c.Shortcut(0, ui.KeyEscape) {
			a.kbEnter()
		}
	}
	// The mouse's back and forward buttons step through the places visited, as ⌘[ and ⌘] do, which MyGo
	// matches by the key's place where the layout has no [ (ğ on Turkish Q) (#207).
	if c.Shortcut(0, ui.KeyBack) {
		a.step(-1)
	}
	if c.Shortcut(0, ui.KeyForward) {
		a.step(1)
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
	a.updateView(c)
	a.collSettingsView(c)
	a.propDialog(c)
	a.tagRenameView(c)
	a.historyView(c)
	a.crashView(c)
	a.frameMeterView(c)
	a.askView(c)
	a.tplAskView(c)
	a.triageView(c)
	// the palette over the dialogs: ⌘P with one open shows it above them, not behind (#483)
	a.paletteView(c)
	if a.modal() || a.ctx.open { // under a dialog or a menu the cell is let go
		pn.cellEd.open = false
	}
	a.lightboxView(c)
	a.timeMachineView(c) // the page's history, over everything (timemachine.go)
	if !a.treeHeld {     // the focus is not in the tree: no more arrow keys
		a.treeNav = false
	}
	a.treeHeld = false
	a.runLater()
	if a.notice != "" {
		kind := toastInfo // a notice tells its kind (a.say); one that does not is news
		if a.noticeTone != nil {
			kind = *a.noticeTone
		}
		if undo := a.noticeUndo; undo != nil { // a setting changed: it is one click from taken back
			a.toastOf(kind, msg(a.notice), locales.CommonUndo2(), undo)
		} else if pg := a.noticePage; pg != nil { // a note put away: the page it went to is one click away
			a.toastOf(kind, msg(a.notice), locales.CommonOpen(), func() { a.open(pg) })
		} else {
			a.toastOf(kind, msg(a.notice), "", nil)
		}
		a.notice, a.noticePage, a.noticeUndo, a.noticeTone = "", nil, nil, nil
	}
	if item := a.trashed; item != nil {
		a.trashed = nil
		a.toastAction(locales.TrashMoved(item.Title), locales.CommonUndo2(), func() { a.restore(item) })
		a.loadTrash()
	}
	a.toastView(c)
	a.followSettingsWindow()             // a setting changed here is shown in the settings' window (setwin.go)
	a.cursorFXView(c, root)              // over everything, the pointer's effect when one is chosen (cursorfx.go)
	pn = a.pane()                        // active pane may have changed during panesView/runLater
	if pn.doc != nil && pn.coll == nil { // the boxes were read as the page's editor was built (pageBody)
		a.autosave(c)
	}
}

// bg is the page's background: Ryugo's, but in the dark not its black -
// its card's dark gray, a step below the sidebar.
func (a *app) bg(c *ui.Context) ui.Color {
	if t := ryugo.From(c); a.appTheme().colors(t.Dark).bg == "" { // Ryugo's own, dark and light: the page is its card (#491)
		return t.Colors.Card
	}
	return c.Theme().Background // a theme's page is its own color
}

// pageView is the open page in pn: its title, its properties and its text, one
// page that scrolls as a whole. The editor is as tall as its text (and some
// more, to lay out what is coming), so it has nothing to scroll in itself,
// and the wheel passes to the scroll around it; where the caret is is kept
// in view here (drawCaret asks for it).
func (a *app) pageView(c *ui.Context, pn *paneState, th *richtext.Theme) {
	p := pn.page // where it is shows in the title bar
	// the page is as wide as the window and scrolls in it, the bar at its edge;
	// the text is a column in the middle, with margins either side
	ui.Column(c).Fill().Children(func() {
		sc := ui.Scroll(c.Key("page-scroll:" + p.Dir)).Grow(1)
		a.scrolledPane(c, pn, "page-scroll:"+p.Dir, pn.pageScroll, sc, func() {
			if pn.topSpace > 0 {
				ui.Box(c.Key("page-top")).FillWidth().Height(pn.topSpace).DragWindow()
			}
			// a cover is a band behind the top of the page, over its whole width
			wrap := ui.Column(c.Key("page-wrap")).FillWidth()
			if _, y, over := wrap.PointerPosition(); over && !a.modal() && a.coverKind(p) != coverKindNone {
				a.coverHover = y < a.coverBandOf(p)
			} else {
				a.coverHover = false
			}
			a.paintCover(c, p, wrap)
			wrap.Children(func() {
				a.coverButtons(c, p)
				ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, pageMargin()).Children(func() {
					ui.Column(c).FillWidth().MaxWidth(a.pageWidth()).Gap(4).Children(func() { a.pageBody(c, pn, p, th) })
				})
			})
		})
		// a tree built anew (another theme) has no bounds for its page in its first frame: the last ones stand, or what
		// is drawn over the text would be worked out for every paragraph of a long page, not those in view (#506)
		if b := sc.LayoutBounds(); b.H > 0 {
			pn.scrollR = b
		}
		a.findBar(c, pn) // over the page, at its top right: nothing under it moves
	})
}

// pageBody is the title, the properties and the text.
func (a *app) pageBody(c *ui.Context, pn *paneState, p *Page, th *richtext.Theme) {
	head := ui.Column(c.Key("page-head")).FillWidth()
	head.Children(func() { a.pageHead(c, pn, p) })
	pn.headH = head.LayoutBounds().H
	a.lapseAuto(c, pn, p)                  // the time-lapse, as the settings say (lapseui.go)
	if a.lapseOn(p) && a.lapse.solo != 0 { // one day's words alone, to look at: nothing to type in
		pn.focused = false
		a.lapseView(c)
		a.dayExtras(c, pn, p)
		a.backlinksView(c, pn, p)
		a.mentionsView(c, pn, p)
		return
	}
	if p.Unreadable != "" { // shown, not edited: the editor would save what it shows over the file
		ui.Text(c, locales.AppThisPageCouldNot()+"\n"+p.Unreadable).FontSize(13).TextColor(ryugo.From(c).Colors.Warn).Wrap().FillWidth().Padding(0, 0, 8, 0)
	}
	c.Key("body:" + p.Dir)
	a.refreshQueries()
	textSpacing = spacingOf(a.settings.LetterSpacing, a.font.Size) // the editor's spans and the caret's layout (codecolors.go)
	linkPreviews = !a.settings.NoLinkPreviews                      // links' pages asked for, and shown on their cards (linkcards.go)
	linkIcons = linkPreviews && !a.settings.NoLinkIcons            // the room of a link's icon, as well
	a.codeTheme = editor.DefaultTheme(c.Theme())
	ed := richtext.EditorBase(c, pn.doc, richtext.EditorOptions{
		Label: locales.Page(), Font: a.font, Theme: th,
		Placeholder: locales.AppEditorPlaceholder(),
		Menu:        a.editorMenu,                                                                                          // the app's menu, not the system's (fieldmenu.go)
		OnKey:       func(k ui.Key, m ui.Modifiers) bool { return a.editorKeys(pn, k, m) },                                 // the / menu's keys, and copy and paste, before the editor's (slash.go, clipmodes.go)
		OnCommand:   func(name string) bool { return a.editorCommand(pn, name) },                                           // the menus' copy and paste, as their keys (clipmodes.go)
		Spans:       func(i int, st richtext.ParagraphStyle, s []ui.Span) []ui.Span { return a.editorSpans(pn, i, st, s) }, // code colored, letters spaced (codecolors.go)
		// a click on a link is the app's to open (openLink, from the editor's click): richtext's own open of it,
		// through the system, opened a web link a second time (#578)
		OnLink:       func(string) {},
		SpansVersion: a.spansVersion(pn),
		Space:        func(i int) float32 { return a.paraRoom(pn, i) }, // the room a block dragged opens where it would go (blockui.go), and a parent task's chip (subtasks.go)
		ReadOnly:     p.Unreadable != "",
		// the page scrolls as a whole, the editor as tall as its text, however long: no bar of its own beside the
		// page's, which showed when a split pane narrowed it and the height worked out was a frame behind (#190), and
		// no window on a long text scrolling in itself (#314)
		NoScrollbar: true,
	}).FillWidth().Height(a.editorH(c, pn)).FocusRing(false) // the caret shows where the text has the focus
	// this editor's paragraphs, read right after it is built: with the page open in two panes, each pane's are its
	// own, laid out at its width, and its height is worked out from them (#190)
	pn.boxes = pn.doc.VisibleParagraphs()
	// a new editor (another page, another look) takes the keyboard, as AutoFocus would give it, but the page stays where
	// it is: AutoFocus scrolls the editor into view, which for a text taller than the window puts its top at the
	// window's and the title out of sight, a moment after the page opened at its top. The caret is brought into view, if
	// it must be, by revealCaret.
	if a.editorIsNew(ed) && !pn.titleFocused && !pn.fresh && !a.treeNav && !a.isInactivePane(pn) && !a.kb.normal {
		ed.Focus(ui.FocusOptions{PreventScroll: true})
	}
	if pn.refocus && !a.isInactivePane(pn) {
		ed.Focus(ui.FocusOptions{PreventScroll: true})
		pn.refocus = false
	}
	pn.focused = ed.Focused()
	if pn.focused && pn.cellEd.open && pn.cellEd.was { // the text took the keyboard back from a cell: it is written and let go
		a.commitCellIn(pn)
		pn.cellEd.open = false
	}
	a.tabKey(c, pn)
	a.tableKeys(c, pn)
	pn.edPressed = ed.Pressed()
	a.trackGrip(pn, ed)
	a.settleRooms(pn, ed) // the hole a block dropped left, shutting (blockui.go)
	a.hoverCh, a.hoverTag = nil, ""
	pn.tagSeen, pn.tagHits = pn.tagHits, pn.tagSeen[:0] // this frame's are drawn after
	pn.linkSeen, pn.linkHits = pn.linkHits, pn.linkSeen[:0]
	x, y, over := a.textPointer(pn, ed)
	a.hoverLink(c, pn, pn.editorR.X+x, pn.editorR.Y+y, over) // its card, after a moment (linkcards.go)
	if over {
		a.edPtr = ui.Point{X: x, Y: y} // where a right click opens the menu
		if key, ok := a.tagAt(pn, pn.editorR.X+x, pn.editorR.Y+y); ok {
			a.hoverTag = key
			ed.Cursor(ui.CursorPointer)
		}
		if a.tableCellAt(pn, pn.editorR.X+x, pn.editorR.Y+y) != nil {
			ed.Cursor(ui.CursorPointer)
		}
		if _, ok := a.linkAt(pn, pn.editorR.X+x, pn.editorR.Y+y); ok {
			ed.Cursor(ui.CursorPointer)
		}
		if a.pictureAt(pn, pn.editorR.X+x, pn.editorR.Y+y) != nil {
			ed.Cursor(ui.CursorPointer)
		}
		if h := a.chipAt(pn, pn.editorR.X+x, pn.editorR.Y+y); h != nil {
			a.hoverCh = h.ch
			ed.Cursor(ui.CursorPointer)
		}
	}
	for _, f := range ed.DroppedFiles() { // files dropped on the text
		a.dropFile(f, true)
	}
	a.dragColumns(pn, ed)
	if ed.Clicked() { // on a tag: its pages; on a chip: its page, or its day
		x, y, _ := ed.PointerPosition()
		sa, sc := pn.doc.Selection()
		selecting := sa != sc
		if h := a.tableCellAt(pn, pn.editorR.X+x, pn.editorR.Y+y); h != nil {
			pn.doc.SetSelection(pn.selPrevA, pn.selPrevC)
			a.clickCell(pn, h)
		} else if selecting { // the pointer selected text, it did not click what it ended on
		} else if key, ok := a.tagAt(pn, pn.editorR.X+x, pn.editorR.Y+y); ok {
			a.openTagIn(pn, key)
		} else if url, ok := a.linkAt(pn, pn.editorR.X+x, pn.editorR.Y+y); ok {
			a.openLinkIn(pn, url)
		} else if pic := a.pictureAt(pn, pn.editorR.X+x, pn.editorR.Y+y); pic != nil {
			a.lightbox = pic
		} else {
			a.clickChip(pn, pn.editorR.X+x, pn.editorR.Y+y)
		}
	}
	if !pn.edPressed { // where the caret is when the pointer is not down: the click's own move is not kept
		pn.selPrevA, pn.selPrevC = pn.doc.Selection()
	}
	ed.DrawOver(func(p *ui.Painter, r ui.Rect) {
		pn.editorR = r
		a.drawBlockSel(pn, p, r)
		a.drawMatches(pn, p, r)
		a.drawLinkPending(pn, p, r) // the text a link is being made for, and one just made (linkfx.go)
		a.drawLinkWon(pn, p, r)
		a.drawBlockDrop(pn, p, r)
		a.drawDividers(pn, p, r)
		a.drawListMarks(pn, p, r)
		a.drawTasks(pn, p, r)
		a.drawWhite(pn, p, r)
		a.drawFocusVeil(pn, p, r)
		a.drawTags(pn, p, r)
		a.sealTables(pn)
		a.sealPics(pn)
	})
	a.dayExtras(c, pn, p)
	a.backlinksView(c, pn, p)
	a.mentionsView(c, pn, p)
}

// caretView draws the caret in a layer over the window - the editor clips
// what it draws at its own edge, which cut the caret at the start of a
// line - itself clipped to the page's room.
func (a *app) caretView(c *ui.Context) {
	pn := a.pane()
	if pn.doc == nil || a.noPageOf(pn) || pn.page == nil {
		return
	}
	ww, wh := c.Size()
	a.winRect = ui.Rect{W: ww, H: wh}
	ui.Overlay(c, func() {
		ui.Box(c.Key("caret-layer")).Absolute().Left(0).Top(0).Size(ww, wh).PassThrough().Draw(func(p *ui.Painter, _ ui.Rect) {
			a.drawConfetti(p)
			a.drawCaret(p, pn, pn.editorR)
			if tc := pn.titleCaret; tc.on {
				a.paintCaret(p, pn, tc.r, tc.cw, tc.box, kick{wrote: tc.wrote, edited: tc.edited}, tc.sel)
			}
		})
	})
}

// pageHead is the page's title and its properties.
func (a *app) pageHead(c *ui.Context, pn *paneState, p *Page) {
	ink := a.coverInk(c, p)
	if a.coverKind(p) != coverKindNone { // the room of the cover above the title (cover.go)
		ui.Box(c.Key("cover-room")).FillWidth().Height(a.coverRoomOf(p))
	}
	if pn.retitle.p != nil && pn.retitle.p != p { // the pane went on to another page while its title was typed
		a.endTitleEditIn(pn)
	}
	c.Key("title:" + p.Dir)
	if p.Coll.Journal { // a day's title is the journal's format, not typed
		c.Key("title:" + p.Dir)
		ttl := ui.Text(c, p.Title).FontSize(a.titleSize(p.Title, pn.titleW)).FontWeight(titleWeight).Padding(6, 0, 10, 0).Label(locales.CommonTitle()).Selectable().Wrap().FillWidth().TextColor(ink)
		a.titleMenu(pn, ttl, p, false)
		a.titleRoom(c, pn, ttl, false)
		pn.titleFocused = false
		a.coverGap(c, p)
		a.propsSection(c, pn, p)
		return
	}
	// the title wraps, at a size its length gives it, and is as high as its lines (titlesize.go); it is one line of
	// text all the same: a line break typed or pasted is not kept
	shown := pn.title // what the field shows, its placeholder while a new page's title is empty: what it is measured by
	if shown == "" {
		shown = locales.CommonUntitled()
	}
	size := a.titleSize(shown, pn.titleW)
	_, textH := a.titleLayout(shown, size, pn.titleW).Size()
	before := pn.title
	title := ui.TextAreaBase(c, &pn.title).Label(locales.CommonTitle()).FontSize(size).FontWeight(titleWeight).Padding(6, 0, 10, 0).
		FillWidth().Height(textH + 16).TextColor(ink).CaretWidth(0)
	if pn.blank {
		title.Placeholder(locales.CommonUntitled()).PlaceholderColor(ink.Alpha(0.32))
	}
	a.titleMenu(pn, title, p, true)
	a.titleRoom(c, pn, title, true)
	enter := false
	if strings.ContainsAny(pn.title, "\r\n") {
		// the area takes ↵ as a break: one break typed into the title as it was is ↵, which goes into the text below,
		// as from the one-line input; a break pasted is a space
		if enter = strings.Count(pn.title, "\n") == 1 && strings.Replace(pn.title, "\n", "", 1) == before; enter {
			pn.title = before
		} else {
			pn.title = strings.Join(strings.Fields(pn.title), " ")
		}
	}
	if pn.fresh && !a.isInactivePane(pn) {
		title.AutoFocus()
		pn.fresh = false
	}
	if title.Changed() {
		name := strings.TrimSpace(pn.title)
		if name == "" {
			name = locales.CommonUntitled()
		}
		a.titleTyped(pn, p, name)
	}
	if pn.blank && pn.titleFocused && !title.Focused() { // a new page's title left: "Untitled" if nothing was typed
		pn.blank = false
		if strings.TrimSpace(pn.title) == "" {
			pn.title = p.Title
		}
	}
	pn.titleFocused = title.Focused()
	if !pn.titleFocused && pn.retitle.p == p { // the field left: the rename is done, and its links follow
		a.endTitleEditIn(pn)
	}
	pn.titleCaret = titleCaretSpec{}
	if b := title.LayoutBounds(); title.Focused() && b.W > 0 { // the caret is ours, as the page's
		anchor, caret := title.TextAnchorCaret()
		cr, cw := a.inputCaretIn(pn.title, caret, size, titleWeight, pn.titleW, b.X, b.Y+6, b.H-16)
		n := utf8.RuneCountInString(pn.title)
		spec := titleCaretSpec{on: true, r: cr, cw: cw, box: b, sel: anchor != caret}
		if pn.titleSeen && n != pn.titleLen {
			spec.edited, spec.wrote = true, max(n-pn.titleLen, 0)
		}
		pn.titleSeen, pn.titleLen = true, n
		pn.titleCaret = spec
	} else if !title.Focused() {
		pn.titleSeen = false
	}
	a.coverGap(c, p)
	// ↓ or ↵ in the title goes into the text
	if title.Focused() && (enter || title.Shortcut(0, ui.KeyDown) && a.titleLastLine(pn, size, titleCaretOf(title))) {
		pn.doc.SetSelection(0, 0)
		pn.refocus = true
	}
	a.propsSection(c, pn, p)
}

// editorH is editorHeight, asking for another frame while it grows.
func (a *app) editorH(c *ui.Context, pn *paneState) float32 {
	h := a.editorHeight(pn)
	if h != pn.edH {
		pn.edH = h
		c.Invalidate()
	}
	return h
}

// editorHeight is the editor's: its text, and more while there is text
// still to lay out; at least what is left of the page's room.
func (a *app) editorHeight(pn *paneState) float32 {
	least := max(pn.scrollR.H-pn.topSpace-pn.headH-pn.linksH-pn.dayH-8, 160)
	// the text's height as the editor knows it, every paragraph measured or as tall as it was: with another theme or
	// font the editor lays out again only what shows, and the rest as it comes into view (#506)
	if h, ok := pn.doc.ContentHeight(); ok {
		return max(h+96, least)
	}
	n := len(pn.boxes)
	if n == 0 {
		// nothing laid out yet (a page just opened): room for what the text
		// should take, more than it will, so the editor does not scroll
		// itself to the caret and then slide as it grows. However long the
		// text: a long page is laid out whole in its first frame, once, rather
		// than its height growing frame after frame, the page's bar with it (#314)
		t := pn.doc.Text()
		line := a.fontSize() * a.lineHeight()
		return min(max(least, (float32(t.ParagraphCount())+float32(t.Len())/55)*line*1.5+240), 4e6)
	}
	last := pn.boxes[n-1]
	slack := max(pn.scrollR.H*2, 1600)
	if last.Index >= pn.doc.Text().ParagraphCount()-1 {
		slack = 96 // the last paragraph is in
	}
	return max(last.Top+last.Height+slack, least)
}

// track notes what the user did since the last frame: letters written (for
// Mizan's pace), any other edit, and that the file needs saving.
func (a *app) track(c *ui.Context) {
	pn := a.pane()
	v := pn.doc.Version()
	_, caret := pn.doc.Selection()
	text := pn.doc.Text()
	style := text.ParagraphStyleAt(caret)
	if v == pn.version {
		if a.slash.open && caret != pn.caretAt { // the caret moved: ↑ ↓ in the / menu
			a.trackSlash(text, caret, false, false)
			_, caret = pn.doc.Selection()
		}
		if caret != pn.caretAt {
			pn.revealWant = true
		}
		pn.caretAt, pn.style = caret, style
		return
	}
	if a.keepHidden(pn.prevText, text, caret) { // a fold's or a zoom's chip taken away: what it held, back
		_, caret = pn.doc.Selection()
		text, v = pn.doc.Text(), pn.doc.Version()
		style = text.ParagraphStyleAt(caret)
	}
	n := text.Len()
	// an edit that moves nothing - a box ticked, a mark changed - does not take the page to the caret:
	// the caret may be far from what was clicked
	if caret != pn.caretAt || n != pn.length {
		pn.revealWant = true
	}
	// An undo or a redo puts text back as it was: it was not typed now, and what typing turns into something else
	// (a chip, a divider, the / menu) is not done to it again, as a new edit that took the redo away (#210). After an
	// undo the document can redo, and after anything typed it cannot. A few letters to the caret are typed, as two
	// keys that came in one frame (a slow frame, a key held) are: "---" with the last two at once is a divider.
	undone := pn.doc.CanRedo()
	typed := func() bool {
		grew := n - pn.length
		return !undone && grew >= 1 && grew <= 4 && caret-pn.caretAt == grew
	}
	// the / menu (slash.go)
	if a.trackSlash(text, caret, true, !undone && n-pn.length == 1 && caret-pn.caretAt == 1) {
		_, caret = pn.doc.Selection()
		text, v = pn.doc.Text(), pn.doc.Version()
		style = text.ParagraphStyleAt(caret)
		n = text.Len()
		pn.length, pn.caretAt = n, caret
	}
	// "---" and "```" typed: a divider, a block of code (blocks.go)
	if a.blockShortcuts(text, caret, typed()) {
		_, caret = pn.doc.Selection()
		text, v = pn.doc.Text(), pn.doc.Version()
		style = text.ParagraphStyleAt(caret)
		pn.length, pn.caretAt = text.Len()-1, caret-1 // still the letter typed, to the caret
		n = text.Len()
	}
	// "[[Page]]": the "]" typed over the one the "[[" wrote, then a chip (chips.go)
	// typed, as it was before the one ] typed over the one written is taken away
	keyed := typed()
	if a.overtype(text, caret, !undone && n-pn.length == 1 && caret-pn.caretAt == 1) {
		text, v = pn.doc.Text(), pn.doc.Version()
		n = text.Len()
	}
	if a.chipShortcut(text, caret, keyed) || a.checkShortcut(text, caret, keyed) || a.expandSnippet(text, caret, keyed) {
		_, caret = pn.doc.Selection()
		text, v = pn.doc.Text(), pn.doc.Version()
		style = text.ParagraphStyleAt(caret)
		n = text.Len()
		pn.length, pn.caretAt = n, caret
	}
	// pasted text with [[links]] in it: they are chips, as if typed (a link copied from
	// the page's menu is such text)
	if grew := n - pn.length; !undone && grew > 1 && caret-pn.caretAt == grew && (a.tableInPasted(caret-grew, caret) || a.chipsInPasted(caret-grew, caret)) {
		_, caret = pn.doc.Selection()
		text, v = pn.doc.Text(), pn.doc.Version()
		style = text.ParagraphStyleAt(caret)
		n = text.Len()
		pn.length, pn.caretAt = n, caret
	}
	switch grew := n - pn.length; {
	case grew > 0 && caret-pn.caretAt == grew && grew <= 8:
		pn.wrote += grew
		pn.typed += strings.ReplaceAll(text.Slice(caret-grew, caret).String(), string(richtext.ObjectReplacement), "")
	case grew < 0 && grew >= -48 && caret == pn.caretAt+grew: // Backspace: what was behind the caret
		a.eraseOf(pn.prevText, caret, pn.caretAt, -1)
		pn.edited = true
	case grew < 0 && grew >= -48 && caret == pn.caretAt: // Delete: what was ahead
		a.eraseOf(pn.prevText, caret, caret-grew, 1)
		pn.edited = true
	default:
		pn.edited = true
	}
	// A Markdown shortcut ("[] ", "# "…) just turned an empty paragraph into
	// another kind: what is typed there starts plain. (MyGo's richtext took
	// the empty paragraph's attributes from its end mark, which a bold word
	// closed at the end of the paragraph above left bold; seen with v0.7.4.)
	if s, e := text.ParagraphRange(caret); n < pn.length && style != pn.style && s == e {
		pn.doc.SetTypingAttributes(richtext.Attributes{})
	}
	pn.version, pn.length, pn.caretAt, pn.style = v, n, caret, style
	pn.prevText = text
	pn.dirty, pn.editAt, a.saveErr = true, a.clock(), ""
}

// revealCaret scrolls the page, when the caret was moved, to bring it into
// view with some room around it.
func (a *app) revealCaret(pn *paneState, cr ui.Rect) {
	if !pn.revealWant || pn.scrollR.H == 0 {
		return
	}
	pn.revealWant = false
	const room = 56
	vp := pn.scrollR
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
		pn.pageScroll.Y = max(0, pn.pageScroll.Y+d)
		if d < 0 && pn.pageScroll.Y < pn.headH+room { // near the top: the title and the properties too
			pn.pageScroll.Y = 0
		}
		if a.frame != nil {
			a.frame()
		}
	}
}

// caretFallback caches CaretRect layout lookups for oversized paragraphs whose runes fail direct positioning.
type caretFallback struct {
	doc   *richtext.Document
	ver   uint64
	spans int
	font  ui.Font
	at    int
	w     float32
	r     ui.Rect
}

// drawCaret paints the custom caret over the editor, falling back to cached CaretRect on oversized paragraphs (#213).
func (a *app) drawCaret(p *ui.Painter, pn *paneState, r ui.Rect) {
	if !pn.focused {
		a.caretDraws++
		// the find bar has the focus, and the match it steps to is the selection: the page goes to it all the same.
		// Only the caret's place is needed, so it is the editor's own (CaretRect), not laid out again (#306).
		if pn.revealWant && pn.find.open {
			if cr, ok := pn.doc.CaretRect(); ok {
				cr.X += r.X
				cr.Y += r.Y
				a.revealCaret(pn, cr)
			}
		}
		return
	}
	_, caret := pn.doc.Selection()
	cb, ok := a.locateRune(pn, caret, r.W)
	if !ok {
		b, hasBox := a.boxAt(pn, caret)
		if !hasBox && !pn.revealWant {
			return
		}
		if hasBox && pn.scrollR.H > 0 && !pn.revealWant && (r.Y+b.Top+b.Height <= pn.scrollR.Y || r.Y+b.Top >= pn.scrollR.Y+pn.scrollR.H) {
			return
		}
		ver, spans := pn.doc.Version(), a.spansVersion(pn)
		var cr ui.Rect
		fb := &a.caretFB
		if fb.doc == pn.doc && fb.ver == ver && fb.spans == spans && fb.font == a.font && fb.at == caret && fb.w == r.W {
			cr = fb.r
		} else {
			var cok bool
			cr, cok = pn.doc.CaretRect()
			if !cok {
				return
			}
			a.caretFBReads++
			*fb = caretFallback{doc: pn.doc, ver: ver, spans: spans, font: a.font, at: caret, w: r.W, r: cr}
		}
		para, top := 0, cr.Y
		if hasBox {
			para, top = b.Start, b.Top
		} else if pn.doc != nil {
			s, _ := pn.doc.Text().ParagraphRange(caret)
			para = s
		}
		cw := cr.H * 0.5
		if cw < 1 {
			cw = a.font.Size * 0.5
		}
		cb = caretBox{r: cr, full: cr, cw: cw, top: top, para: para}
	}
	// scrolled: the caret went with its paragraph, the text under it the
	// same - the jelly goes with it and is not kicked
	top := r.Y + cb.top
	if cb.para == pn.caretPara && cb.r.Y-cb.top == pn.caretRel && top != pn.caretTop {
		a.jelly.shift(0, top-pn.caretTop)
		a.mizan.shift(top - pn.caretTop)
		a.eachFX(func(c caretFX) { c.shift(top - pn.caretTop) })
	}
	pn.caretPara, pn.caretRel, pn.caretTop = cb.para, cb.r.Y-cb.top, top
	cr := cb.r
	cr.X += r.X
	cr.Y += r.Y
	pn.caretWin = cr
	a.revealCaret(pn, cr)
	if pn.scrollR.H > 0 && (cr.Y+cr.H <= pn.scrollR.Y || cr.Y >= pn.scrollR.Y+pn.scrollR.H) {
		return
	}
	a.caretDraws++
	anchor, caret := pn.doc.Selection()
	if a.caretBlinkOff(p, pn, anchor, caret) { // the off half of a blink
		return
	}
	if anchor != caret || a.settings.NoCaretFX || fx.still { // over a selection, with the effects off or reduced motion, a plain caret: the jelly would hide what is selected
		a.paintCaret(p, pn, cr, cb.cw, r, kick{}, true)
		return
	}
	k := kick{wrote: pn.wrote, edited: pn.edited, erased: pn.erased, dir: pn.eraseDir, gone: pn.gone, typed: pn.typed, style: a.styleAt(caret)}
	pn.wrote, pn.edited, pn.erased, pn.gone, pn.typed = 0, false, 0, "", ""
	doc, boxes, font, th := pn.doc, pn.boxes, a.font, a.theme
	a.nearFn = func(n int) []nearGlyph { return nearLetters(doc, boxes, r.X, r.Y, r.W, font, th, n) }
	a.wordsFn = &caretWords{doc: doc, boxes: boxes, x: r.X, y: r.Y, w: r.W, font: font, th: th}
	a.paintCaret(p, pn, cr, cb.cw, r, k, false)
	a.nearFn, a.wordsFn = nil, nil
}

// caretBlink is where the caret last was, and since when: it blinks only once it has been still.
type caretBlink struct {
	anchor, caret int
	since         time.Time
}

// caretBlinkOff reports whether the caret is in the off half of its blink: with the blink set, once it has stood
// still a moment, it is hidden and shown again every CaretBlink ms; typing or moving it shows it at once.
func (a *app) caretBlinkOff(p *ui.Painter, pn *paneState, anchor, caret int) bool {
	rate := time.Duration(a.settings.CaretBlink) * time.Millisecond
	now := p.Now()
	b := &a.blink
	if b.anchor != anchor || b.caret != caret || pn.wrote > 0 || pn.edited || b.since.IsZero() {
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
func (a *app) paintCaret(p *ui.Painter, pn *paneState, cr ui.Rect, cw float32, bounds ui.Rect, k kick, selecting bool) {
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
	var room ui.Rect
	if pn != nil {
		room = pn.scrollR
	}
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
		styledDraw(p, f, boxOf(f), func() { a.jelly.draw(p, cr, cw, cursorColor(p), dark) })
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
	if !a.wframe.on { // in the frame, the card's edge is the line
		ui.Box(c.Key("status-line")).FillWidth().Height(1).Background(ryugo.From(c).Colors.Hairline) // the page ends here
	}
	bar := ui.Row(c.Key("status-bar")).Padding(3, 16, 3, 10).Justify(ui.End).AlignItems(ui.Center).Gap(12)
	a.statusH = bar.LayoutBounds().H // last frame's: what the notes at the window's bottom keep clear of (updhello.go)
	bar.Children(func() {
		a.statusHints(c)
		// the MCP server on: a dot of the accent, which opens the settings
		if a.mcpServer != nil {
			m := ui.Row(c.Key("mcp-status")).Gap(5).Padding(2, 6).Radius(rad(6)).AlignItems(ui.Center).
				Label(locales.AppMcpSettings()).Role(ui.RoleButton)
			ryugo.Tooltip(c, m, locales.AppMcpServerOn(a.mcpURL()))
			if m.Hovered() {
				m.Background(t.SurfaceHover)
			}
			m.Children(func() {
				a.pulseDot(c)
				ui.Text(c, locales.Mcp()).FontSize(11.5).FontWeight(600).TextColor(t.TextMuted)
			})
			if m.Clicked() {
				a.openSettingsAt(tabMCP)
			}
		}
		a.agentStatus(c)
		a.syncBadge(c)
		a.statusExtras(c)
		// no file paths: what the settings ask for - the letters, how it is saved, the caret
		if a.settings.ShowChars {
			_, chars := a.docStats()
			ui.Text(c, locales.StatusChars(chars)).FontSize(11.5).TextColor(t.TextMuted)
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
			LinearGradient(ryugo.InOklch(ui.LinearGradient{Angle: 180, From: ryugo.Mix(base, ui.Hex("#ffffff"), 0.55), To: ryugo.Mix(base, ui.Hex("#000000"), 0.1)})).
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
	pn := a.pane()
	if !pn.dirty || pn.page == nil || a.settings.NoAutoSave {
		return
	}
	if a.saveErr != "" { // it failed: again a while after, not at every frame
		if wait := retrySave - a.clock().Sub(a.saveTried); wait > 0 {
			c.After(wait)
			return
		}
	}
	if wait := a.saveAfter() - a.clock().Sub(pn.editAt); wait > 0 {
		c.After(wait)
		return
	}
	if pn.doc.Text().Len() >= bigSave {
		a.saveAway()
		return
	}
	a.savePane(pn)
}

// bigSave is the length of a page's text, in letters, from which its autosave is made away from the frame: a page of
// a book's length (500 000 letters) takes some 30 ms to write and count, a pause in the typing that follows (#198).
// A smaller one is written in the frame, as a save asked for (flush, ⌘S) always is.
const bigSave = 100_000

// saveAway writes the open page as autosave does, its text made and counted in a goroutine and written in a.do. What
// it wrote is the page as it was then: if the page changed meanwhile, was saved, or another is open, nothing is
// written, and the next autosave writes it as it is.
func (a *app) saveAway() {
	if a.savingAway {
		return
	}
	pn := a.pane()
	a.rollRecurring()
	p, doc, t, ver := pn.page, pn.doc, pn.doc.Text(), pn.doc.Version()
	a.savingAway = true
	go func() {
		defer a.guard()
		defer a.do(func() error { a.savingAway = false; return nil }) // a panic too: the next autosave can begin
		body := encodeBody(t)
		words := len(strings.Fields(unfoldText(t).PlainText()))
		a.do(func() error {
			if pn.page != p || pn.doc != doc || doc.Version() != ver || !pn.dirty || a.settings.NoAutoSave {
				return nil
			}
			a.wroteBody(pn, body, words)
			return nil
		})
	}()
}

func (a *app) save() { a.savePane(a.pane()) }

func (a *app) savePane(pn *paneState) {
	a.rollRecurring()
	a.wroteBody(pn, encodeBody(pn.doc.Text()), -1)
}

// wroteBody writes body as pn's page's file, and notes it: words are its words, counted with a fold's lines too
// (folding is not writing, #128), or -1 to count them here, which is done only when the body is new.
func (a *app) wroteBody(pn *paneState, body string, words int) {
	if body == pn.lastSave {
		pn.dirty = false
		return
	}
	if err := pn.page.Save(body); err != nil {
		a.saveErr, a.saveTried = err.Error(), a.clock()
		return
	}
	a.saveErr = ""
	if words < 0 {
		words = len(strings.Fields(unfoldText(pn.doc.Text()).PlainText()))
	}
	a.countWriting(pn.lastWords, words)
	pn.lastSave, pn.lastWords, pn.dirty = body, words, false
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
	pn := a.pane()
	if pn.doc == nil {
		return 0, 0
	}
	if v := pn.doc.Version(); v != pn.statsVer || pn.statChars == 0 {
		s := pn.doc.Text().String()
		pn.statWords, pn.statChars, pn.statsVer = len(strings.Fields(s)), len([]rune(strings.ReplaceAll(strings.ReplaceAll(s, "\n", ""), string(richtext.ObjectReplacement), ""))), v
	}
	return pn.statWords, pn.statChars
}

// handleSecondInstance handles command-line arguments forwarded from another launch.
func (a *app) handleSecondInstance(args []string) {
	if slices.Contains(args, mcpFlag) { // `celestine mcp` found the server off: it is turned on without persisting (#182)
		a.do(func() error {
			if a.mcpServer == nil {
				a.startMCP(true, false)
			}
			return nil
		})
	}
	a.bringToFront()
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
