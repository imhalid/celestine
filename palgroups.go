package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// The commands of the palette, with nothing typed after ">": in groups under headings, and a family
// of commands that differ by what comes after their ":" (Caret: …, Sayfayı taşı: …) as one line,
// which opens to its members. Typed, every command is found by itself, as before.
// cmdCategory is the group of a command in the palette, by its id: the commands of the app and the settings; the
// rest are the app's (cmdCat).
var cmdCategory = map[string]string{
	"settings_readSettingsFile": "filesAndSync", "newPaneThisScreen": "panes", "panes_settingsBeside": "panes", "panes_tasksBeside": "panes", "panes_trashBeside": "panes",
	"accountProfile":             "common_go",
	"addASubPage":                "page",
	"appTypeface":                "common_appearance",
	"biggerText":                 "writing",
	"caret":                      "common_appearance",
	"changeTheCaret":             "common_appearance",
	"closeThePane":               "panes",
	"codeTypeface":               "writing",
	"collectionSettings":         "collection",
	"common_alwaysShowFocusRing": "common_appearance",
	"common_automaticCover":      "common_appearance",
	"common_autosave":            "filesAndSync",
	"common_back":                "common_go",
	"common_backUpNow":           "filesAndSync",
	"common_caretSleep":          "common_appearance",
	"common_colorTheme":          "common_appearance",
	"common_cornerRoundness":     "common_appearance",
	"common_corners":             "common_appearance",
	"common_cursorFx":            "common_appearance",
	"common_firstLineIndent":     "writing",
	"common_focusMode":           "common_appearance",
	"common_fontSize":            "writing",
	"common_fontWeight":          "writing",
	"common_goDate":              "common_go",
	"common_jellyEffect":         "common_appearance",
	"common_language":            "common_appearance",
	"common_ligatures":           "writing",
	"common_lineLength":          "writing",
	"common_lineSpacing":         "writing",
	"common_newPage":             "page",
	"common_pageWidth":           "writing",
	"common_paragraphSpacing":    "writing",
	"common_profile":             "common_appearance",
	"common_quickNote":           "page",
	"common_resetAllDefaults":    "filesAndSync",
	"common_save":                "page",
	"common_saveDelay":           "filesAndSync",
	"common_settings":            "common_go",
	"common_sidePanel":           "panes",
	"common_statistics":          "common_go",
	"common_syncNow":             "filesAndSync",
	"common_tabWidth":            "writing",
	"common_tasks":               "common_go",
	"common_theme":               "common_appearance",
	"common_trash":               "common_go",
	"common_undo2":               "page",
	"common_writeSettingsFile":   "filesAndSync",
	"copyAsMarkdown":             "filesAndSync",
	"duplicateTheBlock":          "page",
	"exportAsMarkdownFolder":     "filesAndSync",
	"exportAsMarkdownZip":        "filesAndSync",
	"findInPage2":                "page",
	"foldUnfold":                 "page",
	"forward":                    "common_go",
	"goBackToBeforeTheLast":      "filesAndSync",
	"importAMarkdownFolder2":     "filesAndSync",
	"moveTheBlockDown":           "page",
	"moveTheBlockUp":             "page",
	"moveTheCollectionDown":      "collection",
	"moveTheCollectionUp":        "collection",
	"moveThePage":                "page",
	"moveThePageDownInIts":       "page",
	"moveThePageUpInIts":         "page",
	"moveThePaneToAWindow":       "panes",
	"moveToTrash":                "page",
	"newCollection":              "collection",
	"nextDay":                    "common_go",
	"openBlockBeside":            "page",
	"openInASidePane":            "panes",
	"pasteATableCsv":             "page",
	"pasteAsMarkdown":            "filesAndSync",
	"previousDay":                "common_go",
	"randomDay":                  "common_go",
	"redo":                       "page",
	"resetTheFontSize":           "writing",
	"restoreFromABackup":         "filesAndSync",
	"searchTheLibrary":           "common_go",
	"shortcutHints":              "common_appearance",
	"showParagraphEnds":          "common_appearance",
	"showTabs":                   "common_appearance",
	"showWhitespaceInCodeBlocks": "common_appearance",
	"sidebar":                    "panes",
	"smallerText":                "writing",
	"switchPane":                 "panes",
	"sync4":                      "filesAndSync",
	"syncInterval":               "filesAndSync",
	"takeThePageOutOfIts":        "page",
	"todaysPage":                 "page",
	"toggleTheSidePanel":         "panes",
	"toggleTheSidebar":           "panes",
	"typefacePreset":             "writing",
}

// cmdCats are the palette's groups, in their order; app is the rest.
var cmdCats = []string{"page", "collection", "panes", "common_go", "filesAndSync", "writing", "common_appearance", "app"}

// cmdCat is the group of a command, by its id, or by the id before ": " ("collectionSettings: Books").
func cmdCat(id string) string {
	if c, ok := cmdCategory[id]; ok {
		return c
	}
	if head, _, ok := strings.Cut(id, ": "); ok {
		if c, ok := cmdCategory[head]; ok {
			return c
		}
	}
	return "app"
}

// cmdFam is the family of a command: what its Turkish title says before ": ", "" for none.
func cmdFam(title string) string {
	if i := strings.Index(title, ": "); i > 0 {
		return title[:i]
	}
	return ""
}

// famFrom is the least a family has to be one line.
const famFrom = 3

// groupCommands lays the commands out with nothing typed: those for what is shown first, then the
// groups in their order, each family of famFrom or more as one line.
func (a *app) groupCommands(items []palItem) []palItem {
	var here []palItem
	byCat := map[string][]palItem{}
	fams := map[string]int{}
	for _, it := range items {
		if it.fam != "" {
			fams[it.cat+"\x00"+it.fam]++
		}
	}
	seen := map[string]bool{}
	for _, it := range items {
		key := it.cat + "\x00" + it.fam
		if it.fam != "" && fams[key] >= famFrom {
			if seen[key] {
				continue
			}
			seen[key] = true
			it = a.famItem(it, fams[key])
		}
		if it.boost > 0 { // for what is shown: the page, the collection, the selection
			it.group = "thisScreen"
			here = append(here, it)
			continue
		}
		it.group = it.cat
		byCat[it.cat] = append(byCat[it.cat], it)
	}
	if c := a.pal.cat; c != "" { // a group opened: its commands, and the way back
		back := palItem{title: locales.PaletteAllGroups(), glyph: iconBack, group: c, run: func() { a.pal = palette{open: true, query: ">"} }}
		return append([]palItem{back}, byCat[c]...)
	}
	out := here
	for _, g := range cmdCats { // each group a line, which opens it
		if n := len(byCat[g]); n > 0 {
			name := g
			out = append(out, palItem{title: msg(name), keys: locales.PaletteOptions(n), glyph: catGlyph(byCat[name]), group: "groups",
				run: func() { a.pal = palette{open: true, query: ">", cat: name} }})
		}
	}
	return out
}

// catGlyph is the icon of a group's line: its first command's.
func catGlyph(items []palItem) *ui.SVG {
	return items[0].glyph
}

// famItem is the line of a family of n commands, of which it is one: chosen, the palette shows them all.
func (a *app) famItem(it palItem, n int) palItem {
	name := msg(it.fam)
	return palItem{title: name + "…", keys: locales.PaletteOptions(n), glyph: it.glyph, cat: it.cat, boost: it.boost,
		run: func() { a.pal = palette{open: true, query: ">" + name + ": "} }}
}
