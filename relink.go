package main

import (
	"encoding/json"
	"slices"
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
// for its links to be written when the field is left (endTitleEdit), and keeps it in the settings too, so that a
// rename the app did not live to end is ended at its next start (finishRenames).
func (a *app) titleTyped(pn *paneState, p *Page, name string) {
	if pn.retitle.p != p {
		a.endTitleEditIn(pn)
		pn.retitle = titleEdit{p: p, from: p.Title}
		if a.settings.PendingRenames == nil {
			a.settings.PendingRenames = map[string]string{}
		}
		a.settings.PendingRenames[p.Dir] = p.Title
		a.saveSettingsSoon()
	}
	a.retitled(pn, p, name)
}

// retitled gives page p the title name in memory at once, and has it written with the page's next save: a key typed
// in a title wrote the whole page's file at each key (#192). The open page is saved by its autosave; another, as a
// side pane's, at once.
func (a *app) retitled(pn *paneState, p *Page, name string) {
	if p != pn.page || pn.doc == nil {
		if err := p.SetTitle(name); err != nil {
			a.fail(err)
		}
		return
	}
	p.Title = name
	PagesChanged()
	pn.lastSave = ""                      // what is saved next is written, the body unchanged or not, with the title
	pn.dirty, pn.editAt = true, a.clock() // saved once the typing pauses, as the text is
}

// endTitleEditIn ends the rename being typed in pn, if there is one: the links to the page are written after
// the view (the pages written may be shown in other panes).
func (a *app) endTitleEditIn(pn *paneState) {
	r := pn.retitle
	pn.retitle = titleEdit{}
	a.endRename(r)
}

// endTitleEdit ends the rename being typed in the active pane, if there is one.
func (a *app) endTitleEdit() { a.endTitleEditIn(a.pane()) }

// endRename writes the links of rename r, typed in a pane that is done with it, after the view.
func (a *app) endRename(r titleEdit) {
	if r.p == nil {
		return
	}
	a.later(func() { a.relinkRenamed(r.p, r.from, false) })
}

// finishRenames ends the renames the settings say were being typed when the app last went (killed, or crashed): the
// links to each page are written from the title it had to the one it has. It runs at the start, after the first view.
func (a *app) finishRenames() {
	if len(a.settings.PendingRenames) == 0 {
		return
	}
	var rs []renamed
	for dir, from := range a.settings.PendingRenames {
		if p := a.pageAt(dir); p != nil {
			rs = append(rs, renamed{p, from})
		}
	}
	a.later(func() { a.relinkAll(rs, false) })
}

// linkChange is a page whose links a rename wrote: its body before, and as it was written.
type linkChange struct {
	p             *Page
	before, after string
}

// renamed is a page renamed, and the title it had.
type renamed struct {
	p    *Page
	from string
}

// relinkRenamed writes the links to page p, which was titled from, to its title now, in every page, and says how
// many it wrote with an Undo. agent says an agent renamed it: the pages are kept as the agent's edits are
// (agentTouch). It returns how many links were written.
func (a *app) relinkRenamed(p *Page, from string, agent bool) int {
	return a.relinkAll([]renamed{{p, from}}, agent)
}

// untitled says whether title is what a page without one is called, in any of the window's languages: a new page
// named for the first time is not a rename, and the links to "Untitled" are not its (#560).
func untitled(title string) bool {
	for _, l := range languages {
		if strings.EqualFold(strings.TrimSpace(title), locales.In(l.code).CommonUntitled()) {
			return true
		}
	}
	return false
}

// relinkAll is relinkRenamed for several pages renamed at once, as a journal's days are when its format changes:
// each page that links to them is written once, each link by the title it names, and one notice tells them all.
// What it settles the settings no longer keep as a rename to be ended (finishRenames).
func (a *app) relinkAll(rs []renamed, agent bool) int {
	for _, r := range rs {
		if a.settings.PendingRenames[r.p.Dir] != "" {
			delete(a.settings.PendingRenames, r.p.Dir)
			a.saveSettingsSoon()
		}
	}
	a.flush() // what is typed in the pages is on disk before they are read
	a.linksStale()
	a.linksTo(nil, a.clock(), true)
	to := map[string]string{} // the old titles, in lower case, and what they are now
	var pages []*Page
	listed := map[*Page]bool{}
	warn := ""
	for _, r := range rs {
		p := r.p
		from, now := strings.TrimSpace(r.from), strings.TrimSpace(p.Title)
		if !a.inLibrary(p) || strings.EqualFold(from, now) || untitled(from) || !library.LinkableTitle(from) || !library.LinkableTitle(now) {
			continue
		}
		key := strings.ToLower(from)
		if len(a.links.into[key]) == 0 {
			continue
		}
		if slices.ContainsFunc(a.titleIndex()[key], func(q *Page) bool { return q != p && strings.EqualFold(strings.TrimSpace(q.Title), from) }) {
			warn = locales.LinksKeptShared(from)
			continue
		}
		if slices.ContainsFunc(a.titleIndex()[strings.ToLower(now)], func(q *Page) bool { return q != p && strings.EqualFold(strings.TrimSpace(q.Title), now) }) {
			warn = locales.LinksKeptTaken(from, now)
			continue
		}
		to[key] = now
		for _, q := range a.links.into[key] {
			if !listed[q] {
				listed[q] = true
				pages = append(pages, q)
			}
		}
	}
	if len(to) == 0 {
		if warn != "" {
			a.say(toastWarn, warn)
		}
		return 0
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
		nb, n := relinkBody(body, to)
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
func (a *app) takeOutsideAll(p *Page) { a.takeOutside(p) }

// relinkBody is body with its links to the pages renamed written to their titles now (to: each old title, trimmed
// and in lower case, and the title it is now), and how many: the links and embeds, what they say instead of the
// title kept, and the [[links]] typed in a table's cells, in the text outside code, and in the held blocks whose
// text is read as text (a callout, a footnote, a table of Markdown), not in code, math or HTML (#560). It works on
// the body's own JSON, so nothing else of it changes; a body that cannot be read is given back as it is.
func relinkBody(body string, to map[string]string) (string, int) {
	var paras []bodyPara
	if json.Unmarshal([]byte(body), &paras) != nil {
		return body, 0
	}
	n := 0
	block := "" // the held block the paragraph is in (bodyMarkdown): "" in the text, "code", or a raw block's kind
	for i := range paras {
		st := paras[i].S
		switch {
		case st == nil || !st.CodeBlock:
			block = ""
		case strings.HasPrefix(st.ID, rawIDPrefix):
			block = strings.TrimPrefix(st.ID, rawIDPrefix)
		case strings.HasPrefix(st.ID, codeIDPrefix) || block == "":
			block = "code"
		}
		text := block == "" || block == "callout" || block == "footnote" || block == "table"
		for j := range paras[i].R {
			r := &paras[i].R[j]
			var k int
			switch o := r.O; {
			case o != nil && (o.K == "page" || o.K == "embed"):
				if now, ok := to[strings.ToLower(strings.TrimSpace(o.Target))]; ok {
					o.Target, k = now, 1
				} else if page, heading := splitHeading(strings.TrimSpace(o.Target)); heading != "" { // [[Page#Heading]]
					if now, ok := to[strings.ToLower(page)]; ok {
						o.Target, k = now+"#"+heading, 1
					}
				}
			case o != nil && o.K == "table":
				for _, row := range o.Rows {
					for c := range row {
						var m int
						row[c], m = library.RenameLinksTo(row[c], to)
						k += m
					}
				}
			case o == nil && text && !r.C:
				r.T, k = library.RenameLinksTo(r.T, to)
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
