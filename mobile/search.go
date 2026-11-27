package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/internal/textnorm"
	"github.com/imhalid/celestine/mobile/locales"
)

// The Search screen finds pages by their titles and their words as they are typed: a title that matches before one
// whose words do, Turkish letters and their plain ones alike (textnorm).

// wordsMatch is textnorm.Match's least score when every word of the query is in the text: below it, only its letters
// are, in order, which a page's many words nearly always have.
const wordsMatch = 200

// searchMax is how many pages a search shows at most.
const searchMax = 60

// found is a page a search found, and how well.
type found struct {
	p     *library.Page
	score int
}

// search is the pages that match q, the best first.
func (a *app) search(q string) []found {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	var out []found
	for _, p := range a.allPages() {
		s := 0
		if m := textnorm.Match(q, titleOf(p)); m > 0 { // a title matches by its letters in order, too
			s = m * 4
		} else if m := textnorm.Match(q, plainOf(p)); m >= wordsMatch { // the words, only by every word of the query
			s = m
		}
		if s > 0 {
			out = append(out, found{p, s})
		}
	}
	slices.SortStableFunc(out, func(x, y found) int {
		if x.score != y.score {
			return y.score - x.score
		}
		return y.p.Modified.Compare(x.p.Modified)
	})
	return out[:min(len(out), searchMax)]
}

func (a *app) searchScreen(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		a.topBar(c, false, nil)
		ui.Column(c).Padding(6, side, 14, side).Gap(14).Shrink(0).Children(func() {
			ui.Text(c, locales.TabSearch()).FontSize(38).FontWeight(800).Role(ui.RoleHeading).Level(1)
			field := ui.Row(c.Key("field")).FillWidth().Height(58).Padding(0, 18).Gap(12).AlignItems(ui.Center).
				Radius(29).Background(pal.block)
			shadow(field)
			field.Children(func() {
				ui.Icon(c, iconSearch).Size(22, 22).TextColor(pal.accent)
				ui.TextInput(c.Key("q"), &a.query).Placeholder(locales.SearchPlaceholder()).FontSize(17).Grow(1).
					Background(pal.block).Border(0, pal.block).Label(locales.SearchPlaceholder()).ReturnKey(ui.ReturnSearch)
			})
		})
		a.screenScroll(c).Children(func() {
			ui.Column(c).FillWidth().Padding(0, side, 140, side).Gap(gap).Children(func() {
				if strings.TrimSpace(a.query) == "" {
					ui.Text(c, locales.SearchHint()).FontSize(16).TextColor(pal.muted).Wrap().Padding(4, 4)
					return
				}
				res := a.search(a.query)
				if len(res) == 0 {
					ui.Text(c, locales.SearchNone()).FontSize(16).TextColor(pal.muted).Padding(4, 4)
				}
				for _, f := range res {
					a.pageBlock(c, f.p, true)
				}
			})
		})
	})
}
