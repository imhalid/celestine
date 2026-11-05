package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// Backlinks: under a page, the pages that link to it with [[Its title]].
// The links of every page are read from the files into an index when first
// asked for, and again after an edit (at most every few seconds, and at once
// when a page is opened): a library of a few thousand pages reads in a
// moment, and nothing is kept in the files for it.

type linkIndex struct {
	into     map[string][]*Page // a title, lower case, to the pages that link to it
	tags     map[string][]*Page // a tag (tags.go), lower case, to the pages that have it
	tagNames map[string]string  // and as it was first written
	tasks    []taskItem         // the tasks of every page (tasks.go)
	texts    map[*Page]string   // the text of every page, plain, for the search (search.go)
	built    time.Time
	stale    bool
}

// linksStale says the pages changed since the index was made.
func (a *app) linksStale() { a.links.stale = true }

// linksTo are the pages that link to p, the title first.
func (a *app) linksTo(p *Page, now time.Time, force bool) []*Page {
	l := &a.links
	if l.into == nil || (l.stale && (force || now.Sub(l.built) > 4*time.Second)) {
		l.into, l.tags, l.tagNames, l.tasks = map[string][]*Page{}, map[string][]*Page{}, map[string]string{}, nil
		l.texts = map[*Page]string{}
		l.stale, l.built = false, now
		for _, c := range a.lib.Collections {
			for _, q := range c.All() {
				body, err := q.Body()
				if err != nil {
					continue
				}
				l.tasks = append(l.tasks, tasksOf(q, body)...)
				l.texts[q] = plainText(body)
				for _, name := range tagsOf(body) {
					k := tagKey(name)
					if _, ok := l.tagNames[k]; !ok {
						l.tagNames[k] = name
					}
					l.tags[k] = append(l.tags[k], q)
				}
				if !strings.Contains(body, "[[") {
					continue
				}
				seen := map[string]bool{}
				for _, m := range wikiInText.FindAllStringSubmatch(body, -1) {
					target, _, _ := strings.Cut(m[1], "|")
					key := strings.ToLower(strings.TrimSpace(target))
					if key != "" && !seen[key] {
						seen[key] = true
						l.into[key] = append(l.into[key], q)
					}
				}
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
		ui.Text(c, strconv.Itoa(len(links))+" sayfa bu sayfaya bağlanıyor").FontSize(12).FontWeight(650).TextColor(cl.Muted).Padding(0, 6, 4, 6)
		for _, q := range links {
			q := q
			row := ui.Column(c.Key("bl:"+q.Dir)).FillWidth().Padding(6, 8).Gap(2).Radius(8).
				Label("Bağlantı: " + q.Title).Role(ui.RoleButton).Cursor(ui.CursorPointer)
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
	if a.ctxCache == nil || len(a.ctxCache) > 800 {
		a.ctxCache = map[string][]seg{}
	}
	var out []seg
	if body, err := q.Body(); err == nil {
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
