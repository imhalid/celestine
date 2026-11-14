package main

import (
	"testing"
	"time"
)

// TestSyncWaitsOffline: offline, a sync to a server tries no pass and says it waits; back online, the next frame
// starts one. A sync to a folder does not wait (#340).
func TestSyncWaitsOffline(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	a.mu.Lock()
	a.settings.SyncMode, a.settings.SyncURL = "server", "http://127.0.0.1:1"
	a.setOnline(false)
	a.mu.Unlock()
	tt.Advance(300 * time.Millisecond)
	a.mu.Lock()
	tried, waits := a.sy.tried, a.syncWaits()
	a.mu.Unlock()
	if !tried.IsZero() || !waits {
		t.Fatalf("offline, a pass was tried (%v) or it does not wait (%v)", tried, waits)
	}
	a.mu.Lock()
	a.setOnline(true)
	a.mu.Unlock()
	tt.Advance(300 * time.Millisecond)
	a.mu.Lock()
	tried = a.sy.tried
	a.settings.SyncMode = "dir"
	a.setOnline(false)
	waits = a.syncWaits()
	a.settings.SyncMode = ""
	a.mu.Unlock()
	if tried.IsZero() {
		t.Fatal("back online, no pass started")
	}
	if waits {
		t.Fatal("a sync to a folder waits for the network")
	}
}
