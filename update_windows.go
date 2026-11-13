package main

import (
	"os"
	"time"
)

// processGone waits until pid exits by its Windows process handle.
func processGone(pid int, _ time.Time) {
	if p, err := os.FindProcess(pid); err == nil {
		p.Wait()
	}
}
