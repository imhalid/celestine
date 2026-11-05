package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui/richtext"
)

// The library is a folder of plain files, laid out as Locus lays notes out,
// so any tool - an editor, a script, an agent - reads and writes the same
// thing the app shows:
//
//	library/
//	  notlar/                 a collection: a folder…
//	    collection.json       …with its name and settings
//	    hos-geldin/           a page: a folder with its index.md
//	      index.md
//	      alt-sayfa/          a sub-page: a page's folder inside another's
//	        index.md
//
// A page's title and creation time are the front matter of its index.md;
// its body is Markdown. Folder names are made from titles when a page is
// made and stay as they are when it is renamed, so links to the files hold.
// This file knows nothing of the UI: the app and, later, MCP use it alike.

type Library struct {
	Root        string
	Collections []*Collection
}

type Collection struct {
	Name     string `json:"name"`
	Icon     string `json:"icon,omitempty"`
	SubPages bool   `json:"subPages"`
	// Views are the ways the collection's page shows its pages, the first of
	// them by default; ActiveView is the one shown last.
	Views []View `json:"views,omitempty"`
	// Properties are the fields its pages have, as Thymer's collections:
	// none until one is added (properties.go).
	Properties []Property `json:"properties,omitempty"`
	// Statuses is what came before properties: read, made a "Durum" choice.
	Statuses   []string  `json:"statuses,omitempty"`
	ActiveView string    `json:"activeView,omitempty"`
	Dir        string    `json:"-"`
	Pages      []*Page   `json:"-"` // top-level pages, oldest first
	Open       bool      `json:"-"` // shown expanded in the sidebar
	seen       time.Time // when its collection.json was last read or written here (watch.go)
}

// View is a way of showing a collection's pages: a list grouped by date or a
// table, sorted by a field.
type View struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"` // "list" | "table" | "board"
	Sort  string `json:"sort"` // "title" | "created" | "modified" | "prop:<id>"
	Desc  bool   `json:"desc,omitempty"`
	// GroupBy is the choice property a board's columns are the options of.
	GroupBy string `json:"groupBy,omitempty"`
}

// defaultViews are the views a collection starts with.
func defaultViews() []View {
	return []View{
		{ID: "list", Label: "Liste", Type: "list", Sort: "modified", Desc: true},
		{ID: "table", Label: "Tablo", Type: "table", Sort: "title"},
	}
}

// View returns the view shown: the active one, else the first.
func (c *Collection) View() *View {
	if len(c.Views) == 0 {
		c.Views = defaultViews()
	}
	for i := range c.Views {
		if c.Views[i].ID == c.ActiveView {
			return &c.Views[i]
		}
	}
	return &c.Views[0]
}

// SetView shows view id and keeps the choice.
func (c *Collection) SetView(id string) error {
	c.ActiveView = id
	return c.save()
}

// SetSort sorts view v by field, the other way round when it already is.
func (c *Collection) SetSort(v *View, field string) error {
	if v.Sort == field {
		v.Desc = !v.Desc
	} else {
		v.Sort, v.Desc = field, field != "title"
	}
	return c.save()
}

// Rename names the collection; its folder keeps its name.
func (c *Collection) Rename(name string) error {
	c.Name = name
	return c.save()
}

// All is every page of the collection, sub-pages after their parent.
func (c *Collection) All() []*Page {
	var out []*Page
	var walk func([]*Page)
	walk = func(ps []*Page) {
		for _, p := range ps {
			out = append(out, p)
			walk(p.Children)
		}
	}
	walk(c.Pages)
	return out
}

type Page struct {
	Title    string
	Created  time.Time
	Modified time.Time         // when its file was last written
	Order    int               // its place among its siblings, from 1; 0: not yet placed (by creation)
	Props    map[string]string // its properties' values, by id, and other front matter kept as it was
	Rank     int               // its place in its column on the board, from 1; 0: not yet placed
	Dir      string
	Coll     *Collection
	Parent   *Page
	Children []*Page
	Open     bool // shown expanded in the sidebar

	excerpt   string // the start of its text, for a list
	excerptOf time.Time
}

const collectionFile = "collection.json"

// OpenLibrary reads the library at root, making the folder when there is none.
func OpenLibrary(root string) (*Library, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	l := &Library{Root: root}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, collectionFile))
		if err != nil {
			continue // a folder that is not a collection
		}
		c := &Collection{SubPages: true, Open: true}
		if err := json.Unmarshal(data, c); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(dir, collectionFile), err)
		}
		if c.Name == "" {
			c.Name = e.Name()
		}
		c.Dir, c.seen = dir, modTime(filepath.Join(dir, collectionFile))
		c.Pages = readPages(dir, c, nil)
		if c.migrate() {
			if err := c.save(); err != nil {
				return nil, err
			}
		}
		l.Collections = append(l.Collections, c)
	}
	sort.SliceStable(l.Collections, func(i, j int) bool { return l.Collections[i].Name < l.Collections[j].Name })
	return l, nil
}

// readPages reads the pages in dir: each subfolder holding an index.md.
func readPages(dir string, c *Collection, parent *Page) []*Page {
	entries, _ := os.ReadDir(dir)
	var out []*Page
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		pdir := filepath.Join(dir, e.Name())
		src, err := os.ReadFile(filepath.Join(pdir, "index.md"))
		if err != nil {
			continue
		}
		meta, _ := splitFrontMatter(string(src))
		p := &Page{Title: meta["title"], Dir: pdir, Coll: c, Parent: parent}
		if p.Title == "" {
			p.Title = e.Name()
		}
		fmt.Sscan(meta["order"], &p.Order)
		p.readMeta(meta)
		if t, err := time.Parse(time.RFC3339, meta["created"]); err == nil {
			p.Created = t
		} else if info, err := e.Info(); err == nil {
			p.Created = info.ModTime()
		}
		if info, err := os.Stat(filepath.Join(pdir, "index.md")); err == nil {
			p.Modified = info.ModTime()
		}
		p.Children = readPages(pdir, c, p)
		out = append(out, p)
	}
	sortSiblings(out)
	return out
}

// NewCollection makes a collection named name.
func (l *Library) NewCollection(name string) (*Collection, error) {
	dir, err := uniqueDir(l.Root, slug(name))
	if err != nil {
		return nil, err
	}
	c := &Collection{Name: name, SubPages: true, Dir: dir, Open: true}
	if err := c.save(); err != nil {
		return nil, err
	}
	l.Collections = append(l.Collections, c)
	return c, nil
}

func (c *Collection) save() error {
	data, _ := json.MarshalIndent(c, "", "  ")
	file := filepath.Join(c.Dir, collectionFile)
	if err := writeAtomic(file, append(data, '\n')); err != nil {
		return err
	}
	c.seen = modTime(file)
	return nil
}

// modTime is when file was last written; now if it can't be read.
func modTime(file string) time.Time {
	if info, err := os.Stat(file); err == nil {
		return info.ModTime()
	}
	return time.Now()
}

// NewPage makes a page in c, under parent when it is not nil (a sub-page),
// with an empty body.
func (c *Collection) NewPage(parent *Page, title string) (*Page, error) {
	if parent != nil && !c.SubPages {
		return nil, errors.New("this collection has no sub-pages")
	}
	in := c.Dir
	if parent != nil {
		in = parent.Dir
	}
	dir, err := uniqueDir(in, slug(title))
	if err != nil {
		return nil, err
	}
	siblings := c.Pages
	if parent != nil {
		siblings = parent.Children
	}
	p := &Page{Title: title, Created: time.Now().Truncate(time.Second), Dir: dir, Coll: c, Parent: parent, Order: lastOrder(siblings) + 1}
	if err := p.Save(""); err != nil {
		return nil, err
	}
	if parent != nil {
		parent.Children = append(parent.Children, p)
		parent.Open = true
	} else {
		c.Pages = append(c.Pages, p)
	}
	c.Open = true
	return p, nil
}

func (p *Page) file() string { return filepath.Join(p.Dir, "index.md") }

// Body reads the page's Markdown, without its front matter.
func (p *Page) Body() (string, error) {
	src, err := os.ReadFile(p.file())
	if err != nil {
		return "", err
	}
	_, body := splitFrontMatter(string(src))
	return body, nil
}

// Save writes the page: its body, and its front matter as it is on disk
// with our keys changed (frontmatter.go).
func (p *Page) Save(body string) error {
	f := &front{}
	if src, err := os.ReadFile(p.file()); err == nil {
		f, _ = readFront(string(src))
	}
	f.set("title", p.Title)
	if !p.Created.IsZero() {
		f.set("created", p.Created.Format(time.RFC3339))
	}
	f.setInt("order", p.Order)
	if p.Coll != nil {
		for _, pr := range p.Coll.Properties {
			if v := p.Props[pr.ID]; v != f.get(pr.ID) {
				f.set(pr.ID, v)
			}
		}
	}
	f.setInt("rank", p.Rank)
	var sb strings.Builder
	if fm := f.text(); fm != "" {
		sb.WriteString("---\n" + fm)
		if !strings.HasSuffix(fm, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("---\n\n")
	}
	sb.WriteString(strings.TrimLeft(body, "\n"))
	if !strings.HasSuffix(body, "\n") {
		sb.WriteString("\n")
	}
	if err := writeAtomic(p.file(), []byte(sb.String())); err != nil {
		return err
	}
	p.Modified = modTime(p.file())
	if f.bad {
		return errBadFront
	}
	return nil
}

// Excerpt is the start of the page's text, plain, on one line: what a list
// shows under its title. It is read again only when the page changed.
func (p *Page) Excerpt() string {
	if p.excerptOf.Equal(p.Modified) && !p.Modified.IsZero() {
		return p.excerpt
	}
	body, err := p.Body()
	if err != nil {
		return ""
	}
	text := strings.Join(strings.Fields(richtext.ParseMarkdown(body).PlainText()), " ")
	if r := []rune(text); len(r) > 180 {
		text = strings.TrimSpace(string(r[:180])) + "…"
	}
	p.excerpt, p.excerptOf = text, p.Modified
	return text
}

// SetTitle) renames the page; its folder keeps its name.
func (p *Page) SetTitle(title string) error {
	body, err := p.Body()
	if err != nil {
		return err
	}
	p.Title = title
	return p.Save(body)
}

// Depth is how deep the page is: 0 at the top of its collection.
func (p *Page) Depth() int {
	d := 0
	for q := p.Parent; q != nil; q = q.Parent {
		d++
	}
	return d
}

var turkishFold = strings.NewReplacer("ç", "c", "ğ", "g", "ı", "i", "ö", "o", "ş", "s", "ü", "u", "Ç", "c", "Ğ", "g", "İ", "i", "Ö", "o", "Ş", "s", "Ü", "u")

// slug makes a folder name from a title: lower case, ASCII where the letter
// has an obvious one, words joined by hyphens.
func slug(title string) string {
	s := turkishFold.Replace(strings.ToLower(title))
	var sb strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			sb.WriteRune(r)
			dash = false
		case !dash && sb.Len() > 0:
			sb.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimSuffix(sb.String(), "-")
	if out == "" {
		out = "sayfa"
	}
	if len(out) > 60 {
		out = strings.TrimRight(out[:60], "-")
	}
	return out
}

// uniqueDir makes a new folder in parent named name, or name-2, name-3…
func uniqueDir(parent, name string) (string, error) {
	for i := 1; ; i++ {
		n := name
		if i > 1 {
			n = fmt.Sprintf("%s-%d", name, i)
		}
		dir := filepath.Join(parent, n)
		err := os.Mkdir(dir, 0o755)
		if err == nil {
			return dir, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
}

// writeAtomic writes a file through a temporary one, so a crash never leaves half of it.
func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// trashDir is where pages and collections go when they are thrown away: a
// folder of the library, so nothing is lost and they can be put back by hand.
const trashDir = ".cop"

// trashNote is the file a thrown-away folder keeps in it: what it was and
// where it was, to put it back there.
const trashNote = ".trashed.json"

// TrashItem is a page or a collection in the trash.
type TrashItem struct {
	Kind    string    `json:"kind"` // "page" | "collection"
	Title   string    `json:"title"`
	From    string    `json:"from"`            // its folder, from the library's root
	Where   string    `json:"where,omitempty"` // where it was, to show: its collection and the pages above it
	Trashed time.Time `json:"trashed"`
	Dir     string    `json:"-"` // its folder in the trash
}

// trash moves dir into the library's trash, under a name that keeps when it
// went and what it was, with a note of where it was.
func (l *Library) trash(dir string, item TrashItem) (*TrashItem, error) {
	bin := filepath.Join(l.Root, trashDir)
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return nil, err
	}
	now := time.Now()
	to, err := uniqueDir(bin, now.Format("20060102-150405")+"-"+filepath.Base(dir))
	if err != nil {
		return nil, err
	}
	os.Remove(to) // uniqueDir made it; the rename takes its place
	if err := os.Rename(dir, to); err != nil {
		return nil, err
	}
	item.From, _ = filepath.Rel(l.Root, dir)
	item.Trashed, item.Dir = now.Truncate(time.Second), to
	data, _ := json.MarshalIndent(item, "", "  ")
	if err := writeAtomic(filepath.Join(to, trashNote), data); err != nil {
		return nil, err
	}
	return &item, nil
}

// TrashPage moves page p, its sub-pages with it, to the trash.
func (l *Library) TrashPage(p *Page) (*TrashItem, error) {
	where := p.Coll.Name
	if crumb := parents(p); crumb != "" {
		where += " › " + crumb
	}
	item, err := l.trash(p.Dir, TrashItem{Kind: "page", Title: p.Title, Where: where})
	if err != nil {
		return nil, err
	}
	if p.Parent != nil {
		p.Parent.Children = without(p.Parent.Children, p)
	} else {
		p.Coll.Pages = without(p.Coll.Pages, p)
	}
	return item, nil
}

// TrashCollection moves collection c, its pages with it, to the trash.
func (l *Library) TrashCollection(c *Collection) (*TrashItem, error) {
	item, err := l.trash(c.Dir, TrashItem{Kind: "collection", Title: c.Name})
	if err != nil {
		return nil, err
	}
	l.Collections = slices.DeleteFunc(l.Collections, func(x *Collection) bool { return x == c })
	return item, nil
}

// Trash is what is in the trash, the last thrown away first. A folder put
// there by hand, without a note, is a page from nowhere: put back, it goes
// to the first collection.
func (l *Library) Trash() []*TrashItem {
	bin := filepath.Join(l.Root, trashDir)
	entries, _ := os.ReadDir(bin)
	var out []*TrashItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(bin, e.Name())
		item := &TrashItem{Kind: "page", Title: e.Name()}
		if data, err := os.ReadFile(filepath.Join(dir, trashNote)); err == nil {
			json.Unmarshal(data, item)
		} else {
			if info, err := e.Info(); err == nil {
				item.Trashed = info.ModTime()
			}
			if data, err := os.ReadFile(filepath.Join(dir, collectionFile)); err == nil {
				var c Collection
				json.Unmarshal(data, &c)
				item.Kind, item.Title = "collection", c.Name
			} else if src, err := os.ReadFile(filepath.Join(dir, "index.md")); err == nil {
				if meta, _ := splitFrontMatter(string(src)); meta["title"] != "" {
					item.Title = meta["title"]
				}
			}
		}
		item.Dir = dir
		out = append(out, item)
	}
	slices.SortStableFunc(out, func(a, b *TrashItem) int { return b.Trashed.Compare(a.Trashed) })
	return out
}

// Restore puts item back where it was: a collection back in the library, a
// page under the page or in the collection it was in - if that is gone,
// the top of its collection, else of the first one. It returns what came back.
func (l *Library) Restore(item *TrashItem) (*Collection, *Page, error) {
	if item.Kind == "collection" {
		name := filepath.Base(item.From)
		if name == "." || name == "" {
			name = slug(item.Title)
		}
		to, err := l.putBack(item, l.Root, name)
		if err != nil {
			return nil, nil, err
		}
		data, err := os.ReadFile(filepath.Join(to, collectionFile))
		if err != nil {
			return nil, nil, err
		}
		c := &Collection{SubPages: true, Open: true}
		if err := json.Unmarshal(data, c); err != nil {
			return nil, nil, err
		}
		c.Dir = to
		c.Pages = readPages(to, c, nil)
		l.Collections = append(l.Collections, c)
		sort.SliceStable(l.Collections, func(i, j int) bool { return l.Collections[i].Name < l.Collections[j].Name })
		return c, nil, nil
	}
	// where it was: the collection or page whose folder held it
	var coll *Collection
	var parent *Page
	home := filepath.Join(l.Root, filepath.Dir(item.From))
	for _, c := range l.Collections {
		if c.Dir == home {
			coll = c
		}
		for _, p := range c.All() {
			if p.Dir == home {
				coll, parent = c, p
			}
		}
	}
	if coll == nil { // gone too: its collection's top, or the first one's
		first := strings.Split(filepath.ToSlash(item.From), "/")[0]
		for _, c := range l.Collections {
			if filepath.Base(c.Dir) == first {
				coll = c
			}
		}
		if coll == nil && len(l.Collections) > 0 {
			coll = l.Collections[0]
		}
		if coll == nil {
			return nil, nil, errors.New("no collection to put the page back in")
		}
	}
	if parent != nil && !coll.SubPages {
		parent = nil
	}
	in, siblings := coll.Dir, &coll.Pages
	if parent != nil {
		in, siblings = parent.Dir, &parent.Children
	}
	to, err := l.putBack(item, in, filepath.Base(item.From))
	if err != nil {
		return nil, nil, err
	}
	src, err := os.ReadFile(filepath.Join(to, "index.md"))
	if err != nil {
		return nil, nil, err
	}
	meta, body := splitFrontMatter(string(src))
	p := &Page{Title: meta["title"], Dir: to, Coll: coll, Parent: parent}
	p.readMeta(meta)
	if p.Title == "" {
		p.Title = item.Title
	}
	if t, err := time.Parse(time.RFC3339, meta["created"]); err == nil {
		p.Created = t
	}
	p.Children = readPages(to, coll, p)
	p.Order = lastOrder(*siblings) + 1 // at the end of where it goes
	if err := p.Save(body); err != nil {
		return nil, nil, err
	}
	*siblings = append(*siblings, p)
	if parent != nil {
		parent.Open = true
	}
	coll.Open = true
	return coll, p, nil
}

// putBack moves item's folder into folder in, named name (or a free name
// like it), without its note.
func (l *Library) putBack(item *TrashItem, in, name string) (string, error) {
	if name == "" || name == "." {
		name = "geri"
	}
	to, err := uniqueDir(in, name)
	if err != nil {
		return "", err
	}
	os.Remove(to)
	if err := os.Rename(item.Dir, to); err != nil {
		return "", err
	}
	os.Remove(filepath.Join(to, trashNote))
	return to, nil
}

// DeleteForever deletes item from the trash, from the disk.
func (l *Library) DeleteForever(item *TrashItem) error {
	bin := filepath.Join(l.Root, trashDir)
	if filepath.Dir(item.Dir) != bin { // only what is in the trash
		return errors.New("not in the trash")
	}
	return os.RemoveAll(item.Dir)
}

func without(ps []*Page, p *Page) []*Page {
	var out []*Page
	for _, q := range ps {
		if q != p {
			out = append(out, q)
		}
	}
	return out
}

// sortSiblings puts pages in their order: those placed by hand (Order) by
// it, the others after them by when they were made.
func sortSiblings(ps []*Page) {
	sort.SliceStable(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		switch {
		case a.Order > 0 && b.Order > 0:
			return a.Order < b.Order
		case a.Order > 0 || b.Order > 0:
			return a.Order > 0
		}
		return a.Created.Before(b.Created)
	})
}

func lastOrder(ps []*Page) int {
	n := 0
	for _, p := range ps {
		n = max(n, p.Order)
	}
	return max(n, len(ps))
}

// Contains reports whether q is p or a page under it.
func (p *Page) Contains(q *Page) bool {
	for ; q != nil; q = q.Parent {
		if q == p {
			return true
		}
	}
	return false
}

// MovePage puts p in collection c, under parent (nil: at the top), at index
// among its new siblings: its folder - sub-pages and all - moves when its
// place in the tree changes, and the order of the siblings it left and joined
// is written. A page cannot go under itself, nor under a page of a
// collection without sub-pages.
func (l *Library) MovePage(p *Page, c *Collection, parent *Page, index int) error {
	if parent != nil && p.Contains(parent) {
		return errors.New("a page cannot go under itself")
	}
	if parent != nil && !c.SubPages {
		return errors.New("this collection has no sub-pages")
	}
	from := &p.Coll.Pages
	if p.Parent != nil {
		from = &p.Parent.Children
	}
	to := &c.Pages
	container := c.Dir
	if parent != nil {
		to, container = &parent.Children, parent.Dir
	}
	at := slices.Index(*from, p)
	*from = slices.Delete(*from, at, at+1)
	if from == to && at < index {
		index--
	}
	index = max(0, min(index, len(*to)))
	if filepath.Dir(p.Dir) != container {
		dir, err := uniqueDir(container, filepath.Base(p.Dir))
		if err == nil {
			os.Remove(dir) // made only to find a free name; the rename takes its place
			err = os.Rename(p.Dir, dir)
		}
		if err != nil {
			*from = slices.Insert(*from, at, p) // as it was
			return err
		}
		p.relocate(p.Dir, dir)
	}
	p.Parent = parent
	p.setColl(c)
	*to = slices.Insert(*to, index, p)
	if err := renumber(*to); err != nil {
		return err
	}
	if from != to {
		return renumber(*from)
	}
	return nil
}

// relocate rewrites the folders of p and the pages under it after their
// folder moved from old to now.
func (p *Page) relocate(old, now string) {
	p.Dir = now + strings.TrimPrefix(p.Dir, old)
	for _, ch := range p.Children {
		ch.relocate(old, now)
	}
}

func (p *Page) setColl(c *Collection) {
	p.Coll = c
	for _, ch := range p.Children {
		ch.setColl(c)
	}
}

// renumber writes the order of siblings, as they stand, into the pages whose order changed.
func renumber(ps []*Page) error {
	for i, p := range ps {
		if p.Order == i+1 {
			continue
		}
		p.Order = i + 1
		body, err := p.Body()
		if err != nil {
			return err
		}
		if err := p.Save(body); err != nil {
			return err
		}
	}
	return nil
}
