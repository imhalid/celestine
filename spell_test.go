package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/imhalid/celestine/internal/spell/model"
	"github.com/imhalid/celestine/locales"
)

// Turned on, the spell checker downloads its model once into the app's folder; turned off and on again it does not
// download it again; the settings delete it and download it again (#623).
func TestSpellModelDownloadedOnce(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	saved, savedBase := model.Files, spellBase
	defer func() { model.Files, spellBase = saved, savedBase }()
	body := "a tiny lexicon"
	sum := sha256.Sum256([]byte(body))
	model.Files = []model.File{{Name: "lexicon.bin", Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:])}}
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		w.Write([]byte(body))
	}))
	defer srv.Close()
	spellBase = srv.URL + "/"

	a, tt := newTestApp(t, "x\n")
	wait := func(want spellStatus) { // no frame drawn meanwhile: one would load the tiny files here as an engine
		t.Helper()
		for range 400 {
			var st spellStatus
			a.do(func() error { st = a.spell.status; return nil })
			if st == want {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("status %d, want %d (%s)", a.spell.status, want, a.spell.err)
	}
	a.setSpellCheck(true)
	wait(spellReady)
	a.settings.SpellCheck = false // the tiny files are no engine: not loaded by the frames to come
	if !model.Installed(model.Dir(spellRoot())) || asked.Load() != 1 || filepath.Dir(filepath.Dir(model.Dir(spellRoot()))) != spellRoot() {
		t.Fatalf("installed %v, asked %d", model.Installed(model.Dir(spellRoot())), asked.Load())
	}
	a.spell = spellState{} // as the app opens again: the device is looked at anew
	a.spellLook()
	if a.spell.status != spellReady || asked.Load() != 1 {
		t.Fatalf("downloaded again: status %d, asked %d", a.spell.status, asked.Load())
	}
	a.spellDelete()
	if a.spell.status != spellMissing || model.Installed(model.Dir(spellRoot())) {
		t.Fatal("not deleted")
	}
	a.spellDownload(true)
	wait(spellReady)
	if asked.Load() != 2 {
		t.Fatalf("asked %d", asked.Load())
	}
	// the settings show it, with its buttons
	tt.SetSize(1400, 6400)
	a.sidebarClosed = true
	a.pane().settingsOpen, a.pane().settingsTab = true, tabEditor
	tt.Advance(600 * time.Millisecond)
	for _, s := range []string{msg("spell_title"), locales.SpellDownloadAgain(), msg("common_delete")} {
		if _, ok := tt.Find(s); !ok {
			t.Fatalf("no %q in the settings", s)
		}
	}
	if err := tt.Click(msg("common_delete")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.spell.status != spellMissing {
		t.Fatal("the settings' Delete did not delete it")
	}
	if _, ok := tt.Find(locales.SpellDownload()); !ok {
		t.Fatal("no Download once deleted")
	}
}

// With the model on the device (TINYFIX_DATA, the files of model.Files), a word is fixed as the space after it is
// typed, an undo leaves it as typed, a word found in the text is underlined with a wave, and the editor's menu over
// it offers its fix (#623).
func TestSpellInTheEditor(t *testing.T) {
	data := os.Getenv("TINYFIX_DATA")
	if data == "" {
		t.Skip("TINYFIX_DATA: the folder of tinyfix-tr's files")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := model.Dir(spellRoot())
	os.MkdirAll(dir, 0o755)
	for _, f := range model.Files {
		b, err := os.ReadFile(filepath.Join(data, f.Name))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, f.Name), b, 0o644)
	}
	a, tt := newTestApp(t, "Ankaraya herkez geldi.\n")
	tt.SetSize(1000, 700)
	a.settings.SpellCheck = true
	for range 400 { // the engine loads, and the paragraph is checked, outside the frame
		tt.Frame()
		if a.spellEng.eng != nil && len(a.spellEng.checks) > 0 && len(a.spellEng.queue) == 0 && !a.spellEng.working {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if a.spellEng.eng == nil {
		t.Fatalf("the engine did not load: status %d, looked %v, installed %v, loading %v, %s", a.spell.status, a.spell.looked, model.Installed(dir), a.spellEng.loading, a.spell.err)
	}
	tt.Frame()
	waved := ""
	for _, s := range a.paintSpans(a.pane(), 0, a.pane().doc.Text().ParagraphStyleAt(0), spansOfPara(a, 0)) {
		if s.WavyUnderline {
			waved += s.Text + "|"
		}
	}
	if !strings.Contains(waved, "herkez") || !strings.Contains(waved, "Ankaraya") {
		t.Fatalf("underlined: %q", waved)
	}
	tt.Frame() // where the words are drawn, for the menu
	tt.Frame()
	var at int = -1
	for _, h := range a.pane().spellSeen {
		if h.at == strings.Index(a.pane().doc.Text().String(), "herkez") {
			at = h.at
		}
	}
	items := a.spellItems(at)
	if len(items) == 0 || items[0].title != "herkes" {
		t.Fatalf("the menu over herkez: %+v", items)
	}
	items[0].run()
	tt.Frame()
	if !strings.Contains(a.pane().doc.Text().String(), "herkes geldi") {
		t.Fatalf("the fix chosen: %q", a.pane().doc.Text().String())
	}
	// typed: fixed as the space comes
	pn := a.pane()
	pn.doc.SetSelection(pn.doc.Text().Len(), pn.doc.Text().Len())
	tt.Frame()
	typeEach(tt, " Bugün cok ")
	tt.Frame()
	if got := pn.doc.Text().String(); !strings.HasSuffix(got, "Bugün çok ") {
		t.Fatalf("typed: %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ) // the fix taken back: the word as typed
	tt.Frame()
	if got := pn.doc.Text().String(); !strings.HasSuffix(got, "Bugün cok ") {
		t.Fatalf("undone: %q", got)
	}
}
