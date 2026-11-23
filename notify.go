package main

import (
	"errors"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/imhalid/celestine/locales"
)

// The system's notifications, through MyGo (#360): what needs the user while Celestine is behind other apps - a sync
// that stopped, a conflict, removals held back, a new version - is said in a notification, and its click brings the
// window back where it is dealt with. While the app is in front a toast says it, as before, and no notification is
// shown. None is shown before the user has said yes: the first time one would have been, the app asks with a toast as
// it comes back to the front, and the switch in Settings ▸ Sounds asks the same; the system may still ask (macOS), and
// a no there turns the switch off again.

// Settings.Notify is what the user said: "" not asked yet, "on" or "off".
const (
	notifyOn  = "on"
	notifyOff = "off"
)

// notifyIDs are the notifications' own IDs, one per thing they are about: a newer one replaces the one still shown,
// and the click of one left from an earlier run still finds what it opens (notifyClicked).
const (
	notifySync   = "celestine-sync"
	notifyUpdate = "celestine-update"
)

// notifyState is what the notifications keep.
type notifyState struct {
	active atomic.Bool // the app is the one in front; set on the main thread, read under a.mu
	missed string      // the title of one that would have been shown before the user was asked: the ask says it
	asked  bool        // the ask was put up this run
}

// notifyShow shows a notification; the tests and the browser, where there is none, set it to nothing.
var notifyShow = func(id, title, body string) error {
	if !mygo.NotificationsSupported() {
		return errNoNotifications
	}
	return mygo.NewNotification(mygo.NotificationOptions{ID: id, Title: title, Body: body, Group: id}).Show()
}

var errNoNotifications = errors.New("notifications are not supported here")

// watchActive follows whether the app is in front, and what a click on one of its notifications opens, from main
// before the window is made: a click that launched the app is told before WhenReady.
func (a *app) watchActive() {
	a.nt.active.Store(true)
	// what takes the app's lock is done away from the call, as windowIn does it: the system calls these on the
	// window's thread, which a frame holding the lock may be waiting for (#545)
	mygo.App.OnDidBecomeActive(func() {
		a.nt.active.Store(true)
		mygo.ClearNotifications() // what is in front of the user is not left waiting in the system's list
		go func() {
			defer a.guard()
			a.do(func() error { a.askNotify(); return nil })
		}()
	})
	mygo.App.OnDidResignActive(func() { a.nt.active.Store(false) })
	mygo.App.OnNotificationClick(func(id string) {
		a.bringToFront()
		go func() {
			defer a.guard()
			a.do(func() error { a.notifyClicked(id); return nil })
		}()
	})
}

// notify says title and body in a notification about id, when the app is behind others and the user said yes; it
// is called under a.mu, and shows it on a goroutine of its own.
func (a *app) notify(id, title, body string) {
	if a.nt.active.Load() || a.settings.Notify == notifyOff {
		return
	}
	if a.settings.Notify == "" {
		a.nt.missed = title // asked when the app comes back (askNotify)
		return
	}
	go func() {
		defer a.guard()
		if err := notifyShow(id, title, body); errors.Is(err, mygo.ErrNotificationsDenied) {
			a.do(func() error { a.notifyDenied(); return nil })
		}
	}()
}

// notifyClicked opens what the notification id is about: the sync's settings, or the update.
func (a *app) notifyClicked(id string) {
	switch id {
	case notifySync:
		a.openSettings()
		a.settingsTab = tabSync
	case notifyUpdate:
		if a.updRel != nil {
			a.updOpen = true
		}
	}
}

// askNotify asks, once a run, whether notifications may be shown, when one would have been and the user has not
// said: a toast naming what they missed, with a button that says yes.
func (a *app) askNotify() {
	if a.settings.Notify != "" || a.nt.missed == "" || a.nt.asked {
		return
	}
	a.nt.asked = true
	a.toastOf(toastInfo, locales.NotifyAsk(a.nt.missed), locales.NotifyAllow(), func() { a.setNotify(true) })
}

// setNotify turns the notifications on or off as the user said. On, a first one says so, which is where the
// system asks for its own permission (macOS); a no there turns them off again.
func (a *app) setNotify(on bool) {
	a.nt.missed = ""
	if !on {
		a.settings.Notify = notifyOff
		a.saveSettings()
		return
	}
	a.settings.Notify = notifyOn
	a.saveSettings()
	title, body := locales.NotifyOnTitle(), locales.NotifyOnBody()
	go func() {
		defer a.guard()
		err := notifyShow("celestine-on", title, body)
		a.do(func() error {
			switch {
			case errors.Is(err, mygo.ErrNotificationsDenied):
				a.notifyDenied()
			case errors.Is(err, errNoNotifications):
				a.say(toastWarn, locales.NotifyUnsupported())
			}
			return nil
		})
	}()
}

// notifyDenied is the system's no: the switch goes off, and the user is told where it is said yes to.
func (a *app) notifyDenied() {
	if a.settings.Notify == notifyOff {
		return
	}
	a.settings.Notify = notifyOff
	a.saveSettings()
	a.say(toastWarn, locales.NotifyDenied())
}
