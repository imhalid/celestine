package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// How the text is set and kept, as the settings say (Ayarlar ▸ Editör): the room between paragraphs,
// the first line's indent, when it is saved, and the white space shown.

const (
	defaultParaGap = 0.55 // ems between two paragraphs
	defaultSaveAt  = 0.7  // seconds from the last edit to the save
	minFont        = 12
	maxFont        = 28
)

// saveAfter is how long the text is still before it is saved, when it is saved by itself.
func (a *app) saveAfter() time.Duration {
	if s := a.settings.SaveAfter; s >= 0.2 && s <= 30 {
		return time.Duration(s * float64(time.Second))
	}
	return time.Duration(defaultSaveAt * float64(time.Second))
}

// paraGap is the room between two paragraphs, in ems.
func (a *app) paraGap() float32 {
	if g := a.settings.ParaGap; g != nil && *g >= 0 && *g <= 2 {
		return float32(*g)
	}
	return defaultParaGap
}

// setFontSize puts the text's size at s, as far as it can go, and keeps it.
func (a *app) setFontSize(s float64) {
	s = min(max(s, minFont), maxFont)
	a.settings.FontSize = s
	a.saveSettingsSoon()
}

// stepFont makes the text bigger by d.
func (a *app) stepFont(d float64) { a.setFontSize(float64(a.fontSize()) + d) }

// resetFont puts the text back to its size.
func (a *app) resetFont() {
	a.settings.FontSize = 0
	a.saveSettingsSoon()
}

// saveNow writes the page, whether it saves by itself or not.
func (a *app) saveNow() {
	if a.page != nil && a.doc != nil && a.dirty {
		a.save()
	}
}

// savedAt is the time of the last save, shown in the status bar.
func (a *app) savedText() string {
	switch {
	case a.saveErr != "":
		return locales.EditorCouldNotSave(a.saveErr)
	case a.dirty && a.settings.NoAutoSave:
		return locales.EditorNotSaved()
	case a.dirty:
		return locales.EditorSaving()
	case !a.savedAt.IsZero():
		return locales.EditorSavedAt(clockTime(a.savedAt))
	}
	return locales.EditorSaved()
}

// whiteMarks reports whether any white space is shown.
func (a *app) whiteMarks() bool {
	s := &a.settings
	return s.ShowSpaces || s.ShowTabs || s.ShowBreaks
}

// maxMarked is the longest paragraph that is marked: a line of a megabyte is not laid out for a dot.
const maxMarked = 4000

// drawWhite marks the white space of the paragraphs in view over the text: a dot for a space, an arrow
// for a tab, a pilcrow where a paragraph ends. They are thin and gray, so that the caret and the
// selection are not hidden; spaces at the end of a line are in the warning color.
func (a *app) drawWhite(p *ui.Painter, r ui.Rect) {
	if !a.whiteMarks() || a.doc == nil {
		return
	}
	text := a.doc.Text()
	cl := ryugo.Painting(p).Colors
	faint := cl.Muted.Alpha(0.55)
	warn := cl.Warn.Alpha(0.75)
	// only in the selection, when that is set: and nothing with none
	sa, sc := a.doc.Selection()
	selS, selE := min(sa, sc), max(sa, sc)
	onlySel := a.settings.WhiteInSel
	if onlySel && selS == selE {
		return
	}
	for _, b := range a.shown(r) {
		s, e := text.ParagraphRange(b.Start)
		if e-s > maxMarked || onlySel && (e < selS || s >= selE) {
			continue
		}
		var para richtext.Paragraph
		for _, pp := range text.Slice(s, e).Paragraphs() {
			para = pp
			break
		}
		para.Style = text.ParagraphStyleAt(s)
		if para.Style.CodeBlock && !a.settings.MarkCode {
			continue
		}
		var line strings.Builder
		for _, run := range para.Runs {
			line.WriteString(run.Text)
		}
		l, left := a.paraLayout(s, para, r.W)
		runes := []rune(line.String())
		pre := make([]int, len(runes)+1) // where each rune starts, in the layout's units: once, not once a rune (#198)
		for i, r := range runes {
			pre[i+1] = pre[i] + 1
			if r > 0xFFFF {
				pre[i+1]++
			}
		}
		end := len(runes)
		for end > 0 && (runes[end-1] == ' ' || runes[end-1] == '\t' || runes[end-1] == 0xA0) {
			end--
		}
		rectOf := func(i int) (ui.Rect, bool) {
			if i < 0 || i >= len(runes) {
				return ui.Rect{}, false
			}
			rects := l.SelectionRects(ui.TextInputRange{Start: pre[i], End: pre[i+1]})
			if len(rects) == 0 {
				return ui.Rect{}, false
			}
			rc := rects[0]
			return ui.Rect{X: r.X + left + rc.X, Y: r.Y + b.Top + rc.Y, W: rc.W, H: rc.H}, true
		}
		// at is where rune i is. A tab has no box of its own in the layout: it is the room between
		// the letter before it and the one after.
		at := func(i int) (ui.Rect, bool) {
			if rc, ok := rectOf(i); ok && rc.W > 0 {
				return rc, true
			}
			prev, okp := rectOf(i - 1)
			next, okn := rectOf(i + 1)
			switch {
			case okp && okn && next.Y == prev.Y && next.X > prev.X+prev.W:
				return ui.Rect{X: prev.X + prev.W, Y: prev.Y, W: next.X - prev.X - prev.W, H: prev.H}, true
			case okp:
				return ui.Rect{X: prev.X + prev.W, Y: prev.Y, W: a.font.Size * 1.5, H: prev.H}, true
			case okn:
				return ui.Rect{X: next.X - a.font.Size*1.5, Y: next.Y, W: a.font.Size * 1.5, H: next.H}, true
			}
			return ui.Rect{}, false
		}
		for i, ch := range runes {
			if onlySel && (s+i < selS || s+i >= selE) {
				continue
			}
			col := faint
			if i >= end {
				col = warn
			}
			switch {
			case (ch == ' ' || ch == 0xA0) && a.settings.ShowSpaces:
				if rc, ok := at(i); ok && rc.W > 0 {
					d := max(1.4, a.font.Size*0.09)
					if ch == 0xA0 { // a space that does not break: a ring
						p.Stroke(ui.Rect{X: rc.X + rc.W/2 - d, Y: rc.Y + rc.H*0.55 - d, W: 2 * d, H: 2 * d}, col, d, 1)
					} else {
						p.Fill(ui.Rect{X: rc.X + rc.W/2 - d/2, Y: rc.Y + rc.H*0.55 - d/2, W: d, H: d}, col, d/2)
					}
				}
			case ch == '\t' && a.settings.ShowTabs:
				if rc, ok := at(i); ok && rc.W > 2 {
					y := rc.Y + rc.H*0.55
					x0, x1 := rc.X+2, rc.X+rc.W-2
					arrow := &ui.Path{}
					arrow.MoveTo(x0, y).LineTo(x1, y).MoveTo(x1-3.5, y-3).LineTo(x1, y).LineTo(x1-3.5, y+3)
					p.StrokePath(arrow, 1, col)
				}
			}
		}
		if a.settings.ShowBreaks && e < text.Len() && (!onlySel || e >= selS && e < selE) { // not after the last one: nothing follows it
			if n := len(runes); n > 0 {
				if rc, ok := at(n - 1); ok {
					a.pilcrow(p, rc.X+rc.W+2, rc.Y, rc.H, faint)
				}
			} else if rc, ok := a.emptyLine(l, r, b, left); ok {
				a.pilcrow(p, rc.X+2, rc.Y, rc.H, faint)
			}
		}
	}
}

// emptyLine is the place of an empty paragraph: its first line.
func (a *app) emptyLine(l *ui.TextLayout, r ui.Rect, b richtext.ParagraphBox, left float32) (ui.Rect, bool) {
	h := a.font.Size * a.font.LineHeight
	return ui.Rect{X: r.X + left, Y: r.Y + b.Top, W: 0, H: h}, true
}

// pilcrow draws ¶ at x, in a line of height h that starts at y.
func (a *app) pilcrow(p *ui.Painter, x, y, h float32, col ui.Color) {
	s := a.font.Size * 0.7
	top, bot := y+h*0.5-s*0.5, y+h*0.5+s*0.5
	stem := x + s*0.55
	g := &ui.Path{}
	g.MoveTo(stem, top).LineTo(stem, bot)
	g.MoveTo(stem+s*0.32, top).LineTo(stem+s*0.32, bot)
	g.MoveTo(stem+s*0.32, top).LineTo(x+s*0.1, top)
	p.StrokePath(g, 1, col)
	bow := &ui.Path{}
	bow.MoveTo(stem, top).CubeTo(x-s*0.15, top, x-s*0.15, top+s*0.55, stem, top+s*0.55)
	p.StrokePath(bow, 1, col)
}

// settingsText is the editor's settings of how the text is set and kept: the room between paragraphs,
// when the page is saved, and the white space shown.
func (a *app) settingsText(c *ui.Context) {
	gap := float64(a.paraGap())
	settingGroup(c, "set-space",
		func() {
			settingRow(c, locales.CommonParagraphSpacing(), locales.EditorTheRoomBetweenTwo(), func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-gap"), &gap, 0, 1.5, ryugo.SliderOptions{Label: locales.CommonParagraphSpacing(), Step: 0.05, Format: func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) + " em" }})
				})
			})
		})
	if g := float32(gap); g != a.paraGap() {
		v := gap
		a.settings.ParaGap = &v
		a.saveSettingsSoon()
	}

	auto := !a.settings.NoAutoSave
	after := a.saveAfter().Seconds()
	settingGroup(c, "set-save",
		func() {
			settingRow(c, locales.CommonAutosave(), locales.EditorWhenOffPageSaved(), func() {
				ryugo.Switch(c.Key("set-autosave"), &auto, ryugo.SwitchOptions{}).Label(locales.CommonAutosave())
			})
		},
		func() {
			if !auto {
				ui.Text(c, locales.EditorSavingByHandUnsaved()).FontSize(ryugo.FsSm).TextColor(ryugo.From(c).Colors.Muted)
				return
			}
			settingRow(c, locales.CommonSaveDelay(), locales.EditorItSavedThisLong(), func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-after"), &after, 0.5, 10, ryugo.SliderOptions{Label: locales.CommonSaveDelay(), Step: 0.5, Format: func(v float64) string { return locales.DatesSeconds(strconv.FormatFloat(v, 'f', 1, 64)) }})
				})
			})
		})
	if auto != !a.settings.NoAutoSave {
		a.settings.NoAutoSave = !auto
		a.saveSettings()
	}
	if after != a.saveAfter().Seconds() {
		a.settings.SaveAfter = after
		a.saveSettingsSoon()
	}

	toggle := func(key, title, help string, v *bool) func() {
		return func() {
			on := *v
			settingRow(c, title, help, func() { ryugo.Switch(c.Key(key), &on, ryugo.SwitchOptions{}).Label(msg(title)) })
			if on != *v {
				*v = on
				a.saveSettings()
			}
		}
	}
	chars := float64(a.settings.LineChars)
	settingGroup(c, "set-linechars", func() {
		settingRow(c, locales.CommonLineLength(), locales.EditorTheTextColumnAt(), func() {
			ui.Box(c).Width(260).Children(func() {
				ryugo.Slider(c.Key("set-linechars-s"), &chars, 0, 120, ryugo.SliderOptions{Label: locales.CommonLineLength(), Step: 5, Format: func(v float64) string {
					if v < 20 {
						return locales.EditorAsWideAsPage()
					}
					return locales.EditorCharacters(int(v))
				}})
			})
		})
	})
	if v := int(chars); v != a.settings.LineChars && (v >= 20 || v == 0) {
		a.settings.LineChars = v
		a.saveSettingsSoon()
	} else if v > 0 && v < 20 && a.settings.LineChars != 0 {
		a.settings.LineChars = 0
		a.saveSettingsSoon()
	}
	focus := a.focusMode()
	typewriter := a.settings.Typewriter
	tabs := float64(a.settings.TabSpaces)
	sleep := a.caretSleep().Seconds()
	lite := a.settings.NoCaretFX
	ring := a.settings.AlwaysRing
	settingGroup(c, "set-focus",
		func() {
			settingRow(c, locales.CommonFocusMode(), locales.EditorLineOrParagraphOne(), func() {
				choiceBar(c, "set-focus-bar", locales.CommonFocusMode(), []string{"", "line", "paragraph"}, []string{locales.CommonOff(), locales.CommonLine(), locales.EditorParagraph()}, &focus)
			})
		},
		func() {
			settingRow(c, locales.CommonTypewriterScrolling(), locales.EditorTheLineYouWrite(), func() {
				ryugo.Switch(c.Key("set-typewriter"), &typewriter, ryugo.SwitchOptions{}).Label(locales.CommonTypewriterScrolling())
			})
		},
		func() {
			settingRow(c, locales.CommonTabWidth(), locales.EditorHowManySpacesTab(), func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-tabs"), &tabs, 0, 8, ryugo.SliderOptions{Label: locales.CommonTabWidth(), Step: 1, Format: func(v float64) string {
						if v == 0 {
							return locales.EditorTab()
						}
						return locales.EditorSpacesN(int(v))
					}})
				})
			})
		},
		func() {
			settingRow(c, locales.CommonCaretSleep(), locales.EditorTheCaretDozesOff(), func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-sleep"), &sleep, 1, 60, ryugo.SliderOptions{Label: locales.CommonCaretSleep(), Step: 1, Format: func(v float64) string { return locales.DatesSeconds(strconv.Itoa(int(v))) }})
				})
			})
		},
		func() {
			settingRow(c, locales.CommonAlwaysShowFocusRing(), locales.EditorTheRowChosenWith(), func() { ryugo.Switch(c.Key("set-ring"), &ring, ryugo.SwitchOptions{}).Label(locales.EditorFocusRing()) })
		},
		func() {
			settingRow(c, locales.CommonLightMode(), locales.EditorAPlainLineDrawn(), func() { ryugo.Switch(c.Key("set-lite"), &lite, ryugo.SwitchOptions{}).Label(locales.CommonLightMode()) })
		})
	if focus != a.focusMode() {
		a.settings.Focus = focus
		a.saveSettings()
	}
	if typewriter != a.settings.Typewriter {
		a.settings.Typewriter = typewriter
		a.saveSettings()
	}
	if int(tabs) != a.settings.TabSpaces {
		a.settings.TabSpaces = int(tabs)
		a.saveSettingsSoon()
	}
	if sleep != a.caretSleep().Seconds() {
		a.settings.SleepSecs = sleep
		a.saveSettingsSoon()
	}
	if ring != a.settings.AlwaysRing {
		a.settings.AlwaysRing = ring
		a.saveSettings()
	}
	if lite != a.settings.NoCaretFX {
		a.settings.NoCaretFX = lite
		a.saveSettings()
	}
	esc := a.settings.EscNormal
	settingGroup(c, "set-esc-normal", func() {
		settingRow(c, locales.SettingsEscNormal(), locales.SettingsEscNormalHelp(), func() {
			ryugo.Switch(c.Key("set-esc-normal-sw"), &esc, ryugo.SwitchOptions{}).Label(locales.SettingsEscNormal())
		})
	})
	if esc != a.settings.EscNormal {
		a.settings.EscNormal = esc
		a.saveSettings()
	}
	previews := !a.settings.NoLinkPreviews
	settingGroup(c, "set-link-previews", func() {
		settingRow(c, locales.SettingsLinkPreviews(), locales.SettingsLinkPreviewsHelp(), func() {
			ryugo.Switch(c.Key("set-link-previews-sw"), &previews, ryugo.SwitchOptions{}).Label(locales.SettingsLinkPreviews())
		})
	})
	if previews == a.settings.NoLinkPreviews {
		a.settings.NoLinkPreviews = !previews
		a.saveSettings()
	}
	if previews { // the site icons in the text, which only previews can have (#509)
		icons := !a.settings.NoLinkIcons
		settingGroup(c, "set-link-icons", func() {
			settingRow(c, locales.SettingsLinkIcons(), locales.SettingsLinkIconsHelp(), func() {
				ryugo.Switch(c.Key("set-link-icons-sw"), &icons, ryugo.SwitchOptions{}).Label(locales.SettingsLinkIcons())
			})
		})
		if icons == a.settings.NoLinkIcons {
			a.settings.NoLinkIcons = !icons
			a.saveSettings()
		}
	}
	vim := a.settings.VimKeys
	settingGroup(c, "set-vim", func() {
		settingRow(c, locales.CommonVimMotions(), locales.EditorAfterEscHJ(), func() {
			ryugo.Switch(c.Key("set-vim-sw"), &vim, ryugo.SwitchOptions{}).Label(locales.CommonVimMotions())
		})
	})
	if vim != a.settings.VimKeys {
		a.settings.VimKeys = vim
		a.saveSettings()
	}
	settingGroup(c, "set-white",
		toggle("set-sp", "showSpaces", "aSpaceIsADotA", &a.settings.ShowSpaces),
		toggle("set-tab", "showTabs", "aTabIsAnArrow", &a.settings.ShowTabs),
		toggle("set-eol", "showParagraphEnds", "aAtTheEndOfEach", &a.settings.ShowBreaks),
		toggle("set-code", "inCodeBlocksToo", "theInsideOfCodeBlocksIs", &a.settings.MarkCode),
	)
}

// charsWidth is how wide n characters of the text are, in the editor's font: the width of a line of n zeros.
func (a *app) charsWidth(n int) float32 {
	f := ui.Font{Family: a.font.Family, Size: a.fontSize(), Weight: a.fontWeight(), Features: a.fontFeatures()}
	if f.Family == "" {
		f.Family = sansStack
	}
	key := f.Family + "|" + strconv.FormatFloat(float64(f.Size), 'f', 1, 32) + "|" + strconv.Itoa(f.Weight) + "|" + strconv.Itoa(n)
	if w, ok := a.charW[key]; ok {
		return w
	}
	gs := ui.Shape(strings.Repeat("0", n), f)
	var w float32
	if len(gs) > 0 {
		last := gs[len(gs)-1]
		w = last.X + last.Advance
	}
	if a.charW == nil {
		a.charW = map[string]float32{}
	}
	a.charW[key] = w
	return w
}
