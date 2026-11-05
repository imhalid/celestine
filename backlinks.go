package main

import (
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
		l.stale, l.built = false, now
		for _, c := range a.lib.Collections {
			for _, q := range c.All() {
				body, err := q.Body()
				if err != nil {
					continue
				}
				l.tasks = append(l.tasks, tasksOf(q, body)...)
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
			row := ui.Row(c.Key("bl:"+q.Dir)).FillWidth().Height(34).Padding(0, 8).Gap(10).Radius(8).AlignItems(ui.Center).
				Label("Bağlantı: " + q.Title).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			if row.Hovered() {
				row.Background(cl.Fill)
			}
			a.contextOn(row, func() []menuItem { return a.pageMenu(q, false) })
			row.Children(func() {
				ui.Icon(c, iconPage).Width(15).Height(15).TextColor(cl.Muted)
				ui.Text(c, q.Title).FontSize(14).TextColor(cl.Text).MaxLines(1).Shrink(1)
				ui.Text(c, q.Coll.Name).FontSize(12).TextColor(cl.Muted).MaxLines(1).Shrink(2).Grow(1)
			})
			if row.Clicked() {
				a.open(q)
			}
		}
	})
	a.linksH = box.LayoutBounds().H
}
