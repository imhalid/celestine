package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// restartFlag tells a start that the program before it is quitting to make room for it: an update's restart
// or a rollback's. It carries that program's pid, and the start waits for it to be gone before it takes the
// one-instance lock, or it would hand its command line to the leaving copy and go, leaving no window (#232).
const restartFlag = "--restarted="

// restartArgs is the command line of the start that follows this one: its own, with this pid.
func restartArgs() []string {
	args, _ := withoutRestartFlag(os.Args[1:])
	return append(args, restartFlag+strconv.Itoa(os.Getpid()))
}

// withoutRestartFlag takes restartFlag out of args, giving the pid it carried (0 without it).
func withoutRestartFlag(args []string) ([]string, int) {
	pid := 0
	out := make([]string, 0, len(args))
	for _, s := range args {
		if v, ok := strings.CutPrefix(s, restartFlag); ok {
			pid, _ = strconv.Atoi(v)
			continue
		}
		out = append(out, s)
	}
	return out, pid
}

// waitForExit waits until the process pid is gone, or for at most max: the program before quits after
// saving and giving the sync a few seconds.
func waitForExit(pid int, max time.Duration) {
	if pid <= 0 || pid == os.Getpid() {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		processGone(pid, time.Now().Add(max))
	}()
	select {
	case <-done:
	case <-time.After(max):
	}
}
