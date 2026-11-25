package main

import (
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The demo videos: the app films itself. A script drives the real view with
// a Tester, whose clock moves only when it is told, so each frame is drawn at
// its own time however slow the drawing is; a camera crops each frame and the
// frames go to ffmpeg, then again with the captions put on. Run with
// DEMO=dir: dir/carets.mp4, and dir/marine.mp4 with dir/marine.gif.

const (
	demoW, demoH   = 1280, 800  // the window, in points
	demoScale      = 3          // pixels a point: close up, the picture is still sharp
	demoOW, demoOH = 1920, 1200 // the video
	demoFPS        = 60
)

// demoText is what is written in the carets video, a line a caret.
var demoText = []struct {
	caret string
	text  string
}{
	{"Slime", "The night train left the city without a sound. "},
	{"torch", "Somewhere, a single lamp still burned in a window, "},
	{"firefly", "and around it the fireflies kept their small, patient lights. "},
	{"plant", "By morning, something green had grown between the rails, "},
	{"wind", "and the wind carried the rest of the story away."},
}

// film is a video being made: the app, its Tester, the camera and ffmpeg.
type film struct {
	t    *testing.T
	a    *app
	tt   *ui.Tester
	path string
	out  io.WriteCloser
	cmd  *exec.Cmd
	buf  *image.RGBA // the frame as it goes out

	cam, camV [3]float64 // where the camera looks, in points (x, y), and how close (zoom); and how fast they change
	want      [3]float64 // where it is going
	stiff     float64    // how hard it is pulled there
	frames    int
	pace      float64 // how long a letter takes to write, 1 as a person writes fast
	rnd       uint32
	captions  []caption
}

// caption is a line shown at the foot of the video for a while, in seconds.
type caption struct {
	text     string
	from, to float64
}

// newFilm opens the app, in English, on a library like the one a new user
// has, with an empty page called title open, light or dark, and starts
// ffmpeg writing path.
func newFilm(t *testing.T, path, title string, dark bool) *film {
	setLang("en")
	t.Cleanup(func() { setLang("tr") })
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := seed(lib); err != nil {
		t.Fatal(err)
	}
	p, err := lib.Collections[0].NewPage(nil, title)
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(lib)
	a.open(p)
	if dark {
		a.settings.Theme = 2
	}
	tt := ui.NewTester(a.view, demoW, demoH)
	tt.SetScale(demoScale)
	tt.Advance(500 * time.Millisecond)
	if _, ok := tt.TextCaret(); !ok {
		t.Fatal("the editor has no caret")
	}
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "rawvideo", "-pix_fmt", "rgba", "-s", fmt.Sprintf("%dx%d", demoOW, demoOH), "-r", fmt.Sprint(demoFPS), "-i", "-",
		"-c:v", "libx264", "-preset", "fast", "-crf", "10", "-pix_fmt", "yuv420p", path+".raw.mp4")
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	f := &film{t: t, a: a, tt: tt, path: path, out: in, cmd: cmd, buf: image.NewRGBA(image.Rect(0, 0, demoOW, demoOH)), stiff: 10, pace: 1, rnd: 7}
	f.cam = [3]float64{demoW / 2, demoH / 2, 1}
	f.want = f.cam
	return f
}

// close ends the video and puts the captions on it, each in a box at the
// foot: the first fading in, the last out, one following another at once.
func (f *film) close() {
	f.out.Close()
	if err := f.cmd.Wait(); err != nil {
		f.t.Fatal(err)
	}
	raw := f.path + ".raw.mp4"
	// the font under a name the filter can say: its own has brackets
	font := filepath.Join(f.t.TempDir(), "font.ttf")
	if data, err := os.ReadFile(demoFont); err != nil {
		f.t.Fatal(err)
	} else if err := os.WriteFile(font, data, 0o644); err != nil {
		f.t.Fatal(err)
	}
	vf := "null"
	if len(f.captions) > 0 {
		vf = ""
		for i, c := range f.captions {
			if i > 0 {
				vf += ","
			}
			fade := "1"
			switch {
			case len(f.captions) == 1:
				fade = fmt.Sprintf("min(1,min(t-%.3f,%.3f-t)/0.25)", c.from, c.to)
			case i == 0:
				fade = fmt.Sprintf("min(1,(t-%.3f)/0.25)", c.from)
			case i == len(f.captions)-1:
				fade = fmt.Sprintf("min(1,(%.3f-t)/0.25)", c.to)
			}
			vf += fmt.Sprintf("drawtext=fontfile=%s:text='%s':fontsize=36:fontcolor=white:box=1:boxcolor=0x1d1d22@0.85:boxborderw=20:"+
				"x=(w-text_w)/2:y=h-text_h-90:enable='between(t,%.3f,%.3f)':alpha='%s'", font, c.text, c.from, c.to, fade)
		}
	}
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-i", raw, "-vf", vf,
		"-c:v", "libx264", "-preset", "slow", "-crf", "18", "-pix_fmt", "yuv420p", "-movflags", "+faststart", f.path)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		f.t.Fatal(err) // the raw video stays, to try the captions again by hand
	}
	os.Remove(raw)
	f.t.Logf("%s: %d frames, %.1f s", f.path, f.frames, f.now())
}

// demoFont is the captions' font.
const demoFont = "/usr/share/fonts/google-noto-vf/NotoSans[wght].ttf"

// now is where the video is, in seconds.
func (f *film) now() float64 { return float64(f.frames) / demoFPS }

// frame moves the clock on by a frame, the camera after where it is going,
// and sends the frame.
func (f *film) frame() {
	dt := 1.0 / demoFPS
	f.tt.Advance(time.Second / demoFPS)
	// a spring, damped just enough not to overshoot
	k := f.stiff
	for i := range f.cam {
		f.camV[i] += (k*k*(f.want[i]-f.cam[i]) - 2*k*f.camV[i]) * dt
		f.cam[i] += f.camV[i] * dt
	}
	z := f.cam[2]
	w, h := demoW/z, demoH/z
	x := math.Max(0, math.Min(demoW-w, f.cam[0]-w/2))
	y := math.Max(0, math.Min(demoH-h, f.cam[1]-h/2))
	crop(f.buf, f.tt.Image(), x*demoScale, y*demoScale, w*demoScale, h*demoScale)
	if _, err := f.out.Write(f.buf.Pix); err != nil {
		f.t.Fatal(err)
	}
	f.frames++
}

// hold films d of whatever goes on.
func (f *film) hold(d time.Duration) {
	for n := int(d.Seconds() * demoFPS); n > 0; n-- {
		f.frame()
	}
}

// follow points the camera at the caret at zoom z: its line in the middle,
// and the text's column too while the column fits; closer, the caret a
// little right of the middle, where it is going, the column's sides kept in.
func (f *film) follow(z float64) {
	r, ok := f.tt.TextCaret()
	if !ok {
		return
	}
	e := f.a.pane().editorR
	x := float64(e.X + e.W/2)
	if w, margin := demoW/z, 24.0; w < float64(e.W)+2*margin {
		x = math.Max(float64(e.X)-margin+w/2, math.Min(float64(e.X+e.W)+margin-w/2, float64(r.X)-w*0.1))
	}
	f.want = [3]float64{x, float64(r.Y+r.H/2) - 40/z, z}
}

// write types s as a person does: a letter after another, a little faster
// or slower each time, a pause after a word and a longer one after a stop.
func (f *film) write(s string, z float64) {
	for _, r := range s {
		f.tt.Type(string(r))
		f.follow(z)
		gap := (0.055 + 0.04*f.random()) * f.pace
		switch r {
		case ' ':
			gap += 0.03
		case ',':
			gap += 0.22
		case '.':
			gap += 0.45
		}
		f.hold(time.Duration(gap * float64(time.Second)))
	}
}

// erase takes n letters back with Backspace, one every gap, the camera
// after the caret; with mods (Ctrl) a word each time.
func (f *film) erase(n int, mods ui.Modifiers, gap time.Duration, z float64) {
	for range n {
		f.tt.Key(mods, ui.KeyBackspace)
		f.follow(z)
		f.hold(gap)
	}
}

// random is a number in [0, 1), the same ones every time.
func (f *film) random() float64 {
	f.rnd ^= f.rnd << 13
	f.rnd ^= f.rnd >> 17
	f.rnd ^= f.rnd << 5
	return float64(f.rnd%10000) / 10000
}

// crop draws the part x, y, w, h of src over all of dst, its pixels
// blended between the four nearest: the camera between two pixels does not
// jump from one to the other.
func crop(dst, src *image.RGBA, x, y, w, h float64) {
	dw, dh := dst.Rect.Dx(), dst.Rect.Dy()
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	sx, sy := w/float64(dw), h/float64(dh)
	var wg sync.WaitGroup
	rows := 8
	for part := range rows {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := part * dh / rows; j < (part+1)*dh/rows; j++ {
				fy := y + (float64(j)+0.5)*sy - 0.5
				y0 := int(math.Floor(fy))
				ty := fy - float64(y0)
				y0c, y1c := clampInt(y0, sh-1), clampInt(y0+1, sh-1)
				r0, r1 := src.Pix[y0c*src.Stride:], src.Pix[y1c*src.Stride:]
				d := dst.Pix[j*dst.Stride:]
				for i := range dw {
					fx := x + (float64(i)+0.5)*sx - 0.5
					x0 := int(math.Floor(fx))
					tx := fx - float64(x0)
					a, b := clampInt(x0, sw-1)*4, clampInt(x0+1, sw-1)*4
					for c := range 4 {
						top := float64(r0[a+c])*(1-tx) + float64(r0[b+c])*tx
						bot := float64(r1[a+c])*(1-tx) + float64(r1[b+c])*tx
						d[i*4+c] = uint8(top*(1-ty) + bot*ty + 0.5)
					}
				}
			}
		}()
	}
	wg.Wait()
}

func clampInt(v, hi int) int { return max(0, min(v, hi)) }

// caretNamed is the caret called name.
func caretNamed(t *testing.T, name string) caretKind {
	for k := caretKind(0); k < caretCount(); k++ {
		if caretName(k) == name {
			return k
		}
	}
	t.Fatalf("no caret called %q", name)
	return 0
}

// TestDemoCarets films the app as it is, then the camera going close to the
// page while a few lines are written in it, the caret changing as it goes.
func TestDemoCarets(t *testing.T) {
	dir := os.Getenv("DEMO")
	if dir == "" {
		t.Skip()
	}
	f := newFilm(t, filepath.Join(dir, "carets.mp4"), "Night Train", true)
	f.a.caretKind = caretNamed(t, demoText[0].caret)
	f.hold(1500 * time.Millisecond)
	const z = 3.1 // close: a few words of the line, the caret big
	f.stiff = 3.2 // the camera comes in slowly
	f.follow(z)
	f.hold(1800 * time.Millisecond)
	f.stiff = 6
	for _, line := range demoText {
		f.a.caretKind = caretNamed(t, line.caret)
		from := f.now()
		f.write(line.text, z)
		f.captions = append(f.captions, caption{msg(caretName(f.a.caretKind)), from, f.now()})
	}
	f.captions[len(f.captions)-1].to += 1
	f.hold(1200 * time.Millisecond)
	f.stiff = 3.2 // and goes back
	f.want = [3]float64{demoW / 2, demoH / 2, 1}
	f.hold(2500 * time.Millisecond)
	f.close()
}

// TestDemoMarine films the Marine close, following it while a heading is
// written: a slip taken back a letter at a time, a run written fast enough
// for it to grin and its eyes to go gold, and a word taken back at once.
func TestDemoMarine(t *testing.T) {
	dir := os.Getenv("DEMO")
	if dir == "" {
		t.Skip()
	}
	path := filepath.Join(dir, "marine.mp4")
	f := newFilm(t, path, "Notes", true)
	f.a.caretKind = caretNamed(t, "Marine")
	f.tt.Type("#") // a heading: the mark and the space one after the other, as they are typed
	f.tt.Frame()
	f.tt.Type(" ")
	f.tt.Advance(1500 * time.Millisecond) // the mark taken away is an erase to it: its flinch is over before the film starts
	const z = 3.4
	f.follow(z)
	f.cam = f.want // close from the first frame
	f.stiff = 7
	f.hold(900 * time.Millisecond)
	f.pace = 2.2
	f.write("Zero-Downtime ", z)
	f.pace = 1.4
	f.write("Databsae", z)
	f.hold(600 * time.Millisecond)
	f.erase(3, 0, 300*time.Millisecond, z)
	f.write("ase ", z)
	f.pace = 0.8 // fast: it grins, and after a while its eyes go gold
	f.write("Schema Migraitons", z)
	f.hold(700 * time.Millisecond)
	f.erase(1, ui.Ctrl, 900*time.Millisecond, z)
	f.write("Migrations", z)
	f.hold(1800 * time.Millisecond)
	f.close()
	gif := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-i", path,
		"-vf", "fps=24,scale=720:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=128:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle",
		filepath.Join(dir, "marine.gif"))
	gif.Stderr = os.Stderr
	if err := gif.Run(); err != nil {
		t.Fatal(err)
	}
}

// TestDemoToast films a toast coming: the window's bottom right for three seconds, from a moment before the toast
// is put up to its light's last turn round its edge. The toast's shader is on, which the tests turn off; the clock
// moves only by a frame, so the CPU renderer's slowness does not change the timing. Run with DEMO=dir:
// dir/toast.mp4. Its going (toastOut) does not fit these seconds.
func TestDemoToast(t *testing.T) {
	dir := os.Getenv("DEMO")
	if dir == "" {
		t.Skip()
	}
	toastShaders = true
	t.Cleanup(func() { toastShaders = false })
	setLang("en")
	t.Cleanup(func() { setLang("tr") })
	lib, err := OpenLibrary(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := seed(lib); err != nil {
		t.Fatal(err)
	}
	p, err := lib.Collections[0].NewPage(nil, "Notes")
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(lib)
	a.open(p)
	a.settings.Theme = 2
	tt := ui.NewTester(a.view, demoW, demoH)
	tt.SetScale(demoScale)
	tt.Advance(500 * time.Millisecond)

	const (
		w, h  = 480.0, 270.0 // the part filmed, in points
		secs  = 3
		start = 6 // the frame the toast is put up on
	)
	ow, oh := int(w*demoScale), int(h*demoScale)
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "rawvideo", "-pix_fmt", "rgba", "-s", fmt.Sprintf("%dx%d", ow, oh), "-r", fmt.Sprint(demoFPS), "-i", "-",
		"-c:v", "libx264", "-preset", "slow", "-crf", "14", "-pix_fmt", "yuv420p", "-movflags", "+faststart", filepath.Join(dir, "toast.mp4"))
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for n := range secs * demoFPS {
		if n == start {
			a.toastOf(toastOK, "Page moved to the trash", "Undo", func() {})
		}
		tt.Advance(time.Second / demoFPS)
		crop(buf, tt.Image(), (demoW-w)*demoScale, (demoH-h)*demoScale, w*demoScale, h*demoScale)
		if _, err := in.Write(buf.Pix); err != nil {
			t.Fatal(err)
		}
	}
	in.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

// TestDemoCaretSurvey films each caret for a moment, close: run with
// DEMO_SURVEY=dir to choose the ones a video shows.
func TestDemoCaretSurvey(t *testing.T) {
	dir := os.Getenv("DEMO_SURVEY")
	if dir == "" {
		t.Skip()
	}
	for k := caretKind(0); k < caretCount(); k++ {
		f := newFilm(t, filepath.Join(dir, fmt.Sprintf("%02d.mp4", k)), caretName(k), false)
		f.a.caretKind = k
		f.stiff = 30
		f.follow(1.9)
		f.write("The night train left the city without a sound. Somewhere", 1.9)
		f.hold(800 * time.Millisecond)
		f.close()
	}
}
