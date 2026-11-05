package main

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// Tags: #etiket in a page's text, as it is typed, with nothing kept for it in
// the file. The editor shows it as a soft pill over the word; a click opens
// the tag - the pages that have it - and the palette and the side panel list
// the tags. The index of them is the backlinks' (backlinks.go), made in the
// same reading of the pages.

// tagWord is a tag: a # at the start of a word, then letters, digits, - _ /,
// the first a letter.
var tagWord = regexp.MustCompile(`(^|\s)#(\pL[\pL\pN_/-]*)`)

var (
	fenceLine  = regexp.MustCompile("^\\s*(```|~~~)")
	inlineCode = regexp.MustCompile("`[^`\n]*`")
)

// tagKey is how tags are told apart: lower case, Turkish letters kept.
func tagKey(name string) string { return strings.ToLower(strings.TrimSuffix(name, "/")) }

// tagsOf are the tags in Markdown body, as written, once each; code is left out.
func tagsOf(body string) []string {
	if !strings.Contains(body, "#") {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	inFence := false
	for _, ln := range strings.Split(body, "\n") {
		if fenceLine.MatchString(ln) {
			inFence = !inFence
			continue
		}
		if inFence || !strings.Contains(ln, "#") {
			continue
		}
		ln = inlineCode.ReplaceAllString(ln, " ")
		for _, m := range tagWord.FindAllStringSubmatch(ln, -1) {
			if k := tagKey(m[2]); !seen[k] {
				seen[k] = true
				out = append(out, m[2])
			}
		}
	}
	return out
}

// tagSpan is a tag in a paragraph: where it is, in runes of the paragraph.
type tagSpan struct {
	from, to int
	name     string
}

// tagSpans are the tags in the paragraph line, not in code.
func tagSpans(line string, code bool) []tagSpan {
	if code || !strings.Contains(line, "#") {
		return nil
	}
	var out []tagSpan
	for _, m := range tagWord.FindAllStringSubmatchIndex(line, -1) {
		from := utf8.RuneCountInString(line[:m[4]-1]) // the #
		out = append(out, tagSpan{from, utf8.RuneCountInString(line[:m[5]]), line[m[4]:m[5]]})
	}
	return out
}

// tagHit is a tag drawn, where in the window, for a click on it.
type tagHit struct {
	r   ui.Rect
	key string
}

// tagAt is the tag at (x, y) of the window, as the last frame drew it.
func (a *app) tagAt(x, y float32) (string, bool) {
	for _, h := range a.tagSeen {
		if x >= h.r.X && x < h.r.X+h.r.W && y >= h.r.Y && y < h.r.Y+h.r.H {
			return h.key, true
		}
	}
	return "", false
}

// drawTags puts a soft pill over each tag of the paragraphs in view.
func (a *app) drawTags(p *ui.Painter, r ui.Rect) {
	if a.doc == nil {
		return
	}
	th := ryugo.Painting(p).Colors
	text := a.doc.Text()
	a.tagHits = a.tagHits[:0]
	for _, b := range a.boxes {
		s, e := text.ParagraphRange(b.Start)
		if e <= s {
			continue
		}
		var para richtext.Paragraph
		for _, pp := range text.Slice(s, e).Paragraphs() {
			para = pp
			break
		}
		para.Style = text.ParagraphStyleAt(s)
		var line string
		for _, run := range para.Runs {
			line += run.Text
		}
		spans := tagSpans(line, para.Style.CodeBlock)
		if len(spans) == 0 {
			continue
		}
		l, left := layoutParagraph(para, r.W, a.font, a.theme)
		for _, sp := range spans {
			if text.AttributesAt(s + sp.from).Code {
				continue
			}
			key := tagKey(sp.name)
			hot := key == a.hoverTag
			for _, rc := range l.SelectionRects(ui.TextInputRange{Start: units(line, sp.from), End: units(line, sp.to)}) {
				box := ui.Rect{X: r.X + left + rc.X - 2, Y: r.Y + b.Top + rc.Y + 1, W: rc.W + 4, H: rc.H - 2}
				p.Fill(box, th.Accent.Alpha(map[bool]float32{true: 0.26, false: 0.13}[hot]), 5)
				a.tagHits = append(a.tagHits, tagHit{box, key})
			}
		}
	}
}

// openTag shows the pages that have tag key.
func (a *app) openTag(key string) {
	a.flush()
	a.tag, a.coll, a.trashOpen = key, nil, false
	a.visit()
}

// noPage reports that something other than a page is shown: a collection,
// the trash, a tag.
func (a *app) noPage() bool { return a.coll != nil || a.trashOpen || a.tag != "" }

// tagList is every tag of the library, the most used first.
type tagInfo struct {
	key, name string
	pages     []*Page
}

func (a *app) allTags() []tagInfo {
	a.linksTo(a.page, a.clock(), false) // the index is made with the backlinks'
	var out []tagInfo
	for k, ps := range a.links.tags {
		out = append(out, tagInfo{k, a.links.tagNames[k], ps})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].pages) != len(out[j].pages) {
			return len(out[i].pages) > len(out[j].pages)
		}
		return out[i].key < out[j].key
	})
	return out
}

// tagPages are the pages that have the tag, the last changed first.
func (a *app) tagPages(key string) []*Page {
	a.linksTo(a.page, a.clock(), false)
	out := append([]*Page{}, a.links.tags[key]...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out
}

// tagView is the pages of a tag, with the other tags above them to go to.
func (a *app) tagView(c *ui.Context) {
	t := c.Theme()
	cl := ryugo.From(c).Colors
	now := a.clock()
	all := a.allTags()
	name := a.tag
	for _, ti := range all {
		if ti.key == a.tag {
			name = ti.name
		}
	}
	pages := a.tagPages(a.tag)
	ui.Column(c).Fill().MaxWidth(viewMax).Gap(10).Children(func() {
		ui.Text(c, "#"+name).FontSize(32).FontWeight(720).Padding(2, 0, 2, 0)
		sub := strconv.Itoa(len(pages)) + " sayfa bu etiketi taşıyor"
		if len(pages) == 0 {
			sub = "Bu etiketi taşıyan sayfa yok."
		}
		ui.Text(c, sub).FontSize(13.5).TextColor(t.TextMuted).Padding(0, 0, 4, 0)
		if len(all) > 1 {
			ui.Row(c.Key("tag-cloud")).Wrap().Gap(6).Padding(0, 0, 6, 0).Children(func() {
				for _, ti := range all {
					a.tagPill(c, cl, ti, ti.key == a.tag)
				}
			})
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(2).Padding(0, 0, 40, 0).Children(func() {
				for _, q := range pages {
					q := q
					row := ui.Row(c.Key("tp:"+q.Dir)).Padding(8, 10).Gap(10).Radius(8).AlignItems(ui.Center).
						Label("Sayfa: " + q.Title).Role(ui.RoleButton).Cursor(ui.CursorPointer)
					if row.Hovered() {
						row.Background(t.SurfaceHover)
					}
					a.contextOn(row, func() []menuItem { return a.pageMenu(q, false) })
					row.Children(func() {
						ui.Icon(c, iconPage).Width(17).Height(17).TextColor(t.TextMuted)
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
							ui.Text(c, q.Title).FontSize(15).FontWeight(600).MaxLines(1)
							where := q.Coll.Name
							if crumb := parents(q); crumb != "" {
								where += " › " + crumb
							}
							ui.Text(c, where+"  ·  "+ago(q.Modified, now)).FontSize(12.5).TextColor(t.TextMuted).MaxLines(1)
						})
					})
					if row.Clicked() {
						a.open(q)
					}
				}
			})
		})
	})
}

// tagPill is a tag as a small button: its name and how many pages have it.
func (a *app) tagPill(c *ui.Context, cl ryugo.Colors, ti tagInfo, on bool) {
	pill := ui.Row(c.Key("tag:"+ti.key)).Padding(3, 9).Gap(6).Radius(11).AlignItems(ui.Center).
		Label("Etiket: " + ti.name).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	hot := pill.AnimateTo("on", map[bool]float32{true: 1, false: 0}[on], fadeQuick)
	if on || pill.Hovered() {
		pill.Background(cl.Accent.Alpha(0.12 + 0.12*hot))
	} else {
		pill.Background(cl.Fill)
	}
	pill.Children(func() {
		ui.Text(c, "#"+ti.name).FontSize(12.5).FontWeight(580).TextColor(ryugo.Mix(cl.Text2, cl.Accent, hot))
		ui.Text(c, strconv.Itoa(len(ti.pages))).FontSize(11.5).TextColor(cl.Muted)
	})
	if pill.Clicked() {
		a.openTag(ti.key)
	}
}

// panelTags is the side panel's list of the page's tags.
func (a *app) panelTags(c *ui.Context, cl ryugo.Colors) {
	names := a.pageTags()
	if len(names) == 0 {
		return
	}
	all := map[string]tagInfo{}
	for _, ti := range a.allTags() {
		all[ti.key] = ti
	}
	ui.Column(c.Key("panel-tags")).FillWidth().Gap(4).Children(func() {
		panelTitle(c, cl, "ETİKETLER")
		ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
			for _, n := range names {
				ti, ok := all[tagKey(n)]
				if !ok {
					ti = tagInfo{key: tagKey(n), name: n}
				}
				a.tagPill(c, cl, ti, false)
			}
		})
	})
}

// pageTags are the tags in the open page's text, as it is now.
func (a *app) pageTags() []string {
	if a.doc == nil {
		return nil
	}
	if v := a.doc.Version(); v != a.ptagsVer || a.ptagsDoc != a.doc {
		a.ptags, a.ptagsVer, a.ptagsDoc = tagsOf(bodyMarkdown(a.doc.Text())), v, a.doc
	}
	return a.ptags
}
