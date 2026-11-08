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
	body, err := p.Body()
	if err != nil {
		p.Cover = old // the page file cannot be read: it is not written over, and the picture is not kept
		p.fs().Remove(filepath.Join(p.Dir, name))
		return err
	}
	if err := p.Save(body); err != nil {
		return err
	}
	if old != "" && old != coverNone && old != name {
		p.fs().Remove(filepath.Join(p.Dir, old))
	}
	return nil
}

// removeCover takes the cover away, its file with it, and says the page wants none: not the made one either.
func (a *app) removeCover(p *Page) {
	if p.Cover == coverNone {
		return
	}
	if p == a.page {
		a.flush()
	}
	old := p.Cover
	p.Cover = coverNone // no cover, the made one either: the page said so
	body, err := p.Body()
	if err != nil {
		p.Cover = old
		a.saveErr = err.Error()
		return
	}
	if err := p.Save(body); err != nil {
		a.saveErr = err.Error()
		return
	}
	if old != "" && old != coverNone {
		p.fs().Remove(filepath.Join(p.Dir, old))
	}
}

// restoreAutoCover puts the made cover back on a page that had none.
func (a *app) restoreAutoCover(p *Page) {
	if p == a.page {
		a.flush()
	}
	old := p.Cover
	p.Cover = ""
	body, err := p.Body()
	if err != nil {
		p.Cover = old
		a.saveErr = err.Error()
		return
	}
	if err := p.Save(body); err != nil {
		a.saveErr = err.Error()
	}
}

// pickCover asks for the cover in a dialog.
func (a *app) pickCover(p *Page) {
	go func() {
		defer a.guard()
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

// The cover is a band across the top of the page, behind its title: the
// picture cropped to the band, darker toward its foot, where the title lies;
// the page's text goes on under it. coverBand is the band, drawn behind the
// page (an absolute layer in the page's wrapper); pageHead leaves coverRoom
// above the title, and coverGap the foot below it.

const (
	coverBandH = 232 // the band's height: the room, the title, the foot
	coverRoom  = 150 // above the title, in the band
	coverFoot  = 22  // below the title, in the band
)

func (a *app) coverGap(c *ui.Context, p *Page) {
	if a.coverKind(p) != coverKindNone {
		ui.Box(c.Key("cover-foot")).FillWidth().Height(coverFoot)
	}
}

// paintCover is the band's picture, drawn as the wrapper's own, under what
// is in it: cropped to the width, darker toward the foot. A page with no
// picture of its own has the pattern made for it (autocover.go), still, its
// foot already dark.
func (a *app) paintCover(c *ui.Context, p *Page, wrap ui.Element) {
	switch a.coverKind(p) {
	case coverKindNone:
		return
	case coverKindAuto:
		ac := a.autoCoverOf(p, ryugo.From(c).Dark)
		fade := a.bg(c) // its foot goes into the page
		wrap.Draw(func(pt *ui.Painter, r ui.Rect) {
			if !a.coverAway {
				ac.paint(pt, ui.Rect{X: r.X, Y: r.Y, W: r.W, H: autoFullH}, fade)
			}
		})
		return
	}
	cl := ryugo.From(c).Colors
	src := a.loadImage(p, filepath.Join(p.Dir, p.Cover))
	wrap.Draw(func(pt *ui.Painter, r ui.Rect) {
		if a.coverAway {
			return
		}
		band := ui.Rect{X: r.X, Y: r.Y, W: r.W, H: coverBandH + barH}
		if src == nil {
			pt.Fill(band, cl.Well, 0)
			return
		}
		pt.Image(src, band, ui.Cover)
		// the foot darkens, so that the title reads on any picture
		pt.FillGradient(band, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{
			{Color: ui.Hex("#000000").Alpha(0), At: 0.25}, {Color: ui.Hex("#000000").Alpha(0.62), At: 1},
		}}, 0)
	})
}

// coverButtons are the buttons on the cover, as the pointer is in the band.
func (a *app) coverButtons(c *ui.Context, p *Page) {
	if a.coverKind(p) == coverKindNone {
		return
	}
	cl := ryugo.From(c).Colors
	row := ui.Row(c.Key("cover-btns")).Absolute().Right(18).Top(10).Gap(6)
	row.Opacity(row.AnimateTo("hot", map[bool]float32{true: 1, false: 0}[a.coverHover && !a.coverAway], fadeQuick))
	row.Children(func() {
		if ryugo.Button(c.Key("cover-change"), T("Kapağı değiştir"), ryugo.ButtonOptions{Icon: iconCover, Size: ryugo.Sm}).Clicked() {
			a.pickCover(p)
		}
		if ryugo.Button(c.Key("cover-remove"), T("Kapağı kaldır"), ryugo.ButtonOptions{Icon: iconTrash, IconOnly: true, Color: cl.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
			a.removeCover(p)
		}
	})
}

// decodeCheck reports whether data decodes as a picture.
func decodeCheck(data []byte) (ui.ImageSource, error) { return ui.DecodeImage(data) }

func readFile(name string) ([]byte, error) { return os.ReadFile(name) }

// coverThumb is the cover of p as a small picture, cropped to w by h, or
// nothing when the page has none (or the file is gone).
func (a *app) coverThumb(c *ui.Context, p *Page, w, h float32, radius float32) {
	if !p.imageCover() {
		return
	}
	src := a.loadImage(p, filepath.Join(p.Dir, p.Cover))
	if src == nil {
		return
	}
	box := ui.Box(c.Key("thumb:" + p.Dir)).Height(h).Radius(rad(radius)).Clip().Shrink(0)
	if w > 0 {
		box.Width(w)
	} else {
		box.FillWidth()
	}
	box.Draw(func(pt *ui.Painter, r ui.Rect) { pt.Clip(r, radius, func() { pt.Image(src, r, ui.Cover) }) })
}
