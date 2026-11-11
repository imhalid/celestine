package main

import (
	"os"
	"time"
)

// processGone returns when pid no longer runs: on Windows any process can be waited for by its handle.
func processGone(pid int, _ time.Time) {
	if p, err := os.FindProcess(pid); err == nil {
		p.Wait()
	}
}
