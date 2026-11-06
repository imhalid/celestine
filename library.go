package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
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
	fs          FS
	Root        string
	Collections []*Collection
}

type Collection struct {
	Name  string `json:"name"`
	Icon  string `json:"icon,omitempty"`  // one of collIcons; "" the stack
	Color string `json:"color,omitempty"` // one of Ryugo's c1…c8; "" none
	// Description says what the collection is for, under its name.
	Description string `json:"description,omitempty"`
	// PageNoun is what a page of it is called ("Tarif"): "Yeni Tarif".
	PageNoun string `json:"pageNoun,omitempty"`
	// Template is the Markdown a new page of it starts with.
	Template string `json:"template,omitempty"`
	// Templates are more of them, each with a name (templates.go).
	Templates []NamedTemplate `json:"templates,omitempty"`
	// Query makes the collection a saved search (dynamic.go): it has no pages of
	// its own, and shows the pages of the library that the search finds.
	Query    string `json:"query,omitempty"`
	SubPages bool   `json:"subPages"`
	// Journal marks the collection of the days' pages (journal.go): every
	// library has one, and it is not thrown away.
	Journal bool `json:"journal,omitempty"`
	// JournalFormat is how the journal's days are titled, as YYYY-MM-DD or
	// D MMMM YYYY, dddd (journal.go); set in the journal's settings only.
	JournalFormat string `json:"journalFormat,omitempty"`
	creating      bool   // the journal is making today's page
	fs            FS     `json:"-"`
	// Pinned collections come first in the sidebar; Order places a
	// collection among the ones pinned or not, as the user moved it (0: by name).
	Pinned bool `json:"pinned,omitempty"`
	Order  int  `json:"order,omitempty"`
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
	Type  string `json:"type"` // "list" | "table" | "board" | "calendar"
	Sort  string `json:"sort"` // "title" | "created" | "modified" | "prop:<id>"
	Desc  bool   `json:"desc,omitempty"`
	// GroupBy is the choice property a board's columns are the options of.
	GroupBy string `json:"groupBy,omitempty"`
	// Columns are a table's columns, in order ("parent", "created",
	// "modified", "prop:<id>"; the title is always first); all if none.
	Columns []string `json:"columns,omitempty"`
	// Fields are the properties a list's rows and a board's cards show.
	Fields []string `json:"fields,omitempty"`
	// NoExcerpt hides the start of the text in a list's rows and a board's cards.
	NoExcerpt bool `json:"noExcerpt,omitempty"`
	// Collapsed are a board's columns folded to a strip, by option ("" none).
	Collapsed []string `json:"collapsed,omitempty"`
	// DateField is what places a page on a calendar's day: "created" (the
	// default), "modified", or "prop:<id>" of a date property.
	DateField string `json:"dateField,omitempty"`
}

// defaultViews are the views a collection starts with.
func defaultViews() []View {
	return []View{
		{ID: "list", Label: "Liste", Type: "list", Sort: "modified", Desc: true},
		{ID: "table", Label: "Tablo", Type: "table", Sort: "title"},
		{ID: "calendar", Label: "Takvim", Type: "calendar", Sort: "created", DateField: "created"},
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
	writes   int               // how many times it was written here: two writes in one tick of the file's clock are still two
	Order    int               // its place among its siblings, from 1; 0: not yet placed (by creation)
	Props    map[string]string // its properties' values, by id, and other front matter kept as it was
	Rank     int               // its place in its column on the board, from 1; 0: not yet placed
	Cover    string            // the picture across its top: a file of its folder (cover.go)
	Dir      string
	Coll     *Collection
	Parent   *Page
	Children []*Page
	Open     bool // shown expanded in the sidebar

	excerpt   string // the start of its text, for a list
	excerptOf time.Time
	excerptAt int
}

const collectionFile = "collection.json"

// OpenLibrary reads the library at root, making the folder when there is none.
func OpenLibrary(root string) (*Library, error) { return OpenLibraryIn(osFS{}, root) }

// OpenLibraryIn reads the library at root of fsys, making it when there is none.
func OpenLibraryIn(fsys FS, root string) (*Library, error) {
	if err := fsys.MkdirAll(root); err != nil {
		return nil, err
	}
	l := &Library{Root: root, fs: fsys}
	entries, err := fsys.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		data, err := fsys.ReadFile(filepath.Join(dir, collectionFile))
		if err != nil {
			continue // a folder that is not a collection
		}
		c := &Collection{SubPages: true, Open: true, fs: fsys}
		if err := json.Unmarshal(data, c); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(dir, collectionFile), err)
		}
		if c.Name == "" {
			c.Name = e.Name()
		}
		c.Dir, c.seen = dir, modTime(fsys, filepath.Join(dir, collectionFile))
		c.Pages = readPages(dir, c, nil)
		if c.migrate() {
			if err := c.save(); err != nil {
				return nil, err
			}
		}
		l.Collections = append(l.Collections, c)
	}
	sortCollections(l.Collections)
	return l, nil
}

// readPages reads the pages in dir: each subfolder holding an index.md.
func readPages(dir string, c *Collection, parent *Page) []*Page {
	entries, _ := c.fs.ReadDir(dir)
	var out []*Page
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		pdir := filepath.Join(dir, e.Name())
		src, err := c.fs.ReadFile(filepath.Join(pdir, pageFileName))
		if err != nil {
			if src, err = migrateLegacyPage(c.fs, pdir); err != nil {
				continue
			}
		}
		meta := pageMeta(src)
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
		if info, err := c.fs.Stat(filepath.Join(pdir, pageFileName)); err == nil {
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
	dir, err := uniqueDir(l.fs, l.Root, slug(name))
	if err != nil {
		return nil, err
	}
	c := &Collection{Name: name, SubPages: true, Dir: dir, Open: true, fs: l.fs}
	if err := c.save(); err != nil {
		return nil, err
	}
	l.Collections = append(l.Collections, c)
	return c, nil
}

func (c *Collection) save() error {
	data, _ := json.MarshalIndent(c, "", "  ")
	file := filepath.Join(c.Dir, collectionFile)
	if err := c.fs.WriteFile(file, append(data, '\n')); err != nil {
		return err
	}
	c.seen = modTime(c.fs, file)
	return nil
}

// modTime is when file was last written; now if it can't be read.
func modTime(fsys FS, file string) time.Time {
	if info, err := fsys.Stat(file); err == nil {
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
	if c.Dynamic() {
		return nil, errors.New("arama koleksiyonuna sayfa eklenmez: sayfalarını sorgusu bulur")
	}
	if c.Journal && !c.creating {
		return nil, errors.New("günlüğe yalnız bugünün sayfası eklenir")
	}
	in := c.Dir
	if parent != nil {
		in = parent.Dir
	}
	dir, err := uniqueDir(c.fs, in, slug(title))
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

func (p *Page) file() string { return filepath.Join(p.Dir, pageFileName) }

// fs is where the page's collection keeps its files.
func (p *Page) fs() FS {
	if p.Coll != nil && p.Coll.fs != nil {
		return p.Coll.fs
	}
	return osFS{}
}

// Body reads the page's body, as the file keeps it (body.go).
func (p *Page) Body() (string, error) {
	src, err := p.fs().ReadFile(p.file())
	if err != nil {
		return "", err
	}
	_, body := splitPage(src)
	return body, nil
}

// Text reads the page's text for what looks into it, as plain lines (see
// bodyText): its tags, tasks and links, what to search for.
func (p *Page) Text() (string, error) {
	body, err := p.Body()
	if err != nil {
		return "", err
	}
	text := bodyText(body)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text, nil
}

// SaveText writes the page with a body made of Markdown: what comes in from
// outside (an import, an agent's text), as the body it makes.
func (p *Page) SaveText(md string) error { return p.Save(encodeBody(importMarkdown(md))) }

// Save writes the page: its body, and what is known of it.
func (p *Page) Save(body string) error {
	meta := map[string]string{}
	for k, v := range p.Props {
		if v != "" && !slices.Contains(reserved, k) {
			meta[k] = v
		}
	}
	meta["title"] = p.Title
	if !p.Created.IsZero() {
		meta["created"] = p.Created.Format(time.RFC3339)
	}
	if p.Order != 0 {
		meta["order"] = strconv.Itoa(p.Order)
	}
	if p.Rank != 0 {
		meta["rank"] = strconv.Itoa(p.Rank)
	}
	if p.Cover != "" {
		meta["cover"] = p.Cover
	}
	if err := p.fs().WriteFile(p.file(), joinPage(meta, body)); err != nil {
		return err
	}
	p.Modified = modTime(p.fs(), p.file())
	p.writes++
	return nil
}

// Excerpt is the start of the page's text, plain, on one line: what a list
// shows under its title. It is read again only when the page changed.
func (p *Page) Excerpt() string {
	if p.excerptOf.Equal(p.Modified) && p.excerptAt == p.writes && !p.Modified.IsZero() {
		return p.excerpt
	}
	body, err := p.Body()
	if err != nil {
		return ""
	}
	text := strings.Join(strings.Fields(decodeBody(body).PlainText()), " ")
	if r := []rune(text); len(r) > 180 {
		text = strings.TrimSpace(string(r[:180])) + "…"
	}
	p.excerpt, p.excerptOf, p.excerptAt = text, p.Modified, p.writes
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
func uniqueDir(fsys FS, parent, name string) (string, error) {
	for i := 1; ; i++ {
		n := name
		if i > 1 {
			n = fmt.Sprintf("%s-%d", name, i)
		}
		dir := filepath.Join(parent, n)
		err := fsys.Mkdir(dir)
		if err == nil {
			return dir, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
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
	if err := l.fs.MkdirAll(bin); err != nil {
		return nil, err
	}
	now := time.Now()
	to, err := uniqueDir(l.fs, bin, now.Format("20060102-150405")+"-"+filepath.Base(dir))
	if err != nil {
		return nil, err
	}
	l.fs.Remove(to) // uniqueDir made it; the rename takes its place
	if err := l.fs.Rename(dir, to); err != nil {
		return nil, err
	}
	item.From, _ = filepath.Rel(l.Root, dir)
	item.Trashed, item.Dir = now.Truncate(time.Second), to
	data, _ := json.MarshalIndent(item, "", "  ")
	if err := l.fs.WriteFile(filepath.Join(to, trashNote), data); err != nil {
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
	if c.Journal {
		return nil, errors.New("günlük çöpe taşınamaz")
	}
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
	entries, _ := l.fs.ReadDir(bin)
	var out []*TrashItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(bin, e.Name())
		item := &TrashItem{Kind: "page", Title: e.Name()}
		if data, err := l.fs.ReadFile(filepath.Join(dir, trashNote)); err == nil {
			json.Unmarshal(data, item)
		} else {
			if info, err := e.Info(); err == nil {
				item.Trashed = info.ModTime()
			}
			if data, err := l.fs.ReadFile(filepath.Join(dir, collectionFile)); err == nil {
				var c Collection
				json.Unmarshal(data, &c)
				item.Kind, item.Title = "collection", c.Name
			} else if src, err := l.fs.ReadFile(filepath.Join(dir, pageFileName)); err == nil {
				if meta := pageMeta(src); meta["title"] != "" {
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
		data, err := l.fs.ReadFile(filepath.Join(to, collectionFile))
		if err != nil {
			return nil, nil, err
		}
		c := &Collection{SubPages: true, Open: true, fs: l.fs}
		if err := json.Unmarshal(data, c); err != nil {
			return nil, nil, err
		}
		c.Dir = to
		c.Pages = readPages(to, c, nil)
		l.Collections = append(l.Collections, c)
		sortCollections(l.Collections)
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
	src, err := l.fs.ReadFile(filepath.Join(to, pageFileName))
	if err != nil {
		return nil, nil, err
	}
	meta, body := splitPage(src)
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
	to, err := uniqueDir(l.fs, in, name)
	if err != nil {
		return "", err
	}
	l.fs.Remove(to)
	if err := l.fs.Rename(item.Dir, to); err != nil {
		return "", err
	}
	l.fs.Remove(filepath.Join(to, trashNote))
	return to, nil
}

// DeleteForever deletes item from the trash, from the disk.
func (l *Library) DeleteForever(item *TrashItem) error {
	bin := filepath.Join(l.Root, trashDir)
	if filepath.Dir(item.Dir) != bin { // only what is in the trash
		return errors.New("not in the trash")
	}
	return l.fs.RemoveAll(item.Dir)
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
		dir, err := uniqueDir(l.fs, container, filepath.Base(p.Dir))
		if err == nil {
			l.fs.Remove(dir) // made only to find a free name; the rename takes its place
			err = l.fs.Rename(p.Dir, dir)
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

// sortCollections puts collections in the sidebar's order: the pinned first,
// those placed by hand (Order) before the others, then by name.
func sortCollections(cs []*Collection) {
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		switch {
		case a.Pinned != b.Pinned:
			return a.Pinned
		case a.Order > 0 && b.Order > 0 && a.Order != b.Order:
			return a.Order < b.Order
		case (a.Order > 0) != (b.Order > 0):
			return a.Order > 0
		}
		return a.Name < b.Name
	})
}

// PinCollection pins c, which then comes first in the sidebar, or lets it go.
func (l *Library) PinCollection(c *Collection, pin bool) error {
	if c.Pinned == pin {
		return nil
	}
	c.Pinned, c.Order = pin, 0
	sortCollections(l.Collections)
	return c.save()
}

// MoveCollection moves c up (-1) or down (1) among the collections pinned
// as it is, or among the others, and keeps the places.
func (l *Library) MoveCollection(c *Collection, delta int) error {
	sortCollections(l.Collections)
	var group []*Collection
	for _, x := range l.Collections {
		if x.Pinned == c.Pinned {
			group = append(group, x)
		}
	}
	i := slices.Index(group, c)
	j := i + delta
	if i < 0 || j < 0 || j >= len(group) {
		return nil
	}
	group[i], group[j] = group[j], group[i]
	for k, x := range group {
		if x.Order != k+1 {
			x.Order = k + 1
			if err := x.save(); err != nil {
				return err
			}
		}
	}
	sortCollections(l.Collections)
	return nil
}

// MoveCollectionBefore puts c before the collection before - of those pinned
// as c is, or the others - or last of them when it is nil, and keeps the places.
func (l *Library) MoveCollectionBefore(c, before *Collection) error {
	sortCollections(l.Collections)
	var group []*Collection
	for _, x := range l.Collections {
		if x.Pinned == c.Pinned && x != c {
			group = append(group, x)
		}
	}
	at := len(group)
	if before != nil {
		if i := slices.Index(group, before); i >= 0 {
			at = i
		}
	}
	group = slices.Insert(group, at, c)
	for k, x := range group {
		if x.Order != k+1 {
			x.Order = k + 1
			if err := x.save(); err != nil {
				return err
			}
		}
	}
	sortCollections(l.Collections)
	return nil
}
