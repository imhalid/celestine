package main

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
)

// The app's state, grouped by what it is about (#596). Each group is embedded in app, so its fields are the app's
// own (a.links, a.sy) and nothing that reads them changed; what a group holds can be moved out of the app as a
// piece, as the extractions of #570 need. Each says which lock its fields are under: a.mu unless it says otherwise.

// indexState is what the app knows of the library's pages beyond the pages themselves: who links to whom, the
// search's text, the palette's places, and the reading of them all as the app opens (prewarm.go). Under a.mu; the
// reading itself goes on away from the window and hands what it read over through a.do. pageIndexWrites is a
// WaitGroup any goroutine may use, and firstFrame is closed once, by the first frame, and waited on by the reading.
type indexState struct {
	links           linkIndex        // who links to whom (backlinks.go)
	titles          titleIdx         // the pages by their titles, for the chips (chips.go)
	sx              *searchIndex     // the text of every page, for the palette and the search (searchindex.go)
	px              placeIdx         // the palette's places, folded once (palette.go)
	ctxCache        map[string][]seg // the lines that link to a page (backlinks.go)
	prewarming      bool             // the pages are being read away from the window (prewarm.go)
	firstFrame      chan struct{}    // closed once the window's first frame is built: the reading of the pages waits for it (prewarm.go)
	pageIndexWrites sync.WaitGroup   // the writes of keepPageIndexes going on away from the window (backlinks.go)
}

// updateRuntime is the app's updates (update.go): a check, a download, the chip and the card that show them, and the
// note of the version just installed. Under a.mu, but updBusy, which the check's and the download's goroutines set
// as they start and end, and which is atomic.
type updateRuntime struct {
	updKey     string      // a test's key for releases
	updBusy    atomic.Bool // a check or a download goes on
	updErr     string      // what the last check said, when it failed
	updState   updateState // where an update the user asked for is
	updChipAt  time.Time   // when the chip began to unfold: its light's passes are timed from it (updatepanel.go)
	updPanelAt time.Time   // when the update card began to open
	updHeldAt  time.Time   // when the pointer was last on the update card: it stays updStay from then (updatepanel.go)
	updEase    updEase     // the chip's fill as it is drawn, following the download smoothly
	updProg    updProgress // how far the download is
	updRel     *Release    // the newer version found
	updAuto    bool        // a check found a release and the settings say to put it in place
	updNotes   bool        // the dialog shows the notes of the version installed, not a new one
	updOpen    bool        // the dialog of it
	hello      helloState  // the note of the version just installed, at its first start (updhello.go)
}

// mcpRuntime is the MCP server the settings turn on (mcp.go) and what its agents did. mcpServer, agent and mcpErr are
// under a.mu, which an agent's tool holds as a frame does; wob, the shimmer an agent's goroutine sends, is under wobMu.
type mcpRuntime struct {
	mcpServer *http.Server
	mcpErr    string
	agent     agentEdits // what agents changed, to put back
	wob       wobble     // the shimmer, when an agent writes (wobble.go)
	wobMu     sync.Mutex // guards it: an agent's goroutine sends it
}

// syncRuntime is the synchronization (syncapp.go, netsync.go) and the account it goes through (account.go). Under
// a.mu: a pass runs away from the window and writes what it found through a.do. keepRunning is set once, before the
// window opens, and only read after.
type syncRuntime struct {
	sy          syncUI                               // the state of synchronization (syncapp.go)
	ac          acctUI                               // the account: signing in, its devices (account.go)
	keepRunning func(reason string) (release func()) // MyGo's Power.KeepRunning, for a sync pass; nil in tests (netsync.go)
}

// chromeState is what the window's chrome was drawn with and where, around the pages: the frame and its glass, the
// sidebar's and the panes' places, the status bar, the notices, and what the pointer is over. A frame writes it and
// the next reads it: under a.mu, as the frame is.
type chromeState struct {
	winW, winH    float32  // the window's size, last frame
	winRect       ui.Rect  // the window, as the caret layer was last made: the torch darkens all of it
	winRadius     float64  // the window's corners' radius as the system draws them, read each frame (cardR)
	winBehind     bool     // the window is not the one in front: the glass is matte (glassLive)
	barLeft       float32  // where macOS's window buttons end, as the frame's top edge last read it (frameTop)
	topLeftR      ui.Rect  // where the window's top-left buttons are, the sidebar's (frameTop): a pane's bar starts after them
	panesR        ui.Rect  // where the panes were, last frame
	sidebarR      ui.Rect  // where the sidebar was, last frame
	statusH       float32  // the status bar's height, as it was laid out
	edge          edgeDrag // the sidebar or the side panel being dragged wider or narrower
	pageBg        ui.Color // the page's background this frame (bg)
	glassTint     string   // last CSS rgba() tint applied to macOS Liquid Glass
	glassBg       ui.Color // the page background glassTint was made from
	glassDark     bool     // and whether the window was dark
	glassHasTint  bool     // whether glassBg and glassDark are those of the last tint made
	glassTintRuns int      // counts glassTint() string builds for verifying per-frame skip
	// glassAtLaunch is the Glass setting as the window was made with it: turned on since, it shows after a restart
	glassAtLaunch bool

	toasts     toastState // the toasts up (toast.go)
	notice     string     // a toast to show
	noticePage *Page      // the page the notice is about, to open from it
	noticeUndo func()     // takes back what the notice says was done: the notice has a Geri al
	noticeTone *toastKind // the notice's kind, which a.say tells; none is news

	hoverTag    string
	hoverCh     *chip // the chip the pointer is over
	hoverBorder borderKey
	coverHover  bool // the pointer is in the cover band
}
