package main

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// Search over the whole library: the text of every page, not only its title.
// The texts are read with the backlinks' index (backlinks.go), plain and on
// one line; a page has to have every word asked for, in its title or its
// text, Turkish letters taken as their ASCII (as the palette does), and the
// ones with the words in the title, or the most of them, come first. A
// result shows the place it was found, the words marked.

var (
	mdLink    = regexp.MustCompile(`!?\[([^\[\]]*)\]\([^)]*\)`)
	mdWiki    = regexp.MustCompile(`\[\[([^\[\]|]*\|)?([^\[\]]*)\]\]`)
	mdMarks   = regexp.MustCompile("(?m)^\\s*(#{1,6}|>|[-*+]|\\d+[.)])\\s+|^\\s*```.*$|^\\s*\\[[ xX]\\]\\s*|[*_~`]{1,3}|\\|")
	spacesRun = regexp.MustCompile(`\s+`)
)

// plainText is Markdown as the words a reader sees, on one line.
func plainText(md string) string {
	s := mdLink.ReplaceAllString(md, "$1")
	s = mdWiki.ReplaceAllString(s, "$2")
	s = mdMarks.ReplaceAllString(s, " ")
	return strings.TrimSpace(spacesRun.ReplaceAllString(s, " "))
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

// query is what was asked, taken apart: words, "phrases", and the filters
// başlık:word (in the title), koleksiyon:ad, #etiket.
type query struct {
	words   []string // folded
	phrases []string
	title   []string
	colls   []string
	tags    []string
}

var queryPart = regexp.MustCompile(`"([^"]+)"|(\S+)`)

// parseQuery takes q apart.
func parseQuery(q string) query {
	var out query
	seen := map[string]bool{}
	add := func(list *[]string, w string) {
		if w != "" && !seen[fmt.Sprint(list, w)] {
			seen[fmt.Sprint(list, w)] = true
			*list = append(*list, w)
		}
	}
	for _, m := range queryPart.FindAllStringSubmatch(q, -1) {
		if m[1] != "" {
			add(&out.phrases, fold(strings.TrimSpace(m[1])))
			continue
		}
		w := m[2]
		switch {
		case strings.HasPrefix(w, "#") && len(w) > 1:
			add(&out.tags, tagKey(w[1:]))
			continue
		}
		if k, v, ok := strings.Cut(w, ":"); ok && v != "" {
			switch fold(k) {
			case "baslik", "title":
				add(&out.title, fold(v))
				continue
			case "koleksiyon", "kol", "in":
				add(&out.colls, fold(v))
				continue
			case "etiket", "tag":
				add(&out.tags, tagKey(v))
				continue
			}
		}
		add(&out.words, fold(w))
	}
	return out
}

// marks are the words to mark in a result: the words, the phrases, the title's.
func (q query) marks() []string {
	return append(append(append([]string{}, q.phrases...), q.title...), q.words...)
}

func (q query) empty() bool {
	return len(q.words)+len(q.phrases)+len(q.title)+len(q.colls)+len(q.tags) == 0
}

// queryWords are the words of a query, folded, once each.
func queryWords(q string) []string { return parseQuery(q).words }

// snippet is the words of text around the first of the words, marked.
func snippet(text string, words []string) []seg {
	folded := fold(text)
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

// searchPages are the pages that have every word of q, the best first.
func (a *app) searchPages(q string, limit int, force bool) []searchHit {
	pq := parseQuery(q)
	if pq.empty() {
		return nil
	}
	a.linksTo(nil, a.clock(), force)
	marks := pq.marks()
	var out []searchHit
	for _, c := range a.lib.Collections {
		cname := fold(c.Name)
		okColl := true
		for _, w := range pq.colls {
			okColl = okColl && strings.Contains(cname, w)
		}
		if !okColl {
			continue
		}
		for _, p := range c.All() {
			text, title := fold(a.links.texts[p]), fold(p.Title)
			score, ok := 0, true
			for _, w := range pq.words {
				inTitle, n := strings.Contains(title, w), strings.Count(text, w)
				if !inTitle && n == 0 {
					ok = false
					break
				}
				if inTitle {
					score += 50
				}
				score += min(n, 10) * 2
			}
			for _, w := range pq.phrases {
				inTitle, n := strings.Contains(title, w), strings.Count(text, w)
				if !ok || (!inTitle && n == 0) {
					ok = false
					break
				}
				score += map[bool]int{true: 60, false: 0}[inTitle] + min(n, 10)*3
			}
			for _, w := range pq.title {
				if !ok || !strings.Contains(title, w) {
					ok = false
					break
				}
				score += 60
			}
			for _, tg := range pq.tags {
				if !ok || !slices.Contains(a.links.tags[tg], p) {
					ok = false
					break
				}
				score += 20
			}
			if !ok {
				continue
			}
			first := ""
			if len(marks) > 0 {
				first = marks[0]
			}
			out = append(out, searchHit{page: p, score: score, snip: snippet(a.links.texts[p], marks), at: first})
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

// openSearch shows the search, with q asked.
func (a *app) openSearch(q string) {
	a.flush()
	a.searchOpen, a.searchQ, a.searchFocus = true, q, true
	a.tag, a.coll, a.trashOpen, a.tasksOpen = "", nil, false, false
	a.linksStale()
	a.visit()
}

// openFound opens the page of a result with the caret where the word is.
func (a *app) openFound(h searchHit) {
	a.open(h.page)
	if a.doc == nil {
		return
	}
	if at := runeIndex([]rune(fold(a.doc.Text().String())), []rune(h.at)); at >= 0 {
		a.gotoPos(at)
	}
}

// searchView is the search: a field, and the pages it finds.
func (a *app) searchView(c *ui.Context) {
	t := c.Theme()
	cl := ryugo.From(c).Colors
	hits := a.searchPages(a.searchQ, 80, a.links.stale)
	ui.Column(c).Fill().MaxWidth(viewMax).Gap(10).Children(func() {
		ui.Text(c, "Ara").FontSize(32).FontWeight(720).Padding(2, 0, 2, 0)
		in := ryugo.Input(c.Key("search-q"), &a.searchQ, ryugo.InputOptions{Label: "Kitaplıkta ara", Placeholder: "Sayfaların başlığında ve metninde ara…", PrefixIcon: iconSearch, Clearable: true})
		if a.searchFocus {
			in.Focus()
			a.searchFocus = false
		}
		ui.Row(c).Children(func() { a.saveSearchButton(c) })
		sub := "Her kelimeyi içeren sayfalar. \"tam ifade\", başlık:kelime, koleksiyon:ad, #etiket yazabilirsin."
		switch {
		case strings.TrimSpace(a.searchQ) == "":
		case len(hits) == 0:
			sub = "Eşleşen sayfa yok."
		default:
			sub = strconv.Itoa(len(hits)) + " sayfa"
			if len(hits) == 80 {
				sub = "İlk 80 sayfa"
			}
		}
		ui.Text(c, sub).FontSize(13.5).TextColor(t.TextMuted).Padding(2, 0, 4, 0)
		ui.Scroll(c.Key("search-scroll")).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(2).Padding(0, 0, 40, 0).Children(func() {
				for _, h := range hits {
					h := h
					row := ui.Column(c.Key("sr:"+h.page.Dir)).FillWidth().Padding(8, 10).Gap(3).Radius(8).
						Label("Sonuç: " + h.page.Title).Role(ui.RoleButton).Cursor(ui.CursorPointer)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					a.contextOn(row, func() []menuItem { return a.pageMenu(h.page, false) })
					row.Children(func() {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, iconPage).Width(16).Height(16).TextColor(cl.Muted)
							ui.Text(c, h.page.Title).FontSize(15).FontWeight(620).MaxLines(1).Shrink(1)
							where := h.page.Coll.Name
							if crumb := parents(h.page); crumb != "" {
								where += " › " + crumb
							}
							ui.Text(c, where).FontSize(12).TextColor(cl.Muted).MaxLines(1).Shrink(2).Grow(1)
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
					if row.Clicked() {
						a.openFound(h)
					}
				}
			})
		})
	})
}
