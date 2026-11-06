package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestStatusExtras: what the settings turn on is in the status bar; what was taken away of it
// (the path, the reading time, the selected letters, the words, the links, the last edit,
// today's count) is not, whatever an old settings file says.
func TestStatusExtras(t *testing.T) {
	a, tt := newTestApp(t, "bir iki üç\n\nikinci paragraf\n")
	tt.SetSize(1300, 700)
	tt.Frame()
	if tt.HasText("Satır 2") {
		t.Fatalf("the caret's place is shown before it is asked for")
	}
	a.settings.ShowPos = true
	a.doc.SetSelection(13, 13) // in the second paragraph
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if texts := strings.Join(tt.Texts(), "|"); !strings.Contains(texts, "Satır 2, sütun") {
		t.Fatalf("no caret place: %v", tt.Texts())
	}
	a.doc.SetSelection(0, 5)
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	for _, gone := range []string{"harf seçili", "dk okuma", "geri bağlantı", "düzenlendi", "kelime"} {
		if strings.Contains(strings.Join(tt.Texts(), "|"), gone) {
			t.Fatalf("%q is in the status bar: %v", gone, tt.Texts())
		}
	}
}

// An old settings file with the keys that were taken away opens without trouble.
func TestOldStatusKeysIgnored(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	if err := json.Unmarshal([]byte(`{"showWords":true,"showPath":true,"showToday":true,"showLinks":true,"showEdited":true,"showSel":true,"showReading":true,"showPos":true}`), &a.settings); err != nil {
		t.Fatal(err)
	}
	tt.SetSize(1000, 600)
	tt.Frame()
	if !a.settings.ShowPos {
		t.Fatal("the kept setting was lost")
	}
}
