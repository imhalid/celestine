// A block editor on MyGo: the document is blocks, and Markdown is never seen.
// Type "# " for a heading, "- " for a list, "[] " for a task, "**word**" for
// bold - the marks turn into structure as you type. The file it keeps is
// Markdown, so any other tool can read it.
//
// Run with: go run . [file.md]   (default: belge.md here)
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const welcome = `# Blok editör

Burada **Markdown işareti görmezsin**. Yazdığın işaretler anında yapıya dönüşür.

## Dene
- Yeni satırda ` + "`# `" + ` yaz: başlık olur.
- ` + "`- `" + ` liste, ` + "`1. `" + ` numaralı liste, ` + "`[] `" + ` görev, ` + "`> `" + ` alıntı.
- Sözcükleri ` + "`**`" + ` arasına al: **kalın** olur; ` + "`_`" + ` ile _italik_, ` + "`~~`" + ` ile ~~üstü çizili~~.
- ⌘B, ⌘I, ⌘E seçimi biçimlendirir. ⌘Z bir dönüşümü geri alır.
  - Tab ile içeri, ⇧Tab ile dışarı.

- [ ] Kutuya tıkla, ya da ⌘↵
- [x] Bitmiş bir görev

> Dosya ise düz Markdown: başka her araç okuyabilir.

- Satır başında ` + "\\`\\`\\`" + ` kod bloğu açar; son boş satırda Enter dışarı çıkarır.
- ` + "`---`" + ` yatay çizgi.

` + "```" + `go
fmt.Println("merhaba")
` + "```" + `

---

⌘⇧M caret'i değiştirir: Slime ya da Mizan.
`

func main() {
	path := "belge.md"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	path, _ = filepath.Abs(path)
	doc, err := loadFile(path)
	if err != nil {
		doc = newDoc(parseMarkdown(welcome)...)
	}
	last := len(doc.Blocks) - 1
	doc.setCaret(Pos{last, doc.Blocks[last].len()})
	e := newEditor(doc, path)
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu}}))
		mygo.NewWindow(mygo.WindowOptions{Title: filepath.Base(path), Width: 960, Height: 720, Content: ui.View(e.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (e *editor) view(c *ui.Context) {
	t := c.Theme()
	if e.theme != t {
		e.theme = t
		e.cache = map[int]*shaped{}
	}
	ui.Box(c).Fill().Background(t.Background).Cursor(ui.CursorText).
		Label("Belge").Role(ui.RoleTextField).Value(e.focusBlock().Text).
		HandleTextInput(e).AutoFocus().
		HandleInput(e.input).
		Draw(e.draw)
}
