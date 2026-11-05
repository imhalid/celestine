package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

func TestPlainText(t *testing.T) {
	got := plainText("# Başlık\n\n- **kalın** ve [[Sayfa|ad]] [bağ](http://x) ![resim](a.png)\n```go\nkod\n```\n> alıntı | hücre")
	if got != "Başlık kalın ve ad bağ resim kod alıntı hücre" {
		t.Fatalf("%q", got)
	}
}

func TestSnippet(t *testing.T) {
	text := strings.Repeat("uzun bir başlangıç ", 8) + "ŞEKER pancarı ve sonrası " + strings.Repeat("devam eden metin ", 12)
	segs := snippet(text, queryWords("seker"))
	var all, hit string
	for _, s := range segs {
		all += s.text
		if s.hit {
			hit += s.text
		}
	}
	if hit != "ŞEKER" || !strings.HasPrefix(all, "…") || !strings.HasSuffix(all, "…") || !strings.Contains(all, "pancarı") {
		t.Fatalf("hit %q in %q", hit, all)
	}
}

// TestLibrarySearch: every word, in the title or the text, Turkish letters as
// their ASCII; the title first; the palette and the view show the same.
func TestLibrarySearch(t *testing.T) {
	a, tt := newTestApp(t, "bu sayfada çay demlemek anlatılır\n")
	tt.SetSize(1100, 800)
	b, _ := a.page.Coll.NewPage(nil, "Çay notları")
	b.Save("sadece başlıkta çay var gibi\n")
	c, _ := a.page.Coll.NewPage(nil, "Kahve")
	c.Save("kahve ve çay birlikte, ayrıca demlemek\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	hits := a.searchPages("cay demlemek", 0, true)
	if len(hits) != 2 {
		t.Fatalf("%d hits", len(hits))
	}
	for _, h := range hits {
		if h.page == b {
			t.Fatal("a page without every word")
		}
	}
	if got := a.searchPages("çay", 0, true); len(got) != 3 || got[0].page != b {
		t.Fatalf("the title comes first: %d, first %q", len(got), got[0].page.Title)
	}
	// the palette has them after the names
	found := false
	for _, it := range a.paletteItems("demlemek") {
		found = found || strings.HasPrefix(it.sub, "Metinde")
	}
	if !found {
		t.Fatal("not in the palette")
	}
	// the view, from its key
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	tt.Advance(300 * time.Millisecond)
	if !a.searchOpen {
		t.Fatal("the key does not open it")
	}
	typeEach(tt, "demlemek")
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("Sonuç: Kahve") || !tt.HasText("2 sayfa") {
		t.Fatalf("view: %v", tt.Texts())
	}
	if err := tt.Click("Sonuç: Kahve"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(200 * time.Millisecond)
	if a.page != c || a.searchOpen {
		t.Fatalf("page %q", a.page.Title)
	}
	_, caret := a.doc.Selection()
	txt := a.doc.Text().String()
	if want := utf8.RuneCountInString(txt[:strings.Index(txt, "demlemek")]); caret != want {
		t.Fatalf("caret %d", caret)
	}
}
