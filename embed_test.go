package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// embeds counts the cards in the editor's text.
func embeds(a *app) (n int, titles []string) {
	t := a.pane().doc.Text()
	for i, r := range []rune(t.String()) {
		if r != richtext.ObjectReplacement {
			continue
		}
		if att := t.AttributesAt(i).Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "embed" {
				n++
				titles = append(titles, ch.target)
			}
		}
	}
	return
}

// TestEmbed: a line ![[Page]] by itself is a card of that page, written back
// as the line; it follows the page, and opens it.
func TestEmbed(t *testing.T) {
	a, tt := newTestApp(t, "önce\n\n![[Hedef Sayfa]]\n\nsonra ![[Hedef Sayfa]] aynı satırda\n")
	tt.SetSize(1100, 800)
	target, _ := a.pane().page.Coll.NewPage(nil, "Hedef Sayfa")
	target.SaveText("# Hedef Sayfa\n\nilk satır **kalın**\n\n- madde bir\n- [ ] görev\n")
	a.docs = map[*Page]*richtext.Document{}
	other := a.pane().page
	a.pane().page = nil
	a.open(other)
	a.linksStale()
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	n, titles := embeds(a)
	if n != 1 || titles[0] != "Hedef Sayfa" {
		t.Fatalf("cards %d %v", n, titles)
	}
	md := a.pageMarkdown(a.pane().page, a.pane().doc.Text())
	if !strings.Contains(md, "\n![[Hedef Sayfa]]\n") || !strings.Contains(md, "sonra [[Hedef Sayfa]] aynı satırda") && !strings.Contains(md, "sonra ![[Hedef Sayfa]]") {
		t.Fatalf("markdown:\n%s", md)
	}
	// the card has the page's first lines
	lines := a.embedLines(target)
	if len(lines) != 3 || lines[0] != "ilk satır **kalın**" {
		t.Fatalf("lines %q", lines)
	}
	// it is clicked: the page
	tt.Frame()
	var hit *chipHit
	for i := range a.chipSeen {
		if a.chipSeen[i].ch.kind == "embed" {
			hit = &a.chipSeen[i]
		}
	}
	if hit == nil {
		t.Fatal("the card is not drawn")
	}
	tt.ClickAt(hit.r.X+hit.r.W/2, hit.r.Y+hit.r.H/2)
	tt.Advance(200 * time.Millisecond)
	if a.pane().page != target {
		t.Fatalf("page %q", a.pane().page.Title)
	}
}

// TestEmbedFromMenu: Ekle… ▸ Sayfa göm… puts the card of the page chosen.
func TestEmbedFromMenu(t *testing.T) {
	a, tt := newTestApp(t, "satır\n\n")
	tt.SetSize(1100, 800)
	target, _ := a.pane().page.Coll.NewPage(nil, "Gömülecek")
	target.SaveText("içerik\n")
	a.linksStale()
	a.pane().fresh, a.pane().refocus = false, true
	a.pane().doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	typeEach(tt, "/")
	typeEach(tt, "göm")
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Sayfa göm…") {
		t.Fatalf("no item: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Advance(100 * time.Millisecond)
	typeEach(tt, "gömül")
	tt.Advance(100 * time.Millisecond)
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	if n, titles := embeds(a); n != 1 || titles[0] != "Gömülecek" {
		t.Fatalf("cards %d %v; text %q", n, titles, a.pane().doc.Text().String())
	}
	if md := a.pageMarkdown(a.pane().page, a.pane().doc.Text()); !strings.Contains(md, "![[Gömülecek]]") {
		t.Fatalf("%q", md)
	}
}
