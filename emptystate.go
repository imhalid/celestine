package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
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
	o.Title, o.Description = T(o.Title), T(o.Description)
	ui.Column(c.Key("empty-state")).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
		ryugo.EmptyState(c, o)
		// a hint that teaches: the one key that leads to all the others
		ui.Text(c.Key("empty-hint"), locales.PagePCommandsKFind()).FontSize(12).TextColor(ryugo.From(c).Colors.Muted)
	})
}
