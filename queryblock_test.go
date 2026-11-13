package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// queryChips are the query blocks in the editor's text.
func queryChips(a *app) []*chip {
	var out []*chip
	t := a.doc.Text()
	for i, r := range []rune(t.String()) {
		if r != richtext.ObjectReplacement {
			continue
		}
		if att := t.AttributesAt(i).Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "query" {
				out = append(out, ch)
			}
		}
	}
	return out
}

// TestQueryBlock: a ```query block is a block of the pages its query finds, kept in the body as its query and
// written back as the fenced block; a row opens its page, and the results follow the library.
func TestQueryBlock(t *testing.T) {
	a, tt := newTestApp(t, "önce\n\n```query\n#proje   @todo\n```\n\nsonra\n")
	tt.SetSize(1100, 800)
	x, _ := a.page.Coll.NewPage(nil, "Proje X")
	x.SaveText("#proje\n\n- [ ] yaz\n")
	y, _ := a.page.Coll.NewPage(nil, "Proje Y")
	y.SaveText("#proje bitti\n")
	a.docs = map[*Page]*richtext.Document{}
	self := a.page
	a.page = nil
	a.open(self)
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	qs := queryChips(a)
	if len(qs) != 1 || qs[0].target != "#proje @todo" {
		t.Fatalf("blocks %d %v", len(qs), qs)
	}
	if hits := a.queryHits(qs[0].target); len(hits) != 1 || hits[0].page != x {
		t.Fatalf("hits %d", len(hits))
	}
	// the body keeps the query; Markdown is the fenced block
	body := encodeBody(a.doc.Text())
	if !strings.Contains(body, `"o":{"k":"query","q":"#proje @todo"}`) {
		t.Fatalf("body %s", body)
	}
	if back := queryChips2(decodeBody(body)); back != "#proje @todo" {
		t.Fatalf("read back %q", back)
	}
	md := a.pageMarkdown(a.page, a.doc.Text())
	if !strings.Contains(md, "\n```query\n#proje @todo\n```\n") || !strings.Contains(md, "sonra") {
		t.Fatalf("markdown:\n%s", md)
	}
	// a row opens its page
	tt.Frame()
	var hit *chipHit
	for i := range a.chipSeen {
		if a.chipSeen[i].ch.kind == "query" {
			hit = &a.chipSeen[i]
		}
	}
	if hit == nil {
		t.Fatal("the block is not drawn")
	}
	if want := queryHeight(1) - 6; hit.r.H != want {
		t.Fatalf("height %v, not %v", hit.r.H, want)
	}
	tt.ClickAt(hit.r.X+hit.r.W/2, hit.r.Y+queryHeadH+queryRowH/2)
	tt.Advance(200 * time.Millisecond)
	if a.page != x {
		t.Fatalf("page %q", a.page.Title)
	}
	// the library changes: so do the results, once the cache is old
	y.SaveText("#proje\n\n- [ ] oku\n")
	a.linksStale()
	a.links.built = time.Time{}
	a.linksTo(nil, a.clock(), true)
	if hits := a.queryHits("#proje @todo"); len(hits) != 2 {
		t.Fatalf("after a change: %d", len(hits))
	}
}

// queryChips2 is the query of the first query block in t.
func queryChips2(t richtext.Text) string {
	for _, r := range t.Runs(0, t.Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "query" {
				return ch.target
			}
		}
	}
	return ""
}

// TestQueryFromMenu: Ekle… ▸ Sorgu… puts the block of the query typed.
func TestQueryFromMenu(t *testing.T) {
	a, tt := newTestApp(t, "satır\n\n")
	tt.SetSize(1100, 800)
	p, _ := a.page.Coll.NewPage(nil, "Fikirler")
	p.SaveText("#fikir\n")
	a.linksStale()
	a.fresh, a.refocus = false, true
	a.doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	typeEach(tt, "/")
	typeEach(tt, "sorgu")
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Sorgu…") {
		t.Fatalf("no item: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Advance(100 * time.Millisecond)
	typeEach(tt, "#fikir")
	tt.Advance(100 * time.Millisecond)
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	if qs := queryChips(a); len(qs) != 1 || qs[0].target != "#fikir" {
		t.Fatalf("blocks %v; text %q", qs, a.doc.Text().String())
	}
}

// TestQueryBlockGrowsWithResults checks that query block height updates as matching pages are added and removed.
func TestQueryBlockGrowsWithResults(t *testing.T) {
	a, tt := newTestApp(t, "önce\n\n```query\n#proje @todo\n```\n\nsonra\n")
	tt.SetSize(1100, 800)
	x, _ := a.page.Coll.NewPage(nil, "Proje X")
	x.SaveText("#proje\n\n- [ ] bir\n")
	y, _ := a.page.Coll.NewPage(nil, "Proje Y")
	y.SaveText("#proje bitti\n")
	a.docs = map[*Page]*richtext.Document{}
	self := a.page
	a.page = nil
	a.open(self)
	a.linksStale()
	a.links.built = time.Time{}
	a.linksTo(nil, a.clock(), true)
	tt.Frame()

	findHit := func() *chipHit {
		for i := range a.chipSeen {
			if a.chipSeen[i].ch.kind == "query" {
				return &a.chipSeen[i]
			}
		}
		return nil
	}
	hit1 := findHit()
	if hit1 == nil {
		t.Fatal("query block not drawn initially")
	}
	if want := queryHeight(1) - 6; hit1.r.H != want {
		t.Fatalf("initial height %v, want %v", hit1.r.H, want)
	}

	// Add two more matching pages so the query grows to 3 rows.
	y.SaveText("#proje\n\n- [ ] iki\n")
	z, _ := a.page.Coll.NewPage(nil, "Proje Z")
	z.SaveText("#proje\n\n- [ ] üç\n")
	a.linksStale()
	a.links.built = time.Time{}
	a.linksTo(nil, a.clock(), true)
	tt.Frame()
	hit3 := findHit()
	if hit3 == nil {
		t.Fatal("query block missing after growth")
	}
	if want := queryHeight(3) - 6; hit3.r.H != want {
		t.Fatalf("grown height %v, want %v", hit3.r.H, want)
	}
}
