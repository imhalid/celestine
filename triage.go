package main

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Triage (#551): a collection's pages one at a time, as quick capture leaves them, each sorted with one key: M moves
// it to another collection (the collections then numbered, a digit chooses), T tags it (the tag typed, ↵), L links it
// from today's page of the journal, D (or ⌫) moves it to the trash; → or Space leaves it as it is, ← goes back, ↵ opens
// it, Esc ends. After each, the next page. It is how capture first, sort later works.

// triageDigits are the keys that choose the collections to move to, the first nine.
var triageDigits = []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}

// triageState is the triage while it is open: the collection, its pages as they were when it began, the one shown,
// and what a key started (the collections to move to, the tag being typed).
type triageState struct {
	open    bool
	coll    *Collection
	pages   []*Page
	at      int
	moving  bool
	tagging bool
	tag     string
	done    int
}

// openTriage starts the triage of collection c, from its oldest page.
func (a *app) openTriage(c *Collection) {
	if c == nil || c.Dynamic() || c.Journal {
		a.say(toastWarn, locales.TriageNotHere())
		return
	}
	a.flush()
	pages := append([]*Page(nil), c.Pages...)
	if len(pages) == 0 {
		a.say(toastInfo, locales.TriageEmpty())
		return
	}
	a.triage = triageState{open: true, coll: c, pages: pages}
}

// triageTargets are the collections a page can be moved to from the triage: the others with pages of their own.
func (a *app) triageTargets() []*Collection {
	var out []*Collection
	for _, c := range a.lib.Collections {
		if c != a.triage.coll && !c.Dynamic() && !c.Journal {
			out = append(out, c)
		}
	}
	return out
}

// triageNext goes on to the next page that is still in the collection, or ends the triage after the last.
func (a *app) triageNext(step int) {
	tr := &a.triage
	tr.moving, tr.tagging, tr.tag = false, false, ""
	for tr.at += step; tr.at >= 0 && tr.at < len(tr.pages); tr.at += step {
		if p := tr.pages[tr.at]; a.inLibrary(p) && p.Coll == tr.coll {
			return
		}
	}
	if step < 0 {
		tr.at = 0
		return
	}
	a.say(toastOK, locales.TriageDone(tr.done))
	*tr = triageState{}
}

// triageAct does an action to the page shown, and goes on to the next.
func (a *app) triageAct(p *Page, act func() error) {
	if err := act(); err != nil {
		a.fail(err)
		return
	}
	a.triage.done++
	a.linksStale()
	a.triageNext(1)
}

// triageTag adds #tag at the end of page p.
func (a *app) triageTag(p *Page, tag string) error {
	tag = strings.Join(strings.Fields(strings.TrimPrefix(strings.TrimSpace(tag), "#")), "-")
	if tag == "" {
		return nil
	}
	body, err := p.Body()
	if err == nil {
		err = bodyReadable(body)
	}
	if err != nil {
		return err
	}
	if err := p.Save(appendLine(body, "#"+tag)); err != nil {
		return err
	}
	a.takeOutsideAll(p)
	return nil
}

// triageView is the triage's dialog, while it is open.
func (a *app) triageView(c *ui.Context) {
	tr := &a.triage
	if tr.coll == nil {
		return
	}
	if tr.at >= len(tr.pages) || !a.inLibrary(tr.pages[tr.at]) || tr.pages[tr.at].Coll != tr.coll {
		a.triageNext(1)
		if tr.coll == nil {
			return
		}
	}
	p := tr.pages[tr.at]
	cl := ryugo.From(c).Colors
	keys := !tr.tagging // a tag being typed takes the letters
	key := func(k ui.Key) bool { return keys && c.Shortcut(0, k) }
	ryugo.Dialog(c.Key("triage"), &tr.open, ryugo.DialogOptions{Title: locales.TriageTitle(tr.coll.Name, tr.at+1, len(tr.pages)), Width: 560}, func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.Text(c, p.Title).FontSize(20).FontWeight(680).TextColor(cl.Text).Wrap()
			preview := strings.TrimSpace(snippetText(p))
			if r := []rune(preview); len(r) > 600 {
				preview = string(r[:600]) + "…"
			}
			if preview == "" {
				preview = locales.TriageNoText()
			}
			ui.Box(c).FillWidth().MaxHeight(220).Clip().Padding(10, 12).Radius(rad(8)).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
				ui.Text(c, preview).FontSize(13).TextColor(cl.Text2).Wrap()
			})
			switch {
			case tr.moving:
				ui.Text(c, locales.TriageMoveTo()).FontSize(12).FontWeight(650).TextColor(cl.Muted)
				ui.Row(c).FillWidth().Gap(6).Wrap().Children(func() {
					for i, to := range a.triageTargets() {
						to := to
						label := to.Name
						if i < 9 {
							label = strconv.Itoa(i+1) + "  " + label
						}
						if ryugo.Button(c.Key("tr-to:"+to.Dir), label, ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}).Clicked() || i < 9 && key(triageDigits[i]) {
							a.triageAct(p, func() error { return a.lib.MovePage(p, to, nil, len(to.Pages)) })
							return
						}
					}
				})
			case tr.tagging:
				in := a.fieldMenu(ryugo.Input(c.Key("tr-tag"), &tr.tag, ryugo.InputOptions{Label: locales.TriageTag(), Placeholder: locales.TriageTagPlaceholder(), PrefixIcon: iconTag}), &tr.tag, fieldOpts{oneLine: true})
				in.AutoFocus()
				if in.Submitted() {
					tag := tr.tag
					a.triageAct(p, func() error { return a.triageTag(p, tag) })
					return
				}
			}
			ui.Row(c).FillWidth().Gap(6).Wrap().Children(func() {
				btn := func(id, label, hint string, glyph *ui.SVG, k ui.Key, run func()) {
					if ryugo.Button(c.Key("tr-"+id), label+"  "+hint, ryugo.ButtonOptions{Icon: glyph, Size: ryugo.Sm, Variant: ryugo.Soft}).Clicked() || key(k) {
						a.later(run)
					}
				}
				btn("move", locales.TriageMove(), "M", iconArrowRight, ui.KeyM, func() { tr.moving, tr.tagging = !tr.moving, false })
				btn("tag", locales.TriageTag(), "T", iconTag, ui.KeyT, func() { tr.tagging, tr.moving = true, false })
				btn("today", locales.TriageLinkToday(), "L", iconCalendar, ui.KeyL, func() {
					a.triageAct(p, func() error { _, err := a.captureNote(quickDay, "[["+p.Title+"]]"); return err })
				})
				btn("trash", locales.TriageTrash(), "D", iconTrash, ui.KeyD, func() { a.triageAct(p, func() error { a.trashPage(p); return nil }) })
				ui.Spacer(c)
				btn("back", locales.TriageBack(), "←", iconChevronBack, ui.KeyLeft, func() { a.triageNext(-1) })
				btn("skip", locales.TriageSkip(), "→", iconChevron, ui.KeyRight, func() { a.triageNext(1) })
			})
			if key(ui.KeyBackspace) {
				a.later(func() { a.triageAct(p, func() error { a.trashPage(p); return nil }) })
			}
			if key(ui.KeySpace) {
				a.later(func() { a.triageNext(1) })
			}
			if key(ui.KeyEnter) {
				a.later(func() { *tr = triageState{}; a.open(p) })
			}
		})
	})
	if !tr.open { // Esc, the cross or the scrim
		*tr = triageState{}
	}
}
