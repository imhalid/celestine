package main

import (
	"github.com/imhalid/celestine/internal/library"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The task view: every "- [ ] …" line of the library in one place, by the
// page it is in, to be ticked off without going there. The tasks are read
// with the backlinks' index (backlinks.go), in the same pass; ticking one
// writes the page's file (and the page's text, if it is open) at once.

var taskLine = regexp.MustCompile(`^(\s*)(?:[-*+]|\d+[.)])\s+\[([ xX*!?/e])\]\s*(.*?)\s*$`)

// taskBox is the box of a task's line, and what is before it: a kind of task's too (taskkinds.go).
var taskBox = regexp.MustCompile(`^(\s*(?:[-*+]|\d+[.)])\s+)\[[ xX*!?/e]\]`)

// taskItem is a task: its page, which line of the page's Markdown, whether
// it is done, and what it says.
type taskItem struct {
	page  *Page
	line  int
	done  bool
	text  string
	due   time.Time // a day named in it, as [[2026-11-10]]; zero when none
	every string    // how it comes again, as "every week"; "" when it does not
	kind  string    // its kind's id (taskkinds.go), as its box says: "" a plain one, or a done one
	// inl is a task in the running text (a chip, "Buy [ ] milk"): its place among the boxes of its line, from 1; 0
	// for the line's own task. col is where its box starts in the line, -1 when that is not known (#536).
	inl, col int
	folded   string // text, folded for the view's search: once as the page is read, not at every frame (#195)
}

// tasksOf are the tasks in a page's Markdown, not those in code.
func tasksOf(p *Page, body string) []taskItem {
	if !strings.Contains(body, "[") {
		return nil
	}
	var out []taskItem
	inFence := false
	refX := strings.Contains(body, "[x]:") && refDefX.MatchString(body)
	for i, ln := range strings.Split(body, "\n") {
		if fenceLine.MatchString(ln) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if boxes := inlineBoxes(ln, refX); len(boxes) > 0 { // its words without the boxes of the tasks in it (#536)
			for k := len(boxes) - 1; k >= 0; k-- {
				ln = ln[:boxes[k]] + strings.TrimLeft(ln[boxes[k]+3:], " ")
			}
		}
		if m := taskLine.FindStringSubmatch(ln); m != nil && m[3] != "" {
			text, due, every := taskWords(m[3])
			kind := ""
			if k, ok := taskKindByMD(m[2]); ok {
				kind = k.id
			}
			out = append(out, taskItem{page: p, line: i, done: m[2] == "x" || m[2] == "X", text: text, due: due, every: every, kind: kind, folded: foldText(text)})
		}
	}
	return out
}

// taskWords is what a task says, from its Markdown raw: its links as their words, the day it names taken out as its
// due day, and how it comes again taken out as every.
func taskWords(raw string) (text string, due time.Time, every string) {
	text = wikiInText.ReplaceAllStringFunc(raw, func(s string) string {
		inside := s[2 : len(s)-2]
		target, alias, _ := strings.Cut(inside, "|")
		if d, err := parseDay(strings.TrimSpace(target)); err == nil && alias == "" && due.IsZero() {
			due = d
			return ""
		}
		if alias != "" {
			return alias
		}
		return target
	})
	if r, ok := parseEvery(text); ok {
		every = r.label()
		text = everyTok.ReplaceAllString(text, "")
	}
	if text = strings.TrimSpace(text); text == "" {
		text = strings.TrimSpace(raw)
	}
	return text, due, every
}

// inlineTasksOf are the tasks in the running text of a page's Markdown, as parseBody reads them (inlineTaskSpots).
// What one says is the words after its box, up to the next box of its line, or the words before it when none come
// after: "Call [ ] Ana, [ ] Ben" is "Ana," and "Ben" (#536).
func inlineTasksOf(p *Page, body string) []taskItem {
	spots := inlineTaskSpots(body)
	if len(spots) == 0 {
		return nil
	}
	lines := strings.Split(body, "\n")
	var out []taskItem
	for k, s := range spots {
		ln := lines[s.line]
		inl := 1
		for j := k - 1; j >= 0 && spots[j].line == s.line; j-- {
			inl++
		}
		raw := ln
		if s.col >= 0 {
			from, to := s.col+3, len(ln)
			if k+1 < len(spots) && spots[k+1].line == s.line && spots[k+1].col > s.col {
				to = spots[k+1].col
			}
			raw = ln[from:to]
			if strings.TrimFunc(raw, notWord) == "" { // nothing after it: what is before it
				start := 0
				if k > 0 && spots[k-1].line == s.line && spots[k-1].col >= 0 {
					start = spots[k-1].col + 3
				}
				raw = ln[start:s.col]
			}
		}
		raw = strings.TrimFunc(taskLead.ReplaceAllString(raw, ""), notWord)
		text, due, every := taskWords(raw)
		out = append(out, taskItem{page: p, line: s.line, done: s.done, text: text, due: due, every: every, inl: inl, col: s.col, folded: foldText(text)})
	}
	return out
}

// taskLead is what starts a line before its words: a quote's marks, a heading's, a list's marker and its box.
var taskLead = regexp.MustCompile(`^[>\s]*(?:#{1,6}\s+|(?:[-*+]|\d+[.)])\s+(?:\[.\]\s*)?)?`)

// notWord is a rune that is no part of a task's words at its ends: a space or a stop.
func notWord(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune(",;:.!?·–—-", r) }

// pageTasks are all the tasks of a page's Markdown, its lines' and those in its running text, in the order of the
// text: what the task view lists.
func pageTasks(p *Page, body string) []taskItem {
	out := tasksOf(p, body)
	inline := inlineTasksOf(p, body)
	if len(inline) == 0 {
		return out
	}
	out = append(out, inline...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].line != out[j].line {
			return out[i].line < out[j].line
		}
		return out[i].inl < out[j].inl
	})
	return out
}

// allTasks is every task, each page's in the order of its text, the pages in the order they had when the
// task view was opened: those changed last first, and a page that changes while the view is open (a task
// ticked off in it) stays where it was; one new to the view comes first. It is sorted when the index of links
// is made, not for each frame: 20000 tasks.
func (a *app) allTasks(force bool) []taskItem {
	a.linksTo(nil, a.clock(), force)
	l := &a.links
	if l.sorted == nil && len(l.tasks) > 0 {
		out := append([]taskItem{}, l.tasks...)
		rank := func(p *Page) int {
			if r, ok := l.taskRank[p]; ok {
				return r
			}
			return -1
		}
		sort.SliceStable(out, func(i, j int) bool {
			pi, pj := out[i].page, out[j].page
			if pi != pj {
				if ri, rj := rank(pi), rank(pj); ri != rj {
					return ri < rj
				}
				return pi.Modified.After(pj.Modified)
			}
			return out[i].line < out[j].line
		})
		l.taskRank = map[*Page]int{}
		for _, k := range out {
			if _, ok := l.taskRank[k.page]; !ok {
				l.taskRank[k.page] = len(l.taskRank)
			}
		}
		l.sorted = out
	}
	return l.sorted
}

// openTask opens the page of task k with the caret in the task's line, or at its box in the running text.
func (a *app) openTask(k taskItem) { a.openTaskIn(a.pane(), k) }

func (a *app) openTaskIn(pn *paneState, k taskItem) {
	a.openIn(pn, k.page)
	body, _ := k.page.Text()
	if k.inl > 0 {
		if at, ok := inlineChip(pn.doc.Text(), body, k.page, k); ok {
			a.gotoPosIn(pn, at)
		}
		return
	}
	// the slots of the editor's text, without taskParagraphs' check that the two count the same: a fold in it
	// hides tasks after the one gone to, and a caret in the wrong line changes nothing
	slots := taskSlots(pn.doc.Text())
	for i, t := range tasksOf(k.page, body) {
		if t.line == k.line && i < len(slots) && slots[i] >= 0 {
			a.gotoPosIn(pn, slots[i])
			return
		}
	}
}

// paraText is the text of a paragraph.
func paraText(p richtext.Paragraph) string {
	var sb strings.Builder
	for _, r := range p.Runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

// openTasks shows the task view in the active pane.
func (a *app) openTasks() { a.openTasksIn(a.pane()) }

func (a *app) openTasksIn(pn *paneState) {
	if pn == a.pane() {
		a.flush()
	}
	pn.tasksOpen, pn.tag, pn.coll, pn.trashOpen, pn.searchOpen, pn.settingsOpen = true, "", nil, false, false, false
	pn.statsScreen, pn.profileScreen = false, false
	a.links.taskRank, a.links.sorted = nil, nil // the pages as they are now, the changed last first
	a.linksStale()
	a.visitIn(pn)
}

// taskToggle is a task ticked off, or on again, as its page's new body: made away from the window (prepareToggle),
// written under its lock (applyToggle).
type taskToggle struct {
	t      taskItem
	err    error
	stale  bool   // the page changed since the task was read: nothing to write
	saved  string // the page's new body
	writes int    // how many times the page had been written when its body was read (pageRead)
	sum    string // the hash of the page's file as it was read: a write not through the page (a sync's pull) moves it (#561)
	// how the toggle is made again from the page as it is, when the page was written since it was read, and how many
	// times it was made: nil for one made under the window's lock, which nothing writes in between (#530)
	again func(pageRead) taskToggle
	tries int
	// what the indexes keep of the page, when its Markdown is the old one with the task's box changed: the index
	// is not read again from the whole page, which took most of a second on one of thirty thousand lines (#504)
	text   string
	idx    *pageIdx
	plain  string
	folded string
}

// pageRead is where a page's file is and how many times the page had been written, read under the window's lock,
// for its body to be read away from it: a write after it is known by the count (#530).
type pageRead struct {
	fs     FS
	file   string
	writes int
}

// readOf is page p's pageRead, under the window's lock.
func readOf(p *Page) pageRead { return pageRead{p.FS(), p.File(), p.Writes} }

// body reads the page's body from its file, and the hash of the file as it was read.
func (r pageRead) body() (body, sum string, err error) {
	src, err := r.fs.ReadFile(r.file)
	if err != nil {
		return "", "", err
	}
	_, body, err = SplitPageChecked(src)
	return body, HashOf(src), err
}

// prepareToggle reads task t's page from r and makes its body with the task ticked off, or on again. It reads
// nothing of the app's but the page's file, and may run away from the window. The body is changed where the task
// is, not written again from its Markdown, which loses what Markdown cannot say (#123).
func prepareToggle(t taskItem, r pageRead, now time.Time) taskToggle {
	j := taskToggle{t: t, writes: r.writes}
	p := t.page
	body, sum, err := r.body()
	j.sum = sum
	if err == nil {
		err = bodyReadable(body) // what this version does not read would be dropped by writing it again
	}
	if err != nil {
		j.err = err
		return j
	}
	tx := decodeBody(body)
	md := library.TextOfBody(body) // the page's Markdown, kept since its index read it
	if t.inl > 0 {                 // a task in the running text: its chip, done or not done again (#536)
		at, ok := inlineChip(tx, md, p, t)
		if !ok {
			j.stale = true
			return j
		}
		doc := richtext.NewDocument(tx)
		setCheckChip(doc, at, !t.done)
		j.saved = encodeBody(doc.Text())
		if text, ok := withInlineBox(md, t, !t.done); ok {
			j.text = text
			j.idx, j.plain = readPageIdxAt(p, time.Time{}, 0, text)
			j.folded = foldText(j.plain)
		}
		return j
	}
	at, ok := taskParagraph(tx, md, p, t)
	if !ok {
		j.stale = true
		return j
	}
	doc := richtext.NewDocument(tx)
	doc.ToggleChecked(at)
	rollRecurringIn(doc, now, importMarkdown) // a task that comes again: this one is done, and the next is made
	j.saved = encodeBody(doc.Text())
	if doc.Text().ParagraphCount() == tx.ParagraphCount() { // nothing came again: the page's Markdown is the task's box changed
		st := doc.Text().ParagraphStyleAt(at)
		box := " "
		if st.Checked {
			box = "x"
		} else if k, ok := taskKindOf(st); ok {
			box = k.md
		}
		if text, ok := withTaskBox(md, t.line, box); ok {
			j.text = text
			j.idx, j.plain = readPageIdxAt(p, time.Time{}, 0, text)
			j.folded = foldText(j.plain)
		}
	}
	return j
}

// taskToggleTries is how many times a toggle is made again from its page when the page is written meanwhile.
const taskToggleTries = 3

// applyToggle writes a task's toggle made by prepareToggle, under the window's lock: the page's file, its text in
// the editor where it is open, and the indexes. What is typed in the page and not yet saved is saved first; a page
// written since its body was read is not written over, which would lose the newer text: the toggle is made again
// from the page as it is now (#530). Written since is a save of the page, or its file changed another way, as a
// sync's pull writes it (#561); a page the library no longer holds (a pull reads the library again, with pages of
// its own) is not written: the list is read again, and the tick is to be made there.
func (a *app) applyToggle(j taskToggle) {
	p := j.t.page
	if a.docs[p] != nil {
		a.flush()
	}
	if !a.inLibrary(p) {
		a.toggleNotMade()
		return
	}
	moved := p.Writes != j.writes
	if !moved && j.err == nil {
		src, err := p.FS().ReadFile(p.File())
		moved = err != nil || HashOf(src) != j.sum
	}
	if moved {
		if j.again != nil && j.tries < taskToggleTries {
			a.toggleAway(p, j.again, j.tries+1)
		} else {
			a.toggleNotMade()
		}
		return
	}
	switch {
	case j.err != nil:
		a.fail(j.err)
		a.linksStale()
		return
	case j.stale:
		a.linksStale() // the page changed since: read it again
		return
	}
	if err := p.Save(j.saved); err != nil {
		a.fail(err)
		a.linksStale()
		return
	}
	if j.text != "" {
		library.RememberText(j.saved, j.text)
		l := &a.links
		j.idx.mod, j.idx.writes = p.Modified, p.Writes
		if l.pages == nil {
			l.pages = map[*Page]*pageIdx{}
		}
		l.pages[p] = j.idx
		if l.plain == nil {
			l.plain = map[*Page]*plainIdx{}
		}
		l.plain[p] = &plainIdx{mod: p.Modified, writes: p.Writes, folded: j.folded}
	}
	a.takeOutside(p) // its text, in the editor, as the file has it now
	a.linksStale()
}

// toggleNotMade says a tick could not be made, its page changing as it was made: the list is read again, and shows
// the task as the page has it.
func (a *app) toggleNotMade() {
	a.linksStale()
	a.say(toastWarn, locales.TasksToggleNotMade())
}

// prepareClose reads page p from r and makes its body with the open tasks of ts ticked off: one write for them all.
func prepareClose(p *Page, ts []taskItem, r pageRead, now time.Time) taskToggle {
	j := taskToggle{t: taskItem{page: p}, writes: r.writes}
	body, sum, err := r.body()
	j.sum = sum
	if err == nil {
		err = bodyReadable(body)
	}
	if err != nil {
		j.err = err
		return j
	}
	tx := decodeBody(body)
	md := library.TextOfBody(body)
	items, starts := taskParagraphs(tx, md, p)
	byLine := map[int]int{}
	for i, ti := range items {
		byLine[ti.line] = i
	}
	var at, chips []int
	for _, t := range ts {
		if t.done || t.page != p {
			continue
		}
		if t.inl > 0 { // a task in the running text: its chip (#536)
			c, ok := inlineChip(tx, md, p, t)
			if !ok {
				j.stale = true
				return j
			}
			chips = append(chips, c)
			continue
		}
		i, ok := byLine[t.line]
		if !ok || i >= len(starts) || items[i].text != t.text || items[i].done {
			j.stale = true
			return j
		}
		if starts[i] < 0 { // a line that is no task of the text, as in a raw block: nothing there to tick
			continue
		}
		at = append(at, starts[i])
	}
	doc := richtext.NewDocument(tx)
	doc.Edit(func(ed *richtext.Edit) {
		for _, s := range at {
			ed.FormatParagraphs(s, s, func(st *richtext.ParagraphStyle) { st.Checked = true })
		}
	})
	for _, c := range chips { // one rune each, in place: the places of the others stay
		setCheckChip(doc, c, true)
	}
	rollRecurringIn(doc, now, importMarkdown)
	j.saved = encodeBody(doc.Text())
	j.text = library.TextOfBody(j.saved) // written as Markdown here, away from the window, and kept
	j.idx, j.plain = readPageIdxAt(p, time.Time{}, 0, j.text)
	j.folded = foldText(j.plain)
	return j
}

// closeTasks ticks off the open tasks of ts, page p's: the list shows them done at once, the page is written away
// from the window, after any toggle before (#505).
func (a *app) closeTasks(p *Page, ts []taskItem) {
	if p == a.pane().page {
		a.flush()
	}
	ts = append([]taskItem(nil), ts...)
	open := map[string]bool{} // by taskKey: a line's task and those in its running text are each their own
	for _, t := range ts {
		if !t.done {
			open[taskKey(t)] = true
		}
	}
	for _, list := range [][]taskItem{a.links.tasks, a.links.sorted} {
		for i := range list {
			if list[i].page == p && open[taskKey(list[i])] {
				list[i].done = true
			}
		}
	}
	if !stillNow() {
		for _, pn := range a.ps.states {
			if pn.taskFilter != 0 {
				continue
			}
			if pn.taskLeaving == nil {
				pn.taskLeaving = map[string]time.Time{}
			}
			for _, t := range ts {
				if open[taskKey(t)] {
					pn.taskLeaving[taskKey(t)] = a.frameNow
				}
			}
		}
	}
	now := a.clock()
	a.toggleAway(p, func(r pageRead) taskToggle { return prepareClose(p, ts, r, now) }, 0)
}

// toggleAway makes a toggle of page p with prepare away from the window, one toggle after another, each reading
// the page the one before wrote, and writes it under the window's lock (applyToggle). The page's file and its
// count of writes are read under the lock, its body away from it; tries is how many times it was made before.
func (a *app) toggleAway(p *Page, prepare func(pageRead) taskToggle, tries int) {
	a.toggling.Add(1)
	go func() {
		defer a.toggling.Done()
		defer a.guard()
		a.toggleMu.Lock()
		defer a.toggleMu.Unlock()
		var r pageRead
		a.do(func() error {
			r = readOf(p)
			return nil
		})
		j := prepare(r)
		j.again, j.tries = prepare, tries
		a.do(func() error {
			a.applyToggle(j)
			return nil
		})
	}()
}

// toggleTask ticks a task off, or on again, at once: what an agent asks for, which reads the page after.
func (a *app) toggleTask(t taskItem) {
	if a.docs[t.page] != nil {
		a.flush()
	}
	a.applyToggle(prepareToggle(t, readOf(t.page), a.clock()))
}

// toggleTaskSoon is toggleTask from the task view: the list shows the task ticked at once, and the page is read
// and written away from the window, one toggle after another, so that a page of thousands of tasks does not hold
// the window (#504).
func (a *app) toggleTaskSoon(t taskItem) {
	if t.page == a.pane().page {
		a.flush()
	}
	for _, list := range [][]taskItem{a.links.tasks, a.links.sorted} { // as it will be
		for i := range list {
			if list[i].page == t.page && list[i].line == t.line && list[i].inl == t.inl {
				list[i].done = !t.done
			}
		}
	}
	now := a.clock()
	a.toggleAway(t.page, func(r pageRead) taskToggle { return prepareToggle(t, r, now) }, 0)
}

// taskParagraph is where in tx, page p's text, the task t starts: the paragraph of the task that is t's among the
// tasks, counted as tasksOf counts them in the page's Markdown (taskSlots). False when the page has changed since t
// was read (its line is not t's task any more), or when t's line is no task paragraph of the text. The text is
// written as Markdown once, not a paragraph at a time, which took seconds on a page of thousands of tasks (#504).
func taskParagraph(tx richtext.Text, md string, p *Page, t taskItem) (int, bool) {
	items, starts := taskParagraphs(tx, md, p)
	for i, ti := range items {
		if ti.line != t.line {
			continue
		}
		if ti.text != t.text || ti.done != t.done || i >= len(starts) || starts[i] < 0 {
			return 0, false
		}
		return starts[i], true
	}
	return 0, false
}

// taskParagraphs are the tasks of page p's Markdown md, as tasksOf reads them, and where each starts in tx, its
// text: both in one pass, for many tasks at once. A start is -1 where the task's line is no task paragraph of the
// text; the starts are none at all when the two do not count the same tasks, as nothing then says which is which.
func taskParagraphs(tx richtext.Text, md string, p *Page) ([]taskItem, []int) {
	items := tasksOf(p, md)
	starts := taskSlots(tx)
	if len(starts) != len(items) {
		return items, nil
	}
	return items, starts
}

// taskSlots are the lines tasksOf takes for tasks in the Markdown of tx, in their order: for each, where its task
// paragraph starts in tx, or -1 for a line that is a task in Markdown but no task paragraph of the text, as one
// in a raw block of HTML is. A task paragraph in a quote is none of them: its line, "> - [ ] …", is no task to
// tasksOf. Counting the paragraphs alone took the k-th task of the Markdown for the k-th task paragraph, another
// one where the two counts differ (#529). The paragraphs are read once, as bodyMarkdown holds its blocks.
func taskSlots(tx richtext.Text) []int {
	var out []int
	block, raw := false, false // in a code or raw block, and which, as bodyMarkdown groups its paragraphs
	for s, para := range tx.Paragraphs() {
		st := para.Style
		if st.CodeBlock && !st.Quote && st.Indent == 0 && st.List == richtext.NoList {
			if !block || strings.HasPrefix(st.ID, codeIDPrefix) || strings.HasPrefix(st.ID, rawIDPrefix) {
				block, raw = true, strings.HasPrefix(st.ID, rawIDPrefix)
			}
			if m := taskLine.FindStringSubmatch(paraText(para)); raw && m != nil && m[3] != "" { // written as it is
				out = append(out, -1)
			}
			continue
		}
		block, raw = false, false
		if st.List != richtext.Tasks || st.Quote || st.QuoteDepth > 0 || strings.TrimSpace(strings.ReplaceAll(paraText(para), string(richtext.ObjectReplacement), "x")) == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// withTaskBox is the Markdown md with the box of the task on line i holding box; false when that line is no task.
func withTaskBox(md string, i int, box string) (string, bool) {
	start := 0
	for n := 0; n < i; n++ {
		j := strings.IndexByte(md[start:], '\n')
		if j < 0 {
			return "", false
		}
		start += j + 1
	}
	end := len(md)
	if j := strings.IndexByte(md[start:], '\n'); j >= 0 {
		end = start + j
	}
	m := taskLine.FindStringSubmatchIndex(md[start:end])
	if m == nil {
		return "", false
	}
	return md[:start+m[4]] + box + md[start+m[5]:], true
}

// inlineChip is where in tx, page p's text, the task in the running text t is: its chip, the one that is t's among
// the page's tasks in the running text, counted as inlineTasksOf counts them in the Markdown md (checkChips). False
// when the page changed since t was read, or the two do not count the same tasks in the same states: a chip the
// Markdown writes escaped, glued to a word, is no task of it, and nothing then says which is which.
func inlineChip(tx richtext.Text, md string, p *Page, t taskItem) (int, bool) {
	items := inlineTasksOf(p, md)
	at, done := checkChips(tx)
	if len(items) != len(at) {
		return 0, false
	}
	for k, it := range items {
		if it.done != done[k] {
			return 0, false
		}
	}
	for k, it := range items {
		if it.line == t.line && it.inl == t.inl {
			if it.text != t.text || it.done != t.done {
				return 0, false
			}
			return at[k], true
		}
	}
	return 0, false
}

// setCheckChip makes the task in the running text at in doc done, or not done. The chip is a new one, as a click
// makes it (toggleCheckChip): the page's text is written from it, and read again where it is shown.
func setCheckChip(doc *richtext.Document, at int, done bool) {
	ch := &chip{kind: "check"}
	if done {
		ch.target = "x"
	}
	attrs := doc.Text().AttributesAt(at)
	attrs.Attachment = &richtext.Attachment{Name: ch.label(), Data: ch}
	doc.Edit(func(ed *richtext.Edit) { ed.ReplaceString(at, at+1, string(richtext.ObjectReplacement), attrs) })
}

// withInlineBox is the Markdown md with the box of the task in the running text t holding done; false when its box
// is not where t says, or its place in the line is not known.
func withInlineBox(md string, t taskItem, done bool) (string, bool) {
	if t.col < 0 {
		return "", false
	}
	start := 0
	for n := 0; n < t.line; n++ {
		j := strings.IndexByte(md[start:], '\n')
		if j < 0 {
			return "", false
		}
		start += j + 1
	}
	at := start + t.col
	if at+3 > len(md) || (md[at:at+3] != "[ ]" && md[at:at+3] != "[x]") {
		return "", false
	}
	box := "[ ]"
	if done {
		box = "[x]"
	}
	return md[:at] + box + md[at+3:], true
}

// tasksView is the view: a filter, and the tasks under their pages.
func (a *app) tasksView(c *ui.Context, pn *paneState) {
	cl := ryugo.From(c).Colors
	all := a.allTasks(a.links.stale) // a tick shows at once
	now := a.clock()
	var st taskStats
	for _, k := range all {
		st.count(k, now)
	}
	// the collections that have tasks, and the kinds the tasks are, to choose among
	colls := map[*Collection]int{}
	kinds := map[string]int{}
	for _, k := range all {
		colls[k.page.Coll]++
		if !k.done {
			kinds[taskKindKey(k)]++
		}
	}
	if pn.taskKindF != "" && kinds[pn.taskKindF] == 0 {
		pn.taskKindF = "" // no open task of that kind now
	}
	if pn.taskColl != "" && !slices.ContainsFunc(a.lib.Collections, func(co *Collection) bool { return co.Dir == pn.taskColl && colls[co] > 0 }) {
		pn.taskColl = "" // that collection has no tasks now
	}
	ui.Column(c).Fill().MaxWidth(viewMax).Gap(10).Children(func() {
		ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Center).Wrap().Padding(2, 0, 4, 0).Children(func() {
			ui.Text(c, locales.CommonTasks()).FontSize(32).FontWeight(720).Grow(1)
			a.taskStatBars(c, cl, pn, st)
		})
		ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
			ryugo.Segmented(c.Key("task-filter"), &pn.taskFilter, []string{locales.TasksOpen(), locales.TasksDone(), locales.TasksAll()}, ryugo.SegmentedOptions{Label: locales.CommonTasks(), Size: ryugo.Sm})
			if pn.taskFilter == 0 { // the open ones by their day
				ryugo.Segmented(c.Key("task-due"), &pn.taskDue, []string{locales.TasksDueAll(), locales.TasksDueToday(), locales.TasksDueWeek(), locales.TasksDueLate()}, ryugo.SegmentedOptions{Label: locales.TasksDay(), Size: ryugo.Sm})
			}
			ui.Spacer(c)
			if len(kinds) > 1 || pn.taskKindF != "" {
				opts := []ryugo.Option[string]{{Value: "", Label: locales.TasksAllKinds()}, {Value: "plain", Label: locales.TasksPlainTask() + "  ·  " + strconv.Itoa(kinds["plain"])}}
				for _, tk := range taskKinds {
					if kinds[tk.id] > 0 {
						opts = append(opts, ryugo.Option[string]{Value: tk.id, Label: msg(tk.name) + "  ·  " + strconv.Itoa(kinds[tk.id])})
					}
				}
				ui.Box(c.Key("task-kind-wrap")).Width(176).Children(func() {
					ryugo.Select(c.Key("task-kind"), &pn.taskKindF, opts, ryugo.SelectOptions{Label: locales.CommonKind(), Size: ryugo.Sm})
				})
			}
			if len(colls) > 1 {
				opts := []ryugo.Option[string]{{Value: "", Label: locales.TasksAllCollections()}}
				for _, co := range a.lib.Collections {
					if colls[co] > 0 {
						opts = append(opts, ryugo.Option[string]{Value: co.Dir, Label: co.Name + "  ·  " + strconv.Itoa(colls[co])})
					}
				}
				ui.Box(c.Key("task-coll-wrap")).Width(176).Children(func() {
					ryugo.Select(c.Key("task-coll"), &pn.taskColl, opts, ryugo.SelectOptions{Label: locales.Collection(), Size: ryugo.Sm})
				})
			}
			a.fieldMenu(ryugo.Input(c.Key("task-q"), &pn.taskQuery, ryugo.InputOptions{Label: locales.TasksSearchTasks(), Placeholder: locales.TasksSearchTasks2(), PrefixIcon: iconSearch, Clearable: true, Size: ryugo.Sm}), &pn.taskQuery, fieldOpts{oneLine: true}).Width(176)
		})
		q := foldText(strings.TrimSpace(pn.taskQuery))
		titles := map[*Page]string{} // each page's title folded once a frame, not once a task
		titleHas := func(p *Page) bool {
			t, ok := titles[p]
			if !ok {
				t = foldText(p.Title)
				titles[p] = t
			}
			return strings.Contains(t, q)
		}
		var shown []taskItem
		for _, k := range all {
			_, leaving := pn.taskLeaving[taskKey(k)]
			switch {
			case leaving && pn.taskFilter == 0 && k.done: // just done: it stays while it goes out (#494)
				shown = append(shown, k)
			case pn.taskFilter != 2 && (pn.taskFilter == 0) == k.done:
			case pn.taskFilter == 0 && !dueMatches(k, pn.taskDue, now):
			case pn.taskColl != "" && k.page.Coll.Dir != pn.taskColl:
			case pn.taskKindF != "" && (k.done || taskKindKey(k) != pn.taskKindF):
			case q != "" && !strings.Contains(k.folded, q) && !titleHas(k.page):
			default:
				shown = append(shown, k)
			}
		}
		if len(shown) == 0 {
			a.noTasks(c, pn, len(all) == 0)
			return
		}
		// 20000 tasks are 20000 rows: only those in view are built (ui.List), the pages' heads and
		// their tasks in one list
		rows := taskRows(shown, pn.taskFolded)
		pn.taskList.Key = func(i int) any {
			if i == len(rows) {
				return "end"
			}
			return rows[i].key()
		}
		a.scrolledListPane(c, pn, "tasks-list", nil, ui.Box(c.Key("tasks-wrap")).FillWidth().Grow(1).MinHeight(0), func() ui.Element {
			return ui.List(c.Key("tasks-list"), pn.taskList, len(rows)+1, func(i int) {
				switch {
				case i == len(rows): // the room under the last task
					ui.Box(c.Key("tasks-end")).FillWidth().Height(taskEndRoom)
				case rows[i].head:
					a.taskHead(c, cl, pn, rows[i], i > 0)
				default:
					a.taskRow(c, cl, pn, rows[i].task)
				}
			}).Fill()
		})
	})
}

// taskEndRoom is the room under the last task, so that it does not end at the window's edge.
const taskEndRoom = 56

// taskStats are how many tasks there are, open and done, and of the open ones how many are of today and late.
type taskStats struct{ open, done, today, late int }

// count counts task k in, on the day of now.
func (s *taskStats) count(k taskItem, now time.Time) {
	if k.done {
		s.done++
		return
	}
	s.open++
	if !k.due.IsZero() {
		today := startOfDay(now)
		switch {
		case sameDay(k.due, today):
			s.today++
		case k.due.Before(today):
			s.late++
		}
	}
}

// taskStatBars are the small stats beside the title: the open and the done tasks, of all of them, and the
// open ones of today and late, of the open ones, each a card of ticks in a bright color of its own, the same in
// both themes. A click on one shows those.
func (a *app) taskStatBars(c *ui.Context, cl ryugo.Colors, pn *paneState, s taskStats) {
	all, open := max(1, s.open+s.done), max(1, s.open)
	stats := []struct {
		key, label string
		n, of      int
		col        ui.Color
		filter     int
		due        int
	}{
		{"open", locales.TasksOpen(), s.open, all, ui.Hex("#3b82f6"), 0, 0},
		{"done", locales.TasksDone(), s.done, all, ui.Hex("#84cc16"), 1, 0},
		{"today", locales.TasksDueToday(), s.today, open, ui.Hex("#f59e0b"), 0, 1},
		{"late", locales.TasksDueLate(), s.late, open, ui.Hex("#f43f5e"), 0, 3},
	}
	ui.Row(c.Key("task-stats")).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		for _, st := range stats {
			on := pn.taskFilter == st.filter && (st.filter != 0 || pn.taskDue == st.due)
			a.taskStatCard(c, cl, st.key, st.label, st.n, st.of, st.col, on, func() { pn.taskFilter, pn.taskDue = st.filter, st.due })
		}
	})
}

// taskStatTicks is how many ticks a stat's bar has.
const taskStatTicks = 18

// taskStatCard is one stat: a small card, its name and n of of on it, over an inset well of ticks, as many in its
// color as n is of of. The card is tinted in the color, and a border in it says the list shows those; a click
// shows them.
func (a *app) taskStatCard(c *ui.Context, cl ryugo.Colors, key, label string, n, of int, col ui.Color, on bool, show func()) {
	card := ui.Column(c.Key("task-stat:"+key)).Width(138).Padding(5, 5, 5, 5).Gap(4).Radius(rad(11)).
		Background(mix(cl.Card, col, 0.07)).Border(1, col.Alpha(0.16)).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.05)).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label + " " + strconv.Itoa(n) + "/" + strconv.Itoa(of))
	switch {
	case on: // the border keeps its width, so that the ticks inside stay where they are
		card.Border(1, col.Alpha(0.85))
	case card.Hovered():
		card.Border(1, col.Alpha(0.4))
	}
	lit := 0
	if n > 0 {
		lit = max(1, int(float64(n)/float64(max(1, of))*taskStatTicks+0.5))
	}
	card.Children(func() {
		ui.Row(c).FillWidth().Padding(1, 5, 0, 5).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, label).FontSize(12).FontWeight(560).TextColor(cl.Text2).MaxLines(1).Grow(1).MinWidth(0)
			ui.Row(c).AlignItems(ui.Baseline).Children(func() {
				ui.Text(c, strconv.Itoa(n)).FontSize(13).FontWeight(700).TextColor(cl.Text).FontFeatures("tnum")
				ui.Text(c, "/"+strconv.Itoa(of)).FontSize(11).TextColor(cl.Muted).FontFeatures("tnum")
			})
		})
		well := ui.Box(c.Key("ticks")).FillWidth().Height(20).Radius(rad(6)).Background(mix(cl.Well, col, 0.06))
		well.DrawOver(func(p *ui.Painter, r ui.Rect) {
			// on whole pixels, the run of ticks centered: a tick between two pixels looks wider than the others
			const padX, padY = 7, 5
			pitch := float32(int((r.W - 2*padX) / taskStatTicks))
			w := max(2, float32(int(pitch*0.46+0.5)))
			x0 := float32(int(r.X + (r.W-pitch*taskStatTicks)/2 + float32(int((pitch-w)/2)) + 0.5))
			y, h := float32(int(r.Y+padY+0.5)), float32(int(r.H-2*padY+0.5))
			for i := range taskStatTicks {
				tc := cl.Muted.Alpha(0.2)
				if i < lit {
					tc = mix(col, ui.RGBA(255, 255, 255, 1), 0.12*float64(i)/taskStatTicks)
				}
				p.Fill(ui.Rect{X: x0 + float32(i)*pitch, Y: y, W: w, H: h}, tc, w/2)
			}
		})
	})
	if card.Clicked() {
		show()
	}
}

// noTasks says there are no tasks to show: none at all, or none with the filters.
func (a *app) noTasks(c *ui.Context, pn *paneState, none bool) {
	o := ryugo.EmptyStateOptions{Icon: iconTask, Title: locales.TasksNoTaskFitsThis(), Description: locales.TasksLoosenFilterTheyShow()}
	switch {
	case none:
		o.Title, o.Description = locales.TasksEmptyTitle(), locales.TasksEmptyHelp()
	case pn.taskFilter == 1:
		o.Title, o.Description = locales.TasksNoneDone(), locales.TasksNoneDoneHelp()
	case pn.taskFilter == 0 && pn.taskDue == 0 && pn.taskColl == "" && pn.taskKindF == "" && strings.TrimSpace(pn.taskQuery) == "":
		o.Title, o.Description = locales.TasksNoneOpen(), locales.TasksAllDone()
	}
	if !none && (pn.taskColl != "" || strings.TrimSpace(pn.taskQuery) != "" || pn.taskDue != 0 || pn.taskKindF != "") {
		o.Action = func() {
			if ryugo.Button(c.Key("task-clear-filters"), locales.TasksClearFilters(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
				pn.taskColl, pn.taskQuery, pn.taskDue, pn.taskKindF = "", "", 0, ""
			}
		}
	}
	ui.Column(c).FillWidth().Children(func() { ryugo.EmptyState(c, o) })
}

// taskGroup is a page and its tasks.
// taskListRow is a row of the list of tasks: the head of a page's tasks, or one task.
type taskListRow struct {
	head  bool
	task  taskItem
	n     int        // of a head: how many tasks its page has here
	tasks []taskItem // and they
}

// key is what the row is known by, as the list keeps its place by it.
func (r taskListRow) key() string {
	if r.head {
		return "h:" + r.task.page.Dir
	}
	return "t:" + r.task.page.Dir + ":" + strconv.Itoa(r.task.line)
}

// taskRows are the tasks as the rows of a list: the head of each page, and its tasks under it.
func taskRows(shown []taskItem, folded map[*Page]bool) []taskListRow {
	rows := make([]taskListRow, 0, len(shown)+len(shown)/4)
	for i := 0; i < len(shown); {
		j := i
		for j < len(shown) && shown[j].page == shown[i].page {
			j++
		}
		rows = append(rows, taskListRow{head: true, task: shown[i], n: j - i, tasks: shown[i:j]})
		if !folded[shown[i].page] {
			for _, k := range shown[i:j] {
				rows = append(rows, taskListRow{task: k})
			}
		}
		i = j
	}
	return rows
}

// taskHead is the head of a page's tasks: a chevron that folds them away, its title
// and collection, which open it, and how many tasks it has.
func (a *app) taskHead(c *ui.Context, cl ryugo.Colors, pn *paneState, r taskListRow, gap bool) {
	p := r.task.page
	folded := pn.taskFolded[p]
	c.Key("tgh:" + p.Dir)
	head := ui.Row(c).FillWidth().Padding(2, 8, 2, 2).Gap(4).Radius(rad(8)).AlignItems(ui.Center).
		Label(locales.CommonPageLabel(p.Title)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	if gap {
		head.Margin(14, 0, 0, 0) // the room between two pages' tasks
	}
	if head.Hovered() {
		head.Background(cl.Fill)
	}
	a.contextOn(head, func() []menuItem { return a.pageMenu(p, false) })
	var fold ui.Element
	head.Children(func() {
		fold = ui.Box(c.Key("tgh-fold")).Size(26, 26).Shrink(0).Radius(rad(7)).Center().Cursor(ui.CursorPointer).
			Label(map[bool]string{true: locales.TasksUnfoldLabel(p.Title), false: locales.TasksFoldLabel(p.Title)}[folded]).Role(ui.RoleButton)
		if fold.Hovered() {
			fold.Background(cl.Fill)
		}
		angle := fold.AnimateTo("turn", map[bool]float32{true: 0, false: 90}[folded], springTurn)
		fold.Children(func() { ui.Icon(c, iconChevron).Width(13).Height(13).TextColor(cl.Muted).Rotate(angle) })
		ui.Icon(c, iconPage).Width(15).Height(15).TextColor(cl.Muted).Margin(0, 4, 0, 2)
		ui.Text(c, p.Title).FontSize(13.5).FontWeight(650).TextColor(cl.Text).MaxLines(1).Shrink(1)
		ui.Text(c, p.Coll.Name).FontSize(12).TextColor(cl.Muted).MaxLines(1).Shrink(2).Grow(1).Margin(0, 0, 0, 8)
		ui.Text(c, strconv.Itoa(r.n)).FontSize(12).TextColor(cl.Muted).FontFeatures("tnum")
		open := 0
		for _, k := range r.tasks {
			if !k.done {
				open++
			}
		}
		if open > 1 { // every open task shown under it, ticked off at once (#505)
			all := ui.Row(c.Key("tgh-all")).Height(24).Padding(0, 8).Gap(5).Radius(rad(6)).AlignItems(ui.Center).Cursor(ui.CursorPointer).
				Role(ui.RoleButton).Label(locales.TasksCloseAllLabel(p.Title))
			if all.Hovered() {
				all.Background(cl.Fill)
			}
			all.Children(func() {
				ui.Icon(c, kindIcon["tasks"]).Width(13).Height(13).TextColor(cl.Muted)
				ui.Text(c, locales.TasksCloseAll()).FontSize(12).TextColor(cl.Text2).NoWrap()
			})
			if all.Clicked() {
				tasks := r.tasks
				a.ask(locales.TasksCloseAllTitle(), locales.TasksCloseAllBody(open, p.Title), locales.TasksCloseAll(), func() { a.closeTasks(p, tasks) })
			}
		}
	})
	switch {
	case fold.Clicked():
		if pn.taskFolded == nil {
			pn.taskFolded = map[*Page]bool{}
		}
		pn.taskFolded[p] = !folded
	case head.Clicked():
		a.openIn(pn, p)
	}
}

// taskRow is one task: its box, its words (a click goes to its line), its day.
// taskKey is what a task is known by between frames: its page and its line.
func taskKey(k taskItem) string {
	if k.inl > 0 {
		return k.page.Dir + ":" + strconv.Itoa(k.line) + "." + strconv.Itoa(k.inl)
	}
	return k.page.Dir + ":" + strconv.Itoa(k.line)
}

// taskLeave is how long a task just done takes to go out of the open ones: it shows done, struck through and faint,
// for a moment, then its row slides a little and closes (#494, #508).
const taskLeave = 700 * time.Millisecond

func (a *app) taskRow(c *ui.Context, cl ryugo.Colors, pn *paneState, k taskItem) {
	c.Key("tk:" + taskKey(k))
	row := ui.Row(c).FillWidth().Padding(3, 8, 3, 14).Gap(8).Radius(rad(8)).AlignItems(ui.Center)
	leave := float32(-1) // how far it has gone out, 0 to 1; -1 when it is not going
	if t0, ok := pn.taskLeaving[taskKey(k)]; ok {
		if leave = float32(c.Now().Sub(t0)) / float32(taskLeave); leave >= 1 || stillNow() {
			delete(pn.taskLeaving, taskKey(k))
			leave = 1
		}
		c.Invalidate()
		if close := smooth(0.45, 1, float64(leave)); close > 0 { // done for a moment, then it goes
			row.MaxHeight(f32(34*(1-close))).Clip().Opacity(f32(1-close)).Translate(f32(-10*close), 0)
		}
	}
	if row.Hovered() {
		row.Background(cl.Fill.Alpha(0.5))
	}
	row.Children(func() {
		on := k.done || leave >= 0
		ryugo.Checkbox(c.Key("ck"), &on, "", ryugo.CheckOptions{}).Label(locales.TasksTaskLabel(k.text))
		if tk, ok := taskKindByID(k.kind); ok && !k.done { // its kind, in its color
			ui.Icon(c.Key("kind"), tk.icon).Width(15).Height(15).TextColor(ui.Hex(tk.color)).Label(msg(tk.name))
		}
		col, st := cl.Text, false
		if k.done || leave >= 0 {
			col, st = cl.Muted, true
		}
		body := ui.Row(c.Key("tkt")).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Label(locales.TasksGoToLineLabel(k.text)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
		body.Children(func() {
			tx := ui.Text(c, k.text).FontSize(14).TextColor(col).Grow(1).MinWidth(0).Wrap()
			if st {
				tx.Strikethrough()
			}
			if k.every != "" {
				ui.Text(c, "↻ "+k.every).FontSize(11.5).TextColor(cl.Muted).NoWrap()
			}
			if !k.due.IsZero() {
				dc := cl.Muted
				if !k.done && k.due.Before(startOfDay(a.clock())) {
					dc = cl.Up
				}
				ui.Row(c.Key("due")).Padding(1, 7).Radius(rad(6)).Gap(4).AlignItems(ui.Center).Background(dc.Alpha(0.12)).Children(func() {
					ui.Icon(c, iconCalendar).Width(12).Height(12).TextColor(dc)
					ui.Text(c, shortDate(k.due)).FontSize(11.5).TextColor(dc).NoWrap()
				})
			}
		})
		a.kbMark(body, "k:"+k.page.Dir+":"+itoa(k.line), true, func() { a.openTaskIn(pn, k) })
		if body.Clicked() {
			a.openTaskIn(pn, k)
		}
		if on != k.done {
			if on && pn.taskFilter == 0 && !stillNow() { // done in the open ones: it goes out, not at once
				if pn.taskLeaving == nil {
					pn.taskLeaving = map[string]time.Time{}
				}
				pn.taskLeaving[taskKey(k)] = c.Now()
			}
			a.toggleTaskSoon(k)
		}
	})
}

// taskKindKey is what the kind filter knows task k by: its kind's id, or "plain".
func taskKindKey(k taskItem) string {
	if k.kind == "" {
		return "plain"
	}
	return k.kind
}

// startOfDay is d at its day's start.
func startOfDay(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
}

// dueMatches reports whether open task k is in the group of days chosen:
// 0 any, 1 today's, 2 this week's (to the Sunday, today's and after), 3 late.
func dueMatches(k taskItem, group int, now time.Time) bool {
	if group == 0 {
		return true
	}
	if k.due.IsZero() {
		return false
	}
	today := startOfDay(now)
	switch group {
	case 1:
		return sameDay(k.due, today)
	case 2:
		end := weekStart(today).AddDate(0, 0, 7)
		return !k.due.Before(today) && k.due.Before(end)
	}
	return k.due.Before(today)
}

// ---- tasks that come again ----

// everyTok is how a task says it comes again: every:week, every:2week,
// her:hafta (day gün, week hafta, month ay, year yıl). A monthly or a yearly one whose day a shorter month cut says the
// day it is meant for after it: every:month@31 (#214).
var everyTok = regexp.MustCompile(`(?i)\s*\b(?:every|her):(\d*)(day|week|month|year|gün|gun|hafta|ay|yıl|yil)\b(?:@(\d{1,2}))?`)

type recur struct {
	n    int
	unit string // day, week, month, year
	day  int    // the day of the month it is meant for, when a shorter month cut it (@31); 0 for its date's own
}

// parseEvery reads the recurrence a task's text says.
func parseEvery(s string) (recur, bool) {
	m := everyTok.FindStringSubmatch(s)
	if m == nil {
		return recur{}, false
	}
	n := 1
	if m[1] != "" {
		n, _ = strconv.Atoi(m[1])
	}
	if n < 1 {
		n = 1
	}
	unit := map[string]string{"day": "day", "gün": "day", "gun": "day", "week": "week", "hafta": "week", "month": "month", "ay": "month", "year": "year", "yıl": "year", "yil": "year"}[strings.ToLower(m[2])]
	day, _ := strconv.Atoi(m[3])
	if day > 31 || unit != "month" && unit != "year" {
		day = 0
	}
	return recur{n, unit, day}, true
}

// taskLineRe matches a markdown task list item with its checkbox marker.
var taskLineRe = regexp.MustCompile(`^(\s*(?:[-*+]|\d+[.)])\s+)\[[ xX*!?/e]\](\s*)(.*)$`)

// next returns the first recurrence date after d.
func (r recur) next(d time.Time) time.Time { return r.nextN(d, 1) }

// nextN returns the recurrence date steps intervals after d, clamping month ends.
func (r recur) nextN(d time.Time, steps int) time.Time {
	switch r.unit {
	case "week":
		return d.AddDate(0, 0, 7*r.n*steps)
	case "month", "year":
		months := r.n * steps
		if r.unit == "year" {
			months *= 12
		}
		y, m, day := d.Date()
		if r.day > 0 { // the day it is meant for, which a shorter month cut
			day = r.day
		}
		first := time.Date(y, m+time.Month(months), 1, d.Hour(), d.Minute(), d.Second(), d.Nanosecond(), d.Location())
		lastDay := first.AddDate(0, 1, -1).Day()
		if day > lastDay {
			day = lastDay
		}
		return time.Date(first.Year(), first.Month(), day, d.Hour(), d.Minute(), d.Second(), d.Nanosecond(), d.Location())
	}
	return d.AddDate(0, 0, r.n*steps)
}

// label says it as the task view shows it: "every week", "every 2 months".
func (r recur) label() string {
	switch r.unit {
	case "day":
		return locales.TasksEveryDays(r.n)
	case "week":
		return locales.TasksEveryWeeks(r.n)
	case "month":
		return locales.TasksEveryMonths(r.n)
	}
	return locales.TasksEveryYears(r.n)
}

// comesAgain makes the line of the task that follows when the task of line is
// ticked off, if it comes again: the same, open, its day the next one after
// today (counting from its own day, or from today when it has none).
func comesAgain(line string, now time.Time) (string, bool) {
	r, ok := parseEvery(line)
	if !ok {
		return "", false
	}
	m := taskLineRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	text := m[3]
	today := startOfDay(now)
	base, dated := today, false
	text = wikiInText.ReplaceAllStringFunc(text, func(s string) string {
		if dated {
			return s
		}
		inside := strings.TrimSpace(s[2 : len(s)-2])
		if d, err := parseDay(inside); err == nil {
			base, dated = d, true
			return "\x00"
		}
		return s
	})
	step := 1
	next := r.nextN(base, step)
	for !next.After(today) {
		step++
		next = r.nextN(base, step)
	}
	day := "[[" + next.Format("2006-01-02") + "]]"
	// the day it is meant for: written after the token while a shorter month cuts it, as the date alone would keep the
	// cut day for good (the 31st, the 28th in February, then the 28th ever after); taken away when it is met again
	tok := everyTok.FindString(m[3])
	if i := strings.LastIndexByte(tok, '@'); i >= 0 {
		tok = tok[:i]
	}
	if want := max(r.day, base.Day()); (r.unit == "month" || r.unit == "year") && next.Day() < want {
		tok += "@" + strconv.Itoa(want)
	}
	text = everyTok.ReplaceAllString(text, tok)
	if dated {
		text = strings.Replace(text, "\x00", day, 1)
	} else {
		text = strings.TrimSpace(everyTok.ReplaceAllString(text, "")) + " " + day + tok
	}
	return m[1] + "[ ]" + m[2] + text, true
}

// rollRecurring looks in the open page for a task ticked off in the text
// that comes again (every:week): the done one loses its mark, and the next is
// made under it. A task ticked from the task view is done by toggleTask.
func (a *app) rollRecurring() {
	pn := a.pane()
	if pn.doc == nil || pn.page == nil {
		return
	}
	rollRecurringIn(pn.doc, a.clock(), a.parse)
}

// rollRecurringIn is rollRecurring in doc, the text of what comes again made with parse.
func rollRecurringIn(doc *richtext.Document, now time.Time, parse func(string) richtext.Text) {
	text := doc.Text()
	var at []int
	for s, para := range text.Paragraphs() {
		if para.Style.List == richtext.Tasks && para.Style.Checked && everyTok.MatchString(paraText(para)) {
			at = append(at, s)
		}
	}
	for i := len(at) - 1; i >= 0; i-- {
		s, e := text.ParagraphRange(at[i])
		line := strings.TrimRight(bodyMarkdown(text.Slice(s, e)), "\n")
		look := "" // the comment of its list's look over it, when it says the look (listmd.go)
		if k := strings.LastIndexByte(line, '\n'); k >= 0 && strings.HasPrefix(strings.TrimLeft(line, "> "), listMarkStart) {
			look, line = line[:k+1], line[k+1:]
		}
		next, ok := comesAgain(line, now)
		if !ok {
			continue
		}
		done := everyTok.ReplaceAllString(line, "")
		doc.Edit(func(ed *richtext.Edit) { ed.Replace(s, e, parse(look+done+"\n"+next)) })
		text = doc.Text()
	}
}
