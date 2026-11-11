package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// An askState is a question put before an action that cannot be taken back: a dialog with what it does, a button
// that does it and one that does not (#217).
type askState struct {
	open                bool
	title, body, action string
	run                 func()
	keep                func() // what not doing it does, or nothing
}

// ask puts the question title, with body under it, before run; action is the label of the button that runs it.
func (a *app) ask(title, body, action string, run func()) {
	a.asking = askState{open: true, title: title, body: body, action: action, run: run}
}

// askView is the question, while there is one.
func (a *app) askView(c *ui.Context) {
	q := &a.asking
	if q.run == nil {
		return
	}
	was := q.open
	cl := ryugo.From(c).Colors
	ryugo.Dialog(c.Key("ask"), &q.open, ryugo.DialogOptions{Title: q.title, Width: 420, Footer: func() {
		ui.Spacer(c)
		if ryugo.Button(c.Key("ask-no"), locales.CommonCancel2(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}).Clicked() {
			q.open = false
		}
		if ryugo.Button(c.Key("ask-yes"), q.action, ryugo.ButtonOptions{Size: ryugo.Sm, Color: cl.Up, Variant: ryugo.Solid}).Clicked() {
			run := q.run
			*q = askState{}
			run()
		}
	}}, func() {
		ui.Text(c, q.body).FontSize(13).TextColor(cl.Text2).Wrap().FillWidth()
	})
	if was && !q.open && q.run != nil { // not done: closed by Cancel, Esc or the scrim
		keep := q.keep
		*q = askState{}
		if keep != nil {
			keep()
		}
	}
}

// undoSettings puts up text with a button that brings the settings back to before: what changes many settings at
// once (all back to their defaults, a profile) can be taken back as one change can (#217).
func (a *app) undoSettings(text string, before Settings) {
	a.toastAction(text, locales.CommonUndo(), func() {
		a.settings = before
		a.caretKind = a.savedCaret()
		setLang(a.settings.Lang)
		a.saveSettings()
	})
}
