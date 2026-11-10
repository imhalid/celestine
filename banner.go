package main

import (
	"fmt"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// A collection's banner: a picture across the top of its page, above its icon and name. It is a file in the
// collection's folder (afis-….png), named in collection.json, so it travels with the collection when it syncs.

const bannerH = 168

// setBanner makes data the collection's banner, the old one's file going.
func (a *app) setBanner(coll *Collection, data []byte) error {
	ext := imageExt(data)
	if ext == "" {
		return fmt.Errorf("%s", notImage)
	}
	if _, err := decodeCheck(data); err != nil {
		return fmt.Errorf("%s", notImage)
	}
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
	if old != "" && old != name {
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
	coll.FS.Remove(filepath.Join(coll.Dir, old))
}

// pickBanner asks for the banner in a dialog.
func (a *app) pickBanner(coll *Collection) {
	go func() {
		defer a.guard()
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:   T("Choose a banner"),
			Filters: []mygo.FileFilter{{Name: "Resimler", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp", "svg"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.do(func() error {
			data, err := readFile(paths[0])
			if err == nil {
				err = a.setBanner(coll, data)
			}
			if err != nil {
				a.notice = err.Error()
			}
			return nil
		})
	}()
}

// bannerView is the banner across the top of the collection's page, cropped to its band, or nothing.
func (a *app) bannerView(c *ui.Context, coll *Collection) {
	if coll.Banner == "" {
		return
	}
	src := a.loadImageIn(coll.FS, filepath.Join(coll.Dir, coll.Banner))
	if src == nil {
		return
	}
	ui.Box(c.Key("banner:"+coll.Dir)).FillWidth().Height(bannerH).Radius(rad(12)).Clip().Shrink(0).Margin(0, 0, 6, 0).
		Label(T("Banner")).Draw(func(pt *ui.Painter, r ui.Rect) { pt.Clip(r, 12, func() { pt.Image(src, r, ui.Cover) }) })
}
