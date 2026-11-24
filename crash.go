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

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The crash log. A panic is caught - in a frame of the window, in a goroutine of the app, or in the
// program - and written, with the version and the stack, to a file in the config folder: the library is
// not touched, nothing is sent anywhere. A panic nothing recovers (in one of MyGo's goroutines, or before
// the guard) is written by the runtime to MyGo's crash.log in the logs folder. The next time the app
// starts, if the last session did not end as it should (a file "running" is still there), it says so, and
// shows the log: what crash.log took in since that session began, or else the newest of the app's own.

const crashKeep = 20 // the logs that are kept

// crashDir is where the logs are written.
func crashDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "celestine", "crashes")
	}
	return ""
}

// crashLog writes what a panic was, with the stack, to a new file of dir and prunes the old; it gives the
// file's path, or "" when it could not be written.
func crashLog(dir string, what any, stack []byte, now time.Time) string {
	if dir == "" {
		return ""
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	hardenPrivateDir(dir)
	var b strings.Builder
	fmt.Fprintf(&b, "celestine %s · %s/%s · %s\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	fmt.Fprintf(&b, "%s\n\n", now.Format(time.RFC3339))
	fmt.Fprintf(&b, "panic: %v\n\n%s\n", what, stack)
	path := filepath.Join(dir, "crash-"+now.Format("20060102-150405.000")+".log")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
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

// runtimeCrashLog is the file the runtime writes the panic that ends the app to (MyGo's crash.log), or ""
// where there is no logs folder.
func runtimeCrashLog() string {
	dir, err := mygo.App.Path(mygo.PathLogs)
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "crash.log")
}

// maxRuntimeCrash is how much of what crash.log took in is shown: a crash's head and its goroutines' stacks
// are well inside it.
const maxRuntimeCrash = 64 << 10

// markRunning notes that a session has begun, and reports whether the last one did not end as it
// should, with the newest log it left (which may be none: a kill, a power cut). rt is the runtime's crash
// log; the mark keeps its size, and what it took in since the last session began is that session's end,
// given as text, once.
func markRunning(dir, rt string, now time.Time) (unclean bool, log, rtText string) {
	if dir == "" {
		return false, "", ""
	}
	mark := runningMark(dir)
	rtSize := int64(0)
	if st, err := os.Stat(rt); rt != "" && err == nil {
		rtSize = st.Size()
	}
	if st, err := os.Stat(mark); err == nil {
		unclean = true
		if l := crashLogs(dir); len(l) > 0 {
			if ls, err := os.Stat(l[0]); err == nil && !ls.ModTime().Before(st.ModTime()) {
				log = l[0]
			}
		}
		if b, err := os.ReadFile(mark); err == nil && rtSize > 0 {
			from := int64(0)
			if f := strings.Fields(string(b)); len(f) > 2 {
				fmt.Sscan(f[2], &from)
			}
			if from > rtSize { // started over since: all of it is new
				from = 0
			}
			if rtSize > from {
				rtText = readTail(rt, from, maxRuntimeCrash)
			}
		}
	}
	os.MkdirAll(filepath.Dir(mark), 0o700)
	hardenPrivateDir(filepath.Dir(mark))
	os.WriteFile(mark, []byte(fmt.Sprintf("%s %s %d\n", now.Format(time.RFC3339), version, rtSize)), 0o600)
	_ = os.Chmod(mark, 0o600)
	return unclean, log, strings.TrimSpace(rtText)
}

// lastRuntimeCrash is the last crash of text, what the runtime wrote to its crash log: from the last line that
// begins one ("panic: ", "fatal error: ") on, or all of it when none does.
func lastRuntimeCrash(text string) string {
	at := -1
	for _, head := range []string{"panic: ", "fatal error: "} {
		if strings.HasPrefix(text, head) {
			at = max(at, 0)
		}
		if i := strings.LastIndex(text, "\n"+head); i >= 0 {
			at = max(at, i+1)
		}
	}
	return strings.TrimSpace(text[max(at, 0):])
}

// runtimeLogOf is the runtime's crash log this app reads: MyGo's, unless a test has its own.
func (a *app) runtimeLogOf() string {
	if a.crashDir != "" {
		return a.crashRT
	}
	return runtimeCrashLog()
}

// readTail is the text of file path from byte from on, its first max bytes at most.
func readTail(path string, from, max int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	b := make([]byte, max)
	n, _ := f.ReadAt(b, from)
	return string(b[:n])
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
		stack := debug.Stack()
		a.mu.Lock() // the fields it sets are the window's, read in its frames
		a.noteCrash(r, stack)
		a.mu.Unlock()
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
		ui.Text(c, locales.SomethingWentWrong()).FontSize(22).FontWeight(700).TextColor(cl.Text)
		ui.Text(c, a.crashText).FontSize(13).TextColor(cl.Muted).MaxWidth(560).Wrap()
		if a.crashPath != "" {
			a.textMenu(ui.Text(c, locales.AppLogAt(a.crashPath)).FontSize(12).Font(monoStack).TextColor(cl.Muted).MaxWidth(560).Wrap().Selectable(), func() string { return locales.AppLogAt(a.crashPath) }, nil)
		}
		if ryugo.Button(c.Key("crash-retry"), locales.Continue2(), ryugo.ButtonOptions{}).Clicked() {
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
	title := locales.AnUnexpectedErrorOccurred()
	if a.crashPrev != "" || a.crashPath == "" && strings.HasPrefix(a.crashText, "Son oturum") {
		title = locales.TheLastSessionEndedUnexpectedly()
	}
	ryugo.Dialog(c.Key("crash"), &a.crashOpen, ryugo.DialogOptions{Title: title, Width: 620}, func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.Text(c, locales.AppYourLibraryWasNot()).FontSize(13).TextColor(cl.Muted).Wrap()
			a.scrolled(c, "crash-scroll", nil, ui.Scroll(c.Key("crash-scroll")).MaxHeight(260), func() {
				ui.Box(c).FillWidth().Padding(10, 12).Radius(rad(8)).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
					a.textMenu(ui.Text(c, a.crashText).FontSize(11.5).Font(monoStack).TextColor(cl.Text2).Selectable().Wrap(), func() string { return a.crashText }, nil)
				})
			})
			if a.crashPath != "" {
				a.textMenu(ui.Text(c, a.crashPath).FontSize(11.5).Font(monoStack).TextColor(cl.Muted).Selectable().Wrap(), func() string { return a.crashPath }, nil)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ryugo.Button(c.Key("crash-copy"), locales.CommonCopy(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.copyText(a.crashText)
					a.say(toastOK, locales.TheLogWasCopiedToThe())
				}
				if ryugo.Button(c.Key("crash-close"), locales.Close(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
					a.crashOpen = false
				}
			})
		})
	})
}

// showCrash shows the newest log, or says there is none.
func (a *app) showCrash() {
	text, path := latestCrash(a.crashDirOf())
	if rt := a.runtimeLogOf(); rt != "" { // the runtime's, when it wrote after the newest of ours
		st, err := os.Stat(rt)
		own, ownErr := os.Stat(path)
		if err == nil && st.Size() > 0 && (path == "" || ownErr == nil && st.ModTime().After(own.ModTime())) {
			if t := lastRuntimeCrash(readTail(rt, max(st.Size()-maxRuntimeCrash, 0), maxRuntimeCrash)); t != "" {
				text, path = t, rt
			}
		}
	}
	if text == "" {
		a.say(toastWarn, locales.ThereIsNoCrashLog())
		return
	}
	a.crashText, a.crashPath, a.crashPrev, a.crashOpen = text, path, "", true
}
