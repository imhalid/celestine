package main

import (
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Unlinked mentions (#548): under the backlinks, the pages whose text names this page - its title or an alias, as a
// whole word - without a link to it, each with the words around the name and a button that makes the first such
// mention a [[link]]. They are looked for when the section is opened, not at every frame, and again when the pages
// change. A page linked so is kept as a version first, and the notice's Undo puts it back as it was.

// mentionMin is how many letters a name must have to be looked for: a title of one or two letters is in every text.
const mentionMin = 3

// mentionsState is the section of the open page: whether it is open, and what was found, for which page and which
// state of the pages.
type mentionsState struct {
	open  bool
	dir   string
	built uint64
	found []mention
}

// mention is a page that names the open one without a link: the name it uses, and the words around it.
type mention struct {
	page *Page
	name string
	segs []seg
}

// mentionNames are the names page p is mentioned by: its title and aliases long enough to be looked for.
func mentionNames(p *Page) []string {
	var out []string
	for _, n := range append([]string{p.Title}, p.Aliases()...) {
		if n = strings.TrimSpace(n); len([]rune(n)) >= mentionMin && !slices.ContainsFunc(out, func(o string) bool { return strings.EqualFold(o, n) }) {
			out = append(out, n)
		}
	}
	return out
}

// mentionAt is where name is in text as a whole word, any case, from rune from on, and not inside what skip says is
// not text (a chip, code): its start in runes, or -1.
func mentionAt(text []rune, name string, from int, skip func(i int) bool) int {
	nr := []rune(name)
	word := func(i int) bool {
		return i >= 0 && i < len(text) && (unicode.IsLetter(text[i]) || unicode.IsDigit(text[i]))
	}
	for i := from; i+len(nr) <= len(text); i++ {
		if !strings.EqualFold(string(text[i:i+len(nr)]), name) || word(i-1) || word(i+len(nr)) {
			continue
		}
		hit := true
		for k := i; k < i+len(nr) && skip != nil; k++ {
			if skip(k) {
				hit = false
				break
			}
		}
		if hit {
			return i
		}
	}
	return -1
}

// findMentions are the pages that name p without linking to it.
func (a *app) findMentions(p *Page) []mention {
	names := mentionNames(p)
	if len(names) == 0 {
		return nil
	}
	linked := a.linksTo(p, a.clock(), false)
	var out []mention
	for _, c := range a.lib.Collections {
		for _, q := range c.All() {
			if q == p || slices.Contains(linked, q) {
				continue
			}
			folded := a.plainOf(q).folded
			for _, n := range names {
				if !strings.Contains(folded, foldText(n)) {
					continue
				}
				plain := snippetText(q)
				if mentionAt([]rune(plain), n, 0, nil) < 0 {
					continue
				}
				out = append(out, mention{page: q, name: n, segs: snippet(plain, []string{foldText(n)})})
				break
			}
		}
	}
	return out
}

// mentionsView is the section of unlinked mentions under the backlinks, while the page is one that can be named.
func (a *app) mentionsView(c *ui.Context, p *Page) {
	m := &a.mentions
	if p == nil || len(mentionNames(p)) == 0 {
		return
	}
	if m.dir != p.Dir { // another page: the section starts folded
		*m = mentionsState{dir: p.Dir}
	}
	cl := ryugo.From(c).Colors
	box := ui.Column(c.Key("mentions")).FillWidth().Gap(2).Padding(4, 0, 24, 0)
	box.Children(func() {
		head := ui.Row(c.Key("mentions-head")).Padding(6, 6, 4, 6).Gap(6).AlignItems(ui.Center).Radius(rad(6)).
			Role(ui.RoleDisclosure).Label(locales.MentionsTitle()).Expanded(m.open).Cursor(ui.CursorPointer)
		head.Children(func() {
			ui.Icon(c, iconChevron).Width(12).Height(12).TextColor(cl.Muted).Rotate(map[bool]float32{true: 90}[m.open])
			ui.Text(c, locales.MentionsTitle()).FontSize(12).FontWeight(650).TextColor(cl.Muted)
			if m.open && m.found != nil {
				ui.Text(c, locales.MentionsCount(len(m.found))).FontSize(12).TextColor(cl.Muted)
			}
		})
		if head.Clicked() {
			m.open = !m.open
		}
		if !m.open {
			return
		}
		// looked for again when the pages changed: the tree (PagesGen), or what they say (the index read again)
		if gen := PagesGen.Load() + uint64(a.links.built.UnixNano()); m.found == nil || m.built != gen {
			m.found, m.built = a.findMentions(p), gen
			if m.found == nil {
				m.found = []mention{}
			}
		}
		if len(m.found) == 0 {
			ui.Text(c, locales.MentionsNone()).FontSize(13).TextColor(cl.Muted).Padding(2, 8, 6, 26)
			return
		}
		for _, mn := range m.found {
			mn := mn
			row := ui.Row(c.Key("mn:"+mn.page.Dir)).FillWidth().Padding(6, 8).Gap(10).Radius(rad(8)).AlignItems(ui.Center)
			if row.Hovered() {
				row.Background(cl.Fill)
			}
			a.contextOn(row, func() []menuItem { return a.pageMenu(mn.page, false) })
			row.Children(func() {
				col := ui.Column(c.Key("mn-text")).Grow(1).MinWidth(0).Gap(2).Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.CommonLinkLabel(mn.page.Title))
				col.Children(func() {
					ui.Text(c, mn.page.Title).FontSize(14).TextColor(cl.Text).MaxLines(1)
					if len(mn.segs) > 0 {
						ui.RichText(c, contextSpans(mn.segs, 12.5, cl.Muted, cl.Text, cl.Accent.Alpha(0.18))...).MaxLines(2)
					}
				})
				if col.Clicked() {
					a.open(mn.page)
				}
				if ryugo.Button(c.Key("mn-link"), locales.MentionsLink(), ryugo.ButtonOptions{Icon: iconLink, Size: ryugo.Sm, Variant: ryugo.Soft}).Clicked() {
					a.later(func() { a.linkMention(p, mn) })
				}
			})
		}
	})
	a.linksH += box.LayoutBounds().H
}

// linkMention makes the first unlinked mention of p in mn's page a [[link]]: by the name it uses, which resolves to
// p. The page is kept as a version first, and the notice's Undo writes it back as it was.
func (a *app) linkMention(p *Page, mn mention) {
	q := mn.page
	ch := &chip{kind: "page", target: mn.name}
	if strings.EqualFold(mn.name, p.Title) {
		ch.target = p.Title
	}
	for i := range a.ps.states { // shown in a pane: its text is changed there, and saved as typing is
		if st := &a.ps.states[i]; st.page == q && a.docs[q] != nil {
			doc := a.docs[q]
			t := doc.Text()
			at := mentionIn(t, mn.name)
			if at < 0 {
				return
			}
			attrs := t.AttributesAt(at)
			attrs.Attachment = a.attachment(ch)
			doc.Edit(func(ed *richtext.Edit) {
				ed.ReplaceString(at, at+len([]rune(mn.name)), string(richtext.ObjectReplacement), attrs)
			})
			a.afterMention(q)
			return
		}
	}
	delete(a.docs, q) // a text kept of it would be saved over the file
	raw, err := q.Body()
	if err != nil {
		a.fail(err)
		return
	}
	t := decodeBody(raw)
	at := mentionIn(t, mn.name)
	if at < 0 {
		return
	}
	attrs := t.AttributesAt(at)
	attrs.Attachment = &richtext.Attachment{Name: ch.label(), Data: ch}
	t = t.ReplaceString(at, at+len([]rune(mn.name)), string(richtext.ObjectReplacement), attrs)
	if h, ok := historyOf(q); ok {
		h.SnapshotNow(q.File())
	}
	if err := q.Save(encodeBody(t)); err != nil {
		a.fail(err)
		return
	}
	a.afterMention(q)
	a.toastAction(locales.MentionsLinked(q.Title), locales.CommonUndo(), func() {
		delete(a.docs, q)
		if err := q.Save(raw); err != nil {
			a.fail(err)
		}
		a.linksStale()
	})
}

// afterMention is what a mention linked changes: the backlinks, and what the section found.
func (a *app) afterMention(q *Page) {
	a.linksStale()
	a.links.built = time.Time{} // read again at once
	a.mentions.found = nil
}

// mentionIn is where name is first mentioned in t as text, a whole word outside chips and code, or -1.
func mentionIn(t richtext.Text, name string) int {
	return mentionAt([]rune(t.String()), name, 0, func(i int) bool {
		at := t.AttributesAt(i)
		return at.Attachment != nil || at.Code || t.ParagraphStyleAt(i).CodeBlock
	})
}
