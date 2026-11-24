package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestTagsOf(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"merhaba #fikir ve #Proje/alt.", []string{"fikir", "Proje/alt"}},
		{"# Başlık\n## Alt\nmetin", nil},
		{"(#x) a#b #1 #-", nil},
		{"```\n#include\n```\n`#kod` ama #gerçek", []string{"gerçek"}},
		{"#Fikir ve #fikir", []string{"Fikir"}},
		{"#çalışma #ölçü", []string{"çalışma", "ölçü"}},
	} {
		got := tagsOf(tc.in)
		if !equal(got, tc.want) {
			t.Errorf("%q: %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestTagView: a tag in a page's text lists in the palette and the side
// panel, is drawn as a pill that opens it, and its view lists its pages.
func TestTagView(t *testing.T) {
	a, tt := newTestApp(t, "bir #fikir burada\n")
	tt.SetSize(1100, 800)
	other, _ := a.page.Coll.NewPage(nil, "Öteki")
	other.SaveText("başka #Fikir ve #plan\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	if len(a.tagHits)+len(a.tagSeen) == 0 {
		t.Fatal("the tag is not drawn")
	}
	all := a.allTags()
	if len(all) != 2 || all[0].key != "fikir" || len(all[0].pages) != 2 || all[1].key != "plan" {
		t.Fatalf("tags: %+v", all)
	}
	found := false
	for _, it := range a.paletteItems("fikir") {
		found = found || it.title == "#fikir"
	}
	if !found {
		t.Fatal("not in the palette")
	}
	// the side panel lists the page's tags
	a.settings.PanelOpen = true
	tt.Advance(600 * time.Millisecond)
	if !tt.HasText("ETİKETLER") {
		t.Fatalf("no tags in the panel: %v", tt.Texts())
	}
	// a click on the tag in the text opens its view
	h := a.tagSeen[0]
	tt.ClickAt(h.r.X+h.r.W/2, h.r.Y+h.r.H/2)
	tt.Advance(300 * time.Millisecond)
	if a.tag != "fikir" || !tt.HasText("#fikir") || !tt.HasText("Sayfa: Öteki") {
		t.Fatalf("tag %q: %v", a.tag, tt.Texts())
	}
	// a page of it opens; back returns to the tag
	if err := tt.Click("Sayfa: Öteki"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if a.tag != "" || a.page != other {
		t.Fatalf("page %v tag %q", a.page.Title, a.tag)
	}
	a.step(-1)
	tt.Advance(100 * time.Millisecond)
	if a.tag != "fikir" {
		t.Fatalf("back: %q", a.tag)
	}
}

// TestTagMenu: a # typed in a line offers the tags there are; ↵ takes one,
// or the new name typed; at the start of a line it is a heading's, and offers nothing.
func TestTagMenu(t *testing.T) {
	a, tt := newTestApp(t, "ilk\n")
	tt.SetSize(1100, 800)
	other, _ := a.page.Coll.NewPage(nil, "Öteki")
	other.SaveText("başka #fikir ve #plan\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	a.doc.SetSelection(3, 3)
	tt.Frame()
	typeEach(tt, " gün ")
	tt.Type("#")
	tt.Advance(100 * time.Millisecond)
	if !a.slash.open || a.slash.sign != "#" || !tt.HasText("#fikir") || !tt.HasText("#plan") {
		t.Fatalf("no menu: open %v %v", a.slash.open, tt.Texts())
	}
	typeEach(tt, "fi")
	tt.Advance(100 * time.Millisecond)
	if tt.HasText("#plan") {
		t.Fatalf("not narrowed: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Advance(100 * time.Millisecond)
	if got := a.doc.Text().String(); got != "ilk gün #fikir \n" && got != "ilk gün #fikir " {
		t.Fatalf("text %q", got)
	}
	if a.slash.open {
		t.Fatal("the menu stays")
	}
	// a name that is new
	typeEach(tt, "ve #yeni")
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Yeni etiket: #yeni") {
		t.Fatalf("no new: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Advance(100 * time.Millisecond)
	if got := a.doc.Text().String(); !strings.Contains(got, "#fikir ve #yeni ") {
		t.Fatalf("text %q", got)
	}
	// a space in the name closes it
	typeEach(tt, "de #fi bir")
	tt.Advance(100 * time.Millisecond)
	if a.slash.open {
		t.Fatal("open after a space")
	}
}

// TestHashAtLineStart: # at the start of a line is a heading's: no menu.
func TestHashAtLineStart(t *testing.T) {
	a, tt := newTestApp(t, "")
	other, _ := a.page.Coll.NewPage(nil, "Öteki")
	other.SaveText("#fikir\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	typeEach(tt, "# Başlık")
	tt.Advance(100 * time.Millisecond)
	if a.slash.open {
		t.Fatal("a menu on a heading's #")
	}
	if st := a.doc.Text().ParagraphStyleAt(0); st.Heading != 1 {
		t.Fatalf("not a heading: %+v %q", st, a.doc.Text().String())
	}
}

func TestReplaceTag(t *testing.T) {
	body := encodeBody(importMarkdown("bir #Fikir ve #fikirler `#fikir` #fikir/alt\n```\n#fikir\n```\n(#fikir) x #FIKIR."))
	got, n := renameTagInBody(body, "fikir", "plan")
	want := encodeBody(importMarkdown("bir #plan ve #fikirler `#fikir` #fikir/alt\n```\n#fikir\n```\n(#fikir) x #plan."))
	if got != want || n != 2 {
		t.Fatalf("%d: %s", n, got)
	}
}

// TestTagRename: a tag is renamed in every page that has it, to a new name or
// into another tag; the dialog does it.
func TestTagRename(t *testing.T) {
	a, tt := newTestApp(t, "bir #fikir burada\n")
	tt.SetSize(1100, 800)
	b, _ := a.page.Coll.NewPage(nil, "İki")
	b.SaveText("#Fikir ve #plan\n")
	c3, _ := a.page.Coll.NewPage(nil, "Üç")
	c3.SaveText("#fikirler dokunulmaz\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	a.openTag("fikir")
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Yeniden adlandır"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("tasarım")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(300 * time.Millisecond)
	if a.tagRen.open {
		t.Fatalf("dialog stays: %q", a.tagRen.err)
	}
	if body, _ := a.page.Text(); !strings.Contains(body, "bir #tasarım burada") {
		t.Fatalf("open page file: %q", body)
	}
	if md := a.pageMarkdown(a.page, a.doc.Text()); !strings.Contains(md, "#tasarım") {
		t.Fatalf("open page text: %q", md)
	}
	if body, _ := b.Text(); !strings.Contains(body, "#tasarım ve #plan") {
		t.Fatalf("the other: %q", body)
	}
	if body, _ := c3.Text(); !strings.Contains(body, "#fikirler") {
		t.Fatalf("a longer tag changed: %q", body)
	}
	if a.tag != "tasarım" {
		t.Fatalf("view on %q", a.tag)
	}
	// merged into one that is there
	if n, err := a.renameTag("tasarım", "plan"); err != nil || n != 2 {
		t.Fatalf("merge: %d %v", n, err)
	}
	if got := a.tagPages("plan"); len(got) != 2 {
		t.Fatalf("pages of plan: %d", len(got))
	}
	if _, err := a.renameTag("plan", "1x"); err == nil {
		t.Fatal("a name that is no tag")
	}
}

// TestLinkContext: a page that links here is listed with the line that does.
func TestLinkContext(t *testing.T) {
	a, tt := newTestApp(t, "hedef sayfa\n")
	tt.SetSize(1100, 800)
	src, _ := a.page.Coll.NewPage(nil, "Kaynak")
	src.SaveText("ilk satır\n\nBu cümlede [[Sayfa|hedefe]] bağlanıyor, **kalın** bir de.\n\nson\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	segs := a.linkContext(src, a.page)
	var all, hit string
	for _, s := range segs {
		all += s.text
		if s.hit {
			hit += s.text
		}
	}
	if !strings.Contains(all, "Bu cümlede hedefe bağlanıyor, kalın bir de.") || strings.Contains(all, "[[") {
		t.Fatalf("%q", all)
	}
	_ = hit
	if got := a.linkContext(src, src); len(got) != 0 {
		t.Fatalf("a line that is no link: %v", got)
	}
}

// The dialog's rename reads and rewrites the pages away from the window: a page saved between the read and the write
// keeps what was saved, the tag renamed in it too (#195).
func TestTagRenameSoonKeepsAWrite(t *testing.T) {
	a, tt := newTestApp(t, "bir #fikir\n")
	b, _ := a.page.Coll.NewPage(nil, "İki")
	b.SaveText("#fikir ve fazlası\n")
	tt.Frame()
	done := make(chan int, 1)
	a.mu.Lock() // a frame: the rewrite waits for it to write
	a.renameTagSoon("fikir", "plan", func(n int, err error) {
		if err != nil {
			t.Error(err)
		}
		done <- n
	})
	time.Sleep(200 * time.Millisecond) // the pages read and rewritten meanwhile
	b.SaveText("#fikir, yazıldı arada\n")
	a.mu.Unlock()
	select {
	case n := <-done:
		if n != 2 {
			t.Fatalf("%d pages changed", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the rename did not end")
	}
	if body, _ := b.Text(); !strings.Contains(body, "#plan, yazıldı arada") {
		t.Fatalf("the page written meanwhile: %q", body)
	}
}

// TestTagPillsKeptLaidOut: the pills of the tags in view are drawn from layouts kept for the text's version, not laid
// out again each frame (#204): the theme the editor is given is made anew each frame, and was the key.
func TestTagPillsKeptLaidOut(t *testing.T) {
	a, tt := newTestApp(t, strings.Repeat("Bir paragraf #etiket ve [bağlantı](https://example.com) ile biraz metin.\n\n", 30))
	tt.SetSize(1100, 900)
	tt.Advance(time.Second)
	if len(a.tagHits) == 0 {
		t.Fatal("no pills drawn")
	}
	before := paraLayouts
	for range 5 {
		tt.Frame()
	}
	if n := paraLayouts - before; n > 0 {
		t.Fatalf("%d paragraphs laid out again in five frames of the same text", n)
	}
}
