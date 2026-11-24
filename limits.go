package main

import (
	"bytes"
	"errors"
	"image"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// What is read in has limits (#177): a file dropped on a page, a picture, a Markdown file imported. Each is looked
// at before it is read whole: a file of gigabytes dropped on the text was read into memory and into the library,
// and a picture of a few kilobytes that says it is a hundred thousand pixels wide was decoded into gigabytes each
// time its page opened. The pictures decoded are kept for the pages shown lately, not for every one ever shown.

const (
	maxFileBytes   = 512 << 20  // a file put in a page, or as a property's value
	maxImageBytes  = 64 << 20   // a picture: in the text, a cover, a banner, an imported page's
	maxImportBytes = 32 << 20   // a Markdown file imported
	maxImagePixels = 50_000_000 // a picture's pixels: 200 MB decoded
	keptImages     = 96         // the pictures kept decoded, the ones used last
	readAtOnce     = 8 << 20    // a file read in the frame that asks for it; a bigger one is read away from it
)

// readThen reads the file name, up to max, and gives then what it read: at once for a small file, and for a bigger one
// from a goroutine, through a.do, the window going on meanwhile.
func (a *app) readThen(name string, max int64, then func(data []byte, err error)) {
	if fi, err := os.Stat(name); err != nil || fi.Size() <= readAtOnce {
		then(readCapped(name, max))
		return
	}
	go func() {
		defer a.guard()
		data, err := readCapped(name, max)
		a.do(func() error { then(data, err); return nil })
	}()
}

// readCapped is the file name, read whole, unless it is larger than max: then an error that says so.
func readCapped(name string, max int64) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if fi, err := f.Stat(); err == nil && fi.Size() > max {
		return nil, tooBig(name, fi.Size(), max)
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1)) // it may grow as it is read
	if err == nil && int64(len(data)) > max {
		return nil, tooBig(name, int64(len(data)), max)
	}
	return data, err
}

func tooBig(name string, size, max int64) error {
	return errors.New(locales.FilesTooBig(filepath.Base(name), bytesText(size), bytesText(max)))
}

// bytesText is a size in bytes, in the unit it reads best in.
func bytesText(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f, u := float64(n), 0
	for f >= 1024 && u < len(units)-1 {
		f /= 1024
		u++
	}
	prec := 1
	if u == 0 || f >= 100 {
		prec = 0
	}
	return strconv.FormatFloat(f, 'f', prec, 64) + " " + units[u]
}

// decodePicture is data decoded as a picture, named name, unless its header says it has more than maxImagePixels: a
// picture is decoded whole, four bytes a pixel. A kind the header is not read of (SVG) is decoded as it is.
func decodePicture(name string, data []byte) (ui.ImageSource, error) {
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil && int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return nil, tooManyPixels{locales.FilesTooManyPixels(filepath.Base(name), cfg.Width, cfg.Height)}
	}
	return ui.DecodeImage(data)
}

// tooManyPixels is the error of a picture with more pixels than are decoded.
type tooManyPixels struct{ msg string }

func (e tooManyPixels) Error() string { return e.msg }

// keepImage keeps src, the picture of file, as one of those used last; past keptImages, the one used longest ago goes.
func (a *app) keepImage(file string, src ui.ImageSource) {
	if a.images == nil {
		a.images, a.imageUsed = map[string]ui.ImageSource{}, map[string]uint64{}
	}
	a.images[file] = src
	a.useImage(file)
	if len(a.images) <= keptImages {
		return
	}
	old, oldest := "", uint64(0)
	for f, at := range a.imageUsed {
		if f != file && (old == "" || at < oldest) {
			old, oldest = f, at
		}
	}
	delete(a.images, old)
	delete(a.imageUsed, old)
}

// useImage notes that the picture of file was used now.
func (a *app) useImage(file string) {
	a.imageClock++
	a.imageUsed[file] = a.imageClock
}
