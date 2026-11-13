//go:build !windows

package main

import (
	"os"
	"syscall"
	"time"
)

// processGone waits until pid no longer runs or deadline elapses, probing non-child processes with signal 0.
func processGone(pid int, deadline time.Time) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	for time.Now().Before(deadline) {
		if p.Signal(syscall.Signal(0)) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
