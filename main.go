// ryudit: a block editor on MyGo's rich text editor. The Markdown you type
// turns into structure as you type it and is never seen; the file kept is
// Markdown, so any other tool can read it. The caret is ours: Slime or Mizan.
//
// Run with: go run . [file.md]   (default: belge.md here)
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

const welcome = `# ryudit

Burada **Markdown işareti görmezsin**: yazdığın işaretler anında yapıya dönüşür.

## Dene

- Yeni bir satırda \# ve boşluk: başlık. \- liste, 1. numaralı liste, \[\] görev, \> alıntı.
- Sözcükleri iki yıldız arasına al: **kalın**. Tek yıldız _italik_, ters tırnak ` + "`kod`" + `, iki tilde ~~üstü çizili~~.
- ⌘B, ⌘I, ⌘U biçimlendirir; ⌘Z bir dönüşümü geri alır.
  - Tab içeri, ⇧Tab dışarı.

- [ ] Kutuya tıkla
- [x] Bitmiş bir görev

> Dosya düz Markdown: başka her araç okuyabilir.

⌘⇧M caret'i değiştirir: Slime ya da Mizan.
`

const (
	columnMax = 720
	pageTop   = 56
)

type caretKind int

const (
	caretSlime caretKind = iota
	caretMizan
)

// caretFrame is what a caret is told each frame (see mizan.go).
type caretFrame struct {
	r      ui.Rect
	cw     float32
	dark   bool
	bounds ui.Rect
	wrote  int
	edited bool
}

type app struct {
	doc   *richtext.Document
	path  string
	theme *richtext.Theme // the editor's, resolved: what locate lays paragraphs out with
	font  ui.Font

	caretKind caretKind
	jelly     *slime
	mizan     *mizanCaret

	// What changed since the last frame, for the caret and for saving.
	version  uint64
	length   int
	caretAt  int
	wrote    int
	edited   bool
	editAt   time.Time
	dirty    bool
	saveErr  string
	boxes    []richtext.ParagraphBox
	focused  bool
	clock    func() time.Time
	lastSave string
	style    richtext.ParagraphStyle // the caret paragraph's, last frame
	editorR  ui.Rect                 // the editor's box, from its last frame
}

func newApp(doc *richtext.Document, path string) *app {
	_, c := doc.Selection()
	return &app{doc: doc, path: path, jelly: newSlime(), mizan: &mizanCaret{}, version: doc.Version(),
		length: doc.Text().Len(), caretAt: c, clock: time.Now}
}

func loadDoc(path string) (*richtext.Document, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return richtext.NewDocument(richtext.ParseMarkdown(string(src))), nil
}

func main() {
	path := "belge.md"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	path, _ = filepath.Abs(path)
	doc, err := loadDoc(path)
	if err != nil {
		doc = richtext.NewDocument(richtext.ParseMarkdown(welcome))
	}
	a := newApp(doc, path)
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu}}))
		mygo.NewWindow(mygo.WindowOptions{Title: filepath.Base(path), Width: 960, Height: 720, Content: ui.View(a.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	th := richtext.DefaultTheme(t)
	th.Caret = invisible // ours is drawn over it
	th.ParagraphSpacing = 0.55
	for i := range th.Headings {
		th.Headings[i].Top, th.Headings[i].Bottom = 0.9, 0.3
	}
	a.theme = th
	a.font = ui.Font{Family: t.Font, Size: 16.5, LineHeight: 1.55}
	a.track(c)

	ui.Column(c).Fill().Background(t.Background).Children(func() {
		page := ui.Row(c).Fill().Justify(ui.Center).Padding(pageTop, 0, 0, 0)
		if page.Shortcut(ui.Cmd|ui.Shift, ui.KeyM) {
			a.caretKind = (a.caretKind + 1) % 2
		}
		page.Children(func() {
			ed := richtext.EditorBase(c, a.doc, richtext.EditorOptions{
				Label: "Belge", Font: a.font, Theme: th,
				Placeholder: "Yaz…  # başlık · - liste · [] görev · > alıntı",
			}).Fill().MaxWidth(columnMax).AutoFocus()
			a.focused = ed.Focused()
			ed.DrawOver(a.drawCaret)
		})
		a.status(c)
	})
	a.boxes = a.doc.VisibleParagraphs()
	a.autosave(c)
}

// track notes what the user did since the last frame: letters written (for
// Mizan's pace), any other edit, and that the file needs saving.
func (a *app) track(c *ui.Context) {
	v := a.doc.Version()
	_, caret := a.doc.Selection()
	text := a.doc.Text()
	style := text.ParagraphStyleAt(caret)
	if v == a.version {
		a.caretAt, a.style = caret, style
		return
	}
	n := text.Len()
	if grew := n - a.length; grew > 0 && caret-a.caretAt == grew && grew <= 8 {
		a.wrote += grew
	} else {
		a.edited = true
	}
	// A Markdown shortcut ("[] ", "# "…) just turned an empty paragraph into
	// another kind: what is typed there starts plain. (richtext v0.7.4 takes
	// the empty paragraph's attributes from its end mark, which a bold word
	// closed at the end of the paragraph above left bold.)
	if s, e := text.ParagraphRange(caret); n < a.length && style != a.style && s == e {
		a.doc.SetTypingAttributes(richtext.Attributes{})
	}
	a.version, a.length, a.caretAt, a.style = v, n, caret, style
	a.dirty, a.editAt, a.saveErr = true, a.clock(), ""
}

func (a *app) drawCaret(p *ui.Painter, r ui.Rect) {
	a.editorR = r
	if !a.focused {
		return
	}
	cb, ok := locate(a.doc, a.boxes, r.W, a.font, a.theme)
	if !ok {
		return
	}
	cr := cb.r
	cr.X += r.X
	cr.Y += r.Y
	dark := p.Theme().Dark
	anchor, caret := a.doc.Selection()
	if anchor != caret { // over a selection, a plain caret: the jelly would hide what is selected
		p.Fill(ui.Rect{X: cr.X - 1, Y: cr.Y, W: 2, H: cr.H}, p.Theme().Accent, 1)
		return
	}
	f := caretFrame{r: cr, cw: cb.cw, dark: dark, bounds: r, wrote: a.wrote, edited: a.edited}
	a.wrote, a.edited = 0, false
	p.Clip(r, 0, func() {
		if a.caretKind == caretMizan {
			a.mizan.draw(p, f)
			return
		}
		col := ui.Hex("#59f939")
		if !dark {
			col = ui.Hex("#2f9a1c")
		}
		a.jelly.draw(p, cr, cb.cw, col, dark)
	})
}

// status is the line at the bottom: the file, whether it is saved, the caret.
func (a *app) status(c *ui.Context) {
	t := c.Theme()
	state := "kaydedildi"
	switch {
	case a.saveErr != "":
		state = "kaydedilemedi: " + a.saveErr
	case a.dirty:
		state = "kaydediliyor…"
	}
	caret := map[caretKind]string{caretSlime: "Slime", caretMizan: "Mizan"}[a.caretKind]
	ui.Row(c).Padding(10, 18).Justify(ui.End).Children(func() {
		ui.Text(c, filepath.Base(a.path)+" · "+state+"   ·   caret: "+caret+" (⌘⇧M)").FontSize(11.5).TextColor(t.TextMuted)
	})
}

// autosave writes the file a moment after the last edit.
func (a *app) autosave(c *ui.Context) {
	if !a.dirty || a.path == "" || a.saveErr != "" {
		return
	}
	if wait := 700*time.Millisecond - a.clock().Sub(a.editAt); wait > 0 {
		c.After(wait)
		return
	}
	a.save()
}

func (a *app) save() {
	md := a.doc.Text().Markdown()
	if !strings.HasSuffix(md, "\n") {
		md += "\n"
	}
	if md == a.lastSave {
		a.dirty = false
		return
	}
	if err := os.WriteFile(a.path, []byte(md), 0o644); err != nil {
		a.saveErr = err.Error()
		return
	}
	a.lastSave, a.dirty = md, false
}
