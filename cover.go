package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// A cover: a picture across the top of a page, over its title. It is a file
// beside the page, as pictures in the text are, and named in the page's front
// matter (cover: kapak-….png), so other tools see it as a field of the page.

const coverH = 200

var iconCover = icon(`<rect x="3.5" y="5" width="17" height="10.5" rx="2.5"/><path d="m4 13 4-3.5 3.5 3 3-2.5 5 3.5"/><path d="M7 19h10"/>`)

// setCover makes data the page's cover, the old one's file going.
func (a *app) setCover(p *Page, data []byte) error {
	ext := imageExt(data)
	if ext == "" {
		return fmt.Errorf("%s", notImage)
	}
	if _, err := decodeCheck(data); err != nil {
		return fmt.Errorf("%s", notImage)
	}
	if p == a.page {
		a.flush()
	}
	name := "kapak-" + a.clock().Format("20060102-150405") + ext
	if err := p.fs().WriteFile(filepath.Join(p.Dir, name), data); err != nil {
		return err
	}
	old := p.Cover
	p.Cover = name
	body, _ := p.Body()
	if err := p.Save(body); err != nil {
		return err
	}
	if old != "" && old != name {
		p.fs().Remove(filepath.Join(p.Dir, old))
	}
	return nil
}

// removeCover takes the cover away, its file with it.
func (a *app) removeCover(p *Page) {
	if p.Cover == "" {
		return
	}
	if p == a.page {
		a.flush()
	}
	old := p.Cover
	p.Cover = ""
	body, _ := p.Body()
	if err := p.Save(body); err != nil {
		a.saveErr = err.Error()
		return
	}
	p.fs().Remove(filepath.Join(p.Dir, old))
}

// pickCover asks for the cover in a dialog.
func (a *app) pickCover(p *Page) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:   "Kapak resmi seç",
			Filters: []mygo.FileFilter{{Name: "Resimler", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp", "svg"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.do(func() error { a.coverFile(p, paths[0]); return nil })
	}()
}

// coverFile makes the picture at file the page's cover.
func (a *app) coverFile(p *Page, file string) {
	data, err := readFile(file)
	if err == nil {
		err = a.setCover(p, data)
	}
	if err != nil {
		a.notice = err.Error()
	}
}

// coverView is the cover, over the title: the picture cropped to the column,
// and, as the pointer is on it, the buttons to change and take it away.
func (a *app) coverView(c *ui.Context, p *Page) {
	if p.Cover == "" {
		return
	}
	src := a.loadImage(p, filepath.Join(p.Dir, p.Cover))
	box := ui.Box(c.Key("cover:"+p.Dir)).FillWidth().Height(coverH).Radius(14).Clip().Margin(4, 0, 6, 0).Label("Kapak resmi")
	cl := ryugo.From(c).Colors
	if src == nil { // gone: a plain band with the buttons
		box.Background(cl.Well)
	} else {
		box.Draw(func(pt *ui.Painter, r ui.Rect) { pt.Clip(r, 14, func() { pt.Image(src, r, ui.Cover) }) })
	}
	hot := box.AnimateTo("hot", map[bool]float32{true: 1, false: 0}[box.Hovered()], fadeQuick)
	box.Children(func() {
		ui.Row(c.Key("cover-btns")).Absolute().Right(10).Top(10).Gap(6).Opacity(hot).Children(func() {
			if ryugo.Button(c.Key("cover-change"), "Değiştir", ryugo.ButtonOptions{Icon: iconCover, Size: ryugo.Sm}).Clicked() {
				a.pickCover(p)
			}
			if ryugo.Button(c.Key("cover-remove"), "Kapağı kaldır", ryugo.ButtonOptions{Icon: iconTrash, IconOnly: true, Color: cl.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
				a.removeCover(p)
			}
		})
	})
}

// decodeCheck reports whether data decodes as a picture.
func decodeCheck(data []byte) (ui.ImageSource, error) { return ui.DecodeImage(data) }

func readFile(name string) ([]byte, error) { return os.ReadFile(name) }
