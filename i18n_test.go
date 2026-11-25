package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/imhalid/celestine/locales"
)

func withLang(t *testing.T, l string) {
	old := lang
	lang = l
	langNow.Store(l)
	t.Cleanup(func() { lang = old; langNow.Store(old) })
}

// msg says a message by its id in the window's language, an id with a value after it with the value as it is, and
// what is no id as it is: a page's title is not a hole.
func TestMsg(t *testing.T) {
	withLang(t, "tr")
	if msg("common_settings") != "Ayarlar" {
		t.Fatalf("Turkish: %q", msg("common_settings"))
	}
	withLang(t, "en")
	for id, want := range map[string]string{"common_settings": "Settings", "moveToTrash: Sayfa": "Move to trash: Sayfa", "Hiç çevrilmemiş cümle": "Hiç çevrilmemiş cümle"} {
		if got := msg(id); got != want {
			t.Errorf("msg(%q) = %q, want %q", id, got, want)
		}
	}
	if names("common_settings") != "Settings Ayarlar " && names("common_settings") != "English Settings " && !strings.Contains(names("common_settings"), "Ayarlar") {
		t.Errorf("names: %q", names("common_settings"))
	}
}

// The language is English unless Turkish is chosen, whatever the system's is.
func TestLangDefault(t *testing.T) {
	old := lang
	defer setLang(old)
	t.Setenv("LANG", "tr_TR.UTF-8") // the system's language does not choose it
	setLang("")
	if lang != "en" {
		t.Fatalf("not English: %s", lang)
	}
	setLang("tr")
	if lang != "tr" {
		t.Fatal("choice lost")
	}
}

// With English on, the settings, the palette and the statistics speak it; the language is a setting and a command.
func TestEnglishUI(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	withLang(t, "en")
	a.openSettings()
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	for _, want := range []string{"Appearance", "Editor", "Typeface", "Status bar", "Library", "Sync", "Updates", "Theme"} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %v", want, tt.Texts())
		}
	}
	titles := map[string]bool{}
	for _, it := range a.commands() {
		titles[it.title] = true
	}
	for _, want := range []string{"New collection", "Tasks", "Trash", "Back up now", "Autosave: turn off", "Theme: Dark"} {
		if !titles[want] {
			t.Errorf("no command %q", want)
		}
	}
	a.pane().statsScreen, a.pane().settingsOpen = true, false
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Statistics") || !tt.HasText("Library health") {
		t.Fatalf("stats in English: %v", tt.Texts())
	}
	withLang(t, "tr")
	runCmd(t, a, ">dil", "Dil: English")
	if a.settings.Lang != "en" {
		t.Fatal("the command did not set it")
	}
	withLang(t, "en")
	runCmd(t, a, ">language", "Language: Türkçe")
	if a.settings.Lang != "tr" || lang != "tr" {
		t.Fatal("back to Turkish failed")
	}
}

// Every title of a row of the settings and every heading of the statistics has its English: a row that is
// added with no translation fails here, so that the English window does not fill with Turkish.
func TestEnglishCoversTitles(t *testing.T) {
	srcs, _ := filepath.Glob("*.go")
	re := regexp.MustCompile(`(?:settingRow|statHead)\(c, "([^"]+)"`)
	var missing []string
	for _, f := range srcs {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			if !hasMessage(m[1]) && !strings.HasSuffix(m[1], ": ") {
				missing = append(missing, m[1])
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("no English for: %q", missing)
	}
	// and the lines under them, that are written whole
	helpRe := regexp.MustCompile(`settingRow\(c, "(?:[^"\\]|\\.)+", "((?:[^"\\]|\\.)+)"`)
	var noHelp []string
	for _, f := range srcs {
		if strings.HasSuffix(f, "_test.go") || f == "i18n.go" {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, m := range helpRe.FindAllStringSubmatch(string(b), -1) {
			s, err := strconv.Unquote(`"` + m[1] + `"`)
			if err != nil {
				s = m[1]
			}
			if !hasMessage(s) {
				noHelp = append(noHelp, m[1])
			}
		}
	}
	if len(noHelp) > 0 {
		t.Fatalf("no English for the help lines: %q", noHelp)
	}
	// every setting of the list has its name translated: an older one to English, a newer one to Turkish
	for _, d := range settingDefs() {
		if !hasMessage(d.name) {
			t.Errorf("setting %q has no English", d.name)
		}
	}
}

// Every message written through T or Tf has its English: a new one without it fails here, not on a user's screen.
func TestEnglishCoversMessages(t *testing.T) {
	srcs, _ := filepath.Glob("*.go")
	re := regexp.MustCompile(`\bTf?\(("(?:[^"\\]|\\.)*")`)
	var missing []string
	for _, f := range srcs {
		if strings.HasSuffix(f, "_test.go") || strings.HasPrefix(f, "i18n") {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			s, err := strconv.Unquote(m[1])
			if err != nil || !regexp.MustCompile(`[çğıöşüÇĞİÖŞÜ]`).MatchString(s) {
				continue
			}
			if !hasMessage(s) {
				missing = append(missing, s)
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("no English for: %q", missing)
	}
}

// What the window says outside T - a button's or a field's label, a placeholder, a line of text, a
// toast, an empty screen - has its English too: written through T, or translated where it is shown
// (a toast and an empty screen are), it must be in the dictionary.
func TestEnglishCoversScreen(t *testing.T) {
	srcs, _ := filepath.Glob("*.go")
	lit := `("(?:[^"\\]|\\.)*")`
	pats := []*regexp.Regexp{
		regexp.MustCompile(`ui\.Text\(c[^,]*, ` + lit), regexp.MustCompile(`\.Label\(` + lit), regexp.MustCompile(`a\.notice = ` + lit),
		regexp.MustCompile(`ryugo\.Button\(c[^,]*, ` + lit), regexp.MustCompile(`Label: ` + lit), regexp.MustCompile(`Placeholder: ` + lit),
		regexp.MustCompile(`emptyState\(c, [a-zA-Z]+, ` + lit + `, ` + lit), regexp.MustCompile(`Tooltip\(c, [a-zA-Z.]+, ` + lit),
	}
	turkish := regexp.MustCompile(`[çğıöşüÇĞİÖŞÜ]`)
	var missing []string
	for _, f := range srcs {
		if strings.HasSuffix(f, "_test.go") || strings.HasPrefix(f, "i18n") || strings.HasPrefix(f, "mcp") {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			for _, re := range pats {
				for _, m := range re.FindAllStringSubmatch(line, -1) {
					for _, g := range m[1:] {
						s, err := strconv.Unquote(g)
						if err != nil || !turkish.MatchString(s) {
							continue
						}
						if !hasMessage(s) && !hasMessage(strings.TrimSuffix(s, ": ")) {
							missing = append(missing, f+": "+s)
						}
					}
				}
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("no English for %d: %q", len(missing), missing)
	}
}

// hasMessage reports whether s is the id of a message, which msg says.
func hasMessage(s string) bool {
	_, _, ok := locales.Catalog.Message("en", s)
	return ok
}

// fmtVerbs are the verbs of Go's fmt in a message, which the catalogs keep until phase 2 (#331): the same in every
// language, whatever their order ("%[2]d of %[1]d").
var fmtVerbs = regexp.MustCompile(`%(?:\[\d+\])?[-+# 0]*\d*(?:\.\d+)?[a-zA-Z%]`)

func verbsOf(s string) string {
	var out []string
	for _, v := range fmtVerbs.FindAllString(s, -1) {
		out = append(out, regexp.MustCompile(`\[\d+\]`).ReplaceAllString(v, ""))
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}

// The catalogs of locales/: one per language of the window and no other; each has every message of the source,
// English, and nothing else; every message parses as ICU MessageFormat and has the same fmt verbs in every
// language; every message is said.
func TestCatalogs(t *testing.T) {
	files, _ := filepath.Glob("locales/*.json")
	cats := map[string]map[string]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		raw := map[string]json.RawMessage{}
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		tag := strings.TrimSuffix(filepath.Base(f), ".json")
		m := map[string]string{}
		for k, v := range raw {
			var s string
			if strings.HasPrefix(k, "@") && k != "@@locale" {
				continue // a message's description and placeholders
			}
			if err := json.Unmarshal(v, &s); err != nil {
				t.Errorf("%s: %s is not a string", f, k)
			}
			m[k] = s
		}
		if m["@@locale"] != tag {
			t.Errorf("%s says it is %q", f, m["@@locale"])
		}
		delete(m, "@@locale")
		cats[tag] = m
	}
	var codes []string
	for _, l := range languages {
		codes = append(codes, l.code)
		if cats[l.code] == nil {
			t.Errorf("no catalog for %s", l.code)
		}
	}
	for tag := range cats {
		if !slices.Contains(codes, tag) {
			t.Errorf("locales/%s.json is no language of the window", tag)
		}
	}
	en := cats["en"]
	for tag, m := range cats {
		for id, msg := range m {
			if _, ok := en[id]; !ok {
				t.Errorf("%s: %s is not in the source", tag, id)
				continue
			}
			if _, err := i18n.ParseMessage(msg); err != nil {
				t.Errorf("%s: %s: %v", tag, id, err)
			}
			if verbsOf(msg) != verbsOf(en[id]) {
				t.Errorf("%s: %s has the verbs %q, the source %q", tag, id, verbsOf(msg), verbsOf(en[id]))
			}
		}
		for id := range en {
			if _, ok := m[id]; !ok {
				t.Errorf("%s lacks %s", tag, id)
			}
		}
	}
	// every message is said: called by its function (locales.UpdateTitle()), given as one, or kept by its id in a
	// table ("collectionSettings", "moveToTrash: " + a title)
	var code strings.Builder
	srcs, _ := filepath.Glob("*.go")
	for _, f := range srcs {
		if !strings.HasSuffix(f, "_test.go") && f != "i18n_keys.go" {
			b, _ := os.ReadFile(f)
			code.Write(b)
		}
	}
	// the Worker's errors come to the app in Turkish and are said through T (account.go)
	for _, f := range []string{"cloudflare/sync/auth.js", "cloudflare/sync/worker.js"} {
		b, _ := os.ReadFile(f)
		code.Write(b)
	}
	all := code.String()
	for id := range en {
		if !regexp.MustCompile(`\.`+goName(id)+`\b`).MatchString(all) && !strings.Contains(all, `"`+id+`"`) && !strings.Contains(all, `"`+id+`: `) {
			t.Errorf("%s is said nowhere", id)
		}
	}
}

// goName is the name of the function of message id in locales/catalog_gen.go, as mygo generate makes it: every
// part between _ begun with a capital.
func goName(id string) string {
	var b strings.Builder
	for _, w := range strings.Split(id, "_") {
		if w != "" {
			b.WriteString(strings.ToUpper(w[:1]) + w[1:])
		}
	}
	return b.String()
}

// shownCalls are the functions whose string arguments the window shows; shownFields the fields of options that it
// shows. A literal with letters given to them is a word the catalogs do not translate.
var (
	shownCalls  = map[string]bool{"Text": true, "Button": true, "MenuButton": true, "Label": true, "iconButton": true, "barButton": true, "barIcon": true, "settingRow": true, "settingStack": true, "statHead": true, "statTile": true, "emptyState": true, "Tooltip": true, "panelTitle": true, "menuAt": true, "Checkbox": true, "Subtitle": true, "Item": true, "Submenu": true, "say": true}
	shownFields = map[string]bool{"Label": true, "Title": true, "Placeholder": true, "Help": true, "Description": true, "title": true, "sub": true, "off": true, "group": true}
	// keyLike are the keys of elements and the names of files, which are not shown: "st-today", "collection.json"
	keyLike = regexp.MustCompile(`^[a-z\-:][a-z0-9:_.+\-]*$`)
	// notWords are literals shown as they are in every language: names, signs, keys.
	notWords = regexp.MustCompile(`^(Celestine|MCP|Markdown|Claude Code|JSON|CSV|HTML|URL|CRT|Mizan|Slime|Pacman|Back-man|Shredder|Portal|Aurora|Neon|Shimmer|Press waves|Cursor FX|Time-lapse|v\d.*|[^\pL]*|[⇧⌃⌥⌘]+\S{0,3}|NORMAL|%[.\d]*f [KMG]B|[DMYd, ]+|[A-Z]{1,4}|#.*|\[\[.*)$`)
)

// No word is shown that the catalogs cannot translate: a literal with letters, given to what the window shows, is a
// message (locales.Name()), or a message's id, which the helpers it is given to say with msg (#218).
func TestNoLiteralWords(t *testing.T) {
	srcs, _ := filepath.Glob("*.go")
	fset := token.NewFileSet()
	letters := regexp.MustCompile(`\pL{2}`)
	var found []string
	check := func(f string, n ast.Node) {
		bl, ok := n.(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			return
		}
		s, err := strconv.Unquote(bl.Value)
		if err != nil || !letters.MatchString(s) || notWords.MatchString(strings.TrimSpace(s)) || keyLike.MatchString(s) || hasMessage(s) || hasMessage(strings.TrimSuffix(s, ": ")) {
			return
		}
		found = append(found, fset.Position(bl.Pos()).String()+": "+s)
	}
	// every literal in what is shown, not the expression alone: a concatenation ("New tag: #" + q), a map of two
	// words picked by a value, a literal inside a call's argument (#533); a function literal's body is its own
	inside := func(f string, e ast.Expr) {
		ast.Inspect(e, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			check(f, n)
			return true
		})
	}
	for _, f := range srcs {
		if strings.HasSuffix(f, "_test.go") || strings.HasPrefix(f, "i18n") || strings.HasPrefix(f, "mcp") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				name := ""
				switch fn := n.Fun.(type) {
				case *ast.Ident:
					name = fn.Name
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				}
				if shownCalls[name] {
					for _, arg := range n.Args {
						inside(f, arg)
					}
				}
			case *ast.KeyValueExpr:
				if k, ok := n.Key.(*ast.Ident); ok && shownFields[k.Name] {
					inside(f, n.Value)
				}
			}
			return true
		})
	}
	if len(found) > 0 {
		t.Fatalf("%d words the catalogs cannot translate:\n%s", len(found), strings.Join(found, "\n"))
	}
}

// turkishData are the files whose Turkish literals are data, not words shown: what the user types and the app reads
// (the query and the tasks' syntax, a table's "hayır", :set's names, the commands' Turkish search words), names a
// file has on disk (a backup's kind, a conflict's copy), the Worker's errors said through T, legacy names read from
// old settings and libraries, and the agents' help, which names the query's Turkish words.
var turkishData = map[string]bool{
	"account.go": true, "autocover.go": true, "backup.go": true, "caretfx.go": true, "cmdreg.go": true, "i18n.go": true,
	"importmd.go": true, "journal.go": true, "kb.go": true, "mcp_help.go": true, "mcp_server.go": true, "palette.go": true,
	"searchquery.go": true, "syncapp.go": true, "tablemodel.go": true, "tasks.go": true, "templates.go": true,
	"update.go": true, "watch.go": true,
}

// No word of Turkish is written in the code outside the catalogs but as data (turkishData): the English window shows
// none, wherever the literal goes, a variable, a map or a sum (#533). A letter only Turkish has tells it.
func TestNoTurkishLiterals(t *testing.T) {
	turkish := regexp.MustCompile(`[çğıöşüÇĞİÖŞÜ]`)
	srcs, _ := filepath.Glob("*.go")
	fset := token.NewFileSet()
	var found []string
	for _, f := range srcs {
		if strings.HasSuffix(f, "_test.go") || turkishData[f] {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			if bl, ok := n.(*ast.BasicLit); ok && bl.Kind == token.STRING {
				if s, err := strconv.Unquote(bl.Value); err == nil && turkish.MatchString(s) {
					found = append(found, fset.Position(bl.Pos()).String()+": "+s)
				}
			}
			return true
		})
	}
	if len(found) > 0 {
		t.Fatalf("%d Turkish literals out of the catalogs:\n%s", len(found), strings.Join(found, "\n"))
	}
}

// A tag's "#" in a plural message is the tag's, not the count's (#534).
func TestTagAddedMessage(t *testing.T) {
	if got := locales.In("en").BulkTagAdded(3, "urgent"); got != "Added #urgent to 3 pages" {
		t.Errorf("en: %q", got)
	}
	if got := locales.In("en").BulkTagAdded(1, "urgent"); got != "Added #urgent to 1 page" {
		t.Errorf("en, one: %q", got)
	}
	if got := locales.In("tr").BulkTagAdded(3, "acil"); got != "3 sayfaya #acil eklendi" {
		t.Errorf("tr: %q", got)
	}
}

// A template's words are English, its help names them, and the Turkish ones are still read (#537).
func TestTemplateWords(t *testing.T) {
	now := time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC)
	for _, body := range []string{"# {{title}}\n{{date}} {{today}} {{time}}", "# {{başlık}}\n{{tarih}} {{bugün}} {{saat}}"} {
		if got := expandTemplate(body, "Dune", now, nil); got != "# Dune\n[[2026-03-04]] [[2026-03-04]] 09:30" {
			t.Errorf("%q: %q", body, got)
		}
	}
	if help := locales.In("en").TemplatesEachChosenByName(); !strings.Contains(help, "{{title}}") || strings.Contains(help, "başlık") {
		t.Errorf("the English help: %q", help)
	}
	if help := locales.In("tr").TemplatesEachChosenByName(); !strings.Contains(help, "{{başlık}}") {
		t.Errorf("the Turkish help: %q", help)
	}
}

// A task that comes again says how often in the window's language (#533).
func TestRecurLabel(t *testing.T) {
	cases := map[recur]string{{n: 1, unit: "day"}: "every day", {n: 2, unit: "week"}: "every 2 weeks", {n: 3, unit: "month"}: "every 3 months", {n: 1, unit: "year"}: "every year"}
	withLang(t, "en")
	for r, want := range cases {
		if got := r.label(); got != want {
			t.Errorf("%+v: %q, want %q", r, got, want)
		}
	}
}

// Every hint of the editor names a message the catalogs have: an id they do not know is shown as it is, in any
// language ("move" for "move2").
func TestEditorHintsAreMessages(t *testing.T) {
	af, err := parser.ParseFile(token.NewFileSet(), "blockhints.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(af, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "add" {
			return true
		}
		if bl, ok := call.Args[1].(*ast.BasicLit); ok && bl.Kind == token.STRING {
			if s, _ := strconv.Unquote(bl.Value); !hasMessage(s) {
				t.Errorf("hint %q is no message", s)
			}
		}
		return true
	})
}
