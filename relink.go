package main

import (
	"encoding/json"
	"strings"

	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/locales"
)

// Links follow a page renamed: a link to a page names its title ([[Dune]], chips.go), so when a rename is done
// (the title field left, a row renamed in the sidebar, an agent's update_page) the links to the old title, in every
// page, are written to the new one, what they say instead of the title kept. The title itself is saved at each
// key typed; its links are written once, when the field is left (#546). A toast says how many, with an Undo.
//
// Nothing is written when it could send a link to the wrong page: when another page has the old title too (the
// links may be its), or the new title is another page's already (the links would go to that one); a toast says
// the links were kept. A rename that changes only the case changes nothing either: links match any case.

// titleEdit is a rename being typed in a pane's title field: the page, and its title before the first key.
type titleEdit struct {
	p    *Page
	from string
}

// titleTyped gives page p the title name, typed in its title field: the first change remembers the title it had,
// for its links to be written when the field is left (endTitleEdit).
func (a *app) titleTyped(p *Page, name string) {
	if a.retitle.p != p {
		a.endTitleEdit()
		a.retitle = titleEdit{p: p, from: p.Title}
	}
	if err := p.SetTitle(name); err != nil {
		a.fail(err)
	}
}

// endTitleEdit ends the rename being typed in this pane, if there is one: the links to the page are written after
// the view (a pane's view runs as that pane, and the pages written may be shown in others).
func (a *app) endTitleEdit() {
	r := a.retitle
	a.retitle = titleEdit{}
	if r.p == nil {
		return
	}
	a.later(func() { a.relinkRenamed(r.p, r.from, false) })
}

// linkChange is a page whose links a rename wrote: its body before, and as it was written.
type linkChange struct {
	p             *Page
	before, after string
}

// relinkRenamed writes the links to page p, which was titled from, to its title now, in every page, and says how
// many it wrote with an Undo. agent says an agent renamed it: the pages are kept as the agent's edits are
// (agentTouch). It returns how many links were written.
func (a *app) relinkRenamed(p *Page, from string, agent bool) int {
	from, to := strings.TrimSpace(from), strings.TrimSpace(p.Title)
	if !a.inLibrary(p) || strings.EqualFold(from, to) || !library.LinkableTitle(from) || !library.LinkableTitle(to) {
		return 0
	}
	a.flush() // what is typed in the pages is on disk before they are read
	a.linksStale()
	a.linksTo(nil, a.clock(), true)
	pages := append([]*Page{}, a.links.into[strings.ToLower(from)]...)
	if len(pages) == 0 {
		return 0
	}
	for _, q := range a.titleIndex()[strings.ToLower(from)] {
		if q != p && strings.EqualFold(strings.TrimSpace(q.Title), from) {
			a.say(toastWarn, locales.LinksKeptShared(from))
			return 0
		}
	}
	for _, q := range a.titleIndex()[strings.ToLower(to)] {
		if q != p && strings.EqualFold(strings.TrimSpace(q.Title), to) {
			a.say(toastWarn, locales.LinksKeptTaken(from, to))
			return 0
		}
	}
	var changes []linkChange
	total := 0
	for _, q := range pages {
		if a.unsaved[q] { // its text is not on disk: written over, it would be lost
			continue
		}
		body, err := q.Body()
		if err == nil {
			err = bodyReadable(body) // what this version does not read would be dropped by writing it again
		}
		if err != nil {
			continue
		}
		nb, n := relinkBody(body, from, to)
		if n == 0 {
			continue
		}
		if agent {
			a.agentTouch(q)
		} else if h, ok := historyOf(q); ok {
			h.SnapshotNow(q.File())
		}
		if err := q.Save(nb); err != nil {
			a.fail(err)
			continue
		}
		a.takeOutsideAll(q)
		changes = append(changes, linkChange{q, body, nb})
		total += n
	}
	a.linksStale()
	if total == 0 {
		return 0
	}
	a.say(toastOK, locales.LinksUpdated(total))
	a.noticeUndo = func() { a.undoRelink(changes) }
	return total
}

// undoRelink puts back the pages a rename's links were written in, each as it was before: a page changed since is
// left as it is, what was written in it not lost.
func (a *app) undoRelink(changes []linkChange) {
	a.flush()
	for _, ch := range changes {
		if !a.inLibrary(ch.p) {
			continue
		}
		if now, err := ch.p.Body(); err != nil || now != ch.after {
			continue
		}
		if h, ok := historyOf(ch.p); ok {
			h.SnapshotNow(ch.p.File())
		}
		if err := ch.p.Save(ch.before); err != nil {
			a.fail(err)
			continue
		}
		a.takeOutsideAll(ch.p)
	}
	a.linksStale()
}

// takeOutsideAll takes the text of page p as it is on disk now, in every pane that shows it.
func (a *app) takeOutsideAll(p *Page) {
	a.takeOutside(p)
	for i := range a.ps.states {
		if i != a.ps.active && a.ps.states[i].page == p {
			a.withPane(i, func() { a.takeOutside(p) })
		}
	}
}

// relinkBody is body with its links to the page titled from written to the page titled to, and how many: the
// links and embeds, what they say instead of the title kept, and the [[links]] typed in a table's cells or in the
// text outside code. It works on the body's own JSON, so nothing else of it changes; a body that cannot be read
// is given back as it is.
func relinkBody(body, from, to string) (string, int) {
	var paras []bodyPara
	if json.Unmarshal([]byte(body), &paras) != nil {
		return body, 0
	}
	n := 0
	for i := range paras {
		code := paras[i].S != nil && paras[i].S.CodeBlock
		for j := range paras[i].R {
			r := &paras[i].R[j]
			var k int
			switch o := r.O; {
			case o != nil && (o.K == "page" || o.K == "embed"):
				if strings.EqualFold(strings.TrimSpace(o.Target), from) {
					o.Target, k = to, 1
				}
			case o != nil && o.K == "table":
				for _, row := range o.Rows {
					for c := range row {
						var m int
						row[c], m = library.RenameLinks(row[c], from, to)
						k += m
					}
				}
			case o == nil && !code && !r.C:
				r.T, k = library.RenameLinks(r.T, from, to)
			}
			n += k
		}
	}
	if n == 0 {
		return body, 0
	}
	b, err := json.Marshal(paras)
	if err != nil {
		return body, 0
	}
	return string(b), n
}
