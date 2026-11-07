package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
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

// firstIndent is how far the first line of a paragraph is set in, in ems.
func (a *app) firstIndent() float32 {
	if g := a.settings.Indent; g > 0 && g <= 4 {
		return float32(g)
	}
	return 0
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
		return "kaydedilemedi: " + a.saveErr
	case a.dirty && a.settings.NoAutoSave:
		return T("kaydedilmedi")
	case a.dirty:
		return T("kaydediliyor…")
	case !a.savedAt.IsZero():
		return T("kaydedildi") + " " + a.savedAt.Format("15:04")
	}
	return T("kaydedildi")
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
	for _, b := range a.shown(r) {
		s, e := text.ParagraphRange(b.Start)
		if e-s > maxMarked {
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
		l, left := layoutParagraph(para, r.W, a.font, a.theme)
		runes := []rune(line.String())
		end := len(runes)
		for end > 0 && (runes[end-1] == ' ' || runes[end-1] == '\t' || runes[end-1] == 0xA0) {
			end--
		}
		rectOf := func(i int) (ui.Rect, bool) {
			if i < 0 || i >= len(runes) {
				return ui.Rect{}, false
			}
			rects := l.SelectionRects(ui.TextInputRange{Start: units(line.String(), i), End: units(line.String(), i+1)})
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
		if a.settings.ShowBreaks && e < text.Len() { // not after the last one: nothing follows it
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
// the first line's indent, when the page is saved, and the white space shown.
func (a *app) settingsText(c *ui.Context) {
	gap := float64(a.paraGap())
	ind := float64(a.firstIndent())
	settingGroup(c, "set-space",
		func() {
			settingRow(c, "Paragraf aralığı", "İki paragraf arasındaki boşluk, yazı boyutunun katı olarak.", func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-gap"), &gap, 0, 1.5, ryugo.SliderOptions{Label: "Paragraf aralığı", Step: 0.05, Format: func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) + " em" }})
				})
			})
		},
		func() {
			settingRow(c, "İlk satır girintisi", "Basılı yazıdaki gibi paragrafın ilk satırı içeriden başlar; başlık, liste ve alıntıdan sonrakine uygulanmaz.", func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-indent"), &ind, 0, 3, ryugo.SliderOptions{Label: "İlk satır girintisi", Step: 0.25, Format: func(v float64) string {
						if v == 0 {
							return "yok"
						}
						return strconv.FormatFloat(v, 'f', 2, 64) + " em"
					}})
				})
			})
		})
	if g := float32(gap); g != a.paraGap() {
		v := gap
		a.settings.ParaGap = &v
		a.saveSettingsSoon()
	}
	if float32(ind) != a.firstIndent() {
		a.settings.Indent = ind
		a.saveSettingsSoon()
	}

	auto := !a.settings.NoAutoSave
	after := a.saveAfter().Seconds()
	settingGroup(c, "set-save",
		func() {
			settingRow(c, "Otomatik kayıt", "Kapalıyken sayfa ⌘S ile kaydedilir; sayfa değişirken ve uygulama kapanırken yazılan yine kaydedilir, hiçbir şey kaybolmaz.", func() {
				ryugo.Switch(c.Key("set-autosave"), &auto, ryugo.SwitchOptions{}).Label(T("Otomatik kayıt"))
			})
		},
		func() {
			if !auto {
				ui.Text(c, T("Kayıt elle: kaydedilmemiş sayfada durum çubuğu “kaydedilmedi” der.")).FontSize(ryugo.FsSm).TextColor(ryugo.From(c).Colors.Muted)
				return
			}
			settingRow(c, "Kayıt gecikmesi", "Son yazıdan bu kadar sonra kaydedilir.", func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-after"), &after, 0.5, 10, ryugo.SliderOptions{Label: "Kayıt gecikmesi", Step: 0.5, Format: func(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) + " sn" }})
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
			settingRow(c, title, help, func() { ryugo.Switch(c.Key(key), &on, ryugo.SwitchOptions{}).Label(title) })
			if on != *v {
				*v = on
				a.saveSettings()
			}
		}
	}
	chars := float64(a.settings.LineChars)
	settingGroup(c, "set-linechars", func() {
		settingRow(c, "Satır uzunluğu", "Metin sütunu en çok bu kadar karakter genişliğinde olur (okunaklı satır 60–80 karakterdir); 0 sayfa genişliği kadar.", func() {
			ui.Box(c).Width(260).Children(func() {
				ryugo.Slider(c.Key("set-linechars-s"), &chars, 0, 120, ryugo.SliderOptions{Label: "Satır uzunluğu", Step: 5, Format: func(v float64) string {
					if v < 20 {
						return "sayfa kadar"
					}
					return strconv.Itoa(int(v)) + " karakter"
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
			settingRow(c, "Odak modu", "Satır ya da paragraf: caret'in bulunduğu kalır, sayfanın geri kalanı silikleşir.", func() {
				choiceBar(c, "set-focus-bar", T("Odak modu"), []string{"", "line", "paragraph"}, []string{T("Kapalı"), T("Satır"), T("Paragraf")}, &focus)
			})
		},
		func() {
			settingRow(c, "Daktilo kaydırması", "Yazdığın satır pencerenin ortasında kalır, sayfa altından kayar.", func() {
				ryugo.Switch(c.Key("set-typewriter"), &typewriter, ryugo.SwitchOptions{}).Label(T("Daktilo kaydırması"))
			})
		},
		func() {
			settingRow(c, "Tab uzunluğu", "Tab'ın yazdığı boşluk sayısı; 0 gerçek bir sekme karakteridir.", func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-tabs"), &tabs, 0, 8, ryugo.SliderOptions{Label: "Tab uzunluğu", Step: 1, Format: func(v float64) string {
						if v == 0 {
							return "sekme"
						}
						return strconv.Itoa(int(v)) + " boşluk"
					}})
				})
			})
		},
		func() {
			settingRow(c, "Caret uykusu", "Yazmadan bu kadar durunca caret uyur.", func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-sleep"), &sleep, 1, 60, ryugo.SliderOptions{Label: "Caret uykusu", Step: 1, Format: func(v float64) string { return strconv.Itoa(int(v)) + " sn" }})
				})
			})
		},
		func() {
			settingRow(c, "Odak halkasını her zaman göster", "Klavyeyle seçilen satır, normal kipte olmasan da çerçevelenir.", func() { ryugo.Switch(c.Key("set-ring"), &ring, ryugo.SwitchOptions{}).Label(T("Odak halkası")) })
		},
		func() {
			settingRow(c, "Hafif mod", "Seçili caret ne olursa olsun efektsiz, düz bir çizgi çizilir.", func() { ryugo.Switch(c.Key("set-lite"), &lite, ryugo.SwitchOptions{}).Label("Hafif mod") })
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
	vim := a.settings.VimKeys
	settingGroup(c, "set-vim", func() {
		settingRow(c, "Vim hareketleri", "Esc ile normal kipe geçince metinde h j k l, w b e, 0 $, gg G, x, dd, yy, p, o O, A I çalışır; i ile yazmaya dönülür. Kapalıyken normal kipte yalnız listeler, harf etiketleri (f), leader (Space) ve komutlar vardır.", func() {
			ryugo.Switch(c.Key("set-vim-sw"), &vim, ryugo.SwitchOptions{}).Label("Vim hareketleri")
		})
	})
	if vim != a.settings.VimKeys {
		a.settings.VimKeys = vim
		a.saveSettings()
	}
	settingGroup(c, "set-white",
		toggle("set-sp", "Boşlukları göster", "Boşluk bir nokta, kırılmaz boşluk bir halka; satır sonundaki artık boşluklar uyarı renginde.", &a.settings.ShowSpaces),
		toggle("set-tab", "Sekmeleri göster", "Sekme bir ok.", &a.settings.ShowTabs),
		toggle("set-eol", "Paragraf sonlarını göster", "Her paragrafın sonunda ¶.", &a.settings.ShowBreaks),
		toggle("set-code", "Kod bloklarında da", "Kod bloklarının içi de işaretlenir.", &a.settings.MarkCode),
	)
}

// charsWidth is how wide n characters of the text are, in the editor's font: the width of a line of n zeros.
func (a *app) charsWidth(n int) float32 {
	f := ui.Font{Family: a.font.Family, Size: a.fontSize(), Weight: a.fontWeight(), Features: a.fontFeatures()}
	if f.Family == "" {
		f.Family = ryugo.FontSans
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
