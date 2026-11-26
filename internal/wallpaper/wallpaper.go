// Package wallpaper finds the desktop's wallpaper and makes of it what the app's glass needs (#606): a small blurred
// picture of it, and its color, held within bounds where a light or a dark window's text stays crisp on it. It knows
// nothing of the UI: it asks the system where the picture is, reads it, and gives images and colors back.
package wallpaper

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	_ "image/gif" // the formats a wallpaper can be in that Go reads
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

// ErrNone is a wallpaper that cannot be found or read: the app keeps its own ground.
var ErrNone = errors.New("no wallpaper to read")

// Source is where the wallpaper was found: a file, and when it was last written, to tell a new one by.
type Source struct {
	Path string
	Mod  time.Time
	// Color is the color the system says the desktop has under or around its picture, when it says one (GNOME's
	// primary color): a fallback for a picture that cannot be read.
	Color string
}

// run is how a command is run: a test puts its own.
var run = func(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

// Find is where the wallpaper of the desktop is, for a dark or a light appearance where the desktop has one of each.
func Find(dark bool) (Source, error) {
	var s Source
	switch runtime.GOOS {
	case "darwin":
		p, err := run("osascript", "-e", `tell application "System Events" to get picture of current desktop`)
		if err != nil || p == "" {
			return s, ErrNone
		}
		s.Path = p
	case "windows":
		s.Path = filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Themes", "TranscodedWallpaper")
	case "linux", "freebsd", "openbsd":
		key := "picture-uri"
		if dark {
			key = "picture-uri-dark"
		}
		uri, err := run("gsettings", "get", "org.gnome.desktop.background", key)
		if err == nil && dark && strings.Trim(uri, "'\" ") == "" {
			uri, err = run("gsettings", "get", "org.gnome.desktop.background", "picture-uri")
		}
		if err == nil {
			s.Path = pathOfURI(uri)
			if c, err := run("gsettings", "get", "org.gnome.desktop.background", "primary-color"); err == nil {
				s.Color = strings.Trim(c, "'\" ")
			}
		}
		if s.Path == "" {
			s.Path = kdeWallpaper()
		}
	default:
		return s, ErrNone
	}
	if s.Path == "" {
		return s, ErrNone
	}
	if info, err := os.Stat(s.Path); err == nil {
		s.Mod = info.ModTime()
	} else if s.Color == "" {
		return s, ErrNone
	}
	return s, nil
}

// pathOfURI is the file of a file:// URI as gsettings prints it, quoted.
func pathOfURI(uri string) string {
	uri = strings.Trim(strings.TrimSpace(uri), "'\"")
	p, ok := strings.CutPrefix(uri, "file://")
	if !ok {
		return ""
	}
	return strings.ReplaceAll(p, "%20", " ")
}

// kdeWallpaper is the picture of Plasma's first desktop, from its configuration.
func kdeWallpaper() string {
	home, _ := os.UserHomeDir()
	b, err := os.ReadFile(filepath.Join(home, ".config", "plasma-org.kde.plasma.desktop-appletsrc"))
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "Image="); ok {
			return pathOfURI(v)
		}
	}
	return ""
}

// Load reads the wallpaper's picture. A format Go does not read is read through what the system has for it: macOS's
// sips makes a PNG of a HEIC, and GNOME's own thumbnail of the file stands for a JPEG XL or an SVG. A source with no
// picture to read but a color is a picture of that color.
func Load(s Source) (image.Image, error) {
	if s.Path != "" {
		if img, err := decodeFile(s.Path); err == nil {
			return img, nil
		}
		if runtime.GOOS == "darwin" {
			tmp := filepath.Join(os.TempDir(), "celestine-wallpaper.png")
			if _, err := run("sips", "-Z", "640", "-s", "format", "png", s.Path, "--out", tmp); err == nil {
				defer os.Remove(tmp)
				if img, err := decodeFile(tmp); err == nil {
					return img, nil
				}
			}
		}
		if img, err := decodeFile(thumbnailOf(s.Path)); err == nil {
			return img, nil
		}
	}
	if c, ok := parseHex(s.Color); ok {
		img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
		}
		return img, nil
	}
	return nil, ErrNone
}

func decodeFile(path string) (image.Image, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	return img, err
}

// thumbnailOf is the freedesktop thumbnail of a file, the large one: its name the MD5 of the file's URI.
func thumbnailOf(path string) string {
	home, _ := os.UserHomeDir()
	sum := md5.Sum([]byte("file://" + path))
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		cache = filepath.Join(home, ".cache")
	}
	return filepath.Join(cache, "thumbnails", "large", hex.EncodeToString(sum[:])+".png")
}

// parseHex is a color written #RRGGBB.
func parseHex(s string) (color.NRGBA, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return color.NRGBA{}, false
	}
	var v [3]uint8
	for i := range v {
		var b uint8
		for _, ch := range s[2*i : 2*i+2] {
			b <<= 4
			switch {
			case ch >= '0' && ch <= '9':
				b |= uint8(ch - '0')
			case ch >= 'a' && ch <= 'f':
				b |= uint8(ch-'a') + 10
			case ch >= 'A' && ch <= 'F':
				b |= uint8(ch-'A') + 10
			default:
				return color.NRGBA{}, false
			}
		}
		v[i] = b
	}
	return color.NRGBA{v[0], v[1], v[2], 255}, true
}

// Color is the picture's color: its average, leaning toward its most colorful part, in OKLab so that a busy picture's
// colors mix as the eye mixes them.
func Color(img image.Image) color.NRGBA {
	b := img.Bounds()
	if b.Empty() {
		return color.NRGBA{A: 255}
	}
	var L, A, B, w float64
	const n = 24
	for j := range n {
		for i := range n {
			x := b.Min.X + (2*i+1)*b.Dx()/(2*n)
			y := b.Min.Y + (2*j+1)*b.Dy()/(2*n)
			l, a, bb := toOKLab(img.At(x, y))
			weight := 1 + 4*math.Hypot(a, bb) // a colorful sample counts more: the picture's color, not its gray
			L, A, B, w = L+l*weight, A+a*weight, B+bb*weight, w+weight
		}
	}
	return fromOKLab(L/w, A/w, B/w)
}

// Clamp is c held where a window's ground can be: light and soft for a light window, deep and soft for a dark one,
// so that text and hairlines stay crisp on it whatever the picture is.
func Clamp(c color.NRGBA, dark bool) color.NRGBA {
	l, a, b := toOKLab(c)
	chroma, hue := math.Hypot(a, b), math.Atan2(b, a)
	lo, hi, maxC := 0.86, 0.95, 0.06
	if dark {
		lo, hi, maxC = 0.17, 0.27, 0.07
	}
	l = min(max(l, lo), hi)
	chroma = min(chroma, maxC)
	return fromOKLab(l, chroma*math.Cos(hue), chroma*math.Sin(hue))
}

// Lightness is c's lightness in OKLab, 0 to 1.
func Lightness(c color.Color) float64 {
	l, _, _ := toOKLab(c)
	return l
}

// Blurred is the picture made small (its longer side side pixels at most) and blurred by about radius of those
// pixels: what stands behind the window's glass, which shows only its colors and their shapes.
func Blurred(img image.Image, side, radius int) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	k := float64(side) / float64(max(w, h))
	k = min(k, 1)
	sw, sh := max(1, int(float64(w)*k)), max(1, int(float64(h)*k))
	out := image.NewNRGBA(image.Rect(0, 0, sw, sh))
	// each pixel the average of the source's pixels under it
	for y := range sh {
		y0, y1 := b.Min.Y+y*h/sh, b.Min.Y+max((y+1)*h/sh, y*h/sh+1)
		for x := range sw {
			x0, x1 := b.Min.X+x*w/sw, b.Min.X+max((x+1)*w/sw, x*w/sw+1)
			var r, g, bl, n uint32
			step := max(1, (x1-x0)/4)
			stepY := max(1, (y1-y0)/4)
			for yy := y0; yy < y1; yy += stepY {
				for xx := x0; xx < x1; xx += step {
					cr, cg, cb, _ := img.At(xx, yy).RGBA()
					r, g, bl, n = r+cr>>8, g+cg>>8, bl+cb>>8, n+1
				}
			}
			o := out.PixOffset(x, y)
			out.Pix[o], out.Pix[o+1], out.Pix[o+2], out.Pix[o+3] = uint8(r/n), uint8(g/n), uint8(bl/n), 255
		}
	}
	for range 3 {
		boxBlur(out, radius)
	}
	return out
}

// boxBlur blurs img in place by a box of radius r, along x and then y, the edges held.
func boxBlur(img *image.NRGBA, r int) {
	if r < 1 {
		return
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	tmp := make([]uint8, len(img.Pix))
	pass := func(src, dst []uint8, lines, length, step, stride int) {
		for i := range lines {
			for ch := range 3 {
				var sum int
				at := func(j int) int { return int(src[i*stride+min(max(j, 0), length-1)*step+ch]) }
				for j := -r; j <= r; j++ {
					sum += at(j)
				}
				for j := range length {
					dst[i*stride+j*step+ch] = uint8(sum / (2*r + 1))
					sum += at(j+r+1) - at(j-r)
				}
			}
			for j := range length {
				dst[i*stride+j*step+3] = 255
			}
		}
	}
	pass(img.Pix, tmp, h, w, 4, img.Stride)
	pass(tmp, img.Pix, w, h, img.Stride, 4)
}

// toOKLab is c in OKLab.
func toOKLab(c color.Color) (l, a, b float64) {
	r, g, bl, al := c.RGBA()
	if al == 0 {
		return 0, 0, 0
	}
	lin := func(v uint32) float64 {
		x := float64(v) / float64(al)
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	R, G, B := lin(r), lin(g), lin(bl)
	L := math.Cbrt(0.4122214708*R + 0.5363325363*G + 0.0514459929*B)
	M := math.Cbrt(0.2119034982*R + 0.6806995451*G + 0.1073969566*B)
	S := math.Cbrt(0.0883024619*R + 0.2817188376*G + 0.6299787005*B)
	return 0.2104542553*L + 0.7936177850*M - 0.0040720468*S,
		1.9779984951*L - 2.4285922050*M + 0.4505937099*S,
		0.0259040371*L + 0.7827717662*M - 0.8086757660*S
}

// fromOKLab is the color of l, a, b in OKLab, held within sRGB.
func fromOKLab(l, a, b float64) color.NRGBA {
	L := l + 0.3963377774*a + 0.2158037573*b
	M := l - 0.1055613458*a - 0.0638541728*b
	S := l - 0.0894841775*a - 1.2914855480*b
	L, M, S = L*L*L, M*M*M, S*S*S
	R := 4.0767416621*L - 3.3077115913*M + 0.2309699292*S
	G := -1.2684380046*L + 2.6097574011*M - 0.3413193965*S
	B := -0.0041960863*L - 0.7034186147*M + 1.7076147010*S
	enc := func(x float64) uint8 {
		x = min(max(x, 0), 1)
		if x <= 0.0031308 {
			x *= 12.92
		} else {
			x = 1.055*math.Pow(x, 1/2.4) - 0.055
		}
		return uint8(math.Round(x * 255))
	}
	return color.NRGBA{enc(R), enc(G), enc(B), 255}
}
