package library

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Properties, as Thymer has them: a collection has none until one is
// added - a choice (Durum: Yapılacak, Sürüyor, Bitti), a text, a number, a
// date, a link. Its pages keep their values in their front matter, under
// the property's id; front matter we don't know is kept as it was. A board
// is a view of a choice property: a column per option.

// The kinds of property.
const (
	PropChoice = "choice"
	PropText   = "text"
	PropNumber = "number"
	PropDate   = "date"
	PropURL    = "url"
)

// PropTypes are the kinds of property, in the order the menu shows them,
// with their names.
var PropTypes = []struct{ Type, Name string }{
	{PropChoice, "Seçim"}, {PropText, "Metin"}, {PropNumber, "Sayı"}, {PropDate, "Tarih"}, {PropURL, "Bağlantı"},
}

// PropTypeName is the name of a kind of property.
func PropTypeName(typ string) string {
	for _, t := range PropTypes {
		if t.Type == typ {
			return t.Name
		}
	}
	return typ
}

// Property is a field of a collection's pages.
type Property struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Options []string `json:"options,omitempty"` // a choice's, in order
}

// reserved are the front matter keys of the page itself.
var reserved = []string{"title", "created", "order", "rank", "cover"}

// Prop is the property id, or nil.
func (c *Collection) Prop(id string) *Property {
	for i := range c.Properties {
		if c.Properties[i].ID == id {
			return &c.Properties[i]
		}
	}
	return nil
}

// PropNamed is the property named name (any case), or with that id, or nil.
func (c *Collection) PropNamed(name string) *Property {
	for i := range c.Properties {
		if strings.EqualFold(c.Properties[i].Name, name) || c.Properties[i].ID == name {
			return &c.Properties[i]
		}
	}
	return nil
}

// AddProperty adds a property to the collection's pages. The first choice
// gives the collection a board of it.
func (c *Collection) AddProperty(name, typ string, options []string) (*Property, error) {
	p, err := c.AddPropertyDraft(name, typ, options)
	if err != nil {
		return nil, err
	}
	return p, c.Save()
}

// AddPropertyDraft adds the property without writing the collection: to a draft.
func (c *Collection) AddPropertyDraft(name, typ string, options []string) (*Property, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("a property needs a name")
	}
	if !slices.ContainsFunc(PropTypes, func(t struct{ Type, Name string }) bool { return t.Type == typ }) {
		return nil, fmt.Errorf("no kind of property %q", typ)
	}
	if c.PropNamed(name) != nil {
		return nil, fmt.Errorf("%s already has %q", c.Name, name)
	}
	id := Slug(name)
	if id == "" {
		id = "ozellik"
	}
	for base, n := id, 2; c.Prop(id) != nil || slices.Contains(reserved, id); n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	c.Properties = append(c.Properties, Property{ID: id, Name: name, Type: typ, Options: CleanOptions(options)})
	c.Normalize()
	return c.Prop(id), nil
}

// EditProperty renames property id and, for a choice, gives it options.
// Values no longer among the options are left on the pages, shown as none.
func (c *Collection) EditProperty(id, name string, options []string) error {
	p := c.Prop(id)
	if p == nil {
		return fmt.Errorf("no property %q", id)
	}
	if name = strings.TrimSpace(name); name != "" {
		if q := c.PropNamed(name); q != nil && q.ID != id {
			return fmt.Errorf("%s already has %q", c.Name, name)
		}
		p.Name = name
	}
	if p.Type == PropChoice && options != nil {
		p.Options = CleanOptions(options)
	}
	return c.Save()
}

// RemoveProperty takes property id away from the collection; its values
// stay in the pages' files, unused.
func (c *Collection) RemoveProperty(id string) error {
	c.Properties = slices.DeleteFunc(c.Properties, func(p Property) bool { return p.ID == id })
	c.Normalize()
	return c.Save()
}

func CleanOptions(options []string) []string {
	var out []string
	for _, o := range options {
		if o = strings.TrimSpace(o); o != "" && !slices.Contains(out, o) {
			out = append(out, o)
		}
	}
	return out
}

// Normalize keeps the views fitting the properties: a board for the first
// choice, none without one.
func (c *Collection) Normalize() {
	if len(c.Views) == 0 {
		c.Views = defaultViews()
	}
	var choice string
	for _, p := range c.Properties {
		if p.Type == PropChoice {
			choice = p.ID
			break
		}
	}
	hasBoard := false
	if choice == "" { // a board without a choice to group by
		c.Views = slices.DeleteFunc(c.Views, func(v View) bool { return v.Type == "board" })
		if c.View().Type == "board" {
			c.ActiveView = ""
		}
	}
	for i := range c.Views {
		if c.Views[i].Type == "board" {
			hasBoard = true
			if p := c.Prop(c.Views[i].GroupBy); p == nil || p.Type != PropChoice {
				c.Views[i].GroupBy = choice
			}
		}
		if f, ok := strings.CutPrefix(c.Views[i].Sort, "prop:"); ok && c.Prop(f) == nil {
			c.Views[i].Sort = "title"
		}
	}
	// every collection has a calendar, by the day its pages were made
	if !slices.ContainsFunc(c.Views, func(v View) bool { return v.Type == "calendar" }) {
		c.Views = append(c.Views, View{ID: "calendar", Label: "Takvim", Type: "calendar", Sort: "created", DateField: "created"})
	}
	if !hasBoard && choice != "" {
		c.Views = append(c.Views, View{ID: "board", Label: "Pano", Type: "board", Sort: "rank", GroupBy: choice})
	}
	for i := range c.Views {
		if f, ok := strings.CutPrefix(c.Views[i].DateField, "prop:"); ok && c.Views[i].Type == "calendar" && (c.Prop(f) == nil || c.Prop(f).Type != PropDate) {
			c.Views[i].DateField = "created"
		}
	}
}

// migrate brings a collection made before properties up to them: its
// statuses, or its pages' "status", become a "Durum" choice. It reports
// whether anything changed.
func (c *Collection) migrate() bool {
	changed := false
	if len(c.Statuses) > 0 && c.Prop("status") == nil {
		c.Properties = append(c.Properties, Property{ID: "status", Name: "Durum", Type: PropChoice, Options: c.Statuses})
		changed = true
	}
	if c.Statuses != nil {
		c.Statuses, changed = nil, true
	}
	if c.Prop("status") == nil && c.PropNamed("Durum") == nil {
		var seen []string
		for _, p := range c.All() {
			if v := p.Props["status"]; v != "" && !slices.Contains(seen, v) {
				seen = append(seen, v)
			}
		}
		if len(seen) > 0 {
			opts := []string{"Yapılacak", "Sürüyor", "Bitti"}
			for _, v := range seen {
				if !slices.Contains(opts, v) {
					opts = append(opts, v)
				}
			}
			c.Properties = append(c.Properties, Property{ID: "status", Name: "Durum", Type: PropChoice, Options: opts})
			changed = true
		}
	}
	before := len(c.Views)
	hadBoard := slices.ContainsFunc(c.Views, func(v View) bool { return v.Type == "board" })
	c.Normalize()
	if len(c.Views) != before || hadBoard != slices.ContainsFunc(c.Views, func(v View) bool { return v.Type == "board" }) {
		changed = true
	}
	return changed
}

// ---- values ----

// ReadMeta takes a page's values from its front matter.
func (p *Page) ReadMeta(meta map[string]string) {
	fmt.Sscan(meta["rank"], &p.Rank)
	p.Cover = meta["cover"]
	p.Props = map[string]string{}
	for k, v := range meta {
		if !slices.Contains(reserved, k) {
			p.Props[k] = v
		}
	}
}

// Value is page p's value of property pr: for a choice, one of its
// options or "".
func (p *Page) Value(pr *Property) string {
	v := p.Props[pr.ID]
	if pr.Type == PropChoice && !slices.Contains(pr.Options, v) {
		return ""
	}
	return v
}

// SetValue gives page p value v of property id, and writes it. A choice
// takes one of its options (any case) or nothing; a number, a number; a
// date, a day as 2006-01-02.
func (p *Page) SetValue(id, v string) error {
	pr := p.Coll.Prop(id)
	if pr == nil {
		return fmt.Errorf("%s has no property %q", p.Coll.Name, id)
	}
	v = strings.TrimSpace(v)
	switch pr.Type {
	case PropChoice:
		if v != "" {
			i := slices.IndexFunc(pr.Options, func(o string) bool { return strings.EqualFold(o, v) })
			if i < 0 {
				return fmt.Errorf("%s is one of: %s", pr.Name, strings.Join(pr.Options, ", "))
			}
			v = pr.Options[i]
		}
		if v == p.Value(pr) {
			return nil
		}
		return p.Coll.MoveOnBoard(p, pr.ID, v, len(p.Coll.Column(pr.ID, v)))
	case PropNumber:
		if _, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); v != "" && err != nil {
			return fmt.Errorf("%s is a number", pr.Name)
		}
	case PropDate:
		if _, err := time.Parse("2006-01-02", v); v != "" && err != nil {
			return fmt.Errorf("%s is a day, as 2026-11-04", pr.Name)
		}
	}
	if p.Props[id] == v {
		return nil
	}
	if p.Props == nil {
		p.Props = map[string]string{}
	}
	p.Props[id] = v
	body, err := p.Body()
	if err != nil {
		return err
	}
	return p.Save(body)
}

// Column is the pages whose choice id is v ("": none of its options), in
// their order on the board.
func (c *Collection) Column(id, v string) []*Page {
	pr := c.Prop(id)
	var out []*Page
	for _, p := range c.All() {
		if pr != nil && p.Value(pr) == v {
			out = append(out, p)
		}
	}
	slices.SortStableFunc(out, func(a, b *Page) int {
		switch {
		case a.Rank == b.Rank:
			return a.Created.Compare(b.Created)
		case a.Rank == 0: // not yet placed: after the placed ones
			return 1
		case b.Rank == 0:
			return -1
		}
		return a.Rank - b.Rank
	})
	return out
}

// MoveOnBoard gives p value v of choice id and puts it at index among that
// column's other pages; the pages whose place changed are written.
func (c *Collection) MoveOnBoard(p *Page, id, v string, index int) error {
	col := slices.DeleteFunc(c.Column(id, v), func(q *Page) bool { return q == p })
	index = max(0, min(index, len(col)))
	col = slices.Insert(col, index, p)
	if p.Props == nil {
		p.Props = map[string]string{}
	}
	p.Props[id] = v
	for i, q := range col {
		if q.Rank == i+1 && q != p {
			continue
		}
		q.Rank = i + 1
		body, err := q.Body()
		if err != nil {
			return err
		}
		if err := q.Save(body); err != nil {
			return err
		}
	}
	return nil
}
