package main

import (
	"slices"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// A list of rows reordered by a grip, for the collection's settings: each
// row a tile with its grip (six dots, an icon), what it is, and a pen; the
// one open is outlined in the accent with its editor under it, inside the
// tile; a row picked by its grip stays faint where it was, a line shows
// where it would go, and let go it moves there.

// sortState is a list's drag.
type sortState struct {
	pressed, dragging bool
	from, to          int
	moved             float32
	rects, frozen     []ui.Rect // the rows, as drawn; as they were when the drag began
}

// sortOpts says what a list shows.
type sortOpts struct {
	n        int
	id       func(i int) string
	label    func(i int) string // a row's accessible name
	selected string             // the id of the row open
	content  func(i int)        // the middle of a row
	toggle   func(i int)        // a click on a row
	editor   func(i int)        // under the row open
	move     func(from, to int) // to: the place among the rows as they are, 0..n
}

// moveItem moves items[from] to the place to (0..len), as the rows were.
func moveItem[T any](items []T, from, to int) []T {
	if to == from || to == from+1 || from < 0 || from >= len(items) {
		return items
	}
	v := items[from]
	items = slices.Delete(items, from, from+1)
	if to > from {
		to--
	}
	return slices.Insert(items, to, v)
}

func (a *app) sortList(c *ui.Context, key string, st *sortState, o sortOpts) {
	t := ryugo.From(c)
	cl := t.Colors
	if len(st.rects) != o.n {
		st.rects = make([]ui.Rect, o.n)
	}
	ui.Column(c.Key(key)).FillWidth().Gap(6).Children(func() {
		for i := 0; i < o.n; i++ {
			if st.dragging && st.to == i && st.to != st.from && st.to != st.from+1 {
				ui.Box(c.Key(key + "-drop")).FillWidth().Height(2).Radius(rad(1)).Background(cl.Accent)
			}
			id := o.id(i)
			sel := id == o.selected
			border, fill := cl.Hairline, cl.Card
			if sel {
				border, fill = cl.Accent, ryugo.Mix(cl.Card, cl.Accent, 0.07)
			}
			tile := ui.Column(c.Key(key+":"+id)).FillWidth().Radius(rad(12)).Background(fill).Border(1, border)
			if st.dragging && st.from == i {
				tile.Opacity(0.45)
			}
			tile.Children(func() {
				row := ui.Row(c.Key(key+"-row:"+id)).FillWidth().Height(48).Padding(0, 12, 0, 6).Gap(8).Radius(rad(11)).
					AlignItems(ui.Center).Label(o.label(i)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
				if row.Hovered() && !st.dragging && !sel {
					row.Background(cl.Fill)
				}
				var grip ui.Element
				row.Children(func() {
					grip = ui.Box(c.Key(key+"-grip:"+id)).Size(24, 32).Radius(rad(6)).AlignItems(ui.Center).Justify(ui.Center).
						Cursor(ui.CursorGrab).Label(locales.ViewsSortHandle(o.label(i)))
					if grip.Hovered() {
						grip.Background(cl.Fill)
					}
					grip.Children(func() { gripDots(c, cl.Muted) })
					ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(8).Children(func() { o.content(i) })
					pen := cl.Muted
					if sel {
						pen = cl.Accent
					}
					ui.Icon(c, iconEdit).Width(16).Height(16).TextColor(pen).Shrink(0)
				})
				a.sortDrag(st, i, grip, o)
				if row.Clicked() && !grip.Hovered() && !st.dragging {
					o.toggle(i)
				}
				if sel {
					ui.Box(c.Key(key + "-d:" + id)).FillWidth().Height(1).Background(cl.Hairline)
					o.editor(i)
				}
			})
			st.rects[i] = tile.LayoutBounds()
		}
		if st.dragging && st.to == o.n && st.to != st.from+1 {
			ui.Box(c.Key(key + "-drop")).FillWidth().Height(2).Radius(rad(1)).Background(cl.Accent)
		}
	})
}

// sortDrag follows the grip of row i: pressed and moved it lifts the row,
// the place it would go is the one under the pointer, among the rows as
// they were; let go it moves there.
func (a *app) sortDrag(st *sortState, i int, grip ui.Element, o sortOpts) {
	dx, dy, pressed := grip.Dragged()
	switch {
	case pressed:
		if !st.pressed {
			st.pressed, st.dragging, st.from, st.to, st.moved = true, false, i, i, 0
			st.frozen = slices.Clone(st.rects)
		}
		if st.from != i {
			return
		}
		if st.moved += abs32(dx) + abs32(dy); st.moved > 4 {
			st.dragging = true
		}
		if st.dragging {
			b := grip.LayoutBounds()
			_, py, _ := grip.PointerPosition()
			y := b.Y + py
			st.to = 0
			for k, r := range st.frozen {
				if k < o.n && y > r.Y+r.H/2 {
					st.to = k + 1
				}
			}
		}
	case st.pressed && st.from == i:
		if st.dragging {
			o.move(st.from, st.to)
		}
		st.pressed, st.dragging = false, false
	}
}
