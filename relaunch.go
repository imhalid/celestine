package main

import (
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/egoist/mygo"
)

// Starting the app again: after an update, a trial's restart, or another library opened. MyGo's Relaunch starts the
// program as it runs, which in an AppImage is the program inside its mount: after an update the AppImage file is a
// new one, and the mount the old one's, so Restart started the old version, or nothing once the mount was gone, and
// the update was neither proved nor rolled back (#528). From an AppImage, the AppImage file is started instead, with
// the same arguments, as the app quits: after it has stopped taking the command lines of other instances, so the new
// one waits for this one to exit, which holds the single instance lock until then, as MyGo's own Relaunch does.

// relaunchImage is the AppImage to start as the app quits, and its arguments; "" for none.
var relaunchImage struct {
	path string
	args []string
}

// relaunch starts the app again with args (none: the ones it started with) and quits this one.
func (a *app) relaunch(args ...string) {
	img := appImagePath()
	if img == "" {
		mygo.App.Relaunch(args...)
		return
	}
	if len(args) == 0 {
		args = os.Args[1:]
	}
	relaunchImage.path, relaunchImage.args = img, slices.Clone(args)
	appQuit()
}

// appQuit quits the app; a test sees it asked for.
var appQuit = mygo.App.Quit

// relaunchAtQuit starts the AppImage relaunch asked for, if any: it is the app's last OnQuit.
func relaunchAtQuit() {
	if relaunchImage.path == "" {
		return
	}
	if cmd := relaunchCmd(relaunchImage.path, relaunchImage.args, os.Environ()); cmd.Start() == nil {
		cmd.Process.Release()
	}
}

// relaunchCmd is the command that starts the AppImage img with args: in the folder the app started in, and without
// the mount's APPIMAGE and APPDIR, which the AppImage's runtime sets anew for its own mount.
func relaunchCmd(img string, args, env []string) *exec.Cmd {
	cmd := exec.Command(img, args...)
	cmd.Env = withoutMount(env)
	if dir, err := os.Getwd(); err == nil {
		cmd.Dir = dir
	}
	return cmd
}

// withoutMount is env without the APPIMAGE and APPDIR an AppImage's runtime sets for its mount.
func withoutMount(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, "APPIMAGE=") && !strings.HasPrefix(kv, "APPDIR=") {
			out = append(out, kv)
		}
	}
	return out
}
