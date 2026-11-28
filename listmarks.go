package main

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// Lists of the app's own look (#493): a bulleted list whose mark is an emoji or an icon, a numbered list in Roman
// numerals, and a list of tasks with a bullet or a number before each box. What a list looks like is said by the
// ID of one of its items (listIDPrefix and a spec), the first that has one, and holds for every item of the list:
// an item Enter makes, which has no ID, looks as the others do. richtext draws its own marker; the app's is drawn
// over it (drawListMarks), as the boxes of tasks are (drawTasks).

// listIDPrefix starts the ID of a list's item that says how the list looks: its spec after it.
const listIDPrefix = "celestine:list:"

// listSpec is how a list looks.
type listSpec struct {
	mark string // a bulleted list's mark: an emoji, or iconMarkPrefix and an icon's name
	num  string // a numbered list's numbers, or a list of tasks' with numbers: "roman" or "ROMAN"
	task string // a list of tasks': "bullet" or "number" before each box
}

// iconMarkPrefix marks a bullet's mark that is an icon (listIcons).
const iconMarkPrefix = "icon:"

// listEmojis are the emojis a bulleted list's mark can be.
var listEmojis = []string{"⭐", "👉", "✅", "🔥", "💡", "📌", "➡️", "❤️", "✨", "🎯"}

// listIcons are the icons a bulleted list's mark can be, each with its name in the menu.
var listIcons = []struct {
	name, title string
	icon        *ui.SVG
}{
	{"arrow", "lists_iconArrow", icon(`<path d="M5 12h14M13 6l6 6-6 6"/>`)},
	{"chevron", "lists_iconChevron", icon(`<path d="M9.5 6.5 15 12l-5.5 5.5"/>`)},
	{"star", "lists_iconStar", icon(`<path d="m12 4 2.4 5 5.4.7-4 3.8 1 5.4-4.8-2.6-4.8 2.6 1-5.4-4-3.8 5.4-.7z"/>`)},
	{"heart", "lists_iconHeart", icon(`<path d="M12 19s-7-4.4-7-9.2A3.8 3.8 0 0 1 12 7.5a3.8 3.8 0 0 1 7 2.3C19 14.6 12 19 12 19z"/>`)},
	{"diamond", "lists_iconDiamond", icon(`<path d="m12 4 7 8-7 8-7-8z"/>`)},
	{"dash", "lists_iconDash", icon(`<path d="M7 12h10"/>`)},
	{"check", "lists_iconCheck", icon(`<path d="m5.5 12.5 4.2 4.2 8.8-9.4"/>`)},
}

// listIconNamed is the icon a bullet's mark names, or nil.
func listIconNamed(name string) *ui.SVG {
	for _, ic := range listIcons {
		if ic.name == name {
			return ic.icon
		}
	}
	return nil
}

// parseListSpec reads the spec of a paragraph's ID; false when the ID says none.
func parseListSpec(id string) (listSpec, bool) {
	rest, ok := strings.CutPrefix(id, listIDPrefix)
	if !ok {
		return listSpec{}, false
	}
	v, err := url.ParseQuery(rest)
	if err != nil {
		return listSpec{}, false
	}
	return listSpec{mark: v.Get("mark"), num: v.Get("num"), task: v.Get("task")}, true
}

// id is the spec as an item's ID: "" for a list of the usual look.
func (s listSpec) id() string {
	v := url.Values{}
	if s.mark != "" {
		v.Set("mark", s.mark)
	}
	if s.num != "" {
		v.Set("num", s.num)
	}
	if s.task != "" {
		v.Set("task", s.task)
	}
	if len(v) == 0 {
		return ""
	}
	return listIDPrefix + v.Encode()
}

// romanNumeral is n in Roman numerals, in capitals when upper: past 3999, in digits.
func romanNumeral(n int, upper bool) string {
	if n <= 0 || n >= 4000 {
		return strconv.Itoa(n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"m", "cm", "d", "cd", "c", "xc", "l", "xl", "x", "ix", "v", "iv", "i"}
	var b strings.Builder
	for i, val := range vals {
		for n >= val {
			b.WriteString(syms[i])
			n -= val
		}
	}
	if upper {
		return strings.ToUpper(b.String())
	}
	return b.String()
}

// listItem is what the app knows of a paragraph that is an item of a list: its list's spec, its place among the
// items of its level (from 1), and where its list's items are.
type listItem struct {
	spec  listSpec
	n     int
	group *listGroup
}

// listGroup is a list: the items of one level next to each other, of one kind, the deeper ones between them
// skipped, as richtext numbers them.
type listGroup struct {
	kind  richtext.ListKind
	spec  listSpec
	has   bool  // an item says the spec
	paras []int // the items' paragraphs, by index
	start []int // and where they start
}

// listMarks is what the lists of a text are, read once for each text (as codeLangOf reads languages).
type listMarks struct {
	text    richtext.Text
	items   map[int]*listItem // by paragraph index; only the items of a list with a spec
	all     map[int]*listItem // every item of a list
	byStart map[int]int       // the index of the paragraph that starts at a rune
}

// listsOf reads the lists of t: each item's list and the list's spec.
func listsOf(t richtext.Text) *listMarks {
	m := &listMarks{text: t, items: map[int]*listItem{}, all: map[int]*listItem{}, byStart: map[int]int{}}
	open := map[int]*listGroup{} // the list open at each level
	i := 0
	for at, para := range t.Paragraphs() {
		st := para.Style
		m.byStart[at] = i
		switch {
		case st.List == richtext.NoList:
			clear(open)
		default:
			for lvl := range open {
				if lvl > st.Indent {
					delete(open, lvl)
				}
			}
			g := open[st.Indent]
			if g == nil || g.kind != st.List {
				g = &listGroup{kind: st.List}
				open[st.Indent] = g
			}
			if s, ok := parseListSpec(st.ID); ok && !g.has {
				g.spec, g.has = s, true
			}
			g.paras = append(g.paras, i)
			g.start = append(g.start, at)
			m.all[i] = &listItem{n: len(g.paras), group: g}
		}
		i++
	}
	for p, it := range m.all {
		if it.group.has {
			it.spec = it.group.spec
			m.items[p] = it
		}
	}
	return m
}

// lists is what the lists of the active page's text are.
func (a *app) lists() *listMarks { return a.listsIn(a.pane()) }

func (a *app) listsIn(pn *paneState) *listMarks {
	t := pn.doc.Text()
	if a.listMarks == nil || a.listMarks.text != t {
		a.listMarks = listsOf(t)
	}
	return a.listMarks
}

// listLooks is lists for what is drawn at every frame: the lists that have a look (items). A text whose paragraphs
// have no look at all, as most have, makes none of the maps of every item, which lists made again after each change:
// 5 ms a key on a page of thirty thousand paragraphs (#543).
func (a *app) listLooks(pn *paneState) *listMarks {
	t := pn.doc.Text()
	if a.looks == nil || a.looks.text != t {
		if hasListLook(t) {
			a.looks = a.listsIn(pn)
		} else {
			a.looks = &listMarks{text: t}
		}
	}
	return a.looks
}

// hasListLook reports whether a paragraph of t says how its list looks.
func hasListLook(t richtext.Text) bool {
	for _, para := range t.Paragraphs() {
		if strings.HasPrefix(para.Style.ID, listIDPrefix) {
			return true
		}
	}
	return false
}

// listItemAt is the item paragraph index p is, with its list's spec; nil when its list has none.
func (a *app) listItemAt(pn *paneState, p int) *listItem {
	if pn.doc == nil {
		return nil
	}
	return a.listLooks(pn).items[p]
}

// ownMarker reports whether the app draws the marker of paragraph index p, of style st, itself, for the editor to
// paint none (EditorOptions.NoMarker): a task's box outside a heading (drawTasks), and the bullet or number of a list
// whose spec gives its own (drawListMarks). The app covered the editor's with the page's color before, which on the
// glass of macOS, where the page is a card the desktop shows through, was a dark square round the box (#664).
func (a *app) ownMarker(pn *paneState, p int, st richtext.ParagraphStyle) bool {
	if st.Heading > 0 || a.theme == nil {
		return false
	}
	switch st.List {
	case richtext.Tasks:
		return true
	case richtext.Bullets, richtext.Numbers:
		it := a.listItemAt(pn, p)
		if it == nil || it.group.kind != st.List {
			return false
		}
		if st.List == richtext.Bullets {
			return it.spec.mark != ""
		}
		return it.spec.num != ""
	}
	return false
}

// markedTask reports whether the task of paragraph index p has a bullet or a number before its box.
func (a *app) markedTask(pn *paneState, p int, st richtext.ParagraphStyle) bool {
	if st.List != richtext.Tasks {
		return false
	}
	it := a.listItemAt(pn, p)
	return it != nil && it.spec.task != ""
}

// listMarkText is what marks item it, of kind, as text: an emoji, or a number; "" for an icon or the usual mark.
func listMarkText(it *listItem, kind richtext.ListKind) string {
	switch {
	case kind == richtext.Bullets && it.spec.mark != "" && !strings.HasPrefix(it.spec.mark, iconMarkPrefix):
		return it.spec.mark
	case kind == richtext.Numbers && it.spec.num != "", kind == richtext.Tasks && it.spec.task == "number":
		switch it.spec.num {
		case "roman":
			return romanNumeral(it.n, false) + "."
		case "ROMAN":
			return romanNumeral(it.n, true) + "."
		}
		return strconv.Itoa(it.n) + "."
	case kind == richtext.Tasks && it.spec.task == "bullet":
		if it.spec.mark != "" && !strings.HasPrefix(it.spec.mark, iconMarkPrefix) {
			return it.spec.mark
		}
		return "•"
	}
	return ""
}

// drawListMarks draws the marks of the lists that have a spec where richtext's would be (it paints none: ownMarker),
// in view; r is the editor's box.
func (a *app) drawListMarks(pn *paneState, p *ui.Painter, r ui.Rect) {
	if pn.doc == nil || a.theme == nil {
		return
	}
	lm := a.listLooks(pn)
	if len(lm.items) == 0 {
		return
	}
	size := a.font.Size
	indent := size * a.theme.Indent
	for _, b := range a.shown(pn, r) {
		it := lm.items[b.Index]
		if it == nil {
			continue
		}
		s, e, para := a.paraOfBox(pn, b)
		st := para.Style
		if st.Heading > 0 || st.List != it.group.kind {
			continue
		}
		cb, ok := locateIn(pn.doc, b.Start, b, s, e, para, pn.editorR.W, a.font, a.theme)
		if !ok {
			continue
		}
		x, line := r.X+cb.full.X, ui.Rect{X: r.X + cb.full.X, Y: r.Y + cb.full.Y, W: 0, H: cb.full.H}
		switch st.List {
		case richtext.Bullets:
			if it.spec.mark == "" {
				continue
			}
			a.drawBullet(p, it.spec.mark, x-indent*0.5, line, size)
		case richtext.Numbers:
			if it.spec.num == "" {
				continue
			}
			a.drawNumber(p, listMarkText(it, st.List), x-indent*0.25, line, size)
		case richtext.Tasks:
			if it.spec.task == "" {
				continue
			}
			box := markedTaskBox(x, line, size)
			if it.spec.task == "number" {
				a.drawNumber(p, listMarkText(it, st.List), box.X-size*0.3, line, size)
			} else {
				mark := it.spec.mark
				if mark == "" {
					mark = "•"
				}
				a.drawBullet(p, mark, box.X-size*0.55, line, size)
			}
		}
	}
}

// markedTaskBox is where the box of a task with a mark before it goes: at the end of the marker's room, so that
// the mark has its start; x is where the text starts and line its first line, in the window.
func markedTaskBox(x float32, line ui.Rect, size float32) ui.Rect {
	box := size * 0.95
	return ui.Rect{X: x - box - size*0.32, Y: line.Y + (line.H-box)/2, W: box, H: box}
}

// drawBullet draws a bullet's mark centered at cx on the line: an emoji, an icon, or the usual dot.
func (a *app) drawBullet(p *ui.Painter, mark string, cx float32, line ui.Rect, size float32) {
	if name, ok := strings.CutPrefix(mark, iconMarkPrefix); ok {
		if ic := listIconNamed(name); ic != nil {
			s := size * 0.72
			p.Icon(ic, ui.Rect{X: cx - s/2, Y: line.Y + (line.H-s)/2, W: s, H: s}, a.theme.Marker)
			return
		}
		mark = "•"
	}
	f := a.font
	if utf8.RuneCountInString(mark) > 0 && mark != "•" {
		f.Size = size * 0.86 // an emoji, a little smaller than the letters
	}
	l := ui.ShapeText(mark, f, 0)
	w, h := l.Size()
	p.TextLayout(l, cx-w/2, line.Y+(line.H-h)/2, a.theme.Marker)
}

// drawNumber draws a number's mark ending at right on the line.
func (a *app) drawNumber(p *ui.Painter, mark string, right float32, line ui.Rect, size float32) {
	l := ui.ShapeText(mark, a.font, 0)
	w, h := l.Size()
	p.TextLayout(l, right-w, line.Y+(line.H-h)/2, a.theme.Marker)
}

// setListSpec gives the list paragraph index p is an item of the look spec: said by its first item, the others'
// specs taken off. A list of tasks given a mark before their boxes, or taken off it, keeps the kinds of its tasks
// but its first's, whose ID says the list's look.
func (a *app) setListSpec(p int, spec listSpec) {
	it := a.lists().all[p]
	if it == nil {
		return
	}
	g, id := it.group, spec.id()
	a.pane().doc.Edit(func(ed *richtext.Edit) {
		for k, at := range g.start {
			ed.FormatParagraphs(at, at, func(st *richtext.ParagraphStyle) {
				switch {
				case k == 0:
					if id != "" || strings.HasPrefix(st.ID, listIDPrefix) {
						st.ID = id
					}
				case strings.HasPrefix(st.ID, listIDPrefix):
					st.ID = ""
				}
			})
		}
	})
}

// listToTasks turns the list paragraph index p is an item of into a list of tasks that keeps its bullets or its
// numbers before the boxes.
func (a *app) listToTasks(p int) {
	it := a.lists().all[p]
	if it == nil {
		return
	}
	spec := it.spec
	switch it.group.kind {
	case richtext.Bullets:
		spec.task = "bullet"
	case richtext.Numbers:
		spec.task = "number"
	default:
		return
	}
	g := it.group
	id := spec.id()
	a.pane().doc.Edit(func(ed *richtext.Edit) {
		for k, at := range g.start {
			ed.FormatParagraphs(at, at, func(st *richtext.ParagraphStyle) {
				st.List, st.Checked = richtext.Tasks, false
				if k == 0 {
					st.ID = id
				} else if strings.HasPrefix(st.ID, listIDPrefix) {
					st.ID = ""
				}
			})
		}
	})
}

// listItems are the lines of a block's menu for the paragraph at rune at, an item of a list: how its list looks.
func (a *app) listItems(at int) []menuItem {
	lm := a.lists()
	s, _ := lm.text.ParagraphRange(at)
	p, ok := lm.byStart[s]
	if !ok {
		return nil
	}
	it := lm.all[p]
	if it == nil {
		return nil
	}
	spec := it.spec
	set := func(s listSpec) func() { return func() { a.setListSpec(p, s) } }
	marks := func(task bool) func() []menuItem {
		return func() []menuItem {
			base := spec
			if task {
				base.task = "bullet"
			}
			dot := base
			dot.mark = ""
			out := []menuItem{{title: "lists_markDot", hint: tick(spec.mark == "" && (!task || spec.task == "bullet")), run: set(dot)}}
			for _, ic := range listIcons {
				s := base
				s.mark = iconMarkPrefix + ic.name
				out = append(out, menuItem{title: ic.title, glyph: ic.icon, hint: tick(spec.mark == s.mark && (!task || spec.task == "bullet")), run: set(s)})
			}
			for _, e := range listEmojis {
				s := base
				s.mark = e
				out = append(out, menuItem{title: e, group: "emoji", hint: tick(spec.mark == e && (!task || spec.task == "bullet")), run: set(s)})
			}
			return out
		}
	}
	numbers := func(task bool) func() []menuItem {
		return func() []menuItem {
			var out []menuItem
			for _, n := range []struct{ title, num string }{{"1, 2, 3", ""}, {"i, ii, iii", "roman"}, {"I, II, III", "ROMAN"}} {
				s := spec
				s.num = n.num
				if task {
					s.task = "number"
				}
				on := spec.num == n.num && (!task || spec.task == "number")
				out = append(out, menuItem{title: n.title, hint: tick(on), run: set(s)})
			}
			return out
		}
	}
	switch it.group.kind {
	case richtext.Bullets:
		return []menuItem{
			{title: "lists_mark", glyph: kindIcon["bullets"], items: marks(false)},
			{title: "lists_asTasks", glyph: kindIcon["tasks"], run: func() { a.listToTasks(p) }},
		}
	case richtext.Numbers:
		return []menuItem{
			{title: "lists_numbering", glyph: kindIcon["numbers"], items: numbers(false)},
			{title: "lists_asTasks", glyph: kindIcon["tasks"], run: func() { a.listToTasks(p) }},
		}
	case richtext.Tasks:
		plain := spec
		plain.task = ""
		return []menuItem{{title: "lists_beforeBox", glyph: kindIcon["tasks"], items: func() []menuItem {
			return []menuItem{
				{title: "lists_beforeNothing", hint: tick(spec.task == ""), run: set(plain)},
				{title: "lists_beforeBullet", nav: true, items: marks(true)},
				{title: "lists_beforeNumber", nav: true, items: numbers(true)},
			}
		}}}
	}
	return nil
}

// ---- a task in the running text ----

// checkAtEnd is a box typed after other words: "[ ]", "[]", or "[x]" for one done; the "[" not a wiki link's.
var checkAtEnd = regexp.MustCompile(`(^|[^\[])\[( |x|X)?\]$`)

// checkShortcut turns a box typed in the running text, after other words and outside code, into a task of its own
// there, a chip (#493); one at the start of a line is richtext's, a line that is a task. It reports whether it did,
// the caret after the box.
func (a *app) checkShortcut(text richtext.Text, caret int, typed bool) bool {
	if !typed || caret < 3 || text.Slice(caret-1, caret).String() != "]" {
		return false
	}
	ps, _ := text.ParagraphRange(caret - 1)
	if text.ParagraphStyleAt(ps).CodeBlock || text.AttributesAt(caret-1).Code {
		return false
	}
	line := text.Slice(ps, caret).String()
	m := checkAtEnd.FindStringSubmatchIndex(line)
	if m == nil || strings.TrimSpace(line[:m[3]]) == "" { // nothing before it: a line's box, richtext's
		return false
	}
	start := ps + utf8.RuneCountInString(line[:m[3]])
	ch := &chip{kind: "check"}
	if m[4] >= 0 && strings.EqualFold(line[m[4]:m[5]], "x") {
		ch.target = "x"
	}
	attrs := text.AttributesAt(start)
	attrs.Attachment = a.attachment(ch)
	a.pane().doc.Edit(func(ed *richtext.Edit) {
		ed.ReplaceString(start, caret, string(richtext.ObjectReplacement), attrs)
		ed.Select(start+1, start+1)
	})
	a.pane().doc.SetTypingAttributes(richtext.Attributes{})
	return true
}

// checkChipW is how wide a task in the running text is, in text of size: its box and a little room round it.
func checkChipW(size float32) float32 { return size*0.9 + 6 }

// drawCheckChip draws a task in the running text as a line's task is drawn (#613): BetterTask's open box, lit as it
// lights a box under the pointer while the pointer is on it or the caret is in its task, or its done tile with the
// white tick.
func (a *app) drawCheckChip(p *ui.Painter, r ui.Rect, ch *chip) {
	size := a.fontSize()
	s := size * 0.82
	mid := r.Y + r.H - 0.36*size
	box := ui.Rect{X: r.X + (r.W-s)/2, Y: mid - s/2, W: s, H: s}
	a.chipHits = append(a.chipHits, chipHit{ui.Rect{X: box.X - 3, Y: box.Y - 3, W: box.W + 6, H: box.H + 6}, ch})
	if a.theme == nil {
		return
	}
	f := taskFace{slot: -1, color: ui.Hex(taskDone)}
	if ch.target == "x" {
		a.drawFaceBack(p, box, f, p.Now())
		return
	}
	if a.hoverCh == ch || a.inlineChipActive(ch) {
		a.drawOpenLit(p, box)
		return
	}
	a.drawFace(p, box, f, p.Now())
}

// toggleCheckChip marks a task in the running text done, or not done again, in the active pane.
func (a *app) toggleCheckChip(ch *chip) { a.toggleCheckChipIn(a.pane(), ch) }

// toggleCheckChipIn marks a task in the running text done, or not done again, in pane pn.
func (a *app) toggleCheckChipIn(pn *paneState, ch *chip) {
	if pn == nil || pn.doc == nil {
		return
	}
	if a.pageLocked(pn.page) { // a past day (#622)
		a.sayLocked()
		return
	}
	doc := pn.doc
	at := -1
	for i, r := range doc.Text().Runs(0, doc.Text().Len()) { // this chip, not one that says the same
		if att := r.Attributes.Attachment; att != nil && att.Data == any(ch) {
			at = i
			break
		}
	}
	if at < 0 {
		return
	}
	next := &chip{kind: "check"}
	if ch.target != "x" {
		next.target = "x"
	}
	attrs := doc.Text().AttributesAt(at)
	attrs.Attachment = a.attachment(next)
	doc.Edit(func(ed *richtext.Edit) { ed.ReplaceString(at, at+1, string(richtext.ObjectReplacement), attrs) })
}
