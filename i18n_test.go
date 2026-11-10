package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func withLang(t *testing.T, l string) {
	old := lang
	lang = l
	t.Cleanup(func() { lang = old })
}

// Turkish is what is written; English is what the dictionary has, and what it has not is Turkish, not a hole.
func TestEnglishLoses(t *testing.T) {
	for k, v := range enText {
		if strings.TrimSpace(v) == "" {
			t.Errorf("%q has an empty English", k)
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
	defer func() { lang = old }()
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
	for _, want := range []string{"New collection", "Tasks", "Trash", "Back up now", "Autosave: off", "Theme: Dark"} {
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
	lang = "en"
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
			_, tr := trText[m[1]] // written in English, its Turkish beside it
			if _, ok := enText[m[1]]; !ok && !tr && !strings.HasSuffix(m[1], ": ") {
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
			_, tr := trText[s]
			if _, ok := enText[s]; !ok && !tr {
				noHelp = append(noHelp, m[1])
			}
		}
	}
	if len(noHelp) > 0 {
		t.Fatalf("no English for the help lines: %q", noHelp)
	}
	// every setting of the list has its name translated: an older one to English, a newer one to Turkish
	for _, d := range settingDefs() {
		_, tr := trText[d.name]
		if _, ok := enText[d.name]; !ok && !tr {
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
			if _, ok := enText[s]; !ok {
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
						if _, ok := enText[s]; !ok && enText[strings.TrimSuffix(s, ": ")] == "" {
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

// Every string given to T has its other language: a Turkish one its English in enText, an English one (as new
// strings are written) its Turkish in trText. A string with neither shows in one language in both windows.
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
			_, en := enText[s]
			_, tr := trText[s]
			if !en && !tr {
				missing = append(missing, f+": "+s)
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("no other language for:\n%s", strings.Join(missing, "\n"))
	}
	for k := range trText {
		if _, ok := enText[k]; ok {
			t.Errorf("%q is in both enText and trText", k)
		}
	}
}
