package main

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/internal/textnorm"
	"github.com/imhalid/celestine/mobile/locales"
)

// Today's Living Stationery Sheet: a Swiss-Japanese pocket-almanac layout for one single day. A whisper-thin vertical
// notebook margin rule runs edge-to-edge on the left, the atmospheric hour ("Night · 00:52") sits at the top, a bold
// sculptural day numeral balances the right-aligned weekday & month above a flat 1.5px daylight hairline, and pure open
// grained paper fills everything below.

const saveQuiet = 800 * time.Millisecond

type editorState struct {
	id           string             // Today's page ID once created on disk; "" while still a fresh draft
	isTodayDraft bool               // true until first content is flushed to Today's journal page
	doc          *richtext.Document // live rich-text document
	readOnly     string             // non-empty only if written by a newer version of Celestine
	dirty        bool               // unsaved edits exist
	saveAt       time.Time          // debounced autosave deadline
	writing      bool               // body editor currently focused and editable
	keyboard     bool               // keyboard was open on the previous frame (#656)
}

func (a *app) todayDraftState() editorState {
	return editorState{
		isTodayDraft: true,
		doc:          richtext.NewDocument(richtext.Text{}),
	}
}

func (a *app) stateOf(p *library.Page) editorState {
	ed := editorState{id: p.ID}
	body, err := p.Body()
	switch {
	case err != nil:
		ed.readOnly = err.Error()
	case p.Unreadable != "" || library.BodyReadable(body) != nil:
		ed.readOnly = locales.PageUnreadable()
	case p.Locked != "":
		ed.readOnly = locales.PageLocked()
	}
	text := richtext.Text{}
	if d, err := library.DecodeBody(body); err == nil {
		text = textOf(d)
	}
	ed.doc = richtext.NewDocument(text)
	return ed
}

func docHasContent(d library.Doc) bool {
	for _, para := range d {
		if para.S != nil && para.S.List == library.ListTasks {
			return true
		}
		for _, r := range para.R {
			if strings.TrimSpace(r.T) != "" || r.O != nil {
				return true
			}
		}
	}
	return false
}

// flush persists any pending edits on Today's paper. Empty visits never create ghost files before sync.
func (a *app) flush() {
	if !a.ed.dirty || a.ed.readOnly != "" || a.ed.doc == nil {
		return
	}
	a.ed.dirty = false
	doc := docOf(a.ed.doc.Text())

	var p *library.Page
	if a.ed.isTodayDraft {
		if !docHasContent(doc) {
			return
		}
		var err error
		p = a.dayPage(a.activeDay)
		if p == nil {
			day := a.activeDay
			if day.IsZero() {
				day = a.now()
			}
			p, err = a.makeDay(day)
		}
		if err != nil {
			a.say(locales.PageSaveFailed(err.Error()))
			return
		}
		a.ed.id = p.ID
		a.ed.isTodayDraft = false
		a.activePageID = p.ID
	} else {
		if a.ed.id == "" {
			return
		}
		p = a.lib.PageByID(a.ed.id)
		if p == nil {
			return
		}
	}

	if err := p.Save(library.EncodeBody(doc)); err != nil {
		a.say(locales.PageSaveFailed(err.Error()))
		return
	}
	a.syncSoon()
}

func (a *app) typed(now time.Time) {
	a.ed.dirty, a.ed.saveAt = true, now.Add(saveQuiet)
	time.AfterFunc(saveQuiet+50*time.Millisecond, func() {
		a.update(func() {})
	})
}

// dayProgress returns how much of the current day has elapsed in [0.02, 0.98].
func dayProgress(now time.Time) float32 {
	t := now.Local()
	secs := float32(t.Hour()*3600 + t.Minute()*60 + t.Second())
	return min(max(secs/86400.0, 0.02), 0.98)
}

// countTodayTasks returns (done, total) task paragraphs in the active Today document.
func countTodayTasks(doc *richtext.Document) (done, total int) {
	if doc == nil {
		return 0, 0
	}
	for _, para := range doc.Text().Paragraphs() {
		if para.Style.List != richtext.Tasks {
			continue
		}
		hasText := false
		for _, r := range para.Runs {
			if strings.TrimSpace(r.Text) != "" {
				hasText = true
				break
			}
		}
		if !hasText {
			continue
		}
		total++
		if para.Style.Checked {
			done++
		}
	}
	return done, total
}

func (a *app) todayView(c *ui.Context) {
	if a.ed.dirty && !c.Now().Before(a.ed.saveAt) {
		a.flush()
	}
	ed := &a.ed
	if ed.doc == nil {
		return
	}
	now := a.now()
	day := a.activeDay
	if day.IsZero() {
		day = startOfDay(now)
	}
	words := wordCount(ed.doc.Text().PlainText())
	doneTasks, totalTasks := countTodayTasks(ed.doc)
	top, _, bottom, _ := c.SafeArea()
	skyInset = top

	const (
		spineX = 16 // delicate notebook left margin rule
		padL   = 28 // editorial content left edge (cleanly past spine)
		padR   = 24 // right margin
	)

	ui.Box(c.Key("today-paper")).Fill().Children(func() {
		// The sky, full-bleed behind all content (ZIndex -2): its colors, lights and weather (skydraw.go).
		ui.Box(c.Key("paper-sky")).Absolute().Top(-top).Bottom(-bottom).Left(0).Right(0).ZIndex(-2).
			Draw(drawSky).
			HitTest(func(x, y float32, r ui.Rect) bool { return false })

		// Architectural vertical stationery spine rule running edge-to-edge down the left margin behind content (ZIndex -1).
		ui.Box(c.Key("paper-spine")).Absolute().Top(-top).Bottom(-bottom).Left(spineX).Width(1).ZIndex(-1).
			Background(pal.accent.Alpha(0.22)).
			HitTest(func(x, y float32, r ui.Rect) bool { return false })

		ui.Scroll(c.Key("today-scroll")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Children(func() {
			ui.Column(c).FillWidth().Padding(14, padR, 96, padL).Children(func() {
				// Top atmospheric whisper strip: solar phase + clock + live weather on left, task tally & sync on right.
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Padding(0, 0, 14, 0).Children(func() {
					phase := strings.ToUpper(dayPhaseName(now, a.wx.info)) + "  ·  " + now.Local().Format("15:04")
					if wl := a.wx.info.label(); wl != "" {
						phase += "  ·  " + strings.ToUpper(wl)
					}
					ui.Text(c, phase).FontSize(11).FontWeight(700).TextColor(pal.faint).MaxLines(1).Grow(1).MinWidth(0)

					if totalTasks > 0 {
						ui.Row(c.Key("task-tally")).Height(24).Padding(0, 8).Radius(12).
							Background(pal.accentSoft).Gap(4).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, iconTasks).Size(12, 12).TextColor(pal.accent)
							ui.Text(c, strconv.Itoa(doneTasks)+"/"+strconv.Itoa(totalTasks)).
								FontSize(11).FontWeight(700).TextColor(pal.accent)
						})
					}

					dotTint := pal.faint
					if a.sy.busy {
						dotTint = pal.accent
					} else if a.sy.err != "" {
						dotTint = pal.danger
					}
					trigger := pressable(ui.Box(c.Key("today-settings")).Size(32, 32).Radius(16).
						AlignItems(ui.Center).Justify(ui.Center).
						Role(ui.RoleButton).Label(locales.SettingsSync()))
					if trigger.Pressed() {
						trigger.Background(pal.surface)
					}
					trigger.Children(func() {
						ui.Icon(c, iconCloud).Size(17, 17).TextColor(dotTint)
					})
					if trigger.Clicked() {
						mygo.Haptics.Selection()
						a.sheet = sheetSettings
					}
				})

				// Swiss-Japanese Pocket Almanac Masthead: oversized sculptural day numeral left, editorial date right.
				ui.Row(c).FillWidth().AlignItems(ui.End).Justify(ui.SpaceBetween).Children(func() {
					ui.Text(c, strconv.Itoa(day.Day())).
						FontSize(58).FontWeight(800).LineHeight(0.88).TextColor(pal.text)

					ui.Column(c).AlignItems(ui.End).Gap(2).Padding(0, 0, 3, 0).Children(func() {
						ui.Text(c, formatDay("dddd", day)).
							FontSize(17).FontWeight(700).LineHeight(1.1).TextColor(pal.text)
						ui.Text(c, formatDay("MMMM YYYY", day)).
							FontSize(13).FontWeight(550).TextColor(pal.muted)
						if words > 0 {
							ui.Text(c, locales.PageWords(words)).
								FontSize(11).FontWeight(600).TextColor(pal.faint).Padding(2, 0, 0, 0)
						}
					})
				})

				// Clean flat 1.5px daylight hairline (zero slider knob): quietly traces today from midnight to midnight.
				frac := dayProgress(now)
				ui.Row(c.Key("day-line")).FillWidth().Height(1.5).Margin(18, 0, 24, 0).Children(func() {
					ui.Box(c).Grow(frac).Basis(0).Height(1.5).Radius(1).Background(pal.accent.Alpha(0.6))
					ui.Box(c).Grow(1 - frac).Basis(0).Height(1.5).Radius(1).Background(pal.line)
				})

				if ed.readOnly != "" {
					ui.Row(c.Key("readonly-banner")).FillWidth().Padding(10, 14).Radius(14).
						Background(pal.surface).Gap(10).AlignItems(ui.Center).Margin(0, 0, 14, 0).Children(func() {
						ui.Icon(c, iconLock).Size(15, 15).TextColor(pal.muted).Shrink(0)
						ui.Text(c, ed.readOnly).FontSize(13).LineHeight(1.35).TextColor(pal.muted).Wrap().Grow(1).MinWidth(0)
					})
				}

				// Full-height rich-text paper editor with Android finger-scroll and 1-tap checkbox layer (#654, #655).
				_, winH := c.Size()
				room := max(winH*0.60, 380)
				if h, ok := ed.doc.ContentHeight(); ok {
					room = max(room, h+160)
				}
				edKey := "body:" + day.Format("2006-01-02")

				var body ui.Element
				ui.Box(c.Key("body-box")).FillWidth().Height(room).Children(func() {
					body = richtext.EditorBase(c.Key(edKey), ed.doc, richtext.EditorOptions{
						Label:       locales.PageToday(),
						Placeholder: locales.PagePlaceholder(),
						ReadOnly:    ed.readOnly != "",
						Font:        ui.Font{Family: fontFamily, Size: 18.5, LineHeight: 1.68},
						Theme:       textTheme(),
					}).Fill()
					a.touchLayer(c, ed, body)
				})
				if body.Changed() {
					a.typed(c.Now())
				}
				ed.writing = body.Focused() && ed.readOnly == ""
			})
		})
	})

	sink := a.letGo(c)
	if ed.writing && a.sheet == sheetNone {
		a.writingBar(c, sink)
	}
}

// writingBar is a floating rounded pen-tray pill resting cleanly above the keyboard while editing: Task, Heading,
// Bold, and Done. All controls KeepFocus() so the keyboard never flickers.
func (a *app) writingBar(c *ui.Context, sink ui.Element) {
	ed := &a.ed
	if ed.doc == nil {
		return
	}
	fmtState := ed.doc.Formatting()
	isTask := fmtState.Paragraph.List == richtext.Tasks
	isHeading := fmtState.Paragraph.Heading > 0
	isBold := fmtState.Attributes.Weight >= 600

	ui.Overlay(c, func() {
		_, _, bottom, _ := c.SafeArea()
		bar := ui.Row(c.Key("writing-bar")).Absolute().Left(16).Right(16).Bottom(10+bottom).Height(48).
			Padding(0, 8).Gap(6).Radius(24).AlignItems(ui.Center).
			Background(pal.sheet).Border(1, pal.line)
		shadow(bar)
		bar.Children(func() {
			if a.formatChip(c, "fmt-task", iconTasks, locales.PageTask(), isTask) {
				ed.doc.ToggleList(richtext.Tasks)
				a.typed(c.Now())
				mygo.Haptics.Selection()
			}
			if a.formatChip(c, "fmt-head", iconHeading, locales.PageHeading(), isHeading) {
				if isHeading {
					ed.doc.SetHeading(0)
				} else {
					ed.doc.SetHeading(2)
				}
				a.typed(c.Now())
				mygo.Haptics.Selection()
			}
			if a.formatChip(c, "fmt-bold", iconBold, locales.PageBold(), isBold) {
				ed.doc.ToggleBold()
				a.typed(c.Now())
				mygo.Haptics.Selection()
			}
			ui.Box(c).Grow(1)
			done := pressable(ui.Row(c.Key("fmt-done")).Height(34).Padding(0, 14).Radius(17).
				Background(pal.accentSoft).AlignItems(ui.Center).
				Role(ui.RoleButton).Label(locales.PageDone()).KeepFocus())
			done.Children(func() {
				ui.Text(c, locales.PageDone()).FontSize(13).FontWeight(700).TextColor(pal.accent)
			})
			if done.Clicked() {
				a.flush()
				sink.Focus(ui.FocusOptions{PreventScroll: true})
				ed.writing = false
			}
		})
	})
}

func (a *app) formatChip(c *ui.Context, key string, glyph *ui.SVG, label string, active bool) bool {
	bg, fg := pal.surface, pal.muted
	if active {
		bg, fg = pal.accentSoft, pal.accent
	}
	b := pressable(ui.Box(c.Key(key)).Size(36, 34).Radius(17).
		AlignItems(ui.Center).Justify(ui.Center).Background(bg).
		Role(ui.RoleCheckBox).Checked(active).Label(label).KeepFocus())
	b.Children(func() {
		ui.Icon(c, glyph).Size(18, 18).TextColor(fg)
	})
	return b.Clicked()
}

// letGo drops editor focus cleanly when Android dismisses the keyboard (#656) and returns the focus sink for Done.
func (a *app) letGo(c *ui.Context) ui.Element {
	ed := &a.ed
	_, _, bottom, _ := c.SafeArea()
	up := bottom > 150
	sink := ui.Box(c.Key("focus-sink")).Size(0, 0).Focusable().Label(locales.PageToday())
	if ed.keyboard && !up {
		sink.Focus(ui.FocusOptions{PreventScroll: true})
		ed.writing = false
	}
	ed.keyboard = up
	return sink
}

// touchLayer lets finger drags scroll Today's paper smoothly while taps place the caret or directly tick task boxes.
func (a *app) touchLayer(c *ui.Context, ed *editorState, body ui.Element) {
	l := ui.Box(c.Key("touch")).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(ui.Color{}).KeepFocus()
	if !l.Clicked() {
		return
	}
	x, y, _ := l.PointerPosition()
	a.tapText(ed, body, x, y)
}

func (a *app) tapText(ed *editorState, body ui.Element, x, y float32) {
	if ed.doc == nil || ed.readOnly != "" {
		return
	}
	doc := ed.doc
	text := doc.Text()
	boxes := doc.VisibleParagraphs()
	if len(boxes) == 0 {
		body.Focus()
		return
	}
	at, gap := -1, float32(math.MaxFloat32)
	for i, b := range boxes {
		d := float32(0)
		if y < b.Top {
			d = b.Top - y
		} else if y >= b.Top+b.Height {
			d = y - (b.Top + b.Height)
		}
		if d < gap {
			at, gap = i, d
		}
	}
	if last := boxes[len(boxes)-1]; y >= last.Top+last.Height+12 {
		at = -1
	}
	caret := func(i int) ui.Rect {
		doc.SetSelection(i, i)
		r, _ := doc.CaretRect()
		return r
	}
	if at < 0 {
		end := text.Len()
		doc.SetSelection(end, end)
		body.Focus()
		return
	}
	start, end := text.ParagraphRange(boxes[at].Start)
	if text.ParagraphStyleAt(start).List == richtext.Tasks && x < caret(start).X-2 {
		doc.ToggleChecked(start)
		a.typed(a.now())
		mygo.Haptics.Light()
		if !body.Focused() {
			doc.SetSelection(start, start)
		}
		return
	}
	best := start
	for i := start; i <= end; i++ {
		r := caret(i)
		if y < r.Y || y >= r.Y+r.H {
			if r.Y > y {
				break
			}
			continue
		}
		if r.X <= x+4 {
			best = i
		}
	}
	doc.SetSelection(best, best)
	body.Focus()
}

// Helpers shared with sync.go and sync_test.go.

func wordCount(text string) int { return len(strings.Fields(text)) }

func (a *app) allPages() []*library.Page {
	var out []*library.Page
	for _, c := range a.lib.Collections {
		if !c.Dynamic() {
			out = append(out, c.All()...)
		}
	}
	return out
}

func plainOf(p *library.Page) string {
	md, _ := p.Text()
	return strings.TrimSpace(textnorm.Plain(md))
}

func ago(t time.Time) string {
	now := time.Now()
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return locales.EditedNow()
	case d < time.Hour:
		return locales.EditedMinutes(int(d / time.Minute))
	case d < 24*time.Hour:
		return locales.EditedHours(int(d / time.Hour))
	case startOfDay(t).Equal(startOfDay(now).AddDate(0, 0, -1)):
		return locales.EditedYesterday()
	}
	return locales.EditedDays(int(startOfDay(now).Sub(startOfDay(t)) / (24 * time.Hour)))
}
