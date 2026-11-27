package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
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
	PropPage   = "page" // another page: its folders from the library's root (Library.PageAt)
	PropTags   = "tags" // words, ", " between them
	// PropAliases are other names of the page, ", " between them, which a [[link]] resolves to as it does to the
	// title (#547).
	PropAliases = "aliases"
	PropFile    = "file" // a file in the page's folder: its name
)

// PropTypes are the kinds of property, in the order the menu shows them,
// with their names.
var PropTypes = []struct{ Type, Name string }{
	{PropChoice, "Seçim"}, {PropText, "Metin"}, {PropNumber, "Sayı"}, {PropDate, "Tarih"}, {PropURL, "Bağlantı"},
	{PropPage, "Sayfa"}, {PropTags, "Etiketler"}, {PropFile, "Dosya"}, {PropAliases, "Takma adlar"},
}

// IsList reports whether a kind of property holds a list of words, ", " between them: tags and aliases.
func IsList(typ string) bool { return typ == PropTags || typ == PropAliases }

// Aliases are page p's other names, which a [[link]] resolves to as it does to its title: the values of its
// properties of the aliases kind, its collection's and its own, each once.
func (p *Page) Aliases() []string {
	var out []string
	add := func(prs []Property) {
		for _, pr := range prs {
			if pr.Type != PropAliases {
				continue
			}
			for _, w := range SplitTags(p.Props[pr.ID]) {
				if !slices.ContainsFunc(out, func(o string) bool { return strings.EqualFold(o, w) }) {
					out = append(out, w)
				}
			}
		}
	}
	if p.Coll != nil {
		add(p.Coll.Properties)
	}
	add(p.Own)
	return out
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
var reserved = []string{"id", "title", "created", "order", "rank", "manualrank", "cover", "covershift", "properties"}

// renamedKeys are the reserved keys that were reserved after properties came: before, a property could have one
// for its id (one named "Properties" had "properties"), and its values are under that key in the pages' files. Such
// a property is given another id when it is read, with its values (#531).
var renamedKeys = []string{"manualrank", "cover", "covershift", "properties"}

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
	pages := c.All()
	id := freeID(name, func(id string) bool {
		return c.Prop(id) != nil || slices.ContainsFunc(pages, func(p *Page) bool { return ownProp(p.Own, id) != nil })
	})
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

// freeID is the id of a new property named name: its slug, numbered past those taken and the page's own keys.
func freeID(name string, taken func(string) bool) string {
	id := Slug(name)
	if id == "" {
		id = "ozellik"
	}
	for base, n := id, 2; taken(id) || slices.Contains(reserved, id); n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}

// ownProp is the property id among a page's own, or nil.
func ownProp(own []Property, id string) *Property {
	for i := range own {
		if own[i].ID == id {
			return &own[i]
		}
	}
	return nil
}

// ---- a page's own properties (#487) ----

// Properties are the properties page p has: its collection's, then its own.
func (p *Page) Properties() []*Property {
	var out []*Property
	if p.Coll != nil {
		for i := range p.Coll.Properties {
			out = append(out, &p.Coll.Properties[i])
		}
	}
	for i := range p.Own {
		out = append(out, &p.Own[i])
	}
	return out
}

// Prop is property id of page p, its collection's or its own, or nil.
func (p *Page) Prop(id string) *Property {
	if p.Coll != nil {
		if pr := p.Coll.Prop(id); pr != nil {
			return pr
		}
	}
	return ownProp(p.Own, id)
}

// PropNamed is the property of page p named name (any case), or with that id, or nil.
func (p *Page) PropNamed(name string) *Property {
	if p.Coll != nil {
		if pr := p.Coll.PropNamed(name); pr != nil {
			return pr
		}
	}
	for i := range p.Own {
		if strings.EqualFold(p.Own[i].Name, name) || p.Own[i].ID == name {
			return &p.Own[i]
		}
	}
	return nil
}

// IsOwn reports whether property id is page p's own, not its collection's.
func (p *Page) IsOwn(id string) bool {
	return (p.Coll == nil || p.Coll.Prop(id) == nil) && ownProp(p.Own, id) != nil
}

// AddProperty adds a property to page p alone, and writes it. Its id is one no property of the collection has,
// nor a value the page keeps, so that it starts empty.
func (p *Page) AddProperty(name, typ string, options []string) (*Property, error) {
	if p.Locked != "" {
		return nil, fmt.Errorf("%w: %s", ErrLocked, p.Locked)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("a property needs a name")
	}
	if !slices.ContainsFunc(PropTypes, func(t struct{ Type, Name string }) bool { return t.Type == typ }) {
		return nil, fmt.Errorf("no kind of property %q", typ)
	}
	if p.PropNamed(name) != nil {
		return nil, fmt.Errorf("%s already has %q", p.Title, name)
	}
	if p.ownText != "" { // writing the new one would write over what could not be read (#531)
		return nil, fmt.Errorf("the properties of %s could not be read", p.Title)
	}
	id := freeID(name, func(id string) bool { _, kept := p.Props[id]; return p.Prop(id) != nil || kept })
	p.Own = append(p.Own, Property{ID: id, Name: name, Type: typ, Options: CleanOptions(options)})
	if err := p.saveMeta(); err != nil {
		p.Own = p.Own[:len(p.Own)-1]
		return nil, err
	}
	return ownProp(p.Own, id), nil
}

// EditProperty renames page p's own property id and, for a choice, gives it options.
func (p *Page) EditProperty(id, name string, options []string) error {
	if p.Locked != "" {
		return fmt.Errorf("%w: %s", ErrLocked, p.Locked)
	}
	pr := ownProp(p.Own, id)
	if pr == nil {
		return fmt.Errorf("%s has no property %q of its own", p.Title, id)
	}
	oldName, oldOpts := pr.Name, pr.Options
	if name = strings.TrimSpace(name); name != "" {
		if q := p.PropNamed(name); q != nil && q.ID != id {
			return fmt.Errorf("%s already has %q", p.Title, name)
		}
		pr.Name = name
	}
	if pr.Type == PropChoice && options != nil {
		pr.Options = CleanOptions(options)
	}
	if err := p.saveMeta(); err != nil {
		pr.Name, pr.Options = oldName, oldOpts
		return err
	}
	return nil
}

// RemoveProperty takes page p's own property id away, and its value with it.
func (p *Page) RemoveProperty(id string) error {
	if p.Locked != "" {
		return fmt.Errorf("%w: %s", ErrLocked, p.Locked)
	}
	if ownProp(p.Own, id) == nil {
		return nil
	}
	oldOwn := slices.Clone(p.Own)
	oldVal, hadVal := p.Props[id]
	p.Own = slices.DeleteFunc(p.Own, func(pr Property) bool { return pr.ID == id })
	delete(p.Props, id)
	if err := p.saveMeta(); err != nil {
		p.Own = oldOwn
		if hadVal {
			p.Props[id] = oldVal
		}
		return err
	}
	return nil
}

// saveMeta writes page p with the body it has.
func (p *Page) saveMeta() error { return p.saveMetaTo(p.FS()) }

func (p *Page) saveMetaTo(fsys FS) error {
	body, err := p.bodyFrom(fsys)
	if err != nil {
		return err
	}
	return p.saveTo(fsys, body)
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
		c.Views[i].Filters = slices.DeleteFunc(c.Views[i].Filters, func(f Filter) bool { return f.Prop != "title" && c.Prop(f.Prop) == nil })
		if (c.Views[i].Type == "list" || c.Views[i].Type == "table") && c.Views[i].GroupBy != "" { // a list grouped by a choice that is gone is by date again
			if p := c.Prop(c.Views[i].GroupBy); p == nil || p.Type != PropChoice {
				c.Views[i].GroupBy = ""
			}
		}
		if f, ok := strings.CutPrefix(c.Views[i].Sort, "prop:"); ok && c.Prop(f) == nil {
			c.Views[i].Sort = "title"
		}
	}
	// every collection has a calendar, by the day its pages were made
	if !slices.ContainsFunc(c.Views, func(v View) bool { return v.Type == "calendar" }) {
		c.Views = append(c.Views, View{ID: "calendar", Label: "Calendar", Type: "calendar", Sort: "created", DateField: "created"})
	}
	if !hasBoard && choice != "" {
		c.Views = append(c.Views, View{ID: "board", Label: "Board", Type: "board", Sort: "rank", GroupBy: choice})
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
	changed := c.renameReserved()
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
	fmt.Sscan(meta["manualrank"], &p.Manual)
	p.Cover = meta["cover"]
	p.CoverShift = 0
	fmt.Sscan(meta["covershift"], &p.CoverShift)
	p.CoverShift = max(-100, min(p.CoverShift, 100))
	p.Own, p.ownText = nil, ""
	if own := meta["properties"]; own != "" && json.Unmarshal([]byte(own), &p.Own) != nil {
		p.Own, p.ownText = nil, own // the page is still read, and this is kept as it was (#531)
	}
	p.reservedMeta = nil
	for _, k := range renamedKeys {
		v := meta[k]
		if k == "properties" {
			v = p.ownText // own properties that were read are not a value
		}
		if v != "" {
			if p.reservedMeta == nil {
				p.reservedMeta = map[string]string{}
			}
			p.reservedMeta[k] = v
		}
	}
	p.Props = map[string]string{}
	for k, v := range meta {
		if !slices.Contains(reserved, k) {
			p.Props[k] = v
		}
	}
	for i := range p.Own { // an own property given a key of the page before it was reserved (covershift)
		if from := p.Own[i].ID; slices.Contains(reserved, from) {
			pages := []*Page{p}
			to, first := renamedID(from, p.Own[i].Name, pages, func(id string) bool { return p.Prop(id) != nil })
			p.Own[i].ID = to
			p.takeReserved(from, to, first)
		}
	}
}

// renamedID is the id a property with reserved id from, named name, is given, and whether this is its first
// rename. A rename that was made before (and then undone by an older client writing its collection.json back, or
// cut short) left the property's values in pages under from-N, an id no property has: that id is taken again, so
// that the values stay the property's and a second rename does not make a third id (#531). Otherwise it is a free
// id, as a new property's. defined reports an id a property has.
func renamedID(from, name string, pages []*Page, defined func(string) bool) (string, bool) {
	best := 0
	for _, p := range pages {
		for k, v := range p.Props {
			suffix, ok := strings.CutPrefix(k, from+"-")
			n, err := strconv.Atoi(suffix)
			if !ok || err != nil || n < 2 || strconv.Itoa(n) != suffix || v == "" || defined(k) {
				continue
			}
			if best == 0 || n < best {
				best = n
			}
		}
	}
	if best > 0 {
		return from + "-" + strconv.Itoa(best), false
	}
	return freeID(name, func(id string) bool {
		return defined(id) || slices.ContainsFunc(pages, func(p *Page) bool {
			_, kept := p.Props[id]
			return kept || ownProp(p.Own, id) != nil
		})
	}), true
}

// takeReserved gives page p's property to the value its file holds under reserved key from: the value of a
// property that had that id before the key was reserved (#531). A page that already has a value of the property
// (renamed before) is left as it is. Otherwise "properties", which could not be read as the page's own
// properties, is the property's. A "covershift" that is not a cover's position is the property's too, and taken
// from the cover; one that could be its position is copied to the property on the first rename only, and stays
// the cover's: a position set since cannot be told from the property's old value, and the cover's is kept. So are
// "cover" and "manualrank", reserved long before. It reports whether the page changed.
func (p *Page) takeReserved(from, to string, first bool) bool {
	v := p.reservedMeta[from]
	delete(p.reservedMeta, from)
	if v == "" || p.Props[to] != "" {
		return false
	}
	switch from {
	case "properties":
		p.ownText = ""
	case "covershift":
		if n, err := strconv.Atoi(v); err != nil || n < -100 || n > 100 {
			p.CoverShift = 0
		} else if !first {
			return false
		}
	default:
		if !first {
			return false
		}
	}
	if p.Props == nil {
		p.Props = map[string]string{}
	}
	p.Props[to] = v
	return true
}

// renameReserved gives each of the collection's properties whose id is a key of the page itself another id, with
// its values in the pages and its places in the views, and writes the pages that changed and the collection, as
// one change where the store can make one. Such an id was handed out before the key was reserved, and its values
// were dropped on save (#531). It reports whether anything changed.
func (c *Collection) renameReserved() bool {
	var touched []*Page
	changed := false
	for i := range c.Properties {
		from := c.Properties[i].ID
		if !slices.Contains(reserved, from) {
			continue
		}
		pages := c.All()
		to, first := renamedID(from, c.Properties[i].Name, pages, func(id string) bool { return c.Prop(id) != nil })
		c.Properties[i].ID = to
		for v := range c.Views {
			c.Views[v].renameProp(from, to)
		}
		for _, p := range pages {
			if p.takeReserved(from, to, first) && !slices.Contains(touched, p) {
				touched = append(touched, p)
			}
		}
		changed = true
	}
	if changed {
		InBatch(c.FS, func(tx FS) error { // cut short where it is not one change, the next rename takes the same ids again
			for _, p := range touched {
				p.saveMetaTo(tx) // one that cannot be written keeps the value in its file under the old key, as the page's own field
			}
			return c.saveTo(tx)
		})
	}
	return changed
}

// renameProp makes the view's mentions of property from mentions of property to.
func (v *View) renameProp(from, to string) {
	re := func(s string) string {
		switch s {
		case "prop:" + from:
			return "prop:" + to
		case from:
			return to
		}
		return s
	}
	v.Sort, v.GroupBy, v.DateField = re(v.Sort), re(v.GroupBy), re(v.DateField)
	for _, list := range [][]string{v.Columns, v.Fields, v.Order, v.Hidden} {
		for i := range list {
			list[i] = re(list[i])
		}
	}
	for i := range v.Filters {
		v.Filters[i].Prop = re(v.Filters[i].Prop)
	}
	if w, ok := v.Widths["prop:"+from]; ok {
		delete(v.Widths, "prop:"+from)
		v.Widths["prop:"+to] = w
	}
	if c, ok := v.Calcs["prop:"+from]; ok {
		delete(v.Calcs, "prop:"+from)
		v.Calcs["prop:"+to] = c
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
	if p.Locked != "" { // before its value is changed in memory
		return fmt.Errorf("%w: %s", ErrLocked, p.Locked)
	}
	pr := p.Prop(id)
	if pr == nil {
		return fmt.Errorf("%s has no property %q", p.Title, id)
	}
	if slices.Contains(reserved, id) { // Save would not write it (#531)
		return fmt.Errorf("%q is a key of the page itself, not a property's", id)
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
		if p.Coll.Prop(id) != nil { // the collection's choice has a board, with the page's place in it
			return p.Coll.MoveOnBoard(p, pr.ID, v, len(p.Coll.Column(pr.ID, v)))
		}
	case PropNumber:
		if _, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); v != "" && err != nil {
			return fmt.Errorf("%s is a number", pr.Name)
		}
	case PropDate:
		if _, err := time.Parse("2006-01-02", v); v != "" && err != nil {
			return fmt.Errorf("%s is a day, as 2026-11-04", pr.Name)
		}
	case PropTags, PropAliases:
		v = strings.Join(SplitTags(v), ", ")
	case PropPage:
		v = strings.Trim(filepath.ToSlash(v), "/")
	case PropFile:
		if v != "" && (strings.ContainsAny(v, `/\`) || v == "." || v == "..") {
			return fmt.Errorf("%s is the name of a file in the page's folder", pr.Name)
		}
	}
	if p.Props[id] == v {
		return nil
	}
	if p.Props == nil {
		p.Props = map[string]string{}
	}
	p.Props[id] = v
	if pr.Type == PropAliases { // what a [[link]] resolves to changed, as with a rename
		defer PagesChanged()
	}
	return p.saveMeta()
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
	slices.SortStableFunc(out, byRank)
	return out
}

// Columns are all the columns of choice property id at once, by value ("" for the pages with none), in one pass
// over the pages: a board asked for each with Column, a pass each (#194).
func (c *Collection) Columns(id string) map[string][]*Page {
	out := map[string][]*Page{}
	pr := c.Prop(id)
	if pr == nil {
		return out
	}
	for _, p := range c.All() {
		v := p.Value(pr)
		out[v] = append(out[v], p)
	}
	for _, col := range out {
		slices.SortStableFunc(col, byRank)
	}
	return out
}

// byRank orders a column's pages: by their place on the board, those not yet placed after the others, in the order
// they were made.
func byRank(a, b *Page) int {
	switch {
	case a.Rank == b.Rank:
		return a.Created.Compare(b.Created)
	case a.Rank == 0: // not yet placed: after the placed ones
		return 1
	case b.Rank == 0:
		return -1
	}
	return a.Rank - b.Rank
}

// MoveOnBoard gives p value v of choice id and puts it at index among that
// column's other pages; the pages whose place changed are written.
func (c *Collection) MoveOnBoard(p *Page, id, v string, index int) error {
	if p.Locked != "" {
		return fmt.Errorf("%w: %s", ErrLocked, p.Locked)
	}
	col := slices.DeleteFunc(c.Column(id, v), func(q *Page) bool { return q == p })
	index = max(0, min(index, len(col)))
	col = slices.Insert(col, index, p)
	if p.Props == nil {
		p.Props = map[string]string{}
	}
	oldVal, hadVal := p.Props[id]
	wasRank := make([]int, len(col))
	for i, q := range col {
		wasRank[i] = q.Rank
	}
	p.Props[id] = v
	// one change of the store: all of them or none (#138); locked pages in the column are left untouched
	err := InBatch(c.FS, func(tx FS) error {
		for i, q := range col {
			if q != p && (q.Locked != "" || q.Rank == i+1) {
				continue
			}
			q.Rank = i + 1
			body, err := q.bodyFrom(tx)
			if err != nil {
				return err
			}
			if err := q.saveTo(tx, body); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if hadVal {
			p.Props[id] = oldVal
		} else {
			delete(p.Props, id)
		}
		for i, q := range col {
			q.Rank = wasRank[i]
		}
	}
	return err
}

// CompareManual orders two pages by their place in the collection's own order: placed ones first, by place, the
// rest after them by when they were made.
func CompareManual(a, b *Page) int {
	switch {
	case a.Manual == b.Manual:
		return a.Created.Compare(b.Created)
	case a.Manual == 0:
		return 1
	case b.Manual == 0:
		return -1
	}
	return a.Manual - b.Manual
}

// SetManual puts pages in the order given, in the collection's own order: the places they had among its pages
// (all of them, in that order) are given to them in the new order, so that a list showing some of the pages (a
// search, a filter) moves only those. Every page is then numbered from 1, and only those whose place changed are
// written.
func (c *Collection) SetManual(pages []*Page) error {
	all := c.All()
	slices.SortStableFunc(all, CompareManual)
	moving := map[*Page]bool{}
	for _, p := range pages {
		moving[p] = true
	}
	next := 0
	for i, p := range all {
		if moving[p] && next < len(pages) {
			all[i] = pages[next]
			next++
		}
	}
	// one change of the store: all of them or none (#138); locked pages stay untouched
	wasManual := make([]int, len(all))
	for i, p := range all {
		wasManual[i] = p.Manual
	}
	err := InBatch(c.FS, func(tx FS) error {
		for i, p := range all {
			if p.Locked != "" || p.Manual == i+1 {
				continue
			}
			p.Manual = i + 1
			body, err := p.bodyFrom(tx)
			if err != nil {
				return err
			}
			if err := p.saveTo(tx, body); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		for i, p := range all {
			p.Manual = wasManual[i]
		}
	}
	return err
}

// SplitTags are the tags of a tags value: the words between its commas, trimmed, each once (its case as first
// written).
func SplitTags(v string) []string {
	var out []string
	for _, w := range strings.Split(v, ",") {
		if w = strings.TrimSpace(w); w != "" && !slices.ContainsFunc(out, func(o string) bool { return strings.EqualFold(o, w) }) {
			out = append(out, w)
		}
	}
	return out
}
