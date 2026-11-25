package main

import (
	"errors"
	"math"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// A cover: a picture across the top of a page, over its title. It is a file
// beside the page, as pictures in the text are, and named in the page's front
// matter (cover: kapak-….png), so other tools see it as a field of the page.

const coverH = 200

var iconCover = icon(`<rect x="3.5" y="5" width="17" height="10.5" rx="2.5"/><path d="m4 13 4-3.5 3.5 3 3-2.5 5 3.5"/><path d="M7 19h10"/>`)

var iconReposition = icon(`<path d="M12 4v16M8 8l4-4 4 4M8 16l4 4 4-4"/>`)

// setCover makes data the page's cover, the old one's file going.
func (a *app) setCover(p *Page, data []byte) error {
	ext, err := checkImage(data)
	if err != nil {
		return err
	}
	return a.putCover(p, data, ext)
}

// checkImage is the extension of the picture data is, or an error when it is not one this app shows: decoded whole,
// it is the slow part of setting a picture, and does not need the app's lock (#201).
func checkImage(data []byte) (string, error) {
	ext := imageExt(data)
	if ext == "" {
		return "", errors.New(notImage())
	}
	if _, err := decodePicture("", data); err != nil {
		if errors.As(err, new(tooManyPixels)) {
			return "", err
		}
		return "", errors.New(notImage())
	}
	return ext, nil
}

// putCover makes data, a picture checked, page p's cover.
func (a *app) putCover(p *Page, data []byte, ext string) error {
	if p == a.pane().page {
		a.flush()
	}
	name := "kapak-" + a.clock().Format("20060102-150405") + ext
	if err := p.FS().WriteFile(filepath.Join(p.Dir, name), data); err != nil {
		return err
	}
	oldShift := p.CoverShift
	old := p.Cover
	p.Cover = name
	p.CoverShift = 0 // a new picture lies in the middle of its band
	a.coverFix = nil
	body, err := p.Body()
	if err != nil {
		p.Cover = old // the page file cannot be read: it is not written over, and the picture is not kept
		p.CoverShift = oldShift
		p.FS().Remove(filepath.Join(p.Dir, name))
		return err
	}
	if err := p.Save(body); err != nil {
		return err
	}
	if old != "" && old != CoverNone && old != name && safeLocalAsset(old) {
		p.FS().Remove(filepath.Join(p.Dir, old))
	}
	return nil
}

// removeCover takes the cover away, its file with it, and says the page wants none: not the made one either.
func (a *app) removeCover(p *Page) {
	if p.Cover == CoverNone {
		return
	}
	if p == a.pane().page {
		a.flush()
	}
	oldShift := p.CoverShift
	old := p.Cover
	p.Cover = CoverNone // no cover, the made one either: the page said so
	p.CoverShift = 0
	a.coverFix = nil
	body, err := p.Body()
	if err != nil {
		p.Cover = old
		p.CoverShift = oldShift
		a.fail(err)
		return
	}
	if err := p.Save(body); err != nil {
		a.fail(err)
		return
	}
	if old != "" && old != CoverNone && safeLocalAsset(old) {
		p.FS().Remove(filepath.Join(p.Dir, old))
	}
}

// restoreAutoCover puts the made cover back on a page that had none.
func (a *app) restoreAutoCover(p *Page) {
	if p == a.pane().page {
		a.flush()
	}
	old := p.Cover
	p.Cover = ""
	body, err := p.Body()
	if err != nil {
		p.Cover = old
		a.fail(err)
		return
	}
	if err := p.Save(body); err != nil {
		a.fail(err)
	}
}

// pickCover asks for the cover in a dialog.
func (a *app) pickCover(p *Page) {
	go func() {
		defer a.guard()
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:   locales.PageChooseCover(),
			Filters: []mygo.FileFilter{{Name: "Resimler", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp", "svg"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		data, err := readFile(paths[0]) // read and decoded away from the window
		ext := ""
		if err == nil {
			ext, err = checkImage(data)
		}
		a.do(func() error {
			if err == nil {
				err = a.putCover(p, data, ext)
			}
			if err != nil {
				a.say(toastErr, err.Error())
			}
			return nil
		})
	}()
}

// coverFile makes the picture at file the page's cover.
func (a *app) coverFile(p *Page, file string) {
	data, err := readFile(file)
	if err == nil {
		err = a.setCover(p, data)
	}
	if err != nil {
		a.say(toastErr, err.Error())
	}
}

// A picture cover is frameless (#478): the picture itself, rounded, a little in from the pane's sides and right under
// its bar, as tall as the picture is for that width (within coverMinH and coverMaxH, cropped beyond them), with no
// band of its own and nothing over it but its buttons; the title is the page's own, under it. The made cover
// (autocover.go) is still a band behind the top of the page, its foot fading into the page, the title on it.
// paintCover draws either behind the page (the wrapper's own drawing); pageHead leaves coverRoomOf above the title,
// and coverGap the foot below it.

const (
	coverInset = 16  // the picture's room at the pane's sides
	coverTop   = 4   // and under the bar
	coverGapT  = 30  // from its foot to the title
	coverMinH  = 110 // a wide picture is not thinner than this: it is cropped
	coverMaxH  = 200 // a tall one not taller
	coverDefH  = 170 // until the picture or the pane's width is known
	coverFoot  = 22  // below the title, on the made cover's band
)

// coverPicH is the height of page p's picture cover in the pane: the picture's own for the width it was last drawn
// at, kept within coverMinH and coverMaxH.
func (a *app) coverPicH(p *Page) float32 {
	if !safeLocalAsset(p.Cover) || a.coverW <= 0 {
		return coverDefH
	}
	src := a.loadImage(p, filepath.Join(p.Dir, p.Cover))
	iw, ih := ui.ImageSize(src)
	if iw <= 0 || ih <= 0 {
		return coverDefH
	}
	return min(max(a.coverW*ih/iw, coverMinH), coverMaxH)
}

// coverTopOf is the room above a picture cover: right under the pane's bar when the window has one (it is a row above
// the page), else under the title bar that lies over the page's top, and coverTop beyond it.
func (a *app) coverTopOf() float32 {
	if a.paneBarH() > 0 {
		return coverTop
	}
	return barH + coverTop
}

// coverPicRect is where the picture cover lies in r, the page's wrapper.
func (a *app) coverPicRect(p *Page, r ui.Rect) ui.Rect {
	return ui.Rect{X: r.X + coverInset, Y: r.Y + a.coverTopOf(), W: max(r.W-2*coverInset, 1), H: a.coverPicH(p)}
}

func (a *app) coverGap(c *ui.Context, p *Page) {
	if h := coverFootOf(a.coverKind(p)); h > 0 {
		ui.Box(c.Key("cover-foot")).FillWidth().Height(h)
	}
}

// coverFootOf is the room under the title for a cover of kind: the made cover's band goes on under it, a picture
// has ended above it.
func coverFootOf(kind int) float32 {
	if kind == coverKindAuto {
		return coverFoot
	}
	return 0
}

// coverInk is the title's color on page p: white on the made cover's dark foot, the text's own on its pale light
// mosaic and under a picture, which the title no longer lies on.
func (a *app) coverInk(c *ui.Context, p *Page) ui.Color {
	if a.coverKind(p) == coverKindAuto && ryugo.From(c).Dark {
		return ui.Hex("#ffffff")
	}
	return c.Theme().Text
}

// paintCover is the cover, drawn as the wrapper's own, under what is in it: a picture rounded in its place, or the
// pattern made for a page with no picture of its own (autocover.go), its foot going into the page.
func (a *app) paintCover(c *ui.Context, p *Page, wrap ui.Element) {
	switch a.coverKind(p) {
	case coverKindNone:
		return
	case coverKindAuto:
		ac := a.autoCoverOf(p, ryugo.From(c).Dark)
		fade := a.bg(c) // its foot goes into the page
		wrap.Draw(func(pt *ui.Painter, r ui.Rect) {
			ac.paint(pt, ui.Rect{X: r.X, Y: r.Y, W: r.W, H: autoFullH}, fade)
		})
		return
	}
	t := ryugo.From(c)
	cl := t.Colors
	var src ui.ImageSource
	if safeLocalAsset(p.Cover) {
		src = a.loadImage(p, filepath.Join(p.Dir, p.Cover))
	}
	radius := rad(14)
	if b := wrap.LayoutBounds(); b.W > 0 { // the height follows the width the page was laid out at, a frame late
		if w := max(b.W-2*coverInset, 1); w != a.coverW {
			a.coverW = w
			c.Invalidate() // so the next frame comes, with the height for it
		}
	}
	wrap.Draw(func(pt *ui.Painter, r ui.Rect) {
		pic := a.coverPicRect(p, r)
		if src == nil {
			pt.Fill(pic, cl.Well, radius)
			return
		}
		iw, ih := ui.ImageSize(src)
		pt.Clip(pic, radius, func() { pt.Image(src, coverImageRect(pic, iw, ih, p.CoverShift), ui.FillBox) })
		// no frame: only a hair of the text's color at its edge, so a pale picture still ends on a pale page
		pt.Stroke(pic, cl.Text.Alpha(map[bool]float32{true: 0.10, false: 0.06}[t.Dark]), radius, 1)
	})
}

// coverButtons are the cover's buttons, floating over its top right as the pointer is on it (#478): round, on a
// dark glass that reads on any picture - change the picture, and a menu of the rest.
func (a *app) coverButtons(c *ui.Context, p *Page) {
	kind := a.coverKind(p)
	if kind == coverKindNone {
		return
	}
	if a.coverFix == p && kind == coverKindImage {
		a.coverReposition(c, p)
		return
	}
	right, top := float32(18), float32(barH+10)
	if kind == coverKindImage {
		right, top = coverInset+10, a.coverTopOf()+10
	}
	row := ui.Row(c.Key("cover-btns")).Absolute().Right(right).Top(top).Gap(6)
	row.Opacity(row.AnimateTo("hot", map[bool]float32{true: 1, false: 0}[a.coverHover || a.coverMenu], fadeQuick))
	row.Children(func() {
		if coverRound(c, "cover-change", locales.PageChangeCover(), iconImage).Clicked() {
			a.pickCover(p)
		}
		more := coverRound(c, "cover-more", locales.PageCoverMore(), iconMore)
		if more.Clicked() {
			a.coverMenu = !a.coverMenu
		}
		menuAt(c, "cover-more-menu", "page_coverMore", more.LayoutBounds(), &a.coverMenu, a.coverMenuItems(p))
	})
}

// coverMenuItems are the cover's menu: change it, put the made one back, take it away.
func (a *app) coverMenuItems(p *Page) []menuItem {
	out := []menuItem{{title: "page_changeCover", glyph: iconCover, run: func() { a.pickCover(p) }}}
	if a.coverKind(p) == coverKindImage {
		out = append(out, menuItem{title: "page_repositionCover", glyph: iconReposition, run: func() { a.coverFix = p }})
	}
	if a.coverKind(p) == coverKindImage && a.settings.AutoCover {
		out = append(out, menuItem{title: "page_restoreAutoCover", glyph: iconDice, run: func() {
			a.removeCover(p)
			a.restoreAutoCover(p)
		}})
	}
	return append(out, menuItem{title: "common_removeCover", group: "~", glyph: iconTrash, danger: true, run: func() { a.removeCover(p) }})
}

// coverRound is one of the cover's round buttons: a glyph on dark glass, lighter under the pointer.
func coverRound(c *ui.Context, key, label string, glyph *ui.SVG) ui.Element {
	b := ui.Box(c.Key(key)).Size(30, 30).Radius(15).AlignItems(ui.Center).Justify(ui.Center).
		Label(label).Role(ui.RoleButton).Tooltip(label).Border(1, ui.Hex("#ffffff").Alpha(0.16)).
		Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.18))
	b.Background(ui.Hex("#000000").Alpha(map[bool]float32{true: 0.56, false: 0.38}[b.Hovered()]))
	b.Children(func() { ui.Icon(c, glyph).Width(15).Height(15).TextColor(ui.Hex("#ffffff")) })
	return b
}

// readFile reads a picture chosen for a cover or a banner, unless it is too big to be one (#177).
func readFile(name string) ([]byte, error) { return readCapped(name, maxImageBytes) }

// safeLocalAsset reports whether name is a relative single-folder filename without traversal or drive prefixes.
func safeLocalAsset(name string) bool { return SafeName(name) && filepath.IsLocal(name) }

// coverThumb is the cover of p as a small picture, cropped to w by h, or
// nothing when the page has none (or the file is gone).
func (a *app) coverThumb(c *ui.Context, p *Page, w, h float32, radius float32) {
	if !p.ImageCover() || !safeLocalAsset(p.Cover) {
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
	iw, ih := ui.ImageSize(src)
	box.Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Clip(r, radius, func() { pt.Image(src, coverImageRect(r, iw, ih, p.CoverShift), ui.FillBox) })
	})
}

// coverImageRect is where a picture of iw by ih lies when it covers pic and shift (-100 to 100) says which part of it
// shows: -100 its top (or left), 100 its bottom (or right), 0 the middle. The picture is drawn into it, stretched
// to it as it is the right shape, under a clip of pic.
func coverImageRect(pic ui.Rect, iw, ih float32, shift int) ui.Rect {
	if iw <= 0 || ih <= 0 {
		return pic
	}
	s := max(pic.W/iw, pic.H/ih)
	w, h := iw*s, ih*s
	f := 0.5 + float32(shift)/200
	return ui.Rect{X: pic.X - (w-pic.W)*f, Y: pic.Y - (h-pic.H)*f, W: w, H: h}
}

// coverReposition is a picture cover while the user sets which part of it shows: the picture is dragged inside its
// band (up or down, or sideways for a very wide one), and Done, at the top right where the cover's buttons are,
// writes it (#524). Escape ends it too.
func (a *app) coverReposition(c *ui.Context, p *Page) {
	pic := ui.Rect{W: a.coverW, H: a.coverPicH(p)}
	area := ui.Box(c.Key("cover-fix")).Absolute().Left(coverInset).Top(a.coverTopOf()).Size(pic.W, pic.H).Cursor(ui.CursorGrab).
		Label(locales.PageCoverDrag()).AlignItems(ui.Center).Justify(ui.Center)
	area.Children(func() {
		if !a.coverDrag {
			ui.Box(c.Key("cover-fix-hint")).Padding(6, 12).Radius(14).Background(ui.Hex("#000000").Alpha(0.5)).PassThrough().Children(func() {
				ui.Text(c, locales.PageCoverDrag()).FontSize(13).FontWeight(500).TextColor(ui.Hex("#ffffff"))
			})
		}
	})
	dx, dy, pressed := area.Dragged()
	if pressed && safeLocalAsset(p.Cover) {
		if !a.coverDrag {
			a.coverDrag, a.coverAt = true, float32(p.CoverShift)
		}
		iw, ih := ui.ImageSize(a.loadImage(p, filepath.Join(p.Dir, p.Cover)))
		over := coverImageRect(pic, iw, ih, 0)
		if oy, ox := over.H-pic.H, over.W-pic.W; oy >= ox && oy > 0.5 {
			a.coverAt -= dy * 200 / oy // the picture follows the pointer: dragged down, its top comes into view
		} else if ox > 0.5 {
			a.coverAt -= dx * 200 / ox
		}
		a.coverAt = min(max(a.coverAt, -100), 100)
		p.CoverShift = int(math.Round(float64(a.coverAt)))
	}
	if !pressed && a.coverDrag { // let go: written now, not on every frame of the drag
		a.coverDrag = false
		a.saveCoverShift(p)
	}
	done := ui.Box(c.Key("cover-fix-done")).Absolute().Right(coverInset+10).Top(a.coverTopOf()+10).Height(30).Padding(0, 14).Radius(15).
		AlignItems(ui.Center).Justify(ui.Center).Label(locales.PageCoverDone()).Role(ui.RoleButton).
		Cursor(ui.CursorPointer)
	done.Background(ui.Hex("#ffffff").Alpha(map[bool]float32{true: 1, false: 0.88}[done.Hovered()]))
	done.Children(func() { ui.Text(c, locales.PageCoverDone()).FontSize(13).FontWeight(600).TextColor(ui.Hex("#000000")) })
	if done.Clicked() || c.Shortcut(0, ui.KeyEscape) {
		a.coverFix = nil
	}
}

// saveCoverShift writes p's cover position into its page file.
func (a *app) saveCoverShift(p *Page) {
	if p == a.pane().page {
		a.flush()
	}
	body, err := p.Body()
	if err != nil {
		a.fail(err)
		return
	}
	if err := p.Save(body); err != nil {
		a.fail(err)
	}
}
