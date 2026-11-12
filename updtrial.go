package main

import (
	"fmt"
	"os"
	"time"
)

// The update trial: with CELESTINE_UPDATE=1 the app plays the whole update flow without a release on a server, to
// see and try it (#392). Every check, the one a moment after the start and Check now, finds a waiting version, one
// patch above this one, whatever the settings say; Update now plays a download with its progress and touches no
// file; Restart starts the program again, which shows its "what's new" as an update's first start does. Nothing of
// it is saved to the settings but what closing "what's new" saves, and the real flow is as it is without it.

// updTrial is whether the trial is on; updTrialRestarted, whether this run is the restart of one that played an
// install, and so shows its "what's new" once.
var (
	updTrialRestarted = os.Getenv("CELESTINE_UPDATE") == "restarted"
	updTrial          = os.Getenv("CELESTINE_UPDATE") == "1" || updTrialRestarted
)

// trialRelease is the version the trial finds: one patch above this one, out today; v0.0.1 for a build from the
// source.
func (a *app) trialRelease() *Release {
	next := "v0.0.1"
	if n, _, ok := parseVersion(version); ok {
		next = fmt.Sprintf("v%d.%d.%d", n[0], n[1], n[2]+1)
	}
	return &Release{Version: next, Date: a.clock().Format("2006-01-02")}
}

// trialCheck is checkUpdate in the trial: after a moment, as an answer over the network comes, the version is
// there and its card opens (or, with automatic updates on, it downloads).
func (a *app) trialCheck() {
	if a.updBusy.Swap(true) {
		return
	}
	go func() {
		defer a.guard()
		defer a.updBusy.Store(false)
		time.Sleep(600 * time.Millisecond)
		a.do(func() error {
			a.updErr, a.updRel = "", a.trialRelease()
			if a.updState == updIdle {
				if a.settings.UpdateAuto {
					a.updAuto = true
				} else {
					a.updOpen = true
				}
			}
			return nil
		})
	}()
}

// trialSize is the size of the download the trial plays, and trialTime how long it takes (shorter in a test).
const trialSize = 24 << 20

var trialTime = 4 * time.Second

// trialInstall is startUpdate in the trial: the bytes come in over trialTime, and the version is ready for a
// restart. No file is downloaded or replaced, and no backup taken.
func (a *app) trialInstall() {
	a.updState = updDownloading
	a.updProg.got.Store(0)
	a.updProg.size.Store(trialSize)
	go func() {
		defer a.guard()
		defer a.updBusy.Store(false)
		const steps = 40
		for i := 0; i < steps; i++ {
			time.Sleep(trialTime / steps)
			a.updProg.got.Add(trialSize / steps)
		}
		a.do(func() error {
			a.updState = updReady
			return nil
		})
	}()
}
