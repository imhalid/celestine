package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The crash log. A panic is caught - in a frame of the window, in a goroutine of the app, or in the
// program - and written, with the version and the stack, to a file in the config folder: the library is
// not touched, nothing is sent anywhere. The next time the app starts, if the last session did not end
// as it should (a file "running" is still there), it says so, and shows the log.

const crashKeep = 20 // the logs that are kept

// crashDir is where the logs are written.
func crashDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "ryudit", "crashes")
	}
	return ""
}

// crashLog writes what a panic was, with the stack, to a new file of dir and prunes the old; it gives the
// file's path, or "" when it could not be written.
func crashLog(dir string, what any, stack []byte, now time.Time) string {
	if dir == "" {
		return ""
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ryudit %s · %s/%s · %s\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	fmt.Fprintf(&b, "%s\n\n", now.Format(time.RFC3339))
	fmt.Fprintf(&b, "panic: %v\n\n%s\n", what, stack)
	path := filepath.Join(dir, "crash-"+now.Format("20060102-150405.000")+".log")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return ""
	}
	pruneCrashes(dir, crashKeep)
	return path
}

// pruneCrashes keeps the newest keep of the logs.
func pruneCrashes(dir string, keep int) {
	files := crashLogs(dir)
	for i := keep; i < len(files); i++ {
		os.Remove(files[i])
	}
}

// crashLogs are the logs of dir, newest first.
func crashLogs(dir string) []string {
	m, _ := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	sort.Sort(sort.Reverse(sort.StringSlice(m)))
	return m
}

// runningMark is the file that says a session is on: it is there from the start to a clean end.
func runningMark(dir string) string { return filepath.Join(filepath.Dir(dir), "running") }

// markRunning notes that a session has begun, and reports whether the last one did not end as it
// should, with the newest log it left (which may be none: a kill, a power cut).
func markRunning(dir string, now time.Time) (unclean bool, log string) {
	if dir == "" {
		return false, ""
	}
	mark := runningMark(dir)
	if st, err := os.Stat(mark); err == nil {
		unclean = true
		if l := crashLogs(dir); len(l) > 0 {
			if ls, err := os.Stat(l[0]); err == nil && !ls.ModTime().Before(st.ModTime()) {
				log = l[0]
			}
		}
	}
	os.MkdirAll(filepath.Dir(mark), 0o755)
	os.WriteFile(mark, []byte(now.Format(time.RFC3339)+" "+version+"\n"), 0o644)
	return unclean, log
}

// markClean notes that the session ended as it should.
func markClean(dir string) {
	if dir != "" {
		os.Remove(runningMark(dir))
	}
}

// guard is deferred at the top of a goroutine: a panic in it is written to the log, and the app goes on
// without that goroutine.
func (a *app) guard() {
	if r := recover(); r != nil {
		a.noteCrash(r, debug.Stack())
	}
}

// crashDirOf is where this app writes its logs: the config folder's, unless a test has its own.
func (a *app) crashDirOf() string {
	if a.crashDir != "" {
		return a.crashDir
	}
	return crashDir()
}

// noteCrash writes a panic to the log, once for the same message in a minute, and keeps it to show.
func (a *app) noteCrash(r any, stack []byte) {
	msg := fmt.Sprint(r)
	now := time.Now()
	if a.crashMsg == msg && now.Sub(a.crashAt) < time.Minute {
		return
	}
	a.crashMsg, a.crashAt = msg, now
	a.crashPath = crashLog(a.crashDirOf(), r, stack, now)
	a.crashText = msg
	a.crashOpen = true
}

// latestCrash is the text of the newest log, and its path: "" when there is none.
func latestCrash(dir string) (text, path string) {
	l := crashLogs(dir)
	if len(l) == 0 {
		return "", ""
	}
	b, err := os.ReadFile(l[0])
	if err != nil {
		return "", ""
	}
	return string(b), l[0]
}

// crashFallback is what a frame shows after it panicked: what it was, and a way on.
func (a *app) crashFallback(c *ui.Context) {
	cl := ryugo.From(c).Colors
	ui.Column(c.Key("crashed")).Fill().Padding(40).Gap(14).Justify(ui.Center).AlignItems(ui.Center).Background(c.Theme().Background).Children(func() {
		ui.Text(c, "Bir hata oldu").FontSize(22).FontWeight(700).TextColor(cl.Text)
		ui.Text(c, a.crashText).FontSize(13).TextColor(cl.Muted).MaxWidth(560).Wrap()
		if a.crashPath != "" {
			ui.Text(c, T("Günlük: ")+a.crashPath).FontSize(12).Font(ryugo.FontMono).TextColor(cl.Muted).MaxWidth(560).Wrap().Selectable()
		}
		if ryugo.Button(c.Key("crash-retry"), "Devam et", ryugo.ButtonOptions{}).Clicked() {
			a.crashOpen = false
			c.Invalidate()
		}
	})
}

// crashView is the dialog that tells of a crash: the last session's, at the start, or one of this
// session that the window got over.
func (a *app) crashView(c *ui.Context) {
	if !a.crashOpen {
		return
	}
	cl := ryugo.From(c).Colors
	title := "Beklenmedik bir hata oldu"
	if a.crashPrev != "" || a.crashPath == "" && strings.HasPrefix(a.crashText, "Son oturum") {
		title = "Son oturum beklenmedik kapandı"
	}
	ryugo.Dialog(c.Key("crash"), &a.crashOpen, ryugo.DialogOptions{Title: title, Width: 620}, func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.Text(c, T("Kitaplığına dokunulmadı. Günlük yalnız bu bilgisayarda durur, hiçbir yere gönderilmez.")).FontSize(13).TextColor(cl.Muted).Wrap()
			a.scrolled(c, "crash-scroll", nil, ui.Scroll(c.Key("crash-scroll")).MaxHeight(260), func() {
				ui.Box(c).FillWidth().Padding(10, 12).Radius(rad(8)).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
					ui.Text(c, a.crashText).FontSize(11.5).Font(ryugo.FontMono).TextColor(cl.Text2).Selectable().Wrap()
				})
			})
			if a.crashPath != "" {
				ui.Text(c, a.crashPath).FontSize(11.5).Font(ryugo.FontMono).TextColor(cl.Muted).Selectable().Wrap()
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ryugo.Button(c.Key("crash-copy"), "Kopyala", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.copyText(a.crashText)
					a.notice = "Günlük panoya kopyalandı"
				}
				if ryugo.Button(c.Key("crash-close"), "Kapat", ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.crashOpen = false
				}
			})
		})
	})
}

// showCrash shows the newest log, or says there is none.
func (a *app) showCrash() {
	text, path := latestCrash(a.crashDirOf())
	if text == "" {
		a.notice = "Hiç çökme günlüğü yok."
		return
	}
	a.crashText, a.crashPath, a.crashPrev, a.crashOpen = text, path, "", true
}
