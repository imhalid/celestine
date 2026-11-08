package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The MCP server: turned on in the settings, the app serves its library to
// agents (Claude Code, or any MCP client) at http://127.0.0.1:<port>/mcp,
// with a token they send as "Authorization: Bearer <token>". An agent
// lists, reads, searches, makes, edits, moves and throws away collections
// and pages; what it does shows in the window at once, as if typed there.
//
// The library is the window's: a tool runs holding the app's lock, which a
// frame holds too, and asks for a frame after it.

const defaultPort = 7457

// Settings are the app's own, kept in the user's config folder.
type Settings struct {
	MCP   bool   `json:"mcp"`
	Port  int    `json:"port"`
	Token string `json:"token"`
	// Recent are the kinds of line the / menu was used for last, newest first.
	Recent []string `json:"recent,omitempty"`
	// RecentCmds are the commands of the palette used last, newest first (palette.go).
	RecentCmds []string `json:"recentCmds,omitempty"`
	// Appearance and the editor; zero is the default.
	Theme      int     `json:"theme,omitempty"`      // 0 the system's, 1 light, 2 dark
	AppTheme   int     `json:"appTheme,omitempty"`   // the accent's color: an index of appThemes (appthemes.go), 0 Ryugo's blue
	Caret      int     `json:"caret,omitempty"`      // 0 Slime, 1 Mizan
	CaretName  string  `json:"caretName,omitempty"`  // the caret by name: its number moves as carets are taken away
	Jelly      bool    `json:"jelly,omitempty"`      // the jelly, put on any caret but the slime
	FontSize   float64 `json:"fontSize,omitempty"`   // the text's, 16.5
	LineHeight float64 `json:"lineHeight,omitempty"` // 1.55
	PageWidth  float64 `json:"pageWidth,omitempty"`  // the text column's, 720
	// how the text is set and kept (editing.go)
	ParaGap       *float64                  `json:"paraGap,omitempty"`       // the room between paragraphs, in ems: 0.55 when not set
	Indent        float64                   `json:"indent,omitempty"`        // the first line's indent, in ems
	NoAutoSave    bool                      `json:"noAutoSave,omitempty"`    // the page is saved by ⌘S, not by itself
	SaveAfter     float64                   `json:"saveAfter,omitempty"`     // seconds of stillness before it saves by itself: 0.7 when not set
	ShowSpaces    bool                      `json:"showSpaces,omitempty"`    // white space shown over the text: spaces
	ShowTabs      bool                      `json:"showTabs,omitempty"`      // tabs
	ShowBreaks    bool                      `json:"showBreaks,omitempty"`    // and where a paragraph ends
	MarkCode      bool                      `json:"markCode,omitempty"`      // in blocks of code, too
	UpdateURL     string                    `json:"updateUrl,omitempty"`     // the manifest of releases, when not the program's (update.go)
	UpdateOff     bool                      `json:"updateOff,omitempty"`     // no asking for a new version at all: nothing leaves the computer
	UpdateQuiet   bool                      `json:"updateQuiet,omitempty"`   // a new version is shown in the settings only, not in a dialog
	UpdateAuto    bool                      `json:"updateAuto,omitempty"`    // a new version is downloaded and put in place when it is found
	UpdateChannel string                    `json:"updateChannel,omitempty"` // "beta" for trial versions too
	UpdateHours   int                       `json:"updateHours,omitempty"`   // hours between two checks: 24 when not set
	UpdateAt      int64                     `json:"updateAt,omitempty"`      // when it last asked, unix seconds
	UpdateSnooze  int64                     `json:"updateSnooze,omitempty"`  // not before this: "remind me in three days"
	UpdateSkip    string                    `json:"updateSkip,omitempty"`    // the version that was said no to
	UpdateNotes   string                    `json:"updateNotes,omitempty"`   // the notes of the version that was put in place
	UpdateFor     string                    `json:"updateFor,omitempty"`     // and its number
	UpdateSeen    string                    `json:"updateSeen,omitempty"`    // the version whose notes were shown
	SyncMode      string                    `json:"syncMode,omitempty"`      // "" off, "dir" a folder, "server" a server of one's own (sync.go)
	SyncDir       string                    `json:"syncDir,omitempty"`       // the folder, for "dir"
	SyncURL       string                    `json:"syncUrl,omitempty"`       // the server's address
	SyncToken     string                    `json:"syncToken,omitempty"`     // this device's token
	SyncKey       string                    `json:"syncKey,omitempty"`       // the key of end-to-end encryption, base64: "" when it is off
	AccountUser   string                    `json:"accountUser,omitempty"`   // the account this computer is signed in to (account.go)
	AccountAsked  bool                      `json:"accountAsked,omitempty"`  // the sign-in screen has been shown and answered, once (profile.go)
	SyncOff       []string                  `json:"syncOff,omitempty"`       // collections (their folders) that stay here
	SyncMinutes   int                       `json:"syncMinutes,omitempty"`   // minutes between two syncs: 5 when not set
	Profiles      map[string]map[string]any `json:"profiles,omitempty"`      // looks and ways of writing saved under a name (settingsx.go)
	Focus         string                    `json:"focus,omitempty"`         // "line" or "paragraph": the rest of the page is veiled (focus.go)
	Typewriter    bool                      `json:"typewriter,omitempty"`    // the caret's line stays in the middle of the window
	TabSpaces     int                       `json:"tabSpaces,omitempty"`     // a Tab writes this many spaces; 0 a tab
	SleepSecs     float64                   `json:"sleepSecs,omitempty"`     // seconds at rest before the caret dozes: 5 when not set
	NoCaretFX     bool                      `json:"noCaretFx,omitempty"`     // a plain caret, whatever caret is chosen: light mode
	Keys          map[string]string         `json:"keys,omitempty"`          // a command (its title) to a key of its own (keys.go)
	Macros        map[string][]string       `json:"macros,omitempty"`        // a name to the commands it runs one after another
	AlwaysRing    bool                      `json:"alwaysRing,omitempty"`    // the row the keys are on is ringed, in the normal mode or not (kb.go)
	LineChars     int                       `json:"lineChars,omitempty"`     // the text column is at most this many characters wide: 0, as wide as the page (editing.go)
	HideStatus    bool                      `json:"hideStatus,omitempty"`    // the status bar is not shown at all
	Panes         string                    `json:"panes,omitempty"`         // the panes of the window: the Dock's tree and what each shows (panes.go)
	HideHints     bool                      `json:"hideHints,omitempty"`     // the keys to use are not written at the left of the status bar
	Lang          string                    `json:"lang,omitempty"`          // "tr", "en"; "" the system's (i18n.go)
	VimKeys       bool                      `json:"vimKeys,omitempty"`       // the motions of Vim in the normal mode (vim.go)
	TourDone      bool                      `json:"tourDone,omitempty"`      // the first-run tour was shown or skipped (help.go)
	WelcomeDone   bool                      `json:"welcomeDone,omitempty"`   // the first opening was seen (welcome.go)
	NoStats       bool                      `json:"noStats,omitempty"`       // no figures of writing and use are kept (stats.go)
	NoBackup      bool                      `json:"noBackup,omitempty"`      // no backup of the library by itself once a day (backup.go)
	// the typefaces (fonts.go): by family; "" is the default
	FontApp     string   `json:"fontApp,omitempty"`
	FontEditor  string   `json:"fontEditor,omitempty"`
	FontMono    string   `json:"fontMono,omitempty"`
	FontFavs    []string `json:"fontFavs,omitempty"`    // the families starred
	FontLast    []string `json:"fontLast,omitempty"`    // and those used last
	NoLigatures bool     `json:"noLigatures,omitempty"` // no ligatures in the text
	TabularNums bool     `json:"tabularNums,omitempty"` // digits of one width
	FontWeight  int      `json:"fontWeight,omitempty"`  // the text's weight: 400 when not set
	// the status bar: what is hidden, and what is shown besides
	HideSaved bool `json:"hideSaved,omitempty"`
	ShowChars bool `json:"showChars,omitempty"`
	// more of the status bar, all off to begin with (statusbar.go)
	ShowPos bool `json:"showPos,omitempty"`
	// the sidebar as it was left: shown or not, and what is open in its tree
	SidebarClosed bool           `json:"sidebarClosed,omitempty"`
	PanelOpen     bool           `json:"panelOpen,omitempty"`
	SidebarWidth  int            `json:"sidebarWidth,omitempty"` // dragged at its edge; 0 is the design's own
	PanelWidth    int            `json:"panelWidth,omitempty"`
	Writing       map[string]int `json:"writing,omitempty"`     // the words written, by day (writing.go)
	WriteGoal     int            `json:"writeGoal,omitempty"`   // the words to write in a day
	NoAutoCover   bool           `json:"noAutoCover,omitempty"` // pages with no cover of their own show no made one (autocover.go)
	NoWobble      bool           `json:"noWobble,omitempty"`    // the window does not shake when an agent writes (wobble.go)
	Corner        *int           `json:"corner,omitempty"`      // how round the corners are, in percent (radius.go)
	QuickKey      string         `json:"quickKey,omitempty"`    // the quick note's global key
	TreeSaved     bool           `json:"treeSaved,omitempty"`
	TreeOpen      []string       `json:"treeOpen,omitempty"`
}

// the settings, with what is not set taken as the default
func (a *app) fontSize() float32 {
	if s := a.settings.FontSize; s >= 12 && s <= 28 {
		return float32(s)
	}
	return 16.5
}

func (a *app) lineHeight() float32 {
	if s := a.settings.LineHeight; s >= 1.2 && s <= 2.2 {
		return float32(s)
	}
	return 1.55
}

func (a *app) pageWidth() float32 {
	w := float32(columnMax)
	if s := a.settings.PageWidth; s >= 480 && s <= 1200 {
		w = float32(s)
	}
	if n := a.settings.LineChars; n >= 20 && n <= 200 { // a line of n characters, at most
		if cw := a.charsWidth(n); cw > 0 && cw < w {
			return cw
		}
	}
	return w
}

// setCaret chooses the caret, and keeps the choice.
func (a *app) setCaret(k caretKind) {
	a.caretKind = k
	a.settings.Caret, a.settings.CaretName = int(k), caretName(k)
	a.saveSettings()
}

// loadSettings reads the settings, making a token the first time.
func (a *app) loadSettings() {
	if a.settingsStore != nil { // kept in the library's file, with the token
		if data := a.settingsStore.Meta(settingsKey); data != nil {
			json.Unmarshal(data, &a.settings)
		} else if a.settingsFile != "" { // the settings of before the file: taken in once
			if data, err := os.ReadFile(a.settingsFile); err == nil {
				json.Unmarshal(data, &a.settings)
			}
		}
	} else if a.settingsFile != "" {
		if data, err := os.ReadFile(a.settingsFile); err == nil {
			json.Unmarshal(data, &a.settings)
		}
	}
	if a.settings.Port == 0 {
		a.settings.Port = defaultPort
	}
	a.caretKind = a.savedCaret()
	a.settings.Caret, a.settings.CaretName = int(a.caretKind), caretName(a.caretKind)
	if a.settings.Token == "" {
		a.settings.Token = newToken()
		a.saveSettings()
	}
}

// settingsKey is what the settings are kept under in a library's file.
const settingsKey = "settings"

func (a *app) saveSettings() {
	a.settingsDirty = false
	data, _ := json.MarshalIndent(a.settings, "", "  ")
	if a.settingsStore != nil {
		if err := a.settingsStore.SetMeta(settingsKey, data); err != nil {
			a.saveErr = err.Error()
		}
		return
	}
	if a.settingsFile == "" {
		return
	}
	os.MkdirAll(filepath.Dir(a.settingsFile), 0o700)
	if err := writeFilePrivate(a.settingsFile, data); err != nil {
		a.saveErr = err.Error()
	}
}

// saveSettingsSoon keeps the settings a moment from now: a slider moves them
// at every frame, and a write to the file at each would hold the frames back.
func (a *app) saveSettingsSoon() {
	a.settingsDirty, a.settingsAt = true, a.clock()
}

func newToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// mcpURL is where agents reach the server.
func (a *app) mcpURL() string { return fmt.Sprintf("http://127.0.0.1:%d/mcp", a.settings.Port) }

// mcpCommand is the line that adds the server to Claude Code.
func (a *app) mcpCommand() string {
	return fmt.Sprintf("claude mcp add --transport http ryudit %s --header \"Authorization: Bearer %s\"", a.mcpURL(), a.settings.Token)
}

// do runs fn on the library, holding the app's lock, and asks for a frame.
func (a *app) do(fn func() error) error {
	err := func() error {
		a.mu.Lock()
		defer a.mu.Unlock() // a panic in fn lets go too: the goroutine that recovers it may need the lock (guard)
		return fn()
	}()
	if a.invalidate != nil {
		a.invalidate()
	}
	return err
}

// ---- what an agent sees ----

// A page or collection is named by its folder, from the library's root, as
// "kitaplar/dune/notlar".

type pageInfo struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Properties map[string]string `json:"properties,omitempty"` // by property name
	Children   []pageInfo        `json:"children,omitempty"`
}

type collInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Pages      int        `json:"pages"`
	Properties []Property `json:"properties"`
	SubPages   bool       `json:"subPages"`
	Templates  []string   `json:"templates,omitempty"`
	Query      string     `json:"query,omitempty"`
}

func (a *app) idOf(dir string) string {
	rel, _ := filepath.Rel(a.lib.Root, dir)
	return filepath.ToSlash(rel)
}

func (a *app) findColl(id string) (*Collection, error) {
	id = strings.Trim(id, "/")
	for _, c := range a.lib.Collections {
		if a.idOf(c.Dir) == id || strings.EqualFold(c.Name, id) {
			return c, nil
		}
	}
	return nil, fmt.Errorf("no collection %q: list_collections names them", id)
}

func (a *app) findPage(id string) (*Page, error) {
	id = strings.Trim(id, "/")
	for _, c := range a.lib.Collections {
		for _, p := range c.All() {
			if a.idOf(p.Dir) == id {
				return p, nil
			}
		}
	}
	return nil, fmt.Errorf("no page %q: list_pages names them", id)
}

func (a *app) tree(ps []*Page) []pageInfo {
	var out []pageInfo
	for _, p := range ps {
		out = append(out, pageInfo{ID: a.idOf(p.Dir), Title: p.Title, Properties: values(p), Children: a.tree(p.Children)})
	}
	return out
}

// values is page p's values, by property name.
func values(p *Page) map[string]string {
	out := map[string]string{}
	for i := range p.Coll.Properties {
		pr := &p.Coll.Properties[i]
		if v := p.Value(pr); v != "" {
			out[pr.Name] = v
		}
	}
	return out
}

// setValues gives page p the values vs, by property name or id.
func (a *app) setValues(p *Page, vs map[string]string) error {
	if p == a.page {
		a.flush()
	}
	for name, v := range vs {
		pr := p.Coll.PropNamed(name)
		if pr == nil {
			var names []string
			for _, x := range p.Coll.Properties {
				names = append(names, x.Name)
			}
			return fmt.Errorf("%s has no property %q (it has: %s; add_property adds one)", p.Coll.Name, name, strings.Join(names, ", "))
		}
		if err := p.SetValue(pr.ID, v); err != nil {
			return err
		}
	}
	return nil
}

// bodyOf is what an agent gives as a page's text: Markdown, without front matter.
func bodyOf(md string) string {
	if _, body := splitFrontMatter(md); strings.HasPrefix(md, "---\n") {
		md = body
	}
	md = strings.TrimLeft(md, "\n")
	if md != "" && !strings.HasSuffix(md, "\n") {
		md += "\n"
	}
	return md
}

// writeBody gives page p the text body, on disk and in its document if it
// is open: the window shows it at once. What was typed in it is saved first.
func (a *app) writeBody(p *Page, md string) error {
	if p == a.page {
		a.flush()
	}
	body := encodeBody(importMarkdown(md))
	if err := p.Save(body); err != nil {
		return err
	}
	a.jiggle(len(md))
	a.linksStale()
	if doc := a.docs[p]; doc != nil {
		doc.SetText(a.parsePage(p, body))
		if p == a.page {
			_, caret := doc.Selection()
			a.version, a.length, a.caretAt = doc.Version(), doc.Text().Len(), caret
			a.lastSave, a.lastWords, a.dirty = encodeBody(doc.Text()), len(strings.Fields(doc.Text().PlainText())), false
		}
	}
	return nil
}

// ---- the tools ----

func indexOf(ps []*Page, p *Page) int {
	for i, q := range ps {
		if q == p {
			return i
		}
	}
	return -1
}

func ptr[T any](v T) *T { return &v }
