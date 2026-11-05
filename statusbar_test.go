package main

import (
	"strings"
	"testing"
	"time"
)

func TestReadingText(t *testing.T) {
	for words, want := range map[int]string{0: "1 dk'dan az okuma", 99: "1 dk'dan az okuma", 100: "1 dk okuma", 200: "1 dk okuma", 201: "2 dk okuma", 1000: "5 dk okuma"} {
		if got := readingText(words); got != want {
			t.Errorf("%d: %q, want %q", words, got, want)
		}
	}
}

// TestStatusExtras: what the settings turn on is in the status bar.
func TestStatusExtras(t *testing.T) {
	a, tt := newTestApp(t, "bir iki üç\n\nikinci paragraf\n")
	tt.SetSize(1300, 700)
	other, _ := a.page.Coll.NewPage(nil, "Öteki")
	other.Save("bak [[" + a.page.Title + "]]\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	for _, w := range []string{"dk okuma", "geri bağlantı", "düzenlendi", "Satır 2", "harf seçili"} {
		if tt.HasText(w) {
			t.Fatalf("%q shown before it is asked for", w)
		}
	}
	s := &a.settings
	s.ShowReading, s.ShowPath, s.ShowSel, s.ShowPos, s.ShowEdited, s.ShowLinks = true, true, true, true, true, true
	a.doc.SetSelection(13, 13) // in the second paragraph
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	texts := strings.Join(tt.Texts(), "|")
	for _, w := range []string{"1 dk'dan az okuma", "1 geri bağlantı", "düzenlendi", "Satır 2, sütun", a.page.Coll.Name + " › " + a.page.Title} {
		if !strings.Contains(texts, w) {
			t.Fatalf("no %q: %v", w, tt.Texts())
		}
	}
	a.doc.SetSelection(0, 5)
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	if !tt.HasText("5 harf seçili") {
		t.Fatalf("selection: %v", tt.Texts())
	}
	// a click on the links opens the panel
	if err := tt.Click("Geri bağlantılar: yan paneli aç"); err != nil {
		t.Fatal(err)
	}
	if !s.PanelOpen {
		t.Fatal("panel not opened")
	}
}
