package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/imhalid/celestine/locales"
)

// Sync and the computer's state, through MyGo. A sync to a server waits while the computer is offline and runs as it
// comes back online (mygo.Network, #340): no pass is tried and failed, and no polling of our own finds the network
// back. A sync to a folder needs no network and is not held. A pass in progress keeps the app at full speed while no
// window of it shows (mygo.Power.KeepRunning, #342), so App Nap does not slow it on macOS, and lets go as it ends.

// watchSync follows the network for sync, and gives the passes MyGo's KeepRunning; from WhenReady. Tests, which run no
// app, call neither: the network is taken as there, and nothing is kept running.
func (a *app) watchSync() {
	a.keepRunning = func(reason string) func() { return mygo.Power.KeepRunning(reason) }
	mygo.Network.OnChanged(func(s mygo.NetworkStatus) {
		a.do(func() error { a.setOnline(s.Online); return nil })
	})
	go func() {
		defer a.guard()
		s := mygo.Network.Status() // the first answer: on macOS it comes within milliseconds
		a.do(func() error { a.setOnline(s.Online); return nil })
	}()
}

// setOnline is the network gone or back: gone, a sync to a server says it waits; back, the next frame starts a pass.
func (a *app) setOnline(on bool) {
	was := a.sy.offline
	a.sy.offline = !on
	switch {
	case !on && a.syncWaits():
		if !a.sy.busy {
			a.sy.status = locales.SyncWaitingForNetwork()
		}
	case on && was && a.settings.SyncMode == "server":
		a.sy.tried = time.Time{} // syncTick starts a pass at once, and asks the server again
		a.sy.askedAt = time.Time{}
		a.sy.status = ""
	}
}

// syncWaits reports whether sync waits for the network: it goes to a server, and the computer is offline.
func (a *app) syncWaits() bool { return a.sy.offline && a.settings.SyncMode == "server" }
