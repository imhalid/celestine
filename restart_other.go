//go:build !windows

package main

import (
	"os"
	"syscall"
	"time"
)

// processGone returns when pid no longer runs, or at the deadline: a process that is not this one's child
// cannot be waited for here, so it is asked after with the signal 0.
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
