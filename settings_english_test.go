package main

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// turkishWords are words that only Turkish text has, for the lines without a letter of Turkish's own.
var (
	turkishWords = regexp.MustCompile(`(?i)\b(ve|bir|ile|bu|için|yok|var|veya|ya da|olarak|sunucu|kapalı|açık|klasör|sayfa|sayfalar|koleksiyon|kitaplık|ayar|ayarlar|yazı|tema|gün|saat|dakika|her|hiç|daha|en|kez|göre|gibi|ama|yalnız|önce|sonra|olur|olsun)\b`)
	unixPathRe   = regexp.MustCompile(`(^|[\s"'(=,])(?:/[^\s"')\],]+)+`)
	winPathRe    = regexp.MustCompile(`(?:[A-Za-z]:[/\\]|\\\\)[^\s"')\],]+`)
	apoWordRe    = regexp.MustCompile(`['’]\pL+`)
	trLetterRe   = regexp.MustCompile(`[çğıöşüÇĞİÖŞÜ]`)
)

// turkish reports whether a line shown is Turkish: it has a letter of Turkish's own, or a word only Turkish has.
func turkish(s string) bool {
	s = winPathRe.ReplaceAllString(s, "")    // strip Windows drive/UNC paths like C:\Users\...\bir (#248)
	s = unixPathRe.ReplaceAllString(s, "$1") // strip Unix/macOS paths like /var/folders/... (#248)
	s = apoWordRe.ReplaceAllString(s, "")    // "you've": not the Turkish "ve"
	if trLetterRe.MatchString(s) {
		return true
	}
	// "en" is English too: only counted beside another Turkish word
	m := turkishWords.FindAllString(s, -1)
	m = slices.DeleteFunc(m, func(w string) bool { return strings.EqualFold(w, "en") })
	return len(m) > 0
}

// TestTurkishPathFilter checks that both Unix and Windows filesystem paths are stripped before
// flagging Turkish text while ordinary Turkish words still fail the check.
func TestTurkishPathFilter(t *testing.T) {
	for _, s := range []string{
		`/var/folders/xx/bir/ve/kitaplık`,
		`Library: C:\Users\halit\Belgeler\ve\ayarlar`,
		`Remote: D:/notlar/bir/sayfa`,
		`Share: \\server\paylaşım\bir`,
		`You've selected en`,
	} {
		if turkish(s) {
			t.Errorf("turkish(%q) = true, want false (path/English)", s)
		}
	}
	for _, s := range []string{
		`Kitaplık`,
		`bir/ve`,
		`her gün açık`,
	} {
		if !turkish(s) {
			t.Errorf("turkish(%q) = false, want true", s)
		}
	}
}

// In English, every tab of the settings says everything in English: the names, the lines under them, the
// buttons and the notes, with the MCP server on and off and each kind of sync.
func TestSettingsInEnglish(t *testing.T) {
	setLang("en")
	defer setLang("tr")
	a, tt := newTestApp(t, "")
	tt.SetSize(1180, 2400)
	tt.Frame()
	seen := map[string]bool{} // what the window shows outside the settings: the library's own names
	for _, s := range tt.Texts() {
		seen[s] = true
	}
	seen[langName("tr")] = true // a language is named in its own words
	a.openSettings()
	var found []string
	look := func(where string) {
		tt.Frame()
		tt.Advance(800 * time.Millisecond)
		tt.Frame()
		for _, s := range tt.Texts() {
			if turkish(s) && !seen[s] {
				seen[s] = true
				found = append(found, where+": "+s)
			}
		}
	}
	for tab := range settingsSections {
		a.settingsTab = tab
		look(msg(settingsSections[tab].name))
	}
	a.settingsTab = tabMCP
	a.settings.Port = 0
	a.setMCP(true)
	defer a.setMCP(false)
	look("MCP on")
	a.settingsTab = tabSync
	for _, mode := range []string{"dir", "server"} {
		a.settings.SyncMode = mode
		look("Sync " + mode)
	}
	a.settings.SyncMode = ""
	if len(found) > 0 {
		t.Fatalf("Turkish in the English settings:\n%s", strings.Join(found, "\n"))
	}
}
