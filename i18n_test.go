package main

import (
	"encoding/json"
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

// Turkish is what is written; English is what the dictionary has, and what it has not is Turkish, not a hole.
func TestEnglishLoses(t *testing.T) {
	for code, id := range codeKeys {
		if strings.TrimSpace(locales.Catalog.Format("en", id, nil)) == "" {
			t.Errorf("%q has an empty English", code)
		}
	}
	withLang(t, "tr")
	if T("Ayarlar") != "Ayarlar" {
		t.Fatal("Turkish changed")
	}
	withLang(t, "en")
	for tr, en := range map[string]string{"Ayarlar": "Settings", "Çöpe taşı: Sayfa": "Move to trash: Sayfa", "Hiç çevrilmemiş cümle": "Hiç çevrilmemiş cümle", "Tema": "Theme"} {
		if got := T(tr); got != en {
			t.Errorf("T(%q) = %q, want %q", tr, got, en)
		}
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
	a.statsScreen, a.settingsOpen = true, false
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

// The tour shows once a real library has settings, and not again after it is closed.
func TestTour(t *testing.T) {
	withLang(t, "tr")
	a, tt := newTestApp(t, "x\n")
	a.settingsFile = t.TempDir() + "/s.json"
	a.settings.AccountUser = "ali@ornek.com" // the sign-in comes before the tour (TestLoginGate)
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Yaz") || !tt.HasText("Bir daha gösterme") {
		t.Fatalf("no tour: %v", tt.Texts())
	}
	tt.Click("İleri")
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Her şey ⌘P'de") {
		t.Fatalf("not the next step: %v", tt.Texts())
	}
	tt.Click("Bir daha gösterme")
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if tt.HasText("Bir daha gösterme") || !a.settings.TourDone {
		t.Fatal("the tour did not close")
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
			if _, ok := codeKeys[s]; !ok {
				noHelp = append(noHelp, m[1])
			}
		}
	}
	if len(noHelp) > 0 {
		t.Fatalf("no English for the help lines: %q", noHelp)
	}
	// every setting of the list has its name translated: an older one to English, a newer one to Turkish
	for _, d := range settingDefs() {
		if _, ok := codeKeys[d.name]; !ok {
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

// Every string given to T has a message (codeKeys), which every language has (TestCatalogs): a string with none
// shows as it is written in every window.
func TestTurkishCovers(t *testing.T) {
	srcs, _ := filepath.Glob("*.go")
	re := regexp.MustCompile(`\bTf?\(("(?:[^"\\]|\\.)*")`)
	letters := regexp.MustCompile(`[A-Za-z]{2}`)
	var missing []string
	for _, f := range srcs {
		if strings.HasSuffix(f, "_test.go") || strings.HasPrefix(f, "i18n") {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			s, err := strconv.Unquote(m[1])
			if err != nil || !letters.MatchString(s) {
				continue
			}
			if !hasMessage(s) {
				missing = append(missing, f+": "+s)
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("no other language for:\n%s", strings.Join(missing, "\n"))
	}
}

// hasMessage reports whether the code's text s has a message in the catalogs.
func hasMessage(s string) bool {
	_, ok := codeKeys[s]
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
// language; every text of codeKeys names a message, each its own.
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
	named := map[string]string{}
	for code, id := range codeKeys {
		if _, ok := en[id]; !ok {
			t.Errorf("%q names %s, which is no message", code, id)
		}
		if other, ok := named[id]; ok {
			t.Errorf("%q and %q name the same message %s", code, other, id)
		}
		named[id] = code
	}
	// every message is said: called by its function (locales.UpdateTitle()), or still found through codeKeys; and
	// every text of codeKeys is still written somewhere in the code
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
		if _, ok := named[id]; !ok && !regexp.MustCompile(`\.`+goName(id)+`\b`).MatchString(all) { // called, or given as a function
			t.Errorf("%s is said nowhere", id)
		}
	}
	for text := range codeKeys {
		q := strconv.Quote(text)
		written := strings.Contains(all, q) || strings.Contains(all, "`"+text+"`") ||
			strings.Contains(all, `"`+text+`"`) || // in JavaScript
			strings.Contains(all, q[:len(q)-1]+`: `) || // a notice with a value after it: "Güncellenemedi: " + err
			text == "geri yükleme" // a backup's kind, in the names of files older versions wrote (backup.go)
		if !written {
			t.Errorf("codeKeys has %q, which the code no longer writes", text)
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

// A text the code gives T comes back as it was written in its own language, and in the other as its catalog
// says it: the catalogs were made from the dictionaries without a word changed (#330).
func TestCatalogsAsWritten(t *testing.T) {
	for _, c := range []struct{ code, l, want string }{
		{"Ayarlar", "tr", "Ayarlar"}, {"Ayarlar", "en", "Settings"},
		{"Back to the text", "en", "Back to the text"}, {"Back to the text", "tr", "Metne dön"},
		{"İleri", "en", "Forward"}, {"Yeniden başlat", "tr", "Yeniden başlat"},
	} {
		withLang(t, c.l)
		if got := T(c.code); got != c.want {
			t.Errorf("%s: T(%q) = %q, want %q", c.l, c.code, got, c.want)
		}
	}
	// a sentence with values is one message, its counts said with the language's plural forms
	for _, c := range []struct{ l, got, want string }{
		{"en", locales.In("en").SyncCollectionsSynced(5, 2), "2 of 5 collections are synced."},
		{"en", locales.In("en").SyncCollectionsSynced(1, 1), "1 of 1 collection is synced."},
		{"tr", locales.In("tr").SyncCollectionsSynced(5, 2), "5 koleksiyondan 2 tanesi eşitleniyor."},
		{"en", locales.In("en").CommonPageCount(1), "1 page"}, {"en", locales.In("en").CommonPageCount(1200), "1,200 pages"},
		{"tr", locales.In("tr").CommonPageCount(1200), "1.200 sayfa"},
	} {
		if c.got != c.want {
			t.Errorf("%s: %q, want %q", c.l, c.got, c.want)
		}
	}
}
