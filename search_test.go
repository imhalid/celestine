package main

import (
	"slices"
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
	b.SaveText("sadece başlıkta çay var gibi\n")
	c, _ := a.page.Coll.NewPage(nil, "Kahve")
	c.SaveText("kahve ve çay birlikte, ayrıca demlemek\n")
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
		found = found || (it.group == "Metinde" && len(it.snip) > 0)
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

func TestParseQuery(t *testing.T) {
	q := parseQuery(`çay "sıcak su" başlık:Notlar koleksiyon:Test #fikir etiket:Plan sadece`)
	if !equal(q.words, []string{"cay", "sadece"}) || !equal(q.phrases, []string{"sicak su"}) || !equal(q.title, []string{"notlar"}) ||
		!equal(q.colls, []string{"test"}) || !equal(q.tags, []string{"fikir", "plan"}) {
		t.Fatalf("%+v", q)
	}
	if !parseQuery("  ").empty() || parseQuery("#a").empty() || !parseQuery("( )").empty() {
		t.Fatal("empty")
	}
	// the tree, written back as it was read
	var show func(n *qnode) string
	show = func(n *qnode) string {
		var parts []string
		for _, k := range n.kids {
			parts = append(parts, show(k))
		}
		switch n.kind {
		case qAnd:
			return "(" + strings.Join(parts, " ") + ")"
		case qOr:
			return "(" + strings.Join(parts, " | ") + ")"
		case qNot:
			return "!" + parts[0]
		case qPhrase:
			return `"` + n.text + `"`
		case qTag:
			return "#" + n.text
		case qTodo:
			return "@todo"
		case qDone:
			return "@done"
		case qToday:
			return "@today"
		case qCreated, qModified, qProp:
			return map[qkind]string{qCreated: "created", qModified: "modified", qProp: n.text}[n.kind] + n.op + n.val
		case qTitle:
			return "title:" + n.text
		case qColl:
			return "in:" + n.text
		}
		return n.text
	}
	for q, want := range map[string]string{
		"çay kahve":                         "(cay kahve)",
		"çay OR kahve":                      "(cay | kahve)",
		"çay VEYA kahve süt":                "(cay | (kahve sut))",
		"çay or kahve":                      "(cay or kahve)", // a small or is a word
		"-çay kahve":                        "(!cay kahve)",
		"NOT çay DEĞİL #fikir":              "(!cay !#fikir)",
		"(çay OR kahve) süt":                "((cay | kahve) sut)",
		"(çay OR (kahve süt)) -\"x y\"":     "((cay | (kahve sut)) !\"x y\")",
		"çay çay ÇAY":                       "cay",
		"@todo @DONE @bugün":                "(@todo @done @today)",
		"created:>2026-11-01":               "created>2026-11-01",
		"modified:2026-11-01..2026-11-30":   "modified:2026-11-01..2026-11-30",
		"Öncelik>=2 durum:\"Devam ediyor\"": "(oncelik>=2 durum:Devam ediyor)",
		"(çay":                              "cay",
		"çay) kahve":                        "(cay kahve)",
		"in:Notlar başlık:x":                "(in:notlar title:x)",
	} {
		if got := show(parseQuery(q).root); got != want {
			t.Errorf("%s: %s, not %s", q, got, want)
		}
	}
	// only what every page found has narrows the index; the words of an OR are still marked
	q = parseQuery(`(çay OR kahve) süt -şeker`)
	if !equal(q.words, []string{"sut"}) || !equal(q.marks(), []string{"cay", "kahve", "sut"}) {
		t.Fatalf("words %v, marks %v", q.words, q.marks())
	}
}

// TestSearchLanguage: OR, NOT and groups; tasks, days and properties are looked up on the page.
func TestSearchLanguage(t *testing.T) {
	a, tt := newTestApp(t, "çay demlenir\n\n- [ ] demlik al\n")
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	a.clock = func() time.Time { return now }
	coll := a.page.Coll
	b, _ := coll.NewPage(nil, "Kahve")
	b.SaveText("kahve ve süt\n\n- [x] fincan yıka\n")
	c, _ := coll.NewPage(nil, "Süt")
	c.SaveText("yalnız süt\n")
	pr, err := coll.AddProperty("Öncelik", PropNumber, nil)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := coll.AddProperty("Durum", PropChoice, []string{"Yapılacak", "Sürüyor", "Bitti"})
	b.SetValue(pr.ID, "3")
	c.SetValue(pr.ID, "1")
	b.SetValue(st.ID, "Sürüyor")
	c.SetValue(st.ID, "Bitti")
	a.page.Created, a.page.Modified = now.AddDate(0, 0, -20), now.AddDate(0, 0, -10)
	b.Created, b.Modified = now.AddDate(0, 0, -5), now.Add(-time.Hour)
	c.Created, c.Modified = now.AddDate(0, -2, 0), now.AddDate(0, -2, 0)
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	for _, ready := range []bool{false, true} {
		if ready {
			for !a.sx.warm(a, time.Second) {
			}
		}
		a.sx.ready = ready
		titles := func(q string) string {
			var out []string
			for _, h := range a.searchPages(q, 0, true) {
				out = append(out, h.page.Title)
			}
			slices.Sort(out)
			return strings.Join(out, ",")
		}
		self := a.page.Title
		for q, want := range map[string]string{
			"çay OR kahve":                    "Kahve," + self,
			"süt -kahve":                      "Süt",
			"süt NOT (kahve OR yok)":          "Süt",
			"(çay VEYA yalnız) -demlik":       "Süt",
			"@todo":                           self,
			"@done":                           "Kahve",
			"@today":                          "Kahve",
			"created:>2026-11-01":             "Kahve",
			"created:<=2026-10-20":            self + ",Süt",
			"modified:2026-10-25..2026-11-05": self,
			"modified:2026-11-09":             "Kahve",
			"created:dün":                     "",
			"öncelik>2":                       "Kahve",
			"oncelik<=1":                      "Süt",
			"öncelik:3":                       "Kahve",
			"durum:sürüyor":                   "Kahve",
			"durum<bitti":                     "Kahve",
			"durum:\"Bitti\" OR @todo":        self + ",Süt",
			"yok:kahve":                       "",
		} {
			if got := titles(q); got != want {
				t.Errorf("index %v: %s: %q, not %q", ready, q, got, want)
			}
		}
	}
}

// TestSearchFilters: a phrase is found only whole; başlık: looks at titles;
// koleksiyon: and #etiket narrow it.
func TestSearchFilters(t *testing.T) {
	a, tt := newTestApp(t, "sıcak su ve çay\n")
	other, _ := a.lib.NewCollection("Öteki")
	x, _ := other.NewPage(nil, "Çay rehberi")
	x.SaveText("su sıcak olmalı #çay\n")
	y, _ := a.page.Coll.NewPage(nil, "Üçüncü")
	y.SaveText("çay ve #fikir\n")
	a.linksStale()
	a.links.built = time.Time{}
	tt.Frame()
	n := func(q string) int { return len(a.searchPages(q, 0, true)) }
	if n("sıcak su") != 2 || n(`"sıcak su"`) != 1 {
		t.Fatalf("phrase: %d %d", n("sıcak su"), n(`"sıcak su"`))
	}
	if n("başlık:rehberi") != 1 || n("başlık:sıcak") != 0 {
		t.Fatalf("title: %d %d", n("başlık:rehberi"), n("başlık:sıcak"))
	}
	if n("çay koleksiyon:öteki") != 1 || n("çay koleksiyon:test") != 2 {
		t.Fatalf("collection: %d %d", n("çay koleksiyon:öteki"), n("çay koleksiyon:test"))
	}
	if n("#fikir") != 1 || n("çay #çay") != 1 || n("#yok") != 0 {
		t.Fatalf("tags: %d %d %d", n("#fikir"), n("çay #çay"), n("#yok"))
	}
}
