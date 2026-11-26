package main

import (
	"cmp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/textnorm"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The palette, a field over the window: ⌘K searches the documents - go to a
// page, a collection or a tag by typing a few letters of it, or a word in a
// page's text; empty, it lists the places last shown. A ">" first in it makes
// it the commands: what the app can do, and typed after it, those that match.
// ⌘P opens it so, with the ">" there. Best first, accents and case don't
// matter. ↑↓ move, ↵ goes, Esc closes.

type palette struct {
	open   bool
	query  string // a ">" first in it: the commands; else the documents
	sel    int
	text   string    // what is in the field: the query without the sign of the mode
	hseen  int       // the row last hovered
	reveal bool      // the arrows (or a new query) moved the chosen row: the list scrolls to show it
	was    string    // the query of the last frame, to start from the top when it changes
	hx, hy float32   // where the pointer was last seen over the rows, in the window: a row is chosen by the pointer only when it moves
	snap   *palSnap  // the settings as they were when it opened, while a command is shown as a preview
	pvKey  string    // the command being previewed
	cat    string    // the group of commands opened, with nothing typed (palgroups.go)
	items  []palItem // the lines for key, kept: made again only when what they come from changed (#186)
	key    palKey
	have   bool   // items are made for key (none at all is a list too)
	made   int    // how many times the lines were made, counted for a test
	up     *palUp // the list the group or the family shown was opened from (#603)
	opened string // the query the group or the family opened with: Left goes back while it is unchanged
	move   int    // -1 out of the list shown, 1 into the row chosen: asked by a key in the field (#603)
}

// palKey is what the palette's lines come from: the query, the group opened, the pages of the library, the open page's
// text, and the settings as the palette shows them (its ticks).
type palKey struct {
	query, cat  string
	pages, text uint64
	theme, look int
	caret       caretKind
}

// palSnap is what a preview changes, as it was: it goes back when the palette closes without choosing.
type palSnap struct {
	theme, appTheme int
	caret           caretKind
}

// palItem is a line of the palette.
type palItem struct {
	title, sub, keys string
	glyph            *ui.SVG
	run              func()
	score            int
	to               place  // where it goes, for a page or a collection
	preview          func() // shows it without keeping it (a theme, a caret), while it is the one chosen
	group            string // a heading over it, when the lines are of kinds
	boost            int    // added to its score when the screen it is for is shown: page, collection, selection
	alias            string // other words it is found by, in the other language too
	off              string // why it cannot be run now: it is dim, and says so when it is chosen
	snip             []seg  // a page's words around what was asked, which are marked: a line of two
	cat              string // a command's group, with nothing typed (palgroups.go)
	fam              string // its family: its Turkish title before ": "
	name             string // an app command's Turkish title, as the code names it (appcmds.go), whatever the language
	id               string // what it is whatever the language: a command's or a setting's title as the code writes it; the recent commands and the keys of one's own are kept by it
	kind             palKind
	drill            bool        // it opens a list (a group, a family): Right opens it as Enter does (#603)
	coll             *Collection // collection named by or owning the target of this command, for MCP visibility
	destDir          string      // destination collection folder for moveThePage, matched by Dir rather than Name
}

// A palKind is what a line of the palette is, where it matters: a command and a setting are remembered when run.
type palKind int

const (
	palOther   palKind = iota
	palCommand         // a command
	palSetting         // a setting as a command
)

// openPalette opens the search of the documents (⌘K).
func (a *app) openPalette() {
	a.pal = palette{open: true}
}

// openCommands opens the palette as the commands (⌘P): the ">" is in it.
func (a *app) openCommands() {
	a.pal = palette{open: true, query: ">"}
}

// palCmds reports whether the palette is the commands: a ">" is first in it.
func (a *app) palCmds() bool { return strings.HasPrefix(a.pal.query, ">") }

// palQuery is what is looked for: the query, without the ">" of the commands.
func (a *app) palQuery() string {
	if palModeOf(a.pal.query) != '@' {
		return strings.TrimSpace(a.pal.query[1:])
	}
	return strings.TrimSpace(a.pal.query)
}

// commands are what the palette does, where the app is now.
func (a *app) commands() []palItem {
	cmd := func(title, keys string, glyph *ui.SVG, run func()) palItem {
		return palItem{title: msg(title), sub: locales.PaletteCommand(), kind: palCommand, id: title, keys: keys, glyph: glyph, run: run, cat: cmdCat(title), fam: cmdFam(title)}
	}
	var out []palItem
	ctxFrom := 0
	mark := func() { // what was added since is for what is shown: it comes before the rest
		for i := ctxFrom; i < len(out); i++ {
			out[i].boost = 15
		}
		ctxFrom = len(out)
	}
	if c := a.currentColl(); c != nil {
		if !c.Dynamic() {
			it := cmd(newPageLabel(c)+": "+c.Name, "⌘N", iconPlus, func() { a.newPage(c, nil) })
			it.coll = c
			out = append(out, it)
		}
		cs := cmd("collectionSettings: "+c.Name, "", iconSettings, func() { a.openCollSettings(c, 0) })
		cs.coll = c
		out = append(out, cs)
		if !c.Dynamic() && !c.Journal {
			tr := cmd("triage_sortThePages", "", iconTask, func() { a.openTriage(c) })
			tr.coll = c
			out = append(out, tr)
		}
		if !c.Journal {
			for _, t := range c.AllTemplates() {
				t := t
				it := cmd("palette_newFromTemplate: "+t.Name, "", iconTemplate, func() { a.newPageFrom(c, nil, t.Body) })
				it.title = locales.PaletteNewFromTemplate(t.Name)
				it.coll = c
				out = append(out, it)
			}
		}
	}
	mark()
	if p := a.pane().page; p != nil && !a.noPage() {
		if sa, sc := a.pane().doc.Selection(); sa != sc { // text is selected: its formats
			for _, f := range []struct {
				name string
				do   func()
			}{{"makeTheSelectionBold", a.pane().doc.ToggleBold}, {"makeTheSelectionItalic", a.pane().doc.ToggleItalic}, {"underlineTheSelection", a.pane().doc.ToggleUnderline},
				{"strikeTheSelectionThrough", a.pane().doc.ToggleStrikethrough}, {"makeTheSelectionCode", a.pane().doc.ToggleCode}} {
				f := f
				it := cmd(f.name, "", iconEditing, func() { f.do(); a.pane().refocus = true })
				it.coll = p.Coll
				out = append(out, it)
			}
		}
		ex := cmd("extract_toANewPage", "", iconArrowRight, a.extractToPage) // the selection's blocks, or the caret's
		ex.coll = p.Coll
		out = append(out, ex)
		if p.Coll.SubPages {
			it := cmd("addASubPage: "+p.Title, "", iconPlus, func() { a.newPage(p.Coll, p) })
			it.coll = p.Coll
			out = append(out, it)
		}
		if _, ok := historyOf(p); ok {
			it := cmd(map[bool]string{false: "common_timeLapse", true: "common_backText"}[a.lapse.on], "", iconLapse, a.toggleLapse)
			it.coll = p.Coll
			out = append(out, it)
		}
		undoIt := cmd("common_undo2", "⌘Z", iconBack, func() { a.pane().doc.Undo(); a.pane().refocus = true })
		undoIt.coll = p.Coll
		redoIt := cmd("redo", "⇧⌘Z", iconChevron, func() { a.pane().doc.Redo(); a.pane().refocus = true })
		redoIt.coll = p.Coll
		pasteIt := cmd("pasteATableCsv", "", iconKind, func() { a.pasteTable() })
		pasteIt.coll = p.Coll
		trashIt := cmd("moveToTrash: "+p.Title, "", iconTrash, func() { a.trashPage(p) })
		trashIt.coll = p.Coll
		out = append(out, undoIt, redoIt, pasteIt, trashIt)
	}
	mark()
	if c := a.pane().coll; c != nil {
		for i := range c.Views {
			v := c.Views[i]
			it := cmd("view: "+viewLabel(v), "", iconStack, func() {
				if err := c.SetView(v.ID); err != nil {
					a.fail(err)
				}
			})
			it.coll = c
			out = append(out, it)
		}
	}
	out = append(out, a.moveCommands(cmd)...)
	mark()
	out = append(out, a.paneCommands(cmd)...)
	out = append(out,
		cmd("common_tasks", "", iconTask, a.openTasks),
		cmd("common_writeSettingsFile", "", iconExport, func() {
			p := a.settingsFileOut()
			if err := a.exportSettingsTo(p); err != nil {
				a.say(toastErr, locales.CommonDidntWork(err.Error()))
			} else {
				a.say(toastOK, locales.CommonSavedTo(p))
			}
		}),
		cmd("settings_readSettingsFile", "", iconExport, a.pickSettingsFile),
		cmd("common_resetAllDefaults", "", iconSettings, a.resetAllAsked),
		cmd("goBackToBeforeTheLast", "", iconBack, a.revertToBeforeSave),
		cmd("common_syncNow", "", iconSync, func() {
			if a.settings.SyncMode == "" {
				a.say(toastWarn, locales.SyncIsOffSettingsSync())
				return
			}
			a.syncNow()
		}),
		cmd("checkForUpdates", "", iconUpdate, func() { a.checkUpdate(true) }),
		cmd("whatsNewInThisVersion", "", iconUpdate, func() {
			a.updNotes, a.updOpen = true, true
			if a.settings.UpdateNotes == "" {
				a.settings.UpdateNotes = locales.UpdateNoNotes()
			}
		}),
		cmd("shortcutsAndHelp", "?", iconSearch, func() { a.kb.help = true }),
		cmd("enterTheKeyboardModeEsc", "", iconSearch, a.kbEnter),
		cmd("common_backUpNow", "", iconExport, func() { a.backupNow("elle", false) }),
		cmd("exportAsMarkdownFolder", "", iconExport, func() {
			if st := a.store(); st != nil {
				a.exportLibrarySoon(st, a.sayDone)
			} else {
				a.say(toastInfo, locales.ThisLibraryIsAlreadyMadeOf())
			}
		}),
		cmd("exportAsMarkdownZip", "", iconExport, func() {
			if st := a.store(); st != nil {
				a.sayDone(a.exportZip(st))
			} else {
				a.say(toastInfo, locales.ThisLibraryIsAlreadyMadeOf())
			}
		}),
		cmd("showTheCrashLog", "", iconSettings, a.showCrash),
		cmd("common_statistics", "", iconStats, a.openStats),
		cmd("accountProfile", "", iconUser, a.openProfile),
		cmd("importAMarkdownFolder2", "", iconExport, a.pickImportFolder),
		cmd("randomDay", "", iconCalendar, a.openRandomDay),
	)
	out = append(out, a.appPalette()...) // the app's commands, as the menu bar has them (appcmds.go)
	// the settings, each a command; a theme or a caret is shown as it is passed over, and a ✓ is by the
	// one in use - not what a preview shows
	if sn := a.pal.snap; sn != nil {
		now := palSnap{a.settings.Theme, a.settings.AppTheme, a.caretKind}
		a.settings.Theme, a.settings.AppTheme, a.caretKind = sn.theme, sn.appTheme, sn.caret
		defer func() { a.settings.Theme, a.settings.AppTheme, a.caretKind = now.theme, now.appTheme, now.caret }()
	}
	out = append(out, a.settingCmds(cmd)...)
	for i, b := range backups(a.backupDirOf()) { // a backup to go back to, the newest first
		if i >= 10 {
			break
		}
		b := b
		what := "common_daily"
		if b.Label != "" {
			what = b.Label
		}
		it := cmd("restoreFromABackup: "+dateTime(b.At, false)+" ("+what+")", "", iconExport, func() { a.restoreFrom(b) })
		it.title = msg("restoreFromABackup") + ": " + dateTime(b.At, false) + " (" + backupKind(what) + ")" // what it was taken for, in the window's language
		it.alias = "restore backup yedek"
		out = append(out, it)
	}
	var macroNames []string
	for n := range a.settings.Macros {
		macroNames = append(macroNames, n)
	}
	sort.Strings(macroNames)
	for _, n := range macroNames {
		n := n
		it := cmd("macro: "+n, "", iconSettings, func() { a.runMacro(n) })
		it.alias = "macro"
		out = append(out, it)
	}
	for _, n := range a.profileNames() {
		n := n
		it := cmd("profile2: "+n, "", iconSettings, func() { a.applyProfile(n) })
		it.title = msg("common_profile") + ": " + a.profileLabel(n)
		it.alias = "profile görünüm ayar paketi"
		out = append(out, it)
	}
	for _, k := range []string{"reading", "writing", "code"} {
		k := k
		it := cmd("typefacePreset: "+k, "", iconSettings, func() {
			if !a.fontPreset(k) {
				a.say(toastWarn, locales.FontsNoneFound(k))
			}
		})
		it.title = msg("typefacePreset") + ": " + msg(k) // both its parts in the window's language
		it.alias = "font preset serif sans mono ayar"
		out = append(out, it)
	}
	for i := range out { // a key of one's own shows where the command's key is
		if k := a.settings.Keys[out[i].id]; k != "" && out[i].keys == "" {
			out[i].keys = k
		}
	}
	return out
}

// currentColl is the collection of what is shown.
func (a *app) currentColl() *Collection {
	switch {
	case a.pane().coll != nil:
		return a.pane().coll
	case a.pane().page != nil:
		return a.pane().page.Coll
	case len(a.lib.Collections) > 0:
		return a.lib.Collections[0]
	}
	return nil
}

// places are the pages and collections to go to.
func (a *app) places() []palItem {
	var out []palItem
	for _, c := range a.lib.Collections {
		c := c
		out = append(out, palItem{title: c.Name, sub: locales.PaletteCollection(), glyph: iconStack, to: place{"coll", c.Dir}, run: func() { a.goTo(place{"coll", c.Dir}) }})
		for _, p := range c.All() {
			p := p
			sub := c.Name
			if crumb := Parents(p); crumb != "" {
				sub += " › " + crumb
			}
			out = append(out, palItem{title: p.Title, sub: sub, glyph: iconPage, to: place{"page", p.Dir}, alias: strings.Join(p.Aliases(), " "), run: func() { a.goTo(place{"page", p.Dir}) }})
		}
	}
	for _, ti := range a.allTags() {
		key := ti.key
		out = append(out, palItem{title: "#" + ti.name, sub: locales.PaletteTagSub(len(ti.pages)), glyph: iconTag, to: place{"tag", key}, run: func() { a.goTo(place{"tag", key}) }})
	}
	return out
}

// recent are the places last shown, the last first, without the one shown now.
func (a *app) recent(n int) []palItem {
	var out []palItem
	seen := map[place]bool{a.here(): true}
	all := a.places()
	for i := len(a.hist) - 1; i >= 0 && len(out) < n; i-- {
		pl := a.hist[i]
		if seen[pl] {
			continue
		}
		seen[pl] = true
		for _, it := range all {
			if it.to == pl {
				out = append(out, it)
				break
			}
		}
	}
	return out
}

// paletteItems are the lines for the query, best first.
// placeIdx is the places of the palette, with what a match needs of them already folded, kept until the
// library changes or the palette is opened again: a keystroke does not make a thousand items and fold
// a thousand titles.
type placeIdx struct {
	items  []palItem
	ftitle []string // folded title
	fboth  []string // folded title and where it is
	mask   []uint64 // letterMask of fboth
	dirty  bool
	ready  bool
	rank   []placeRank
}

type placeRank struct {
	score, runes, at int32
}

// placeIndex is the places in the form a match goes through quickly.
func (a *app) placeIndex() *placeIdx {
	px := &a.px
	if px.ready && !px.dirty {
		return px
	}
	items := a.places()
	px.items = items
	px.ftitle, px.fboth, px.mask = make([]string, len(items)), make([]string, len(items)), make([]uint64, len(items))
	for i, it := range items {
		px.ftitle[i] = foldText(it.title)
		px.fboth[i] = foldText(it.title + " " + it.sub + " " + it.alias) // a page's aliases find it too
		px.mask[i] = letterMask(px.fboth[i])
	}
	px.ready, px.dirty = true, false
	return px
}

// placeMatches are the places that match q, the best first, at most max: the same scores as match, from
// the folded titles, the places that lack a letter of q never looked at.
func (a *app) placeMatches(q string, max int) []palItem {
	px := a.placeIndex()
	fq := foldText(strings.TrimSpace(q))
	words := strings.Fields(fq)
	qr := []rune(strings.ReplaceAll(fq, " ", ""))
	qm := letterMask(fq)
	rank := px.rank[:0]
	for i := range px.items {
		if qm&^px.mask[i] != 0 {
			continue
		}
		s := matchFolded(fq, words, qr, px.ftitle[i])
		if s < 0 {
			if s2 := matchFolded(fq, words, qr, px.fboth[i]); s2 >= 0 {
				s = s2 / 2 // in where it is, not in its name: less
			}
		}
		if s < 0 {
			continue
		}
		rank = append(rank, placeRank{int32(s), int32(utf8.RuneCountInString(px.items[i].title)), int32(i)})
	}
	px.rank = rank
	slices.SortFunc(rank, func(p, q placeRank) int {
		switch {
		case p.score != q.score:
			return int(q.score - p.score)
		case p.runes != q.runes:
			return int(p.runes - q.runes)
		}
		return int(p.at - q.at)
	})
	if len(rank) > max {
		rank = rank[:max]
	}
	out := make([]palItem, len(rank))
	for i, r := range rank {
		out[i] = px.items[r.at]
		out[i].group = locales.PaletteGroupPages()
		out[i].score = int(r.score)
	}
	return out
}

func (a *app) paletteItems(q string) []palItem {
	switch palModeOf(q) {
	case '>':
		return a.commandItems(strings.TrimSpace(q[1:]))
	case '#', ':', '/', '!', '?':
		return a.modeItems(palModeOf(q), strings.TrimSpace(q[1:]))
	}
	if strings.TrimSpace(q) == "" {
		if r := a.recent(10); len(r) > 0 {
			return r
		}
		all := a.places()
		return all[:min(10, len(all))]
	}
	out := a.placeMatches(q, 40)
	// what is in the pages' text, after what is in their names
	if len([]rune(strings.TrimSpace(q))) >= 2 {
		have := map[string]bool{}
		for _, it := range out {
			if it.to.kind == "page" {
				have[it.to.dir] = true
			}
		}
		for _, h := range a.searchPages(q, 10, false) {
			if have[h.page.Dir] {
				continue
			}
			h := h
			sub := h.page.Coll.Name
			if crumb := Parents(h.page); crumb != "" {
				sub += " › " + crumb
			}
			out = append(out, palItem{title: h.page.Title, sub: sub, group: "inTheText", snip: h.snip, glyph: iconSearch, to: place{"page", h.page.Dir}, run: func() { a.openFound(h) }})
		}
		query := strings.TrimSpace(q)
		out = append(out, palItem{title: locales.PaletteSearchLibraryFor(query), sub: locales.PaletteCommand(), kind: palCommand, id: "searchTheLibrary", group: locales.PaletteMore(), keys: "⇧⌘F", glyph: iconSearch, run: func() { a.openSearch(query) }})
	}
	return out
}

// commandItems are the commands for the query, best first; for none, those used last and then all of them.
func (a *app) commandItems(q string) []palItem {
	all := a.commands()
	recent := a.settings.RecentCmds
	rank := func(id string) int { // the more lately used, the more it counts
		for i, t := range recent {
			if t == id {
				return len(recent) - i
			}
		}
		return 0
	}
	if strings.TrimSpace(q) == "" {
		var out []palItem
		seen := map[string]bool{}
		for _, t := range recent {
			for _, it := range all {
				if it.id == t && !seen[t] && len(out) < 5 {
					seen[t] = true
					it.group = locales.PaletteRecentlyUsed()
					out = append(out, it)
				}
			}
		}
		var rest []palItem
		for _, it := range all {
			if !seen[it.title] {
				rest = append(rest, it)
			}
		}
		return append(out, a.groupCommands(rest)...)
	}
	out := append(append(a.argItems(q), a.fontNameItems(q)...), a.dateItems(q)...)
	for _, it := range all {
		s := match(q, it.title)
		if s2 := match(q, it.title+" "+it.alias); s < 0 && s2 >= 0 {
			s = s2 / 2 // by another name: less
		}
		if s >= 0 {
			it.score = s + rank(it.title)*6 + it.boost
			if foldText(it.title) == foldText(strings.TrimSpace(q)) { // the very name asked for comes first
				it.score += 300
			}
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return utf8.RuneCountInString(out[i].title) < utf8.RuneCountInString(out[j].title)
	})
	return out
}

// palModes are the signs a palette starts with: what each looks for.
var palModes = []struct {
	sign byte
	name func() string
	help func() string
}{
	{'@', locales.PaletteModePages, locales.PaletteModePagesHelp},
	{'>', locales.PaletteModeCommands, locales.PaletteModeCommandsHelp},
	{'#', locales.PaletteModeTags, locales.PaletteModeTagsHelp},
	{':', locales.PaletteModeLine, locales.PaletteModeLineHelp},
	{'/', locales.PaletteModeFind, locales.PaletteModeFindHelp},
	{'!', locales.PaletteModeRecent, locales.PaletteModeRecentHelp},
	{'?', locales.PaletteModeHelp, locales.PaletteModeHelpHelp},
}

// palModeOf is the sign of the mode a query is in: '@' for none written.
func palModeOf(q string) byte {
	if q != "" && strings.IndexByte(">#:/!?", q[0]) >= 0 {
		return q[0]
	}
	return '@'
}

// modeItems are the lines of the modes after the commands and the pages: tags, a line to go to, a word in
// the page, the commands used last, and the help.
func (a *app) modeItems(mode byte, q string) []palItem {
	var out []palItem
	switch mode {
	case '#':
		for _, ti := range a.allTags() {
			if q != "" && match(q, ti.name) < 0 {
				continue
			}
			key := ti.key
			out = append(out, palItem{title: "#" + ti.name, sub: locales.PaletteTagSub(len(ti.pages)), glyph: iconTag, score: match(q, ti.name), to: place{"tag", key}, run: func() { a.goTo(place{"tag", key}) }})
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
		if len(out) == 0 {
			out = append(out, palItem{title: locales.PaletteNoTags(), sub: locales.PaletteNoTagsSub(), glyph: iconTag, off: locales.PaletteNoSuchTag()})
		}
	case ':':
		return a.colonItems(q)
	case '/':
		if a.pane().page == nil || a.pane().doc == nil || a.noPage() {
			out = append(out, palItem{title: locales.PaletteSearchPage(), sub: locales.PaletteNoPageOpen(), glyph: iconSearch, off: locales.PaletteOpenPageToSearch()})
		} else {
			out = append(out, palItem{title: locales.PaletteSearchPageFor(q), sub: "⌘F", glyph: iconSearch, run: func() {
				a.openFind()
				a.pane().find.query, a.pane().find.ver = q, 0
			}})
		}
	case '!':
		all := a.commands()
		for _, t := range a.settings.RecentCmds {
			for _, it := range all {
				if it.id == t && (q == "" || match(q, it.title) >= 0) {
					out = append(out, it)
				}
			}
		}
		if len(out) == 0 {
			out = append(out, palItem{title: locales.PaletteNoRecent(), glyph: iconSettings, off: locales.PaletteNoRecentHelp()})
		}
	case '?':
		for _, m := range palModes {
			m := m
			out = append(out, palItem{title: string(m.sign) + "  " + m.name(), sub: m.help(), glyph: iconSearch, keys: string(m.sign), run: func() {
				a.pal = palette{open: true}
				if m.sign != '@' {
					a.pal.query = string(m.sign)
				}
			}})
		}
	}
	return out
}

// countParagraphs is how many paragraphs the text has.
func countParagraphs(t richtext.Text) int {
	n := 0
	for range t.Paragraphs() {
		n++
	}
	return max(n, 1)
}

// goToParagraph puts the caret at the start of paragraph n (from 1) and shows it.
func (a *app) goToParagraph(n int) {
	i := 0
	for at := range a.pane().doc.Text().Paragraphs() {
		i++
		if i == n {
			a.pane().doc.SetSelection(at, at)
			break
		}
	}
	a.pane().refocus = true
	a.pane().revealWant = true
}

// foldText normalizes s for matching via textnorm.Fold.
func foldText(s string) string { return textnorm.Fold(s) }

// match scores how well q matches s via textnorm.Match.
func match(q, s string) int { return textnorm.Match(q, s) }

// matchFolded scores a pre-folded query against a pre-folded target string via textnorm.MatchFolded.
func matchFolded(q string, words []string, qr []rune, s string) int {
	return textnorm.MatchFolded(q, words, qr, s)
}

// paletteView shows the palette while it is open.
func (a *app) paletteView(c *ui.Context) {
	if !a.pal.open {
		a.px.dirty = true  // the places are made again when it opens
		a.palRestore()     // closed without choosing: what a preview changed goes back
		a.pal.have = false // and the lines: the commands' state, the backups, the recent pages may have changed
		return
	}
	t := ryugo.From(c)
	cl := t.Colors
	ui.DialogBase(c.Key("palette"), &a.pal.open, func(backdrop, panel ui.Element) {
		panel.Width(480).MaxWidthPercent(100).Radius(rad(ryugo.RadiusPop))
		line := cl.Hairline
		if a.settings.Glass && runtime.GOOS == "darwin" && c.Vibrancy() {
			// as the side panel's glass (#486): the page behind is barely dimmed, as a dark scrim under the glass
			// made it a gray blur that the list did not read on; the card's tint is thick enough for the text, its
			// edge lit from the top, and the line under the field a faint one of the text's color
			backdrop.Justify(ui.Start).Padding(70, 16, 16, 16).Background(cl.Scrim.Alpha(0.1))
			fill := float32(0.66)
			if t.Dark {
				fill = 0.74
			}
			panel.Material(glass.Glass{Clarity: 0.4})
			frameCard(c, panel, cl.Card.Alpha(fill), rad(ryugo.RadiusPop), 2)
			line = c.Theme().Text.Alpha(0.08)
		} else {
			backdrop.Justify(ui.Start).Padding(70, 16, 16, 16).Background(cl.Scrim.Alpha(0.35))
			panel.Background(cl.Card).Border(1, cl.Hairline)
			t.Shadows.ShadowPop.Apply(panel)
		}
		panel.Clip().Label(a.palLabel())
		items := a.palLines()
		if a.pal.query != a.pal.was {
			a.pal.sel, a.pal.was, a.pal.hseen, a.pal.reveal = 0, a.pal.query, -1, true
		}
		a.pal.sel = max(0, min(a.pal.sel, len(items)-1))
		a.palPreview(items)
		panel.Children(func() {
			ui.Row(c).Padding(9, 12).Gap(8).AlignItems(ui.Center).Children(func() {
				// the mode is the sign at the left, not a letter in the field: ">" the commands, "@" the pages,
				// "#" the tags, ":" a line, "/" a word in the page, "!" the last commands, "?" help
				mode := palModeOf(a.pal.query)
				sign := string(mode)
				a.pal.text = a.pal.query
				if mode != '@' {
					a.pal.text = a.pal.query[1:]
				}
				ui.Text(c, sign).FontSize(15).FontWeight(700).TextColor(cl.Accent).Width(15).TextAlign(ui.Center)
				in := ui.TextInputBase(c.Key("palette-in"), &a.pal.text).Label(locales.PaletteInputLabel()).
					Placeholder(palPlaceholder(mode)).PlaceholderColor(cl.Muted).
					FontSize(14).Grow(1).FocusRing(false).AutoFocus()
				a.fieldMenu(in, &a.pal.text, fieldOpts{oneLine: true, changed: func() {
					a.pal.query = a.pal.text
					if mode != '@' {
						a.pal.query = string(mode) + a.pal.text
					}
				}})
				if in.Changed() {
					// a sign written first sets the mode, and is not left in the field
					m := mode
					if t := a.pal.text; t != "" && strings.IndexByte(">@#:/!?", t[0]) >= 0 {
						m, a.pal.text = t[0], t[1:]
					}
					a.pal.query = a.pal.text
					if m != '@' {
						a.pal.query = string(m) + a.pal.text
					}
				}
				// the field takes the arrows and ⌫ before any shortcut around it: what they do in the list is
				// asked of it as they come, and done as the palette is built next (#603)
				drill := a.pal.text == "" && len(items) > 0 && items[a.pal.sel].drill
				canBack, inGroup := a.palCanBack(), a.pal.cat != "" && a.pal.text == ""
				in.OnTextKey(func(k ui.TextKey) bool {
					switch {
					case k.Mods != 0:
						return false
					case k.Key == ui.KeyLeft && canBack, k.Key == ui.KeyBackspace && inGroup:
						a.pal.move = -1 // out of the group or the family, not a caret move
					case k.Key == ui.KeyRight && drill:
						a.pal.move = 1 // into the group or the family chosen
					default:
						return false
					}
					return true
				})
				move := a.pal.move
				a.pal.move = 0
				switch {
				case move < 0:
					a.palBack()
				case move > 0 && len(items) > 0:
					a.runItem(items[a.pal.sel])
				case in.Shortcut(0, ui.KeyDown):
					a.pal.sel, a.pal.reveal = min(a.pal.sel+1, len(items)-1), true
				case in.Shortcut(0, ui.KeyUp):
					a.pal.sel, a.pal.reveal = max(a.pal.sel-1, 0), true
				case in.Submitted() && len(items) > 0:
					a.runItem(items[a.pal.sel])
				}
			})
			ui.Box(c).Height(1).Background(line)
			a.scrolled(c, "palette-list", nil, ui.Scroll(c).MaxHeight(300), func() {
				// more room under the last row than over the first: its highlight clear of the card's round corners (#602)
				ui.Column(c).Padding(4, 4, 8, 4).Gap(1).Children(func() {
					if len(items) == 0 {
						ui.Text(c, locales.PaletteNothingMatches()).FontSize(12).TextColor(cl.Muted).Padding(9, 8)
						return
					}
					head := ""
					for i, it := range items {
						h := it.group
						if h == "" && a.palQuery() == "" {
							h = "recentlyOpened"
							if a.palCmds() {
								h = "common_commands"
							}
						}
						if h != "" && h != head {
							head = h
							ui.Text(c.Key("head:"+h), msg(h)).FontSize(10.5).FontWeight(650).TextColor(cl.Muted).Padding(6, 8, 3, 8)
						}
						a.paletteRow(c, i, it)
					}
				})
			})
		})
	})
}

// palLines are the palette's lines for its query, made only when what they come from changed: made at every frame,
// they were every command with its closures, the backups read from the disk, every page of the library (#186).
func (a *app) palLines() []palItem {
	k := palKey{query: a.pal.query, cat: a.pal.cat, pages: PagesGen.Load(), theme: a.settings.Theme, look: a.settings.AppTheme, caret: a.caretKind}
	if a.pane().doc != nil {
		k.text = a.pane().doc.Version()
	}
	if !a.pal.have || a.pal.key != k {
		a.pal.items, a.pal.key, a.pal.have = a.paletteItems(a.pal.query), k, true
		a.pal.made++
	}
	return a.pal.items
}

func (a *app) palLabel() string {
	for _, m := range palModes {
		if m.sign == palModeOf(a.pal.query) {
			return m.name()
		}
	}
	return locales.CommonFindPage()
}

func palPlaceholder(mode byte) string {
	switch mode {
	case '>':
		return locales.ACommandOrSettingFontSize()
	case '#':
		return locales.PalettePlaceholderTags()
	case ':':
		return locales.PalettePlaceholderLine()
	case '/':
		return locales.PalettePlaceholderFind()
	case '!':
		return locales.PalettePlaceholderRecent()
	case '?':
		return locales.PalettePlaceholderHelp()
	}
	return locales.GoToAPageCommandTag()
}

func (a *app) paletteRow(c *ui.Context, i int, it palItem) {
	t := ryugo.From(c)
	cl := t.Colors
	height := float32(28)
	if len(it.snip) > 0 {
		height = 44
	}
	row := ui.Row(c.Key(i)).Height(height).Padding(0, 8).Gap(8).Radius(rad(ryugo.RadiusControl - 4)).AlignItems(ui.Center).
		Label(it.title).Role(ui.RoleButton)
	text, muted := cl.Text, cl.Muted
	if it.off != "" { // not to be run now: dim
		text, muted = cl.Muted, cl.Muted.Alpha(0.7)
	}
	// one row lit: the pointer's, once it MOVES - a pointer left resting over the list lit whatever row
	// the filter put under it, and Enter then ran that one, not the first
	if x, y, over := row.PointerPosition(); over {
		b := row.LayoutBounds()
		moved := b.X+x != a.pal.hx || b.Y+y != a.pal.hy
		a.pal.hx, a.pal.hy = b.X+x, b.Y+y
		if moved && a.pal.hseen != i {
			a.pal.hseen, a.pal.sel = i, i
			c.Invalidate()
		}
	}
	if i == a.pal.sel {
		if a.pal.reveal {
			row.ScrollIntoView()
			a.pal.reveal = false
		}
		row.LinearGradient(chosenFill(t))
		text, muted = cl.OnAccent, cl.OnAccent.Alpha(0.75)
	}
	row.Children(func() {
		ui.Icon(c, it.glyph).Width(14).Height(14).TextColor(muted)
		if len(it.snip) > 0 { // the page, and the words around what was asked, marked
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, it.title).FontSize(12.5).FontWeight(600).TextColor(text).MaxLines(1).TruncateMiddle().Shrink(1)
					ui.Text(c, it.sub).FontSize(11).TextColor(muted).MaxLines(1).Shrink(2)
				})
				var spans []ui.Span
				for _, sg := range it.snip {
					sp := ui.Span{Text: sg.text, Size: 11.5, Color: muted}
					if sg.hit {
						sp.Weight, sp.Color = 700, text
						if i != a.pal.sel {
							sp.Background, sp.Radius = cl.Accent.Alpha(0.2), 3
						}
					}
					spans = append(spans, sp)
				}
				ui.RichText(c, spans...).MaxLines(1)
			})
		} else {
			ui.Text(c, it.title).FontSize(12.5).TextColor(text).MaxLines(1).TruncateMiddle().Shrink(1)
		}
		if len(it.snip) == 0 && it.sub != "" && it.kind != palCommand {
			ui.Text(c, it.sub).FontSize(11.5).TextColor(muted).MaxLines(1).Shrink(1)
		}
		ui.Spacer(c)
		if it.keys != "" {
			ui.Text(c, it.keys).FontSize(11.5).TextColor(muted)
		}
	})
	if row.Clicked() {
		a.runItem(it)
	}
}

func (a *app) runItem(it palItem) {
	if it.off != "" { // it cannot be run now: the palette stays, and says why
		a.say(toastWarn, it.off)
		return
	}
	a.pal.open = false
	a.palRestore() // the command sets what it sets, from where it was, and keeps it
	if it.run != nil {
		if it.kind != palOther { // a command: remembered, so that it comes first next time
			a.rememberCmd(cmp.Or(it.id, it.title))
			a.noteCommand(cmp.Or(it.id, it.title))
		}
		it.run()
	}
}

// rememberCmd puts the command first among those used last, at most twelve.
func (a *app) rememberCmd(title string) {
	r := []string{title}
	for _, t := range a.settings.RecentCmds {
		if t != title && len(r) < 12 {
			r = append(r, t)
		}
	}
	a.settings.RecentCmds = r
	a.saveSettingsSoon()
}

// palPreview shows the chosen command if it can be shown (a theme, a caret): while the choice moves
// from one to another, the app is in each. Another that cannot be shown puts it back.
func (a *app) palPreview(items []palItem) {
	key := ""
	var show func()
	if a.pal.sel >= 0 && a.pal.sel < len(items) && items[a.pal.sel].preview != nil {
		key, show = items[a.pal.sel].title, items[a.pal.sel].preview
	}
	if key == a.pal.pvKey {
		return
	}
	a.pal.pvKey = key
	if show == nil {
		a.palRestore()
		return
	}
	if s := a.pal.snap; s == nil {
		a.pal.snap = &palSnap{a.settings.Theme, a.settings.AppTheme, a.caretKind}
	} else { // what the one before showed goes first: a color theme after "Theme: Dark" is not shown dark (#512)
		a.settings.Theme, a.settings.AppTheme, a.caretKind = s.theme, s.appTheme, s.caret
	}
	show()
}

// palRestore puts back what a preview changed.
func (a *app) palRestore() {
	s := a.pal.snap
	if s == nil {
		return
	}
	a.pal.snap, a.pal.pvKey = nil, ""
	a.settings.Theme, a.settings.AppTheme, a.caretKind = s.theme, s.appTheme, s.caret
}
