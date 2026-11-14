package main

import (
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// A collection's banner: a picture across the top of its page, above its icon and name. It is a file in the
// collection's folder (afis-….png), named in collection.json, so it travels with the collection when it syncs.

const (
	bannerH    = 168 // until the picture's width is known
	bannerMinH = 120 // a wide picture is cropped below this
	bannerMaxH = 280 // a tall one above this
)

// setBanner makes data the collection's banner, the old one's file going.
func (a *app) setBanner(coll *Collection, data []byte) error {
	ext, err := checkImage(data)
	if err != nil {
		return err
	}
	return a.putBanner(coll, data, ext)
}

// putBanner makes data, a picture checked, collection coll's banner.
func (a *app) putBanner(coll *Collection, data []byte, ext string) error {
	name := "afis-" + a.clock().Format("20060102-150405") + ext
	if err := coll.FS.WriteFile(filepath.Join(coll.Dir, name), data); err != nil {
		return err
	}
	old := coll.Banner
	coll.Banner = name
	if err := coll.Save(); err != nil {
		coll.Banner = old
		coll.FS.Remove(filepath.Join(coll.Dir, name))
		return err
	}
	if old != "" && old != name && safeLocalAsset(old) {
		coll.FS.Remove(filepath.Join(coll.Dir, old))
	}
	return nil
}

// removeBanner takes the banner away, its file with it.
func (a *app) removeBanner(coll *Collection) {
	old := coll.Banner
	if old == "" {
		return
	}
	coll.Banner = ""
	if err := coll.Save(); err != nil {
		coll.Banner = old
		a.fail(err)
		return
	}
	if safeLocalAsset(old) {
		coll.FS.Remove(filepath.Join(coll.Dir, old))
	}
}

// pickBanner asks for the banner in a dialog.
func (a *app) pickBanner(coll *Collection) {
	go func() {
		defer a.guard()
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:   locales.PageChooseBanner(),
			Filters: []mygo.FileFilter{{Name: "Resimler", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp", "svg"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		data, err := readFile(paths[0]) // read and decoded away from the window (#201)
		ext := ""
		if err == nil {
			ext, err = checkImage(data)
		}
		a.do(func() error {
			if err == nil {
				err = a.putBanner(coll, data, ext)
			}
			if err != nil {
				a.say(toastErr, err.Error())
			}
			return nil
		})
	}()
}

// bannerView is the banner at the top of the collection's page, or nothing: a rounded picture with no frame, as tall
// as it is for the page's width (within bannerMinH and bannerMaxH, cropped beyond them), its buttons floating over its
// top right as the pointer is on it, as a page's cover has them (#478).
func (a *app) bannerView(c *ui.Context, coll *Collection) {
	if coll.Banner == "" || !safeLocalAsset(coll.Banner) {
		return
	}
	src := a.loadImageIn(coll.FS, filepath.Join(coll.Dir, coll.Banner))
	if src == nil {
		return
	}
	box := ui.Box(c.Key("banner:"+coll.Dir)).FillWidth().Radius(rad(12)).Clip().Shrink(0).Margin(0, 0, 6, 0).Label(locales.PageBanner())
	h := float32(bannerH)
	if iw, ih := ui.ImageSize(src); iw > 0 && ih > 0 {
		if w := box.LayoutBounds().W; w > 0 {
			h = min(max(w*ih/iw, bannerMinH), bannerMaxH)
		}
	}
	box.Height(h).Draw(func(pt *ui.Painter, r ui.Rect) { pt.Clip(r, rad(12), func() { pt.Image(src, r, ui.Cover) }) })
	hot := box.Hovered() || a.bannerMenu
	box.Children(func() {
		row := ui.Row(c.Key("banner-btns")).Absolute().Right(10).Top(10).Gap(6)
		row.Opacity(row.AnimateTo("hot", map[bool]float32{true: 1, false: 0}[hot], fadeQuick))
		row.Children(func() {
			if coverRound(c, "banner-change", locales.CommandsChangeBanner(), iconImage).Clicked() {
				a.pickBanner(coll)
			}
			more := coverRound(c, "banner-more", locales.PageCoverMore(), iconMore)
			if more.Clicked() {
				a.bannerMenu = !a.bannerMenu
			}
			menuAt(c, "banner-more-menu", "page_coverMore", more.LayoutBounds(), &a.bannerMenu, []menuItem{
				{title: "commands_changeBanner", glyph: iconCover, run: func() { a.pickBanner(coll) }},
				{title: "commands_removeBanner", group: "~", glyph: iconTrash, danger: true, run: func() { a.removeBanner(coll) }},
			})
		})
	})
}
