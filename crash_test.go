package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCrashLogWritten(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "celestine", "crashes")
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	path := crashLog(dir, "boom", []byte("goroutine 1 [running]:\nmain.x()"), now)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"celestine " + version, "panic: boom", "main.x()", "2026-11-07T12:00:00Z"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("no %q in\n%s", want, b)
		}
	}
	// only the newest twenty stay
	for i := 1; i <= 30; i++ {
		crashLog(dir, i, nil, now.Add(time.Duration(i)*time.Second))
	}
	if n := len(crashLogs(dir)); n != crashKeep {
		t.Fatalf("%d logs kept", n)
	}
	if text, _ := latestCrash(dir); !strings.Contains(text, "panic: 30") {
		t.Fatalf("not the newest: %q", text)
	}
}

// A session that does not end cleanly is told at the next start, with the log it left.
func TestUncleanSession(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "celestine", "crashes")
	now := time.Now()
	if unclean, _, _ := markRunning(dir, "", now); unclean {
		t.Fatal("first start unclean")
	}
	markClean(dir)
	if unclean, _, _ := markRunning(dir, "", now); unclean {
		t.Fatal("a clean end was taken as a crash")
	}
	// no markClean: the process died. It left a log, too
	log := crashLog(dir, "x", []byte("stack"), time.Now().Add(time.Second))
	unclean, got, _ := markRunning(dir, "", now)
	if !unclean || got != log {
		t.Fatalf("unclean %v log %q (want %q)", unclean, got, log)
	}
	// a kill, with no log
	markClean(dir)
	markRunning(dir, "", now)
	os.Remove(log)
	unclean, got, _ = markRunning(dir, "", now)
	if !unclean || got != "" {
		t.Fatalf("a kill: unclean %v log %q", unclean, got)
	}
}

// A frame that panics does not take the app down: the panic is written, the window says what it was,
// and the next frame goes on.
func TestFramePanicSurvives(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.crashDir = filepath.Join(t.TempDir(), "crashes")
	a.testPanic = "kırık kare"
	tt.Frame() // must not panic
	if !a.crashOpen || a.crashText != "kırık kare" {
		t.Fatalf("not noted: open %v %q", a.crashOpen, a.crashText)
	}
	if a.crashPath == "" {
		t.Fatal("no log written")
	}
	if _, ok := tt.Find(msg("somethingWentWrong")); !ok {
		t.Fatalf("the window does not say: %v", tt.Texts())
	}
	// the same panic again is not written again
	before := len(crashLogs(a.crashDir))
	tt.Frame()
	tt.Frame()
	if len(crashLogs(a.crashDir)) != before {
		t.Fatal("the same panic written over and over")
	}
	// and the app goes on once it does not panic
	a.testPanic = ""
	a.crashOpen = false
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	if _, ok := tt.Find(msg("somethingWentWrong")); ok {
		t.Fatal("still showing the fallback")
	}
}

// The command shows the newest log.
func TestShowCrashCommand(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	a.crashDir = filepath.Join(t.TempDir(), "crashes")
	a.showCrash()
	if a.crashOpen || a.notice == "" {
		t.Fatalf("no log, and it opened: %v %q", a.crashOpen, a.notice)
	}
	crashLog(a.crashDir, "eski", []byte("s"), time.Now())
	a.showCrash()
	if !a.crashOpen || !strings.Contains(a.crashText, "panic: eski") {
		t.Fatalf("not shown: %q", a.crashText)
	}
}

// TestRuntimeCrashLog: a session ended by a panic nothing recovered, which the runtime wrote to MyGo's crash.log, is
// told at the next start with what crash.log took in since that session began, once; the crash view shows the
// runtime's last crash when it is newer than the app's own logs (#579).
func TestRuntimeCrashLog(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "celestine", "crashes")
	rt := filepath.Join(root, "logs", "crash.log")
	os.MkdirAll(filepath.Dir(rt), 0o700)
	os.WriteFile(rt, []byte("panic: an older one\n\ngoroutine 1 [running]:\n"), 0o600)
	now := time.Now()
	if unclean, _, _ := markRunning(dir, rt, now); unclean {
		t.Fatal("first start unclean")
	}
	// the session dies: the runtime appends its report
	f, _ := os.OpenFile(rt, os.O_WRONLY|os.O_APPEND, 0)
	f.WriteString("panic: boom\n\ngoroutine 7 [running]:\nmain.deep()\n")
	f.Close()
	unclean, _, text := markRunning(dir, rt, now)
	if !unclean || !strings.HasPrefix(text, "panic: boom") || strings.Contains(text, "older") {
		t.Fatalf("unclean %v, the crash %q", unclean, text)
	}
	// dies again with nothing written: a kill, and the crash before is not told again
	if unclean, _, text := markRunning(dir, rt, now); !unclean || text != "" {
		t.Fatalf("a kill: unclean %v, %q", unclean, text)
	}
	if got := lastRuntimeCrash("panic: a\n\nx\nfatal error: b\n\ny\n"); got != "fatal error: b\n\ny" {
		t.Fatalf("the last crash %q", got)
	}
	// the crash view: the runtime's last crash, newer than the app's own log
	a, _ := newTestApp(t, "x\n")
	a.crashDir, a.crashRT = dir, rt
	crashLog(dir, "ours", nil, now.Add(-time.Hour))
	old := now.Add(-2 * time.Hour)
	os.Chtimes(crashLogs(dir)[0], old, old)
	a.showCrash()
	if !a.crashOpen || a.crashPath != rt || !strings.HasPrefix(a.crashText, "panic: boom") {
		t.Fatalf("shown %q from %q", a.crashText, a.crashPath)
	}
}
