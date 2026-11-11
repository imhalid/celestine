package main

import (
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The app's own fields where Ryugo's hold a text input the app cannot reach (TagInput, NumberInput):
// built of a Ryugo Input, they have the app's menu on a right click, as every text does.

// tagsField is a field of tags: each a chip with its cross (and Sil on a right click), and a field
// that adds the words typed, at ↵ or a comma; ⌫ in it, empty, takes the last away.
func (a *app) tagsField(c *ui.Context, key string, tags *[]string, placeholder, label string) {
	if a.tagDrafts == nil {
		a.tagDrafts = map[string]*string{}
	}
	draft := a.tagDrafts[key]
	if draft == nil {
		draft = new(string)
		a.tagDrafts[key] = draft
	}
	if a.tagPop == nil {
		a.tagPop = map[string]bool{}
	}
	if a.tagPop[key] && len(*tags) > 0 {
		*tags = (*tags)[:len(*tags)-1]
	}
	delete(a.tagPop, key)
	cl := ryugo.From(c).Colors
	add := func(s string) {
		for _, w := range strings.Split(s, ",") {
			if w = strings.TrimSpace(w); w != "" && !slices.Contains(*tags, w) {
				*tags = append(*tags, w)
			}
		}
	}
	ui.Row(c.Key(key)).FillWidth().Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
		for i, tag := range slices.Clone(*tags) {
			i := i
			chip := ui.Row(c.Key("chip:"+tag)).Height(26).Gap(2).Padding(0, 3, 0, 10).Radius(rad(7)).AlignItems(ui.Center).
				Background(cl.Fill).Label(tag)
			remove := func() { *tags = slices.Delete(slices.Clone(*tags), i, i+1) }
			chip.Children(func() {
				ui.Text(c, tag).FontSize(13).TextColor(cl.Text).NoWrap()
				x := ui.Box(c.Key("x")).Size(18, 18).Center().Radius(rad(5)).Role(ui.RoleButton).Label(locales.TagsRemoveLabel(tag)).Cursor(ui.CursorPointer)
				if x.Hovered() {
					x.Background(cl.Hairline)
				}
				x.Children(func() { ui.Icon(c, iconClose).Size(12, 12).TextColor(cl.Muted) })
				if x.Clicked() {
					remove()
				}
			})
			a.contextOn(chip, func() []menuItem {
				return []menuItem{
					{title: "Kopyala", glyph: iconCopy, run: func() { a.copyText(tag) }},
					{title: "Sil", group: "~", glyph: iconTrash, danger: true, run: remove},
				}
			})
		}
		in := a.fieldMenu(ryugo.Input(c.Key(key+":in"), draft, ryugo.InputOptions{Label: label, Placeholder: placeholder, Size: ryugo.Sm}),
			draft, fieldOpts{oneLine: true}).MinWidth(140)
		in.OnTextKey(func(k ui.TextKey) bool { // ⌫ in the empty field: the last tag
			if k.Key != ui.KeyBackspace || k.Mods != 0 || k.Text() != "" || len(*tags) == 0 {
				return false
			}
			a.tagPop[key] = true // taken as the field is built next: the tags may be made anew each frame
			return true
		})
		switch {
		case in.Submitted():
			add(*draft)
			*draft = ""
		case strings.Contains(*draft, ","):
			add(*draft)
			*draft = ""
		}
	})
}

// countField is a field of a whole number, at least 0: what is typed is taken at ↵ or when it is left;
// what is not a number goes back to the number.
func (a *app) countField(c *ui.Context, key string, n *int, label string) ui.Element {
	if a.tagDrafts == nil {
		a.tagDrafts = map[string]*string{}
	}
	text := a.tagDrafts[key]
	if text == nil {
		text = new(string)
		a.tagDrafts[key] = text
	}
	take := func() {
		if v, err := strconv.Atoi(strings.TrimSpace(*text)); err == nil {
			*n = max(v, 0)
		}
		*text = strconv.Itoa(*n)
	}
	in := a.fieldMenu(ryugo.Input(c.Key(key), text, ryugo.InputOptions{Label: label, Size: ryugo.Sm}), text, fieldOpts{oneLine: true, changed: take})
	switch {
	case in.Submitted():
		take()
	case in.Focused():
		a.countEditing = key
	case a.countEditing == key: // left
		a.countEditing = ""
		take()
	default:
		*text = strconv.Itoa(*n)
	}
	return in
}
