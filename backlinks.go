package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Backlinks: under a page, the pages that link to it with [[Its title]].
// The links of every page are read from the files into an index when first
// asked for, and again after an edit (at most every few seconds, and at once
// when a page is opened): a library of a few thousand pages reads in a
// moment, and nothing is kept in the files for it.

type linkIndex struct {
	into     map[string][]*Page  // a title, lower case, to the pages that link to it
	tags     map[string][]*Page  // a tag (tags.go), lower case, to the pages that have it
	tagNames map[string]string   // and as it was first written
	tasks    []taskItem          // the tasks of every page (tasks.go)
	sorted   []taskItem          // and sorted for their view, until the index is made again
	taskRank map[*Page]int       // where each page's tasks stood when sorted last: ticking one keeps them there (allTasks)
	pages    map[*Page]*pageIdx  // what was read of each page, kept while its file is as it was
	plain    map[*Page]*plainIdx // the text of a page, plain, for the search (search.go), made when asked for
	built    time.Time
	stale    bool
	warm     []*Page // the pages being read the first time, over a few frames (warmLinks), and how far it is
	warmAt   int
}

// linksStale says the pages changed since the index was made.
func (a *app) linksStale() { a.links.stale, a.px.dirty = true, true }

// linksTo are the pages that link to p, the title first.
func (a *app) linksTo(p *Page, now time.Time, force bool) []*Page {
	l := &a.links
	if l.into == nil && !force && (a.prewarming || !a.warmLinks(linksWarmBudget)) { // the first time, the files are read over a few frames
		if a.frame != nil {
			a.frame()
		}
		return nil
	}
	if l.into == nil || (l.stale && (force || now.Sub(l.built) > 4*time.Second)) {
		l.into, l.tags, l.tagNames, l.tasks = map[string][]*Page{}, map[string][]*Page{}, map[string]string{}, nil
		l.stale, l.built = false, now
		l.sorted = nil
		kept := map[*Page]*pageIdx{}
		for _, c := range a.lib.Collections {
			for _, q := range c.All() {
				e := l.pages[q]
				if e == nil || q.Modified.IsZero() || !e.mod.Equal(q.Modified) || e.writes != q.Writes {
					body, err := q.Text()
					if err != nil {
						continue
					}
					e = readPageIdx(q, body)
				}
				kept[q] = e
				l.tasks = append(l.tasks, e.tasks...)
				for _, name := range e.tags {
					k := tagKey(name)
					if _, ok := l.tagNames[k]; !ok {
						l.tagNames[k] = name
					}
					l.tags[k] = append(l.tags[k], q)
				}
				for _, key := range e.links {
					l.into[key] = append(l.into[key], q)
				}
			}
		}
		l.pages = kept
		for q := range l.plain { // the texts of pages there no more (trashed, or replaced by a reload): let go (#196)
			if _, ok := kept[q]; !ok {
				delete(l.plain, q)
			}
		}
	}
	if p == nil { // only the index was wanted (tags.go)
		return nil
	}
	var out []*Page
	for _, q := range l.into[strings.ToLower(strings.TrimSpace(p.Title))] {
		if q != p {
			out = append(out, q)
		}
	}
	return out
}

// linksWarmBudget is how long a frame reads the pages' files for the index, the first time.
const linksWarmBudget = 6 * time.Millisecond

// warmLinks reads what the index needs of the pages that are not read yet, for at most budget, and
// reports whether all are read: then the index is made from what was read, at once. A big library
// opens without waiting for every file (a window of ten thousand pages read them all in its first frame).
func (a *app) warmLinks(budget time.Duration) bool {
	l := &a.links
	if l.pages == nil {
		l.pages = map[*Page]*pageIdx{}
	}
	if l.warm == nil { // the pages to go through, taken once: each frame goes on from where the last stopped
		for _, c := range a.lib.Collections {
			l.warm = append(l.warm, c.All()...)
		}
	}
	start := time.Now()
	for ; l.warmAt < len(l.warm); l.warmAt++ {
		q := l.warm[l.warmAt]
		if e := l.pages[q]; e != nil && !q.Modified.IsZero() && e.mod.Equal(q.Modified) && e.writes == q.Writes {
			continue
		}
		if time.Since(start) > budget {
			return false
		}
		if body, err := q.Text(); err == nil {
			l.pages[q] = readPageIdx(q, body)
		}
	}
	l.warm, l.warmAt = nil, 0
	return true
}

// backlinksView is the section under the text: who links here.
func (a *app) backlinksView(c *ui.Context, p *Page) {
	links := a.linksTo(p, a.clock(), false)
	box := ui.Column(c.Key("backlinks")).FillWidth().Gap(2).Padding(14, 0, 24, 0)
	if len(links) == 0 {
		a.linksH = 0
		return
	}
	t := ryugo.From(c)
	cl := t.Colors
	box.Children(func() {
		ui.Box(c.Key("bl-line")).FillWidth().Height(1).Background(cl.Hairline).Margin(0, 0, 10, 0)
		ui.Text(c, locales.PageBacklinkCount(len(links))).FontSize(12).FontWeight(650).TextColor(cl.Muted).Padding(0, 6, 4, 6)
		for _, q := range links {
			q := q
			row := ui.Column(c.Key("bl:"+q.Dir)).FillWidth().Padding(6, 8).Gap(2).Radius(rad(8)).
				Label(locales.CommonLinkLabel(q.Title)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			if row.Hovered() {
				row.Background(cl.Fill)
			}
			a.contextOn(row, func() []menuItem { return a.pageMenu(q, false) })
			row.Children(func() {
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconPage).Width(15).Height(15).TextColor(cl.Muted)
					ui.Text(c, q.Title).FontSize(14).TextColor(cl.Text).MaxLines(1).Shrink(1)
					ui.Text(c, q.Coll.Name).FontSize(12).TextColor(cl.Muted).MaxLines(1).Shrink(2).Grow(1)
				})
				if segs := a.linkContext(q, p); len(segs) > 0 {
					ui.RichText(c, contextSpans(segs, 12.5, cl.Muted, cl.Text, cl.Accent.Alpha(0.18))...).Padding(0, 0, 0, 25).MaxLines(2)
				}
			})
			if row.Clicked() {
				a.open(q)
			}
		}
	})
	a.linksH = box.LayoutBounds().H
}

// linkContext is the line of page q that links to page p, as the words one
// reads, the link's marked: what a list of the pages that link here shows
// under each, to tell why. Read again only when q changed.
func (a *app) linkContext(q, p *Page) []seg {
	key := q.Dir + "|" + strings.ToLower(p.Title) + "|" + strconv.FormatInt(q.Modified.UnixNano(), 10)
	if v, ok := a.ctxCache[key]; ok {
		return v
	}
	if a.ctxCache == nil {
		a.ctxCache = map[string][]seg{}
	}
	if len(a.ctxCache) > 4000 { // half of it goes, not all: a page with more backlinks than it holds emptied it and read
		// them all again at every frame (#195); which half does not matter, the lines are read again when asked
		n := 0
		for k := range a.ctxCache {
			if n%2 == 0 {
				delete(a.ctxCache, k)
			}
			n++
		}
	}
	var out []seg
	if body, err := q.Text(); err == nil {
		re := regexp.MustCompile(`(?i)\[\[\s*` + regexp.QuoteMeta(strings.TrimSpace(p.Title)) + `\s*(\|[^\]]*)?\]\]`)
		for _, ln := range strings.Split(body, "\n") {
			if m := re.FindString(ln); m != "" {
				out = snippet(plainText(ln), []string{fold(p.Title)})
				break
			}
		}
	}
	a.ctxCache[key] = out
	return out
}

// contextSpans are the spans of a link's line.
func contextSpans(segs []seg, size float32, muted, text, mark ui.Color) []ui.Span {
	var spans []ui.Span
	for _, s := range segs {
		sp := ui.Span{Text: s.text, Size: size, Color: muted}
		if s.hit {
			sp.Color, sp.Weight, sp.Background, sp.Radius = text, 650, mark, 3
		}
		spans = append(spans, sp)
	}
	return spans
}

// pageIdx is what the index reads of a page: its tasks, tags and the pages it
// links to (their titles, lower case). It is read again only when the file was
// written since (mod): the index is made on every page opened and every few
// seconds of editing, and reading every page of a library of thousands each time
// took seconds, on the thread that draws.
type pageIdx struct {
	mod    time.Time
	writes int
	tasks  []taskItem
	tags   []string
	links  []string
	words  int  // what the statistics count, read once with the rest of the page
	empty  bool // nothing but whitespace and marks
}

func readPageIdx(q *Page, body string) *pageIdx {
	e, _ := readPageIdxPlain(q, body)
	return e
}

// readPageIdxPlain is readPageIdx, and the page's plain text it was read from.
func readPageIdxPlain(q *Page, body string) (*pageIdx, string) {
	return readPageIdxAt(q, q.Modified, q.Writes, body)
}

// readPageIdxAt is readPageIdxPlain for a page as it was when mod and writes were read: a goroutine that reads
// pages away from the window is given them, and does not read the page's own fields, which the window writes.
func readPageIdxAt(q *Page, mod time.Time, writes int, body string) (*pageIdx, string) {
	plain := plainText(body)
	e := &pageIdx{mod: mod, writes: writes, tasks: tasksOf(q, body), tags: tagsOf(body),
		words: countWords(plain), empty: strings.TrimSpace(plain) == ""}
	if !strings.Contains(body, "[[") {
		return e, plain
	}
	seen := map[string]bool{}
	for _, m := range wikiInText.FindAllStringSubmatch(body, -1) {
		target, _, _ := strings.Cut(m[1], "|")
		key := strings.ToLower(strings.TrimSpace(target))
		if key != "" && !seen[key] {
			seen[key] = true
			e.links = append(e.links, key)
		}
	}
	return e, plain
}

// plainIdx is the text of a page, plain, and folded as the search compares it.
type plainIdx struct {
	mod    time.Time
	writes int
	plain  string
	folded string
}

// plainOf is the text of page p, plain, as the search shows it. It is made the
// first time it is asked for, and again when the page was written.
func (a *app) plainOf(p *Page) *plainIdx {
	l := &a.links
	if e := l.plain[p]; e != nil && !p.Modified.IsZero() && e.mod.Equal(p.Modified) && e.writes == p.Writes {
		return e
	}
	if l.plain == nil {
		l.plain = map[*Page]*plainIdx{}
	}
	e := &plainIdx{mod: p.Modified, writes: p.Writes}
	if body, err := p.Text(); err == nil {
		e.plain = plainText(body)
		e.folded = fold(e.plain)
	}
	l.plain[p] = e
	return e
}

// countWords is how many words plain has, as strings.Fields would cut it, without making them.
func countWords(plain string) int {
	n, in := 0, false
	for _, r := range plain {
		if unicode.IsSpace(r) {
			in = false
		} else if !in {
			in = true
			n++
		}
	}
	return n
}
