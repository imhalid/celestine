package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/spell"
	"github.com/imhalid/celestine/internal/spell/model"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Turkish spell checking (#623), with tinyfix-tr: its engine is the library's port (internal/spell), its data is not
// in the app. Turned on in the settings, the model is downloaded from Hugging Face once and kept in the app's folder
// (internal/spell/model), and read from there from then on; the settings delete it, or download it again.

// spellStatus is where the spell checker's model is.
type spellStatus int

const (
	spellMissing     spellStatus = iota // not on the device
	spellDownloading                    // being downloaded
	spellReady                          // on the device
	spellFailed                         // its download failed (err says why)
)

// spellState is the spell checker's model and its download.
type spellState struct {
	status      spellStatus
	looked      bool // the device was looked at for the model
	done, total int64
	err         string
	cancel      context.CancelFunc
}

// spellRoot is the app's folder, where the model's folder is (model.Dir); a test's own folder in a test.
func spellRoot() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" && testing.Testing() {
		return filepath.Join(d, "celestine")
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "celestine")
	}
	return ""
}

// spellBase is where the model is downloaded from: Hugging Face, or a test's server.
var spellBase = model.BaseURL

// spellLook finds out, once, whether the model is on the device.
func (a *app) spellLook() {
	s := &a.spell
	if s.looked {
		return
	}
	s.looked = true
	root := spellRoot()
	switch {
	case root == "":
	case model.Installed(model.Dir(root)):
		s.status = spellReady
	case a.settings.SpellCheck && a.spellBegun(root): // on, and a file added since (the context model): it alone
		a.spellDownload(false)
	}
}

// spellBegun reports whether some of the model's files are on the device under root: those a download left, or the
// files of before one was added to the model.
func (a *app) spellBegun(root string) bool {
	for _, f := range model.Files {
		if _, err := os.Stat(filepath.Join(model.Dir(root), f.Name)); err == nil {
			return true
		}
	}
	return false
}

// spellDownload downloads the model, the files not there yet, or all of them again when again is set; the
// download goes on outside the lock and says how far it is.
func (a *app) spellDownload(again bool) {
	s := &a.spell
	root := spellRoot()
	if root == "" || s.status == spellDownloading {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.status, s.err, s.done, s.total, s.cancel = spellDownloading, "", 0, model.TotalSize(), cancel
	go func() {
		defer a.guard()
		err := model.Download(ctx, spellBase, model.Dir(root), again, func(done, total int64) {
			a.do(func() error {
				if a.spell.status == spellDownloading {
					a.spell.done, a.spell.total = done, total
				}
				return nil
			})
		})
		a.do(func() error {
			s := &a.spell
			if s.status != spellDownloading { // cancelled, or deleted meanwhile
				return nil
			}
			s.cancel = nil
			switch {
			case err == nil:
				s.status = spellReady
				a.say(toastOK, locales.SpellDownloaded())
			case ctx.Err() != nil:
				s.status = spellMissing
			default:
				s.status, s.err = spellFailed, err.Error()
			}
			return nil
		})
	}()
}

// spellCancel stops the download going on.
func (a *app) spellCancel() {
	s := &a.spell
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.status == spellDownloading {
		s.status = spellMissing
		if root := spellRoot(); root != "" && model.Installed(model.Dir(root)) {
			s.status = spellReady
		}
	}
}

// spellDelete deletes the model from the device; it is downloaded again when it is asked for.
func (a *app) spellDelete() {
	a.spellCancel()
	root := spellRoot()
	if root == "" {
		return
	}
	if err := model.Remove(root); err != nil {
		a.fail(err)
		return
	}
	a.spell.status, a.spell.err = spellMissing, ""
	a.spellUnload()
	a.say(toastOK, locales.SpellDeleted())
}

// settingsSpell is the spell checker's settings, in the Editor's section: on or off, and its model: downloaded or
// not, its download going on, deleted, or downloaded again.
func (a *app) settingsSpell(c *ui.Context) {
	a.spellLook()
	cl := ryugo.From(c).Colors
	s := &a.spell
	on := a.settings.SpellCheck
	settingGroup(c, "set-spell",
		func() {
			settingRow(c, "spell_title", "spell_help", func() {
				ryugo.Switch(c.Key("set-spell-on"), &on, ryugo.SwitchOptions{}).Label(msg("spell_title"))
			})
			if on != a.settings.SpellCheck {
				a.setSpellCheck(on)
			}
		},
		func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
						settingTitle(c, cl, locales.SpellModel())
						var state string
						switch s.status {
						case spellReady:
							state = locales.SpellModelReady(bytesText(model.TotalSize()))
						case spellDownloading:
							state = locales.SpellModelDownloading(bytesText(s.done), bytesText(s.total))
						case spellFailed:
							state = locales.SpellModelFailed(s.err)
						default:
							state = locales.SpellModelMissing(bytesText(model.TotalSize()))
						}
						ui.Text(c, state).FontSize(ryugo.FsSm).TextColor(map[bool]ui.Color{true: cl.Up, false: cl.Muted}[s.status == spellFailed]).Wrap()
					})
					btn := func(key, label string, v ryugo.Variant) bool {
						return ryugo.Button(c.Key(key), label, ryugo.ButtonOptions{Size: ryugo.Sm, Variant: v}).Clicked()
					}
					switch s.status {
					case spellDownloading:
						if btn("spell-cancel", msg("common_cancel"), ryugo.Soft) {
							a.spellCancel()
						}
					case spellReady:
						if btn("spell-again", locales.SpellDownloadAgain(), ryugo.Soft) {
							a.spellDownload(true)
						}
						if btn("spell-delete", msg("common_delete"), ryugo.Soft) {
							a.spellDelete()
						}
					default:
						if btn("spell-download", locales.SpellDownload(), ryugo.Solid) {
							a.spellDownload(false)
						}
					}
				})
				if s.status == spellDownloading && s.total > 0 {
					ryugo.Progress(c.Key("spell-progress"), float64(s.done)/float64(s.total), ryugo.ProgressOptions{Label: locales.SpellModel()})
				}
				ui.Text(c, locales.SpellCredits()).FontSize(ryugo.FsXs).TextColor(cl.Muted).Wrap()
			})
		},
	)
}

// setSpellCheck turns the spell checker on or off; on, its model is downloaded when it is not on the device.
func (a *app) setSpellCheck(on bool) {
	a.settings.SpellCheck = on
	a.saveSettings()
	if !on {
		a.spellUnload()
	}
	a.spellLook()
	if on && (a.spell.status == spellMissing || a.spell.status == spellFailed) {
		a.spellDownload(false)
	}
}

// ---- the engine, and the checks of the paragraphs ----

// spellChecks is how many paragraphs' checks are kept, by their text, before they are made again from none.
const spellChecks = 4000

// spellQueue is how many paragraphs wait to be checked at most: the newest asked for (what is being typed, what
// came into view); the context model makes a check cost some milliseconds, not to be spent on a text already gone.
const spellQueue = 6

// spellEngineState is the engine loaded from the model, and the checks of the paragraphs it made.
type spellEngineState struct {
	eng      *spell.Engine
	loading  bool
	checks   map[string]spell.InspectResult // by the paragraph's text as checked (spellText)
	queue    []string                       // texts to check, in the order asked
	ctxQueue []string                       // texts checked without the context model, to be checked with it
	lastFast time.Time                      // when the last check without the context ended
	working  bool                           // a goroutine checks the queues
	gen      int                            // moved by each change of what the underlines show
}

// spellSettle is how long the text stays as it is before its paragraphs are checked with the context model too: the
// check without it is a few milliseconds and comes at once, the one with it some hundreds, and not for every key.
const spellSettle = 800 * time.Millisecond

// spellOn reports whether the spell checker is on and its engine loaded, loading it the first time it is asked for.
func (a *app) spellOn() *spell.Engine {
	if !a.settings.SpellCheck {
		return nil
	}
	a.spellLook()
	s := &a.spellEng
	if s.eng != nil || s.loading || a.spell.status != spellReady {
		return s.eng
	}
	root := spellRoot()
	if root == "" {
		return nil
	}
	s.loading = true
	go func() { // some tens of milliseconds, and the frame does not wait
		defer a.guard()
		eng, err := spell.Load(model.Dir(root), spell.ModeCareful)
		a.do(func() error {
			s := &a.spellEng
			s.loading = false
			if err != nil {
				a.spell.status, a.spell.err = spellFailed, err.Error()
				return nil
			}
			if a.settings.SpellCheck && a.spell.status == spellReady {
				s.eng, s.checks = eng, map[string]spell.InspectResult{}
				s.gen++
			}
			return nil
		})
	}()
	return nil
}

// spellUnload lets the engine go: turned off, or its model deleted.
func (a *app) spellUnload() { a.spellEng = spellEngineState{gen: a.spellEng.gen + 1} }

// spellKey says what the spell checker's underlines in pn show, for the editor to ask for the spans again when it
// changes: its checks, and the caret, whose word is not underlined while it is typed. 0 while it is off.
func (a *app) spellKey(pn *paneState) int {
	if a.spellOn() == nil || pn.doc == nil {
		return a.spellEng.gen & 0xff
	}
	_, c := pn.doc.Selection()
	return (a.spellEng.gen&0xfff)*131071 + c%131071 + 1
}

// spellText is the text of paragraph para as it is checked: inline code and objects as spaces, the same length in
// runes, so that what is found lines up with the paragraph's runes and spans.
func spellText(para richtext.Paragraph) string {
	var b strings.Builder
	for _, run := range para.Runs {
		for _, r := range run.Text {
			if run.Attributes.Code || r == richtext.ObjectReplacement || r == '\n' {
				r = ' '
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

// spellCheckOf is the check of a paragraph's text, or false while it is being made: asked for, it is made outside the
// lock, and the window drawn again when it is there.
func (a *app) spellCheckOf(eng *spell.Engine, text string) (spell.InspectResult, bool) {
	s := &a.spellEng
	if r, ok := s.checks[text]; ok {
		return r, true
	}
	if len(s.checks) > spellChecks {
		s.checks = map[string]spell.InspectResult{}
	}
	if !slices.Contains(s.queue, text) {
		s.queue = append(s.queue, text)
		if len(s.queue) > spellQueue { // the oldest asked for: a paragraph's text of a few keys ago, or out of view
			s.queue = slices.Delete(s.queue, 0, len(s.queue)-spellQueue)
		}
	}
	if !s.working {
		s.working = true
		go a.spellWork(eng)
	}
	return spell.InspectResult{}, false
}

// spellWork checks the paragraphs asked for, in two passes (#623): first without the context model, a few
// milliseconds each, the newest first (what is being typed, or came into view last), whose underlines come at once;
// then, once nothing new was asked for spellSettle, with the context model too, the newest first again, its result
// taking the first's place. A paragraph asked for meanwhile goes first; one checked with the context is not again
// until its text changes. Without a context model the first pass is the whole check.
func (a *app) spellWork(eng *spell.Engine) {
	defer a.guard()
	for {
		var text string
		var withContext, ok bool
		var wait time.Duration
		a.do(func() error {
			s := &a.spellEng
			switch {
			case s.eng != eng:
				s.working = false
			case len(s.queue) > 0:
				text, s.queue, ok = s.queue[len(s.queue)-1], s.queue[:len(s.queue)-1], true
			case len(s.ctxQueue) > 0:
				if left := spellSettle - time.Since(s.lastFast); left > 0 {
					wait = left
					return nil
				}
				text, s.ctxQueue, ok, withContext = s.ctxQueue[len(s.ctxQueue)-1], s.ctxQueue[:len(s.ctxQueue)-1], true, true
			default:
				s.working = false
			}
			return nil
		})
		if wait > 0 {
			time.Sleep(wait)
			continue
		}
		if !ok {
			return
		}
		r := eng.Inspect(text, spell.CheckOptions{NoContext: !withContext})
		a.do(func() error {
			s := &a.spellEng
			if s.eng != eng {
				return nil
			}
			s.checks[text] = r
			s.gen++
			if !withContext {
				s.lastFast = time.Now()
				if eng.HasContext() && !slices.Contains(s.ctxQueue, text) {
					s.ctxQueue = append(s.ctxQueue, text)
					if len(s.ctxQueue) > spellQueue {
						s.ctxQueue = slices.Delete(s.ctxQueue, 0, len(s.ctxQueue)-spellQueue)
					}
				}
			}
			return nil
		})
	}
}

// spellSpans underlines with a wave the words of paragraph p the checker found: a word it would fix in the accent, a
// word it doubts in red. The word the caret is at the end of is not, while it is being typed.
func (a *app) spellSpans(pn *paneState, p int, spans []ui.Span) []ui.Span {
	eng := a.spellOn()
	if eng == nil || a.pageLocked(pn.page) || a.theme == nil {
		return spans
	}
	pt := a.paraTableOf(pn, pn.doc.Text())
	if p < 0 || p >= len(pt.paras) {
		return spans
	}
	text := spellText(pt.paras[p])
	if strings.TrimSpace(text) == "" {
		return spans
	}
	r, ok := a.spellCheckOf(eng, text)
	if !ok {
		return spans
	}
	caret := -1 // the caret in this paragraph's runes
	if _, c := pn.doc.Selection(); pn.focused && c >= pt.start[p] && (p+1 >= len(pt.start) || c < pt.start[p+1]) {
		caret = c - pt.start[p]
	}
	var fixes, doubts [][2]int
	for _, f := range r.Fixes {
		if f.To != caret {
			fixes = append(fixes, [2]int{f.From, f.To})
		}
	}
	for _, f := range r.Flags {
		if f.To != caret {
			doubts = append(doubts, [2]int{f.From, f.To})
		}
	}
	mark := func(segs [][2]int, c ui.Color) {
		if len(segs) > 0 {
			spans = restyleRunes(spans, segs, func(s *ui.Span) {
				if s.ObjectWidth == 0 {
					s.WavyUnderline, s.DecorationColor = true, c
				}
			})
		}
	}
	mark(fixes, a.theme.Link)
	mark(doubts, ui.Hex("#E5484D"))
	return spans
}

// spellFixTyped fixes, as a word is ended by a space or a mark typed after it, what the checker is sure of in it
// (#623): a word it would only doubt is left as it is, underlined. It reports whether it changed the text. typed is
// whether the text grew by what was typed, not an undo nor a paste; an undo of the fix leaves the word as typed.
func (a *app) spellFixTyped(text richtext.Text, caret int, typed bool) bool {
	pn := a.pane()
	if !typed || caret < 2 || pn.page == nil || a.pageLocked(pn.page) {
		return false
	}
	eng := a.spellOn()
	if eng == nil {
		return false
	}
	end := []rune(text.Slice(caret-1, caret).String())
	if len(end) != 1 || !unicode.IsSpace(end[0]) && !strings.ContainsRune(".,;:!?)", end[0]) {
		return false
	}
	ps, pe := text.ParagraphRange(caret - 1)
	if text.ParagraphStyleAt(ps).CodeBlock || text.AttributesAt(caret-2).Code {
		return false
	}
	runes := []rune(text.Slice(ps, pe).String())
	for i := range runes { // inline code and objects are not words
		if runes[i] == richtext.ObjectReplacement || text.AttributesAt(ps+i).Code {
			runes[i] = ' '
		}
	}
	w := caret - 1 - ps // the word just ended: back from the mark typed to a space
	ws := w
	for ws > 0 && !unicode.IsSpace(runes[ws-1]) {
		ws--
	}
	if ws == w {
		return false
	}
	kinds := []string{spell.KindTurkishLetters, spell.KindApostropheFix, spell.KindJoinedSeparate, spell.KindSpelling}
	// the core pass alone: the frame does not wait for the context model, whose decisions come with the paragraph's
	// check, as underlines with their fixes
	fixes := eng.Check(string(runes), spell.CheckOptions{Range: &[2]int{ws, w}, Kinds: kinds, NoContext: true})
	var take []spell.Fix
	for _, f := range fixes {
		if f.From >= 0 && f.To <= w && f.From < f.To { // before the mark typed, the caret's word and what it joins
			take = append(take, f)
		}
	}
	if len(take) == 0 {
		return false
	}
	slices.SortFunc(take, func(x, y spell.Fix) int { return y.From - x.From }) // from the end: the earlier stay where they are
	shift := 0
	pn.doc.Edit(func(ed *richtext.Edit) {
		for _, f := range take {
			attrs := text.AttributesAt(ps + f.From)
			attrs.Attachment = nil
			out := richtext.NewText(f.Replacement)
			out = out.Format(0, out.Len(), func(at *richtext.Attributes) { *at = attrs }) // the look of the word typed
			ed.Replace(ps+f.From, ps+f.To, out)
			shift += len([]rune(f.Replacement)) - (f.To - f.From)
		}
		ed.Select(caret+shift, caret+shift)
	})
	return true
}

// spellItems are the lines of the editor's menu for the word under the pointer, at rune at of the text, that the
// checker found: its fix, or its suggestions, each putting itself in its place, and "Ignore", which leaves the word
// as it is from then on.
func (a *app) spellItems(at int) []menuItem {
	pn := a.pane()
	eng := a.spellOn()
	if eng == nil || pn.doc == nil || a.pageLocked(pn.page) || at < 0 {
		return nil
	}
	t := pn.doc.Text()
	at = min(at, t.Len())
	ps, _ := t.ParagraphRange(at)
	pt := a.paraTableOf(pn, t)
	p := a.paraIndex(pn, t, ps)
	if p >= len(pt.paras) {
		return nil
	}
	text := spellText(pt.paras[p])
	r, ok := a.spellEng.checks[text]
	if !ok {
		return nil
	}
	in := at - ps
	put := func(from, to int, word string) func() {
		return func() {
			attrs := t.AttributesAt(ps + from)
			attrs.Attachment = nil
			out := richtext.NewText(word)
			out = out.Format(0, out.Len(), func(at *richtext.Attributes) { *at = attrs })
			pn.doc.Edit(func(ed *richtext.Edit) {
				ed.Replace(ps+from, ps+to, out)
				ed.Select(ps+from+out.Len(), ps+from+out.Len())
			})
			pn.refocus = true
		}
	}
	var out []menuItem
	original := ""
	for _, f := range r.Fixes {
		if in >= f.From && in <= f.To {
			out = append(out, menuItem{title: f.Replacement, group: "spell", glyph: iconCheck, run: put(f.From, f.To, f.Replacement)})
			original = f.Original
		}
	}
	for _, f := range r.Flags {
		if in >= f.From && in <= f.To {
			for _, s := range f.Suggestions {
				out = append(out, menuItem{title: s, group: "spell", run: put(f.From, f.To, s)})
			}
			original = f.Original
		}
	}
	if original != "" {
		out = append(out, menuItem{title: locales.SpellIgnore(original), group: "spell", run: func() {
			eng.Ignore(original)
			a.spellEng.checks = map[string]spell.InspectResult{} // checked again, without it
			a.spellEng.gen++
		}})
	}
	return out
}

// spellHit is where a word the spell checker found is drawn, and where it is in the text, its first rune.
type spellHit struct {
	r  ui.Rect
	at int
}

// spellPlaces notes where the words the spell checker found in the paragraphs in view are drawn, for the editor's
// menu to find the one under the pointer.
func (a *app) spellPlaces(pn *paneState, r ui.Rect) {
	if pn.doc == nil || a.spellEng.eng == nil || a.pageLocked(pn.page) {
		return
	}
	for _, b := range a.shown(pn, r) {
		s, e, para := a.paraOfBox(pn, b)
		if e <= s {
			continue
		}
		res, ok := a.spellEng.checks[spellText(para)]
		if !ok || len(res.Fixes)+len(res.Flags) == 0 {
			continue
		}
		var line string
		for _, run := range para.Runs {
			line += run.Text
		}
		l, left := a.paraLayout(pn, s, para, r.W)
		place := func(from, to int) {
			for _, rc := range l.SelectionRects(ui.TextInputRange{Start: units(line, from), End: units(line, to)}) {
				pn.spellHits = append(pn.spellHits, spellHit{ui.Rect{X: r.X + left + rc.X, Y: r.Y + b.Top + rc.Y, W: rc.W, H: rc.H}, s + from})
			}
		}
		for _, f := range res.Fixes {
			place(f.From, f.To)
		}
		for _, f := range res.Flags {
			place(f.From, f.To)
		}
	}
}

// spellAt is the first rune of the word the spell checker found under x, y in the window, or -1.
func (a *app) spellAt(pn *paneState, x, y float32) int {
	for _, h := range pn.spellSeen {
		if x >= h.r.X && x < h.r.X+h.r.W && y >= h.r.Y && y < h.r.Y+h.r.H {
			return h.at
		}
	}
	return -1
}
