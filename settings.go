package main

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The settings: a screen of the window (⌘, opens it, and goes back), its sections listed at the left
// and the one chosen at the right, in groups. The section chosen glows in the list and the choices
// that have a look - the theme, the caret - are tiles that lift as the pointer comes.

type settingsSection struct {
	name, blurb string
	glyph       *ui.SVG
}

var (
	iconTheme   = icon(`<circle cx="12" cy="12" r="7.5"/><path d="M12 4.5v15a7.5 7.5 0 0 0 0-15Z" fill="currentColor"/>`)
	iconEditing = icon(`<path d="M5 6.5h14M5 11h9M5 15.5h12M5 20h6"/>`)
	iconBar     = icon(`<rect x="4" y="15" width="16" height="4.5" rx="1.8"/><path d="M7.5 17.2h3M14 17.2h2.5"/><path d="M4 8.5h16M4 5h10" opacity=".55"/>`)
	iconPlug    = icon(`<path d="M9 4v5M15 4v5M6.5 9h11v2.5a5.5 5.5 0 0 1-11 0zM12 17v3.5"/>`)
	iconSync    = icon(`<path d="M5 9a7 7 0 0 1 12-3l2 2M19 15a7 7 0 0 1-12 3l-2-2M19 4v4h-4M5 20v-4h4"/>`)
	iconUpdate  = icon(`<path d="M12 4v10m0 0-4-4m4 4 4-4M5 19h14"/>`)
	iconRestart = icon(`<path d="M19.5 12a7.5 7.5 0 1 1-2.2-5.3L19.5 9M19.5 4.5V9H15"/>`)
	iconFont    = icon(`<path d="M5 19 11 5h2l6 14M7.5 14h9"/>`)
	iconLibrary = icon(`<path d="M4.5 7.5a1.5 1.5 0 0 1 1.5-1.5h4l1.8 2H18a1.5 1.5 0 0 1 1.5 1.5v7.5a1.5 1.5 0 0 1-1.5 1.5H6a1.5 1.5 0 0 1-1.5-1.5z"/>`)
)

// the tabs of the settings, by their place in settingsSections
const (
	tabAppearance = iota
	tabEditor
	tabFonts
	tabStatus
	tabMCP
	tabLibrary
	tabSync
	tabUpdate
	tabSounds
)

var settingsSections = []settingsSection{
	{"common_appearance", "themeAndThePagesWidth", iconTheme},
	{"editor", "textLineSpacingAndCaret", iconEditing},
	{"typeface", "theTypefacesOfTheAppThe", iconFont},
	{"statusBar", "whatIsShownAtTheBottom", iconBar},
	{"mcp", "agentsConnectingToTheLibrary", iconPlug},
	{"common_library", "theFilesAndWhereTheyAre", iconLibrary},
	{"sync4", "matchingOtherComputersEncryption", iconSync},
	{"updates", "newVersionsAndReleaseNotes", iconUpdate},
	{"common_sounds", "sounds_blurb", iconSound},
}

// selSpring is the list's choice settling, and a tile picked.
var selSpring = ui.Animation{Spring: ui.SpringOptions{Duration: 300 * time.Millisecond, Bounce: 0.1}}

// keepSettings writes what saveSettingsSoon left, once the settings have been
// still a moment or the dialog is shut.
func (a *app) keepSettings(c *ui.Context) {
	if !a.settingsDirty {
		return
	}
	// a moment after the last change, wherever it was made: a pane dragged or a title typed changed them at every
	// frame, and each was a marshal and a write of all the settings (#192)
	if wait := 300*time.Millisecond - a.clock().Sub(a.settingsAt); wait > 0 {
		c.After(wait)
		return
	}
	a.saveSettings()
}

// openSettings shows the settings, as a screen of the window like a collection or the trash: it is
// in the history, so ⌘[ goes back to where it was opened from.
func (a *app) openSettings() {
	if a.settingsOpen {
		return
	}
	a.flush()
	a.settingsOpen, a.coll, a.tag, a.trashOpen, a.tasksOpen, a.searchOpen = true, nil, "", false, false, false
	a.statsScreen, a.profileScreen = false, false
	a.visit()
}

// toggleSettings opens the settings, or, shown, goes back to where they were opened from.
func (a *app) toggleSettings() {
	if a.settingsOpen {
		a.closeSettings()
		return
	}
	a.openSettings()
}

// closeSettings goes back to the place before the settings, or, with none, to no page.
func (a *app) closeSettings() {
	if !a.step(-1) {
		a.settingsOpen = false
	}
}

// settingsScreen is the settings: the sections listed at the left and the one chosen at the right,
// in groups. A setting changed is seen at once, as the window it is made in is the window.
func (a *app) settingsScreen(c *ui.Context) {
	cl := ryugo.From(c).Colors
	ui.Row(c).Fill().MaxWidth(viewMax).Gap(20).AlignItems(ui.Stretch).Children(func() {
		if !phone { // on a phone the sections are a row of chips over the section (settingsChips)
			a.settingsNav(c)
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			sec := settingsSections[min(a.settingsTab, len(settingsSections)-1)]
			if phone {
				a.settingsChips(c)
			}
			// A section comes at once, its head and its settings: the bar over them stays, and a move under it only
			// delays what was asked for.
			ui.Column(c.Key("set-head:"+strconv.Itoa(a.settingsTab))).FillWidth().Gap(4).Padding(2, 0, 14, 0).Shrink(0).Children(func() {
				ui.Text(c, msg(sec.name)).FontSize(map[bool]float32{true: 24, false: 32}[phone]).FontWeight(720).TextColor(cl.Text)
				ui.Text(c, msg(sec.blurb)).FontSize(ryugo.FsMd).TextColor(cl.Muted)
			})
			a.settingsTop(c)
			a.scrolled(c, "set-scroll", nil, ui.Scroll(c.Key("set-scroll")).Grow(1).MinHeight(0), func() {
				ui.Column(c.Key("set-body:"+strconv.Itoa(a.settingsTab))).FillWidth().Gap(18).Padding(0, 4, 40, 0).Children(func() {
					if a.settingsExtra(c) {
						return
					}
					switch a.settingsTab {
					case 0:
						a.settingsAppearance(c)
					case 1:
						a.settingsEditor(c)
					case tabFonts:
						a.settingsFonts(c)
					case tabStatus:
						a.settingsStatus(c)
					case tabMCP:
						a.settingsMCP(c)
					case tabSync:
						a.settingsSync(c)
					case tabUpdate:
						a.settingsUpdate(c)
					case tabSounds:
						a.settingsSounds(c)
					default:
						a.settingsLibrary(c)
					}
				})
			})
		})
	})
}

// settingsChips are the sections on a phone: a row of chips that scrolls sideways, the one chosen lit.
func (a *app) settingsChips(c *ui.Context) {
	cl := ryugo.From(c).Colors
	a.scrolledSideways(c, "set-chips", nil, ui.ScrollHorizontal(c.Key("set-chips")).FillWidth().Shrink(0), func() {
		ui.Row(c).Gap(6).Padding(0, 0, 12, 0).Children(func() {
			for i, s := range settingsSections {
				i, sel := i, a.settingsTab == i
				e := ui.Row(c.Key("schip:"+s.name)).Height(36).Padding(0, 14).Gap(7).Radius(rad(18)).AlignItems(ui.Center).Shrink(0).
					Label(msg(s.name)).Role(ui.RoleButton).Border(1, cl.Hairline)
				if sel {
					e.Background(ryugo.Mix(cl.Card, cl.Accent, 0.22)).Border(1, cl.Accent.Alpha(0.5))
				} else {
					e.Background(cl.Card)
				}
				e.Children(func() {
					ui.Icon(c, s.glyph).Width(15).Height(15).TextColor(map[bool]ui.Color{true: cl.Accent, false: cl.Muted}[sel])
					ui.Text(c, msg(s.name)).FontSize(13.5).FontWeight(map[bool]int{true: 650, false: 540}[sel]).TextColor(cl.Text)
				})
				if e.Clicked() {
					a.settingsTab = i
				}
			}
		})
	})
}

// settingsNav is the list of sections: the one chosen glows - the accent in a gentle gradient, as a
// button is - and the others come up as the pointer does.
func (a *app) settingsNav(c *ui.Context) {
	t := ryugo.From(c)
	cl := t.Colors
	ui.Column(c.Key("set-nav")).Width(190).Padding(4, 0, 12, 0).Gap(2).Shrink(0).Children(func() {
		for i, s := range settingsSections {
			sel := a.settingsTab == i && strings.TrimSpace(a.setSearch) == "" && a.setMode == "" // none over a search or a list
			e := ui.Row(c.Key("snav:"+s.name)).FillWidth().Height(34).Padding(0, 10).Gap(9).Radius(rad(9)).AlignItems(ui.Center).
				Label(msg(s.name)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			on := e.AnimateTo("on", b2f(sel), selSpring)
			hov := e.AnimateTo("hov", b2f(e.Hovered() && !sel), fadeQuick)
			if on > 0.01 {
				e.LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true,
					From: ryugo.Mix(cl.Card, cl.Accent, 0.26*on), To: ryugo.Mix(cl.Card, cl.Accent, 0.14*on)}).Border(1, cl.Accent.Alpha(0.42*on))
			} else if hov > 0.01 {
				e.Background(cl.Fill.Alpha(hov))
			}
			e.Translate(2*on, 0)
			a.kbMark(e, "snav:"+s.name, true, func() { a.settingsGo(i) })
			e.Children(func() {
				ui.Icon(c, s.glyph).Width(16).Height(16).TextColor(ryugo.Mix(cl.Muted, cl.Accent, on))
				ui.Text(c, msg(s.name)).FontSize(13.5).FontWeight(int(540 + 120*on)).TextColor(ryugo.Mix(cl.Text2, cl.Text, on)).Grow(1).MaxLines(1)
			})
			if e.Clicked() {
				a.settingsGo(i)
			}
		}
	})
}

// settingsGo shows the section i: a search's words are cleared and a list (Changed, Profiles and the others) closed,
// as they stand in a section's place (#404).
func (a *app) settingsGo(i int) {
	a.settingsTab, a.setSearch, a.setMode = i, "", ""
}

// settingGroup is a card of settings, a line between them.
func settingGroup(c *ui.Context, key string, rows ...func()) {
	cl := ryugo.From(c).Colors
	ui.Column(c.Key(key)).FillWidth().Radius(rad(16)).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
		for i, r := range rows {
			if i > 0 {
				// a line between two rows, inset as the rows are, of the text's color: the hairline is lost on a dark well
				ui.Box(c.Key(key+"-d"+strconv.Itoa(i))).Height(1).Margin(0, 18).AlignSelf(ui.Stretch).Background(cl.Text.Alpha(0.09))
			}
			ui.Column(c.Key(key+"-r"+strconv.Itoa(i))).FillWidth().Padding(14, 18).Children(r)
		}
	})
}

// settingRow is a setting: what it is, and its control.
func settingRow(c *ui.Context, title, help string, control func()) {
	title, help = msg(title), msg(help)
	cl := ryugo.From(c).Colors
	// its control under its name on a phone, and where the row is narrow (a small window): beside it, the name
	// and its help would be squeezed into a column a word wide
	box := ui.Box(c).FillWidth()
	narrow := phone
	if b := box.LayoutBounds(); b.W > 0 && b.W < settingNarrow {
		narrow = true
	}
	box.Children(func() { settingRowIn(c, cl, title, help, narrow, control) })
}

// settingNarrow is the width under which a setting's control goes under its name.
const settingNarrow = 430

func settingRowIn(c *ui.Context, cl ryugo.Colors, title, help string, narrow bool, control func()) {
	if narrow {
		ui.Column(c).FillWidth().Gap(10).Children(func() {
			ui.Column(c).FillWidth().Gap(3).Children(func() {
				ui.Text(c, title).FontSize(15).FontWeight(620).TextColor(cl.Text)
				if help != "" {
					ui.Text(c, help).FontSize(ryugo.FsSm).TextColor(cl.Muted)
				}
			})
			ui.Row(c).Children(control) // as wide as it is, not stretched across the row
		})
		return
	}
	ui.Row(c).Gap(20).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, title).FontSize(15).FontWeight(620).TextColor(cl.Text)
			if help != "" {
				ui.Text(c, help).FontSize(ryugo.FsSm).TextColor(cl.Muted)
			}
		})
		control()
	})
}

// choiceTile is a choice that has a look: a picture over its name, raised as a
// button is; it lifts as the pointer comes, and the chosen one is ringed in
// the accent. It reports a click.
func choiceTile(c *ui.Context, key, label string, on bool, preview func()) bool {
	label = msg(label)
	t := ryugo.From(c)
	cl := t.Colors
	e := ui.Column(c.Key(key)).Width(128).Gap(8).Padding(7, 7, 10, 7).Radius(rad(16)).AlignItems(ui.Center).
		Label(label).Role(ui.RoleButton).Cursor(ui.CursorPointer)
	lift := e.AnimateTo("lift", b2f(e.Hovered()), fadeQuick)
	sel := e.AnimateTo("sel", b2f(on), selSpring)
	e.LinearGradient(ui.LinearGradient{Angle: 180, Oklab: true, From: cl.ControlTop, To: cl.ControlBottom}).
		Border(1.5, ryugo.Mix(cl.ControlBorder, cl.Accent, sel))
	if on {
		t.Shadows.Ring.Apply(e)
	} else {
		t.Shadows.ControlShadow.Apply(e)
	}
	e.Translate(0, -2*lift)
	e.Children(func() {
		ui.Box(c).Width(112).Height(66).Radius(rad(10)).Clip().Children(preview)
		ui.Text(c, label).FontSize(13.5).FontWeight(int(540 + 120*sel)).TextColor(ryugo.Mix(cl.Text2, cl.Text, sel))
	})
	return e.Clicked()
}

// mock is a little window, light or dark, for a tile.
func mock(c *ui.Context, dark bool, w float32) {
	bg, fg := ui.Hex("#f4f4f6"), ui.Hex("#1b1b1f")
	if dark {
		bg, fg = ui.Hex("#17171a"), ui.Hex("#ececf1")
	}
	ui.Row(c).Width(w).Height(66).Background(bg).Clip().Children(func() {
		ui.Box(c).Width(22).Height(66).Background(fg.Alpha(0.08)).Shrink(0)
		ui.Column(c).Padding(12, 8).Gap(6).Children(func() {
			ui.Box(c).Width(40).Height(6).Radius(rad(3)).Background(fg.Alpha(0.8))
			ui.Box(c).Width(54).Height(4).Radius(rad(2)).Background(fg.Alpha(0.3))
			ui.Box(c).Width(46).Height(4).Radius(rad(2)).Background(fg.Alpha(0.3))
			ui.Box(c).Width(28).Height(8).Radius(rad(4)).Background(ryugo.From(c).Colors.Accent)
		})
	})
}

func (a *app) settingsAppearance(c *ui.Context) {
	cl := ryugo.From(c).Colors
	theme := a.settings.Theme % 3
	settingGroup(c, "set-theme",
		func() {
			ui.Column(c).Gap(14).Children(func() {
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, locales.CommonTheme()).FontSize(15).FontWeight(620).TextColor(cl.Text)
					ui.Text(c, locales.SettingsFollowsSystemsLookOr()).FontSize(ryugo.FsSm).TextColor(cl.Muted)
				})
				ui.Row(c).Gap(14).Wrap().Children(func() {
					if choiceTile(c, "tile-sys", locales.CommonSystem(), theme == 0, func() { ui.Row(c).Children(func() { mock(c, false, 56); mock(c, true, 56) }) }) {
						theme = 0
					}
					if choiceTile(c, "tile-light", locales.CommonLight(), theme == 1, func() { mock(c, false, 112) }) {
						theme = 1
					}
					if choiceTile(c, "tile-dark", locales.CommonDark(), theme == 2, func() { mock(c, true, 112) }) {
						theme = 2
					}
				})
			})
		})
	if theme != a.settings.Theme {
		a.settings.Theme = theme
		a.saveSettings()
	}
	settingGroup(c, "set-accent", func() { a.appThemePicker(c) })
	w := float64(a.pageWidth())
	settingGroup(c, "set-width", func() {
		settingRow(c, locales.CommonPageWidth(), locales.SettingsTheWidthTextColumn(), func() {
			ui.Box(c).Width(260).Children(func() {
				ryugo.Slider(c.Key("set-width-s"), &w, 520, 1100, ryugo.SliderOptions{Label: locales.CommonPageWidth(), Step: 20, Format: func(v float64) string { return strconv.Itoa(int(v)) + " px" }})
			})
		})
	})
	if int(w) != int(a.pageWidth()) {
		a.settings.PageWidth = w
		a.saveSettingsSoon()
	}
	corner := float64(a.cornerPercent())
	settingGroup(c, "set-corner", func() {
		settingRow(c, locales.CommonCorners(), locales.SettingsAllCornersAreWorked(), func() {
			ui.Box(c).Width(260).Children(func() {
				ryugo.Slider(c.Key("set-corner-s"), &corner, 0, 200, ryugo.SliderOptions{Label: locales.CommonCornerRoundness(), Step: 10, Format: func(v float64) string { return strconv.Itoa(int(v)) + "%" }})
			})
		})
	})
	if int(corner) != a.cornerPercent() {
		v := int(corner)
		a.settings.Corner = &v
		a.saveSettingsSoon()
	}
	a.settingsMotion(c) // reduce motion, the shader effects (motion.go)
	a.settingsWaves(c)
	card := !a.settings.NoSidebarCard
	settingGroup(c, "set-sidebar-card", func() {
		settingRow(c, locales.SettingsSidebarCard(), locales.SettingsSidebarCardHelp(), func() {
			ryugo.Switch(c.Key("set-sidebar-card-sw"), &card, ryugo.SwitchOptions{}).Label(locales.SettingsSidebarCard())
		})
	})
	if card == a.settings.NoSidebarCard {
		a.settings.NoSidebarCard = !card
		a.saveSettings()
	}
	if runtime.GOOS == "darwin" {
		glassOn := a.settings.Glass
		settingGroup(c, "set-glass", func() {
			ui.Row(c).Gap(20).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, locales.SettingsGlass()).FontSize(15).FontWeight(620).TextColor(cl.Text)
						ryugo.Badge(c, locales.SettingsExperimental(), ryugo.ToneWarn)
					})
					ui.Text(c, locales.SettingsGlassHelp()).FontSize(ryugo.FsSm).TextColor(cl.Muted)
					ui.Text(c, locales.SettingsGlassRestart()).FontSize(ryugo.FsXs).TextColor(cl.Muted)
				})
				ryugo.Switch(c.Key("set-glass-sw"), &glassOn, ryugo.SwitchOptions{}).Label(locales.SettingsGlass())
			})
		})
		if glassOn != a.settings.Glass {
			a.setGlass(glassOn)
		}
	}
	settingGroup(c, "set-welcome", func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			// a list each: there are too many looks for a bar of buttons
			look := func(key, title, help string, opts []ryugo.Option[string], cur *string) {
				settingRow(c, title, help, func() {
					v := *cur
					ui.Box(c).Width(200).Children(func() {
						ryugo.Select(c.Key(key), &v, opts, ryugo.SelectOptions{Label: msg(title), Size: ryugo.Sm})
					})
					if v != *cur {
						*cur = v
						a.saveSettings()
					}
				})
			}
			var grounds, openings []ryugo.Option[string]
			for _, g := range welcomeGrounds {
				grounds = append(grounds, ryugo.Option[string]{Value: g.id, Label: msg(g.name)})
			}
			for _, o := range welcomeOpenings {
				openings = append(openings, ryugo.Option[string]{Value: o.id, Label: msg(o.name)})
			}
			look("set-welcome-ground", "theWelcomesGround", "whatIsUnderTheWordsOver", grounds, &a.settings.WelcomeGround)
			look("set-welcome-open", "opening", "howTheAppsWindowComesAfter", openings, &a.settings.WelcomeOpen)
			settingRow(c, locales.SettingsWelcomeScreen(), locales.SettingsShowsFirstRunsWelcome(), func() {
				if ryugo.Button(c.Key("set-welcome-go"), locales.CommonRestart(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.welcomeAgain()
				}
			})
		})
	})
	settingGroup(c, "set-lang", func() {
		settingRow(c, locales.CommonLanguage(), locales.SettingsTheLanguageInterfaceMenus(), func() {
			was := a.settings.Lang
			if was != "tr" { // nothing chosen is English
				was = "en"
			}
			lang := was
			choiceBar(c, "set-lang-bar", locales.CommonLanguage(), []string{"en", "tr"}, []string{langName("en"), langName("tr")}, &lang)
			if lang != was {
				a.settings.Lang = lang
				setLang(lang)
				a.saveSettings()
			}
		})
	})
}

func (a *app) settingsEditor(c *ui.Context) {
	cl := ryugo.From(c).Colors
	caret := int(a.caretKind)
	if !fx.still { // reduced motion: the caret is plain, and the jelly and the pointer's effect do not move: no card (#402, #405)
		a.settingsCaret(c, cl, &caret)
	}
	if caret != int(a.caretKind) {
		a.setCaret(caretKind(caret))
	}
	a.settingsEditorRest(c, cl)
}

// settingsCaret is the caret's card: the caret, chosen on tiles and tried over them, the jelly and the pointer's
// effect. caret is the one chosen.
func (a *app) settingsCaret(c *ui.Context, cl ryugo.Colors, caret *int) {
	settingGroup(c, "set-caret",
		func() {
			ui.Column(c).FillWidth().Gap(14).Children(func() { // the whole width: the tiles wrap in it, not in a column one tile wide
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, locales.Caret()).FontSize(15).FontWeight(620).TextColor(cl.Text)
					ui.Text(c, locales.SettingsSlimeStretchesAsYou()).FontSize(ryugo.FsSm).TextColor(cl.Muted)
				})
				a.caretPlayground(c) // over the tiles: a tile chosen is tried here at once, as it is in the text
				ui.Row(c).FillWidth().Gap(14).Wrap().Children(func() {
					for k := caretKind(0); k < caretCount(); k++ {
						k := k
						if choiceTile(c, "tile-caret"+itoa(int(k)+1), caretName(k), *caret == int(k), func() { a.caretMock(c, k) }) {
							*caret = int(k)
						}
					}
				})
			})
		},
		func() {
			on := a.settings.Jelly
			settingRow(c, locales.CommonJellyEffect(), locales.SettingsEveryCaretStretchesWay(), func() {
				ryugo.Switch(c.Key("set-jelly"), &on, ryugo.SwitchOptions{}).Label(locales.CommonJellyEffect())
			})
			if on != a.settings.Jelly {
				a.settings.Jelly = on
				a.saveSettings()
			}
		},
		func() { // the mouse pointer's effect: there is no pointer on a phone
			if phone {
				return
			}
			settingRow(c, locales.CommonCursorFx(), locales.SettingsAnEffectThatGoes(), func() {
				v := a.settings.CursorFX
				opts := []ryugo.Option[string]{{Value: "", Label: locales.CommonOff()}}
				for _, p := range cursorPresets {
					opts = append(opts, ryugo.Option[string]{Value: p.name, Label: msg(p.name)})
				}
				ui.Box(c).Width(200).Children(func() {
					ryugo.Select(c.Key("set-cursor-fx"), &v, opts, ryugo.SelectOptions{Label: locales.CommonCursorFx(), Size: ryugo.Sm})
				})
				if v != a.settings.CursorFX {
					a.settings.CursorFX = v
					a.saveSettings()
				}
			})
		})
}

// settingsEditorRest is the editor's section under the caret's card: the type, the cover and the rest.
func (a *app) settingsEditorRest(c *ui.Context, cl ryugo.Colors) {
	fs, lh := float64(a.fontSize()), float64(a.lineHeight())
	settingGroup(c, "set-type",
		func() {
			settingRow(c, locales.CommonFontSize(), "", func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-font"), &fs, 13, 24, ryugo.SliderOptions{Label: locales.CommonFontSize(), Step: 0.5, Format: func(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) + " px" }})
				})
			})
		},
		func() {
			settingRow(c, locales.CommonLineSpacing(), "", func() {
				ui.Box(c).Width(260).Children(func() {
					ryugo.Slider(c.Key("set-lh"), &lh, 1.3, 2, ryugo.SliderOptions{Label: locales.CommonLineSpacing(), Step: 0.05, Format: func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }})
				})
			})
		},
		func() { // how it reads, as it is set
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, locales.SettingsPreview()).FontSize(ryugo.FsSm).FontWeight(600).TextColor(cl.Muted)
				ui.Text(c, locales.SettingsOneIdeaOnePage()).
					FontSize(float32(fs)).LineHeight(float32(lh)).TextColor(cl.Text)
			})
		})
	auto := a.settings.AutoCover
	settingGroup(c, "set-cover", func() {
		settingRow(c, locales.CommonAutomaticCover(), locales.SettingsEveryPageWithNo(), func() {
			ryugo.Switch(c.Key("set-autocover"), &auto, ryugo.SwitchOptions{}).Label(locales.CommonAutomaticCover())
		})
	})
	if auto != a.settings.AutoCover {
		a.settings.AutoCover = auto
		a.saveSettings()
	}
	lapse := !a.settings.NoLapse
	settingGroup(c, "set-lapse", func() {
		settingRow(c, locales.CommonColorWordsByDay(), locales.SettingsTimeLapsePagesWords(), func() {
			ryugo.Switch(c.Key("set-lapse-sw"), &lapse, ryugo.SwitchOptions{}).Label(locales.CommonColorWordsByDay())
		})
	})
	if lapse != !a.settings.NoLapse {
		a.settings.NoLapse = !lapse
		a.saveSettings()
	}
	shake := !a.settings.NoWobble
	settingGroup(c, "set-wobble", func() {
		settingRow(c, locales.CommonShimmer(), locales.SettingsWhileAgentMcpAdds(), func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if ryugo.Button(c.Key("set-wobble-try"), locales.SettingsTry(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.shimmer(400)
				}
				ryugo.Switch(c.Key("set-wobble-sw"), &shake, ryugo.SwitchOptions{}).Label(locales.CommonShimmer())
			})
		})
	})
	if shake != !a.settings.NoWobble {
		a.settings.NoWobble = !shake
		a.saveSettings()
	}
	a.settingsText(c)
	a.settingsQuick(c)
	if float32(fs) != a.fontSize() {
		a.settings.FontSize = fs
		a.saveSettingsSoon()
	}
	if float32(lh) != a.lineHeight() {
		a.settings.LineHeight = lh
		a.saveSettingsSoon()
	}
}

// settingsSounds is the Sounds tab: the switch for all of them, the chord of the window opening (off by default), and
// a Try button for each sound.
func (a *app) settingsSounds(c *ui.Context) {
	snd := !a.settings.NoSound
	settingGroup(c, "set-sound-main", func() {
		settingRow(c, locales.CommonSounds(), locales.SettingsSoundsSub(), func() {
			ryugo.Switch(c.Key("set-sound-main-sw"), &snd, ryugo.SwitchOptions{}).Label(locales.CommonSounds())
		})
	})
	if snd != !a.settings.NoSound {
		a.settings.NoSound = !snd
		a.saveSettings()
	}
	playRow := func(key, title, sub string, kind soundKind) func() {
		return func() {
			settingRow(c, title, sub, func() {
				if ryugo.Button(c.Key(key), locales.SettingsTry(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.playSoundPreview(kind)
				}
			})
		}
	}
	chord := a.settings.StartChord // the window's chord is its own choice, off by default
	startRow := func() {
		settingRow(c, locales.SoundsStartup(), locales.SoundsStartupSub(), func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if ryugo.Button(c.Key("try-sound-startup"), locales.SettingsTry(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.playSoundPreview(soundStartup)
				}
				ryugo.Switch(c.Key("set-sound-startup-sw"), &chord, ryugo.SwitchOptions{}).Label(locales.SoundsStartup())
			})
		})
	}
	settingGroup(c, "set-sound-list",
		startRow,
		playRow("try-sound-welcome", locales.SoundsWelcome(), locales.SoundsWelcomeSub(), soundWelcome),
		playRow("try-sound-info", locales.SoundsInfo(), locales.SoundsInfoSub(), soundInfo),
		playRow("try-sound-ok", locales.SoundsOk(), locales.SoundsOkSub(), soundOK),
		playRow("try-sound-warn", locales.SoundsWarn(), locales.SoundsWarnSub(), soundWarn),
		playRow("try-sound-err", locales.SoundsErr(), locales.SoundsErrSub(), soundErr),
	)
	if chord != a.settings.StartChord {
		a.settings.StartChord = chord
		a.saveSettings()
	}
}

func (a *app) settingsStatus(c *ui.Context) {
	toggle := func(key, title, help string, v *bool, invert bool) func() {
		return func() {
			on := *v != invert
			settingRow(c, title, help, func() { ryugo.Switch(c.Key(key), &on, ryugo.SwitchOptions{}).Label(msg(title)) })
			if on != (*v != invert) {
				*v = on != invert
				a.saveSettings()
			}
		}
	}
	settingGroup(c, "set-status",
		toggle("set-saved", "saveStatus", "savedSaving", &a.settings.HideSaved, true),
		toggle("set-chars", "letterCount", "spacesAndLineEndsAreNot", &a.settings.ShowChars, false),
		toggle("set-hidebar", "hideTheStatusBar", "nothingIsDrawnAtTheBottom", &a.settings.HideStatus, false),
		toggle("set-pos", "lineAndColumn", "theCaretsParagraphAndItsPlace", &a.settings.ShowPos, false),
		toggle("set-hints", "shortcutHints", "theKeysThatCanBeUsed", &a.settings.HideHints, true),
	)
}

func (a *app) settingsLibrary(c *ui.Context) {
	cl := ryugo.From(c).Colors
	pages := 0
	for _, co := range a.lib.Collections {
		pages += len(co.All())
	}
	row := func(label, text, key string, more ...func()) func() { // more: buttons after its copy
		return func() {
			ryugo.Field(c, ryugo.FieldOptions{Label: msg(label)}, func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Padding(8, 10).Radius(rad(ryugo.From(c).R(ryugo.RadiusControl)-2)).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
						a.textMenu(ui.Text(c, text).FontSize(12.5).Font(monoStack).TextColor(cl.Text2).Selectable().MaxLines(1), func() string { return text }, nil)
					})
					ryugo.CopyButton(c.Key(key), text, ryugo.CopyOptions{Label: locales.CommonCopyLabel(msg(label))})
					for _, m := range more {
						m()
					}
				})
			})
		}
	}
	store, inFile := a.lib.FS.(*Store)
	if !inFile {
		settingGroup(c, "set-lib",
			row("libraryFolder", a.lib.Root, "copy-root"),
			row("settingsFile", a.settingsFile, "copy-settings"),
			func() {
				settingRow(c, locales.SettingsLibraryCounts(len(a.lib.Collections), pages), locales.SettingsAllPlainFilesThey(), func() {})
			})
		return
	}
	settingGroup(c, "set-lib",
		// where it is, not its file's name, which says nothing to a user (#408)
		row("libraryFolder", filepath.Dir(store.File), "copy-root", func() {
			if ryugo.Button(c.Key("lib-show"), locales.PropsShowFolder(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
				go a.runShellOpen(store.File, true)
			}
		}),
		func() {
			settingRow(c, locales.SettingsLibraryCounts(len(a.lib.Collections), pages),
				locales.SettingsAllThisOneFile(), func() {})
		},
		func() { a.settingsImport(c) },
		func() {
			note := a.exportNote
			if note == "" {
				note = "asAFolderOfPlainMd"
			}
			settingRow(c, locales.SettingsExportAsMarkdown(), note, func() {
				if ryugo.Button(c.Key("export-md"), locales.CommonExport(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.exportNote, _ = a.exportLibrary(store)
				}
			})
		},
		func() {
			settingRow(c, locales.SettingsMarkdownAsZip(), locales.SettingsTheSameAsSingle(), func() {
				if ryugo.Button(c.Key("export-zip"), locales.SettingsWriteZip(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.exportNote, _ = a.exportZip(store)
				}
			})
		})
	a.settingsBackup(c)
}

// exportLibrary writes the library as a folder of Markdown files next to its file (exportmd.go), and says where
// it went.
func (a *app) exportLibrary(store *Store) (string, bool) {
	dir := filepath.Join(filepath.Dir(store.File), "kitaplik-md-"+time.Now().Format("20060102-150405"))
	a.flush()
	if err := a.lib.ExportMarkdown(dir); err != nil {
		return locales.CommonDidntWork(err.Error()), false
	}
	return locales.CommonSavedTo(dir), true
}

func (a *app) settingsMCP(c *ui.Context) {
	t := ryugo.From(c)
	cl := t.Colors
	wasOn := a.settings.MCP || a.mcpServer != nil
	on := wasOn
	settingGroup(c, "set-mcp", func() {
		settingRow(c, locales.CommonMcpServer(), locales.SettingsWhenAgentsClaudeCode(), func() {
			ryugo.Switch(c.Key("mcp-switch"), &on, ryugo.SwitchOptions{}).Label(locales.SettingsTurnMcpServer())
		})
	})
	if on != wasOn {
		a.setMCP(on)
	}
	if a.mcpErr != "" {
		ui.Text(c, locales.SettingsCouldNotOpenErr(a.mcpErr)).FontSize(ryugo.FsSm).TextColor(cl.Up)
	}
	if !wasOn {
		return
	}
	copyRow := func(label, text, key string, concealed bool) func() {
		return func() {
			ryugo.Field(c, ryugo.FieldOptions{Label: msg(label)}, func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Grow(1).MinWidth(0).Padding(8, 10).Radius(rad(ryugo.RadiusControl-2)).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
						tx := ui.Text(c, text).FontSize(12.5).Font(monoStack).TextColor(cl.Text2)
						if concealed {
							a.secretTextMenu(tx, func() string { return text }, nil)
						} else {
							a.textMenu(tx.Selectable(), func() string { return text }, nil)
						}
					})
					if concealed {
						a.secretCopyButton(c, key, text, locales.CommonCopyLabel(msg(label)))
					} else {
						ryugo.CopyButton(c.Key(key), text, ryugo.CopyOptions{Label: locales.CommonCopyLabel(msg(label))})
					}
				})
			})
		}
	}
	settingGroup(c, "set-mcp-how",
		copyRow("address", a.mcpURL(), "copy-url", false),
		copyRow("toAddItToClaudeCode", a.mcpCommand(), "copy-cmd", true),
		copyRow("forAnAgentThatConnectsOver", a.mcpStdioCommand(), "copy-stdio", false),
		a.mcpSetupRow(c),
		func() {
			settingRow(c, locales.SettingsKey(), locales.SettingsOnlyFromThisComputer(), func() {
				if ryugo.Button(c.Key("new-token"), locales.SettingsMakeNewKey(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.settings.Token = newToken()
					a.saveSettings()
				}
			})
		})
	a.settingsMCPAccess(c)
}

// settingsMCPAccess is what agents may do with each collection: not see it, read it, or read and write it; and with
// the others, new ones among them.
func (a *app) settingsMCPAccess(c *ui.Context) {
	opts := []ryugo.Option[string]{{Value: mcpWrite, Label: locales.SettingsReadsWrites()}, {Value: mcpRead, Label: locales.SettingsReadsOnly()}, {Value: mcpOff, Label: locales.SettingsDoesntSee()}}
	row := func(key, name string) func() {
		return func() {
			v := mcpWrite
			if x, ok := a.settings.MCPAccess[key]; ok {
				v = x
			} else if key != "*" {
				v = a.mcpAccess(nil) // as the others
			}
			was := v
			settingRow(c, name, "", func() {
				ryugo.Select(c.Key("mcp-access:"+key), &v, opts, ryugo.SelectOptions{Label: name, Size: ryugo.Sm}).Width(170)
			})
			if v != was {
				if a.settings.MCPAccess == nil {
					a.settings.MCPAccess = map[string]string{}
				}
				a.settings.MCPAccess[key] = v
				a.saveSettings()
			}
		}
	}
	rows := []func(){func() {
		ui.Text(c, locales.SettingsWhatAgentsMayDo()).FontSize(ryugo.FsSm).TextColor(ryugo.From(c).Colors.Muted).Wrap()
	}}
	for _, coll := range a.lib.Collections {
		rows = append(rows, row(filepath.Base(coll.Dir), coll.Name))
	}
	rows = append(rows, row("*", locales.SettingsTheOthersNewOnes()))
	settingGroup(c, "set-mcp-access", rows...)
}

// settingsQuick is the key of the quick note.
func (a *app) settingsQuick(c *ui.Context) {
	cl := ryugo.From(c).Colors
	cur := "—"
	if a.quick != nil && a.quick.shortcut != "" {
		cur = a.quick.shortcut
	} else if a.settings.QuickKey != "" {
		cur = a.settings.QuickKey
	}
	if a.quickKeyDraft == "" {
		a.quickKeyDraft = cur
	}
	settingGroup(c, "set-quick", func() {
		settingStack(c, locales.SettingsQuickNoteKey(), locales.SettingsOpensPanelFromAnywhere(), func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				in := a.fieldMenu(ryugo.Input(c.Key("set-quick-in"), &a.quickKeyDraft, ryugo.InputOptions{Label: locales.SettingsQuickNoteKey(), Size: ryugo.Sm}), &a.quickKeyDraft, fieldOpts{oneLine: true}).Width(190)
				apply := ryugo.Button(c.Key("set-quick-ok"), locales.CommonApply(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() || in.Submitted()
				if apply && a.quick != nil {
					if err := a.quick.setShortcut(strings.TrimSpace(a.quickKeyDraft)); err != nil {
						a.quickKeyErr = err.Error()
					} else {
						a.quickKeyErr = ""
						a.settings.QuickKey = a.quick.shortcut
						a.saveSettings()
					}
				}
			})
		})
		if a.quickKeyErr != "" {
			ui.Text(c, a.quickKeyErr).FontSize(ryugo.FsSm).TextColor(cl.Up).Wrap()
		}
	})
}

// settingStack is a setting whose control is wide: the control under the words, not beside them.
func settingStack(c *ui.Context, title, help string, control func()) {
	cl := ryugo.From(c).Colors
	ui.Column(c).Gap(8).Children(func() {
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, msg(title)).FontSize(15).FontWeight(620).TextColor(cl.Text)
			if help != "" {
				ui.Text(c, msg(help)).FontSize(ryugo.FsSm).TextColor(cl.Muted)
			}
		})
		control()
	})
}
