package main

import (
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Search over the whole library: the text of every page, not only its title.
// The texts are read with the backlinks' index (backlinks.go), plain and on
// one line; a page has to have every word asked for, in its title or its
// text, Turkish letters taken as their ASCII (as the palette does), and the
// ones with the words in the title, or the most of them, come first. A
// result shows the place it was found, the words marked.

var (
	mdLink = regexp.MustCompile(`!?\[([^\[\]]*)\]\([^)]*\)`)
	mdWiki = regexp.MustCompile(`\[\[([^\[\]|]*\|)?([^\[\]]*)\]\]`)
)

// plainText is Markdown as the words a reader sees, on one line: links their words, a line's mark (a heading's, a
// quote's, a list item's and a task's box) and the lines of a fence left out, the marks of emphasis and a table's
// bars taken for spaces. It goes through the text once, by hand: the regular expressions it was took seconds over
// a page of thirty thousand lines, read twice each time the page changed (#504).
func plainText(md string) string {
	s := md
	if strings.Contains(s, "](") {
		s = mdLink.ReplaceAllString(s, "$1")
	}
	if strings.Contains(s, "[[") {
		s = mdWiki.ReplaceAllString(s, "$2")
	}
	var b strings.Builder
	b.Grow(len(s))
	gap := false // a space is owed before the next letter
	for len(s) > 0 {
		line := s
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			line, s = s[:i], s[i+1:]
		} else {
			s = ""
		}
		line = strings.TrimLeft(line, " \t\r\f")
		if strings.HasPrefix(line, "```") {
			gap = true
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(line, "> "), listMarkStart) {
			continue // a list's look (listmd.go), no words of the page
		}
		line = cutBox(cutLineMark(line))
		for i := 0; i < len(line); i++ { // by bytes: a letter of more than one is copied as it is
			switch c := line[i]; c {
			case ' ', '\t', '\r', '\f', '\v', '*', '_', '~', '`', '|':
				gap = true
			default:
				if gap && b.Len() > 0 {
					b.WriteByte(' ')
				}
				gap = false
				b.WriteByte(c)
			}
		}
		gap = true
	}
	return b.String()
}

// cutLineMark is a line, its leading spaces gone, without the mark that starts it and the spaces after it: a
// heading's #s, a quote's >, a list item's bullet or number. A mark needs a space after it.
func cutLineMark(line string) string {
	n := 0
	switch {
	case strings.HasPrefix(line, "#"):
		for n < len(line) && n < 6 && line[n] == '#' {
			n++
		}
	case strings.HasPrefix(line, ">"), strings.HasPrefix(line, "-"), strings.HasPrefix(line, "*"), strings.HasPrefix(line, "+"):
		n = 1
	default:
		for n < len(line) && line[n] >= '0' && line[n] <= '9' {
			n++
		}
		if n == 0 || n >= len(line) || (line[n] != '.' && line[n] != ')') {
			return line
		}
		n++
	}
	if n >= len(line) || (line[n] != ' ' && line[n] != '\t') {
		return line
	}
	return strings.TrimLeft(line[n:], " \t")
}

// cutBox is a line without the box of a task that starts it, and the spaces after it.
func cutBox(line string) string {
	if len(line) >= 3 && line[0] == '[' && line[2] == ']' && (line[1] == ' ' || line[1] == 'x' || line[1] == 'X') {
		return strings.TrimLeft(line[3:], " \t")
	}
	return line
}

// seg is a piece of a result's text; hit when it is a word asked for.
type seg struct {
	text string
	hit  bool
}

// searchHit is a page found, and a place in its text.
type searchHit struct {
	page  *Page
	score int
	snip  []seg
	at    string // the first word, folded: where in the page's text to go
}

// snippet is the words of text around the first of the words, marked.
func snippet(text string, words []string) []seg {
	folded := foldText(text)
	src := text
	if utf8.RuneCountInString(folded) != utf8.RuneCountInString(text) {
		src = folded // the letters do not line up: the folded text is shown
	}
	rs, fr := []rune(src), []rune(folded)
	first := -1
	for _, w := range words {
		if i := runeIndex(fr, []rune(w)); i >= 0 && (first < 0 || i < first) {
			first = i
		}
	}
	from, to := 0, min(len(rs), 160)
	if first >= 0 {
		from = max(0, first-45)
		to = min(len(rs), first+120)
	}
	var out []seg
	i := from
	for i < to {
		hit := 0
		for _, w := range words {
			if wr := []rune(w); len(wr) > 0 && i+len(wr) <= len(fr) && string(fr[i:i+len(wr)]) == w && len(wr) > hit {
				hit = len(wr)
			}
		}
		if hit > 0 {
			out = append(out, seg{string(rs[i:min(i+hit, len(rs))]), true})
			i += hit
			continue
		}
		j := i + 1
		for j < to {
			stop := false
			for _, w := range words {
				if wr := []rune(w); len(wr) > 0 && j+len(wr) <= len(fr) && string(fr[j:j+len(wr)]) == w {
					stop = true
				}
			}
			if stop {
				break
			}
			j++
		}
		out = append(out, seg{string(rs[i:j]), false})
		i = j
	}
	if from > 0 && len(out) > 0 {
		out[0].text = "…" + strings.TrimLeft(out[0].text, " ")
	}
	if to < len(rs) && len(out) > 0 {
		out[len(out)-1].text += "…"
	}
	return out
}

func runeIndex(hay, needle []rune) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if string(hay[i:i+len(needle)]) == string(needle) {
			return i
		}
	}
	return -1
}

// searchPages are the pages that meet query q (searchquery.go), the best first.
func (a *app) searchPages(q string, limit int, force bool) []searchHit {
	return a.searchKept(q, limit, force, true)
}

// searchList is searchPages without the snippets: what wants only the pages (a dynamic collection, a count).
func (a *app) searchList(q string, limit int, force bool) []searchHit {
	return a.searchKept(q, limit, force, false)
}

// searchKey is what a search's hits come from: the query and how many, whether with snippets, the pages and their
// texts as last indexed, the open page's text, and the day (a query may say "today").
type searchKey struct {
	q             string
	limit         int
	snips         bool
	pages, writes uint64
	built         time.Time
	text          uint64
	day           string
}

// searchKept is the search, its hits kept while what they come from is as it was: a view, the search screen and the
// palette ask for them at every frame, and a query that finds two thousand pages folded each of them a frame (#187).
func (a *app) searchKept(q string, limit int, force, snips bool) []searchHit {
	if parseQuery(q).empty() {
		return nil
	}
	a.linksTo(nil, a.clock(), force)
	k := searchKey{q: q, limit: limit, snips: snips, pages: PagesGen.Load(), writes: a.libWrites() + SavesGen.Load(), built: a.links.built, day: a.clock().Format("2006-01-02")}
	if a.pane().doc != nil {
		k.text = a.pane().doc.Version()
	}
	if hits, ok := a.searchCache[k]; ok {
		return hits
	}
	hits := a.findPages(q, limit, snips)
	a.searchMade++
	if len(a.searchCache) >= 32 { // a handful of queries at a time: the oldest go with the rest
		clear(a.searchCache)
	}
	if a.searchCache == nil {
		a.searchCache = map[searchKey][]searchHit{}
	}
	a.searchCache[k] = hits
	return hits
}

// findPages is the search itself: the pages for query q, best first, at most limit (0: all), with the words around
// what was asked when snips.
func (a *app) findPages(q string, limit int, snips bool) []searchHit {
	pq := parseQuery(q)
	if pq.empty() {
		return nil
	}
	pq.resolve(a.clock())
	marks := pq.marks()
	if a.sx != nil && a.sx.ready { // the index: the pages that may have it, then the snippets of the few shown
		first := ""
		if len(marks) > 0 {
			first = marks[0]
		}
		hits := a.sx.query(a, pq, limit)
		out := make([]searchHit, 0, len(hits))
		for _, h := range hits {
			hit := searchHit{page: h.d.page, score: h.score, at: first}
			if snips {
				hit.snip = snippet(snippetText(h.d.page), marks)
			}
			out = append(out, hit)
		}
		return out
	}
	first := ""
	if len(marks) > 0 {
		first = marks[0]
	}
	var out []searchHit
	for _, c := range a.lib.Collections {
		cname := foldText(c.Name)
		okColl := true
		for _, w := range pq.colls {
			okColl = okColl && strings.Contains(cname, w)
		}
		if !okColl {
			continue
		}
		for _, p := range c.All() {
			score, ok := a.score(pq.root, qdoc{p, c, foldText(p.Title), a.plainOf(p).folded, cname})
			if !ok {
				continue
			}
			hit := searchHit{page: p, score: score, at: first}
			if snips {
				hit.snip = snippet(snippetText(p), marks)
			}
			out = append(out, hit)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].page.Modified.After(out[j].page.Modified)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// openSearch shows the search in the active pane, with q asked.
func (a *app) openSearch(q string) { a.openSearchIn(a.pane(), q) }

func (a *app) openSearchIn(pn *paneState, q string) {
	if pn == a.pane() {
		a.flush()
	}
	if strings.TrimSpace(q) != "" {
		a.noteSearch()
	}
	pn.searchOpen, pn.searchQ, pn.searchFocus = true, q, true
	pn.tag, pn.coll, pn.trashOpen, pn.tasksOpen, pn.settingsOpen = "", nil, false, false, false
	pn.statsScreen, pn.profileScreen = false, false
	a.linksStale()
	a.visitIn(pn)
}

// openFound opens the page of a result with the caret where the word is.
func (a *app) openFound(h searchHit) { a.openFoundIn(a.pane(), h) }

func (a *app) openFoundIn(pn *paneState, h searchHit) {
	a.openIn(pn, h.page)
	if pn.doc == nil {
		return
	}
	if at := runeIndex([]rune(foldText(pn.doc.Text().String())), []rune(h.at)); at >= 0 {
		a.gotoPosIn(pn, at)
	}
}

// searchView is the search: a field, and the pages it finds.
func (a *app) searchView(c *ui.Context, pn *paneState) {
	t := c.Theme()
	cl := ryugo.From(c).Colors
	hits := a.searchPages(pn.searchQ, 80, a.links.stale)
	ui.Column(c).Fill().MaxWidth(viewMax).Gap(10).Children(func() {
		ui.Text(c, locales.CommonSearch()).FontSize(32).FontWeight(720).Padding(2, 0, 2, 0)
		in := a.fieldMenu(ryugo.Input(c.Key("search-q"), &pn.searchQ, ryugo.InputOptions{Label: locales.SearchTheLibrary(), Placeholder: locales.QuerySearchPagesTitlesText(), PrefixIcon: iconSearch, Clearable: true}), &pn.searchQ, fieldOpts{oneLine: true})
		if pn.searchFocus {
			in.Focus()
			pn.searchFocus = false
		}
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() { a.saveSearchButton(c); a.replaceBar(c) }) // replaceBar: libreplace.go
		sub := locales.QueryPagesWithEveryWord()
		switch {
		case strings.TrimSpace(pn.searchQ) == "":
		case len(hits) == 0:
			sub = locales.SearchNoMatches()
		default:
			sub = locales.CommonPageCount(len(hits))
			if len(hits) == 80 {
				sub = locales.SearchFirst80()
			}
		}
		ui.Text(c, sub).FontSize(13.5).TextColor(t.TextMuted).Padding(2, 0, 4, 0)
		if len(hits) == 0 {
			if strings.TrimSpace(pn.searchQ) == "" {
				a.emptyState(c, iconSearch, locales.SearchTheLibrary(), locales.SearchThePagesTitlesAndText(), "", nil)
			} else {
				a.emptyState(c, iconSearch, locales.CommonNoPageMatches(), locales.TryAnotherWordOrTakeThe(), "", nil)
			}
			return
		}
		a.scrolledPane(c, pn, "search-scroll", nil, ui.Scroll(c.Key("search-scroll")).Grow(1), func() {
			ui.Column(c).FillWidth().Gap(2).Padding(0, 0, 40, 0).Children(func() {
				for _, h := range hits {
					h := h
					row := ui.Column(c.Key("sr:"+h.page.Dir)).FillWidth().Padding(8, 10).Gap(3).Radius(rad(8)).
						Label(locales.SearchResultLabel(h.page.Title)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					a.contextOn(row, func() []menuItem { return a.pageMenu(h.page, false) })
					row.Children(func() {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, iconPage).Width(16).Height(16).TextColor(cl.Muted)
							ui.Text(c, h.page.Title).FontSize(15).FontWeight(620).MaxLines(1).Shrink(1)
							where := h.page.Coll.Name
							if crumb := Parents(h.page); crumb != "" {
								where += " › " + crumb
							}
							ui.Text(c, where).FontSize(12).TextColor(cl.Muted).MaxLines(1).Shrink(2).Grow(1)
							a.replaceRowButton(c, h.page)
						})
						var spans []ui.Span
						for _, sg := range h.snip {
							sp := ui.Span{Text: sg.text, Size: 13, Color: cl.Muted}
							if sg.hit {
								sp.Color, sp.Weight, sp.Background, sp.Radius = cl.Text, 650, cl.Accent.Alpha(0.2), 3
							}
							spans = append(spans, sp)
						}
						if len(spans) > 0 {
							ui.RichText(c, spans...).Padding(0, 0, 0, 24).MaxLines(2)
						}
					})
					a.kbMark(row, "s:"+h.page.Dir, true, func() { a.openFoundIn(pn, h) })
					if row.Clicked() {
						a.openFoundIn(pn, h)
					}
				}
			})
		})
	})
}
