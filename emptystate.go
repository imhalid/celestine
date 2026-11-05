package main

import (
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// emptyState is what a view with nothing to show shows: Ryugo's empty state - a
// quiet icon, what is so, what to do about it, and, when there is one thing to do
// at once, a button for it (action and run; none without).
func (a *app) emptyState(c *ui.Context, icon *ui.SVG, title, desc, action string, run func()) {
	o := ryugo.EmptyStateOptions{Icon: icon, Title: title, Description: desc}
	if run != nil {
		o.Action = func() {
			if ryugo.Button(c.Key("empty-action"), action, ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
				run()
			}
		}
	}
	ui.Column(c.Key("empty-state")).FillWidth().Children(func() { ryugo.EmptyState(c, o) })
}
