package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// The commands of the palette, with nothing typed after ">": in groups under headings, and a family
// of commands that differ by what comes after their ":" (Caret: …, Sayfayı taşı: …) as one line,
// which opens to its members. Typed, every command is found by itself, as before.

// cmdCats are the groups of the commands, in their order, each found by how its commands begin (in
// Turkish, before they are translated); a "=" first is the whole title. The first that fits is it.
var cmdCats = []struct {
	name  string
	start []string
}{
	{"Sayfa", []string{"Seçim", "Alt sayfa ekle", "Geri al", "Yinele", "Panodan tablo", "Çöpe taşı", "Sayfayı", "Bloğu", "Katla",
		"Sayfada bul", "=Kaydet", "Yeni sayfa", "Yeni alt sayfa", "Yeni günlük", "Bugünün sayfası", "Hızlı not"}},
	{"Koleksiyon", []string{"Koleksiyon", "Görünüm:", "Şablondan yeni", "Yeni koleksiyon"}},
	{"Bölmeler", []string{"Yan bölmede aç", "Yeni bölme", "Bölme", "Kenar çubuğu", "Yan panel"}},
	{"Git", []string{"=Görevler", "=İstatistikler", "=Hesap profili", "Rastgele gün", "=Ayarlar", "=Çöp", "Kitaplıkta ara", "=Geri", "=İleri"}},
	{"Dosyalar ve senkron", []string{"Senkron", "Şimdi eşitle", "Şimdi yedekle", "Yedek", "Markdown", "Ayar", "Son kayıttan",
		"Kayıt gecikmesi", "Otomatik kayıt", "Hepsini varsayılana"}},
	{"Yazı", []string{"Yazı", "Sayfa genişliği", "Satır", "Paragraf aralığı", "İlk satır", "Tab uzunluğu", "Kod yazı tipi", "Ligatür"}},
	{"Görünüm", []string{"Renk teması", "Tema", "Köşe", "Caret", "Jöle", "Odak", "Otomatik kapak", "Kısayol ipuçları", "Dil",
		"Profil", "Uygulama yazı tipi", "Sekmeleri göster", "Paragraf sonlarını", "Kod bloklarında"}},
	{"Uygulama", nil}, // the rest
}

// cmdCat is the group of a command, by its Turkish title.
func cmdCat(title string) string {
	for _, g := range cmdCats {
		for _, s := range g.start {
			if whole, ok := strings.CutPrefix(s, "="); ok && title == whole || !ok && strings.HasPrefix(title, s) {
				return g.name
			}
		}
	}
	return cmdCats[len(cmdCats)-1].name
}

// cmdFam is the family of a command: what its Turkish title says before ": ", "" for none.
func cmdFam(title string) string {
	if i := strings.Index(title, ": "); i > 0 {
		return title[:i]
	}
	return ""
}

// famFrom is the least a family has to be one line.
const famFrom = 3

// groupCommands lays the commands out with nothing typed: those for what is shown first, then the
// groups in their order, each family of famFrom or more as one line.
func (a *app) groupCommands(items []palItem) []palItem {
	var here []palItem
	byCat := map[string][]palItem{}
	fams := map[string]int{}
	for _, it := range items {
		if it.fam != "" {
			fams[it.cat+"\x00"+it.fam]++
		}
	}
	seen := map[string]bool{}
	for _, it := range items {
		key := it.cat + "\x00" + it.fam
		if it.fam != "" && fams[key] >= famFrom {
			if seen[key] {
				continue
			}
			seen[key] = true
			it = a.famItem(it, fams[key])
		}
		if it.boost > 0 { // for what is shown: the page, the collection, the selection
			it.group = "Bu ekran"
			here = append(here, it)
			continue
		}
		it.group = it.cat
		byCat[it.cat] = append(byCat[it.cat], it)
	}
	if c := a.pal.cat; c != "" { // a group opened: its commands, and the way back
		back := palItem{title: T("Bütün gruplar"), glyph: iconBack, group: c, run: func() { a.pal = palette{open: true, query: ">"} }}
		return append([]palItem{back}, byCat[c]...)
	}
	out := here
	for _, g := range cmdCats { // each group a line, which opens it
		if n := len(byCat[g.name]); n > 0 {
			name := g.name
			out = append(out, palItem{title: T(name), keys: Tf("%d seçenek", n), glyph: catGlyph(byCat[name]), group: "Gruplar",
				run: func() { a.pal = palette{open: true, query: ">", cat: name} }})
		}
	}
	return out
}

// catGlyph is the icon of a group's line: its first command's.
func catGlyph(items []palItem) *ui.SVG {
	return items[0].glyph
}

// famItem is the line of a family of n commands, of which it is one: chosen, the palette shows them all.
func (a *app) famItem(it palItem, n int) palItem {
	name := T(it.fam)
	return palItem{title: name + "…", keys: Tf("%d seçenek", n), glyph: it.glyph, cat: it.cat, boost: it.boost,
		run: func() { a.pal = palette{open: true, query: ">" + name + ": "} }}
}
