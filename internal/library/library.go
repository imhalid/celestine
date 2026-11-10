package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"
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
	FS          FS
	Root        string
	Collections []*Collection
	// Damaged are the collections whose collection.json could not be read, as "folder: what was wrong": left out
	// and left as they are, for the app to say so, instead of the library not opening at all (#134).
	Damaged []string
}

type Collection struct {
	Name  string `json:"name"`
	Icon  string `json:"icon,omitempty"`  // one of collIcons; "" the stack
	Color string `json:"color,omitempty"` // one of Ryugo's c1…c8; "" none
	// Description says what the collection is for, under its name.
	Description string `json:"description,omitempty"`
	// Banner is the picture across the top of its page, a file in its folder ("" none).
	Banner string `json:"banner,omitempty"`
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
	Creating      bool   `json:"-"` // the journal is making today's page
	FS            FS     `json:"-"`
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
	Seen       time.Time `json:"-"` // when its collection.json was last read or written here (watch.go)
}

// View is a way of showing a collection's pages: a list grouped by date or a
// table, sorted by a field.
type View struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"` // "list" | "table" | "board" | "calendar" | "gallery"
	Sort  string `json:"sort"` // "title" | "created" | "modified" | "prop:<id>" | "rank" (a board's) | "manual"
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
	// CardSize is how large a board's cards are: "small", "" (the middle) or "large".
	CardSize string `json:"cardSize,omitempty"`
	// DateField is what places a page on a calendar's day: "created" (the
	// default), "modified", or "prop:<id>" of a date property.
	DateField string `json:"dateField,omitempty"`
	// Widths, Order and Hidden are how a table's columns were left: the widths dragged, the order they were
	// moved to, and those hidden from its menu, by column.
	Widths map[string]float32 `json:"widths,omitempty"`
	Order  []string           `json:"order,omitempty"`
	Hidden []string           `json:"hidden,omitempty"`
	// Filters are the conditions a page meets to be shown (Match).
	Filters []Filter `json:"filters,omitempty"`
	// Calcs are what a table's foot says under its columns, by column: "sum", "avg", "min", "max" or "count".
	Calcs map[string]string `json:"calcs,omitempty"`
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
	return c.Save()
}

// SetSort sorts view v by field, the other way round when it already is.
func (c *Collection) SetSort(v *View, field string) error {
	if v.Sort == field {
		v.Desc = !v.Desc
	} else {
		v.Sort, v.Desc = field, field != "title"
	}
	return c.Save()
}

// Rename names the collection; its folder keeps its name.
func (c *Collection) Rename(name string) error {
	c.Name = name
	return c.Save()
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
	Writes   int               // how many times it was written here: two writes in one tick of the file's clock are still two
	Order    int               // its place among its siblings, from 1; 0: not yet placed (by creation)
	Props    map[string]string // its properties' values, by id, and other front matter kept as it was
	Rank     int               // its place in its column on the board, from 1; 0: not yet placed
	Manual   int               // its place in a view in its own order (sort "manual"), from 1; 0: not yet placed
	Cover    string            // the picture across its top: a file of its folder (cover.go)
	Dir      string
	Coll     *Collection
	Parent   *Page
	Children []*Page
	Open     bool // shown expanded in the sidebar

	SxID  int // its place in the search index, when sxOf is the index
	SxOf  any // the index it is in (searchindex.go): the library does not know it
	SxGen int

	// Unreadable says why the page's text could not be read when it was opened, and while it is set nothing is
	// written over its file (Save refuses): what is shown is not the page, and saving it would replace the file
	// with what was shown (#125). A restore of an earlier copy clears it.
	Unreadable string

	excerpt   string // the start of its text, for a list
	excerptOf time.Time
	excerptAt int
	firstImg  string // the first picture its text shows, for a gallery's card
	imgOf     time.Time
	imgAt     int
}

const CollectionFile = "collection.json"

// OpenLibrary reads the library at root, making the folder when there is none.
func OpenLibrary(root string) (*Library, error) { return OpenLibraryIn(OSFS{}, root) }

// OpenLibraryIn reads the library at root of fsys, making it when there is none. A library written by a newer
// app is not read, nor written: the error is a *NewerFormatError (format.go).
func OpenLibraryIn(fsys FS, root string) (*Library, error) {
	if err := newerFormat(fsys, root); err != nil {
		return nil, err
	}
	if err := fsys.MkdirAll(root); err != nil {
		return nil, err
	}
	if err := markFormat(fsys, root); err != nil {
		return nil, err
	}
	l := &Library{Root: root, FS: fsys}
	entries, err := fsys.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		data, err := fsys.ReadFile(filepath.Join(dir, CollectionFile))
		if err != nil {
			continue // a folder that is not a collection
		}
		c := &Collection{SubPages: true, Open: true, FS: fsys}
		if err := json.Unmarshal(data, c); err != nil {
			l.Damaged = append(l.Damaged, e.Name()+": "+err.Error())
			continue
		}
		if c.Name == "" {
			c.Name = e.Name()
		}
		c.Dir, c.Seen = dir, ModTime(fsys, filepath.Join(dir, CollectionFile))
		c.Pages = ReadPages(dir, c, nil)
		if c.migrate() {
			if err := c.Save(); err != nil {
				return nil, err
			}
		}
		l.Collections = append(l.Collections, c)
	}
	SortCollections(l.Collections)
	return l, nil
}

// readPages reads the pages in dir: each subfolder holding an index.md.
// PagesGen counts the changes to the pages of the libraries: one made, renamed, moved, thrown away or
// brought back, or read from disk. What is kept of them (the titles of chips.go) is made again when it moves.
var PagesGen atomic.Uint64

func PagesChanged() { PagesGen.Add(1) }

func ReadPages(dir string, c *Collection, parent *Page) []*Page {
	defer PagesChanged()
	entries, _ := c.FS.ReadDir(dir)
	var out []*Page
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		pdir := filepath.Join(dir, e.Name())
		src, err := c.FS.ReadFile(filepath.Join(pdir, PageFileName))
		if err != nil {
			if src, err = migrateLegacyPage(c.FS, pdir); err != nil {
				continue
			}
		}
		meta := PageMeta(src)
		p := &Page{Title: meta["title"], Dir: pdir, Coll: c, Parent: parent}
		if p.Title == "" {
			p.Title = e.Name()
		}
		fmt.Sscan(meta["order"], &p.Order)
		p.ReadMeta(meta)
		if t, err := time.Parse(time.RFC3339, meta["created"]); err == nil {
			p.Created = t
		} else if info, err := e.Info(); err == nil {
			p.Created = info.ModTime()
		}
		if info, err := c.FS.Stat(filepath.Join(pdir, PageFileName)); err == nil {
			p.Modified = info.ModTime()
		}
		p.Children = ReadPages(pdir, c, p)
		out = append(out, p)
	}
	sortSiblings(out)
	return out
}

// NewCollection makes a collection named name.
func (l *Library) NewCollection(name string) (*Collection, error) {
	defer PagesChanged()
	dir, err := UniqueDir(l.FS, l.Root, Slug(name))
	if err != nil {
		return nil, err
	}
	c := &Collection{Name: name, SubPages: true, Dir: dir, Open: true, FS: l.FS}
	if err := c.Save(); err != nil {
		return nil, err
	}
	l.Collections = append(l.Collections, c)
	return c, nil
}

func (c *Collection) Save() error {
	data, _ := json.MarshalIndent(c, "", "  ")
	file := filepath.Join(c.Dir, CollectionFile)
	if err := c.FS.WriteFile(file, append(data, '\n')); err != nil {
		return err
	}
	c.Seen = ModTime(c.FS, file)
	return nil
}

// ModTime is when file was last written; now if it can't be read.
func ModTime(fsys FS, file string) time.Time {
	if info, err := fsys.Stat(file); err == nil {
		return info.ModTime()
	}
	return time.Now()
}

// NewPage makes a page in c, under parent when it is not nil (a sub-page),
// with an empty body.
func (c *Collection) NewPage(parent *Page, title string) (*Page, error) {
	defer PagesChanged()
	if parent != nil && !c.SubPages {
		return nil, errors.New("this collection has no sub-pages")
	}
	if c.Dynamic() {
		return nil, errors.New("arama koleksiyonuna sayfa eklenmez: sayfalarını sorgusu bulur")
	}
	if c.Journal && !c.Creating {
		return nil, errors.New("günlüğe yalnız bugünün sayfası eklenir")
	}
	in := c.Dir
	if parent != nil {
		in = parent.Dir
	}
	dir, err := UniqueDir(c.FS, in, Slug(title))
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

func (p *Page) File() string { return filepath.Join(p.Dir, PageFileName) }

// FS is where the page's collection keeps its files.
func (p *Page) FS() FS {
	if p.Coll != nil && p.Coll.FS != nil {
		return p.Coll.FS
	}
	return OSFS{}
}

// Body reads the page's body, as the file keeps it (body.go).
func (p *Page) Body() (string, error) {
	src, err := p.FS().ReadFile(p.File())
	if err != nil {
		return "", err
	}
	_, body, err := SplitPageChecked(src)
	return body, err
}

// Text reads the page's text for what looks into it, as plain lines (see
// bodyText): its tags, tasks and links, what to search for.
func (p *Page) Text() (string, error) {
	body, err := p.Body()
	if err != nil {
		return "", err
	}
	return TextOfBody(body), nil
}

// TextOfBody is Text of a body read.
func TextOfBody(body string) string {
	text := Format.Markdown(body)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text
}

// SaveText writes the page with a body made of Markdown: what comes in from
// outside (an import, an agent's text), as the body it makes.
func (p *Page) SaveText(md string) error { return p.Save(Format.FromMarkdown(md)) }

// Save writes the page: its body, and what is known of it.
func (p *Page) Save(body string) error {
	if p.Unreadable != "" {
		return fmt.Errorf("%w: %s", ErrUnreadable, p.Unreadable)
	}
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
	if p.Manual != 0 {
		meta["manualrank"] = strconv.Itoa(p.Manual)
	}
	if p.Cover != "" {
		meta["cover"] = p.Cover
	}
	src, err := JoinPageChecked(meta, body)
	if err != nil {
		return err
	}
	if err := p.FS().WriteFile(p.File(), src); err != nil {
		return err
	}
	p.Modified = ModTime(p.FS(), p.File())
	p.Writes++
	return nil
}

// Excerpt is the start of the page's text, plain, on one line: what a list
// shows under its title. It is read again only when the page changed.
func (p *Page) Excerpt() string {
	if p.excerptOf.Equal(p.Modified) && p.excerptAt == p.Writes && !p.Modified.IsZero() {
		return p.excerpt
	}
	body, err := p.Body()
	if err != nil {
		return ""
	}
	text := strings.Join(strings.Fields(Format.Plain(body)), " ")
	if r := []rune(text); len(r) > 180 {
		text = strings.TrimSpace(string(r[:180])) + "…"
	}
	p.excerpt, p.excerptOf, p.excerptAt = text, p.Modified, p.Writes
	return text
}

// FirstImage is where the first picture the page's text shows is, as its Markdown names it (relative to the
// page's folder, or a link out), or "": what a gallery's card shows when the page has no cover. It is read
// again only when the page changed.
func (p *Page) FirstImage() string {
	if p.imgOf.Equal(p.Modified) && p.imgAt == p.Writes && !p.Modified.IsZero() {
		return p.firstImg
	}
	body, err := p.Body()
	if err != nil {
		return ""
	}
	ref := ""
	if m := imageRef.FindStringSubmatch(Format.Markdown(body)); m != nil {
		ref = m[1] + m[2] // one of the two: <a path with spaces>, or a plain one
	}
	p.firstImg, p.imgOf, p.imgAt = ref, p.Modified, p.Writes
	return ref
}

// imageRef is a picture in Markdown, ![alt](ref), and not an embedded page, ![[Page]].
var imageRef = regexp.MustCompile(`!\[[^\[\]]*\]\((?:<([^>]+)>|([^)\s]+))(?:\s+"[^"]*")?\)`)

// ErrUnreadable is what Save says of a page whose text could not be read (Page.Unreadable).
var ErrUnreadable = errors.New("the page could not be read, so it is not written over")

// SetTitle) renames the page; its folder keeps its name.
func (p *Page) SetTitle(title string) error {
	defer PagesChanged()
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

var TurkishFold = strings.NewReplacer("ç", "c", "ğ", "g", "ı", "i", "ö", "o", "ş", "s", "ü", "u", "Ç", "c", "Ğ", "g", "İ", "i", "Ö", "o", "Ş", "s", "Ü", "u")

// Slug makes a folder name from a title: lower case, ASCII where the letter
// has an obvious one, words joined by hyphens.
func Slug(title string) string {
	s := TurkishFold.Replace(strings.ToLower(title))
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
	if len(out) > 60 { // at a letter's start: a cut in the middle of one is not a name a folder may have (#143)
		cut := 60
		for cut > 0 && !utf8.RuneStart(out[cut]) {
			cut--
		}
		out = strings.TrimRight(out[:cut], "-")
	}
	return out
}

// UniqueDir makes a new folder in parent named name, or name-2, name-3…
func UniqueDir(fsys FS, parent, name string) (string, error) {
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

// TrashDir is where pages and collections go when they are thrown away: a
// folder of the library, so nothing is lost and they can be put back by hand.
const TrashDir = ".cop"

// TrashNote is the file a thrown-away folder keeps in it: what it was and
// where it was, to put it back there.
const TrashNote = ".trashed.json"

// TrashItem is a page or a collection in the trash.
type TrashItem struct {
	Kind    string    `json:"kind"` // "page" | "collection" | "files" (files of the library's top, a sync took away)
	Title   string    `json:"title"`
	From    string    `json:"from"`            // its folder, from the library's root
	Where   string    `json:"where,omitempty"` // where it was, to show: its collection and the pages above it
	Trashed time.Time `json:"trashed"`
	Dir     string    `json:"-"` // its folder in the trash
}

// MoveToTrash moves dir into the library's MoveToTrash, under a name that keeps when it
// went and what it was, with a note of where it was.
func (l *Library) MoveToTrash(dir string, item TrashItem) (*TrashItem, error) {
	bin := filepath.Join(l.Root, TrashDir)
	if err := l.FS.MkdirAll(bin); err != nil {
		return nil, err
	}
	now := time.Now()
	to, err := UniqueDir(l.FS, bin, now.Format("20060102-150405")+"-"+filepath.Base(dir))
	if err != nil {
		return nil, err
	}
	l.FS.Remove(to) // uniqueDir made it; the rename takes its place
	if err := l.FS.Rename(dir, to); err != nil {
		return nil, err
	}
	item.From, _ = filepath.Rel(l.Root, dir)
	item.Trashed, item.Dir = now.Truncate(time.Second), to
	data, _ := json.MarshalIndent(item, "", "  ")
	if err := l.FS.WriteFile(filepath.Join(to, TrashNote), data); err != nil {
		return nil, err
	}
	return &item, nil
}

// TrashPage moves page p, its sub-pages with it, to the trash.
func (l *Library) TrashPage(p *Page) (*TrashItem, error) {
	defer PagesChanged()
	where := p.Coll.Name
	if crumb := Parents(p); crumb != "" {
		where += " › " + crumb
	}
	item, err := l.MoveToTrash(p.Dir, TrashItem{Kind: "page", Title: p.Title, Where: where})
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
	defer PagesChanged()
	if c.Journal {
		return nil, errors.New("günlük çöpe taşınamaz")
	}
	item, err := l.MoveToTrash(c.Dir, TrashItem{Kind: "collection", Title: c.Name})
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
	bin := filepath.Join(l.Root, TrashDir)
	entries, _ := l.FS.ReadDir(bin)
	var out []*TrashItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(bin, e.Name())
		item := &TrashItem{Kind: "page", Title: e.Name()}
		if data, err := l.FS.ReadFile(filepath.Join(dir, TrashNote)); err == nil {
			json.Unmarshal(data, item)
		} else {
			if info, err := e.Info(); err == nil {
				item.Trashed = info.ModTime()
			}
			if data, err := l.FS.ReadFile(filepath.Join(dir, CollectionFile)); err == nil {
				var c Collection
				json.Unmarshal(data, &c)
				item.Kind, item.Title = "collection", c.Name
			} else if src, err := l.FS.ReadFile(filepath.Join(dir, PageFileName)); err == nil {
				if meta := PageMeta(src); meta["title"] != "" {
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
	defer PagesChanged()
	if item.From != "" && !SafeName(filepath.ToSlash(item.From)) { // the note is a file in the trash: its path is not trusted to stay in the library
		return nil, nil, errors.New("çöp notundaki yol güvenli değil: " + item.From)
	}
	if item.Kind == "files" { // each back at the library's top; one that is there again is not written over
		ents, err := l.FS.ReadDir(item.Dir)
		if err != nil {
			return nil, nil, err
		}
		for _, e := range ents {
			if e.IsDir() || e.Name() == TrashNote {
				continue
			}
			to := filepath.Join(l.Root, e.Name())
			if _, err := l.FS.Stat(to); err == nil {
				return nil, nil, errors.New(e.Name() + ": a file of that name is there already")
			}
			if err := l.FS.Rename(filepath.Join(item.Dir, e.Name()), to); err != nil {
				return nil, nil, err
			}
		}
		return nil, nil, l.FS.RemoveAll(item.Dir)
	}
	if item.Kind == "collection" {
		name := filepath.Base(item.From)
		if name == "." || name == "" {
			name = Slug(item.Title)
		}
		to, err := l.putBack(item, l.Root, name)
		if err != nil {
			return nil, nil, err
		}
		data, err := l.FS.ReadFile(filepath.Join(to, CollectionFile))
		if err != nil {
			return nil, nil, err
		}
		c := &Collection{SubPages: true, Open: true, FS: l.FS}
		if err := json.Unmarshal(data, c); err != nil {
			return nil, nil, err
		}
		c.Dir = to
		c.Pages = ReadPages(to, c, nil)
		l.Collections = append(l.Collections, c)
		SortCollections(l.Collections)
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
	// read before it moves: an item with no page file stays in the trash, not half in the library (#142)
	src, err := l.FS.ReadFile(filepath.Join(item.Dir, PageFileName))
	if err != nil {
		return nil, nil, errors.New(item.Title + ": no page file to bring back")
	}
	meta, body, damaged := SplitPageChecked(src)
	to, err := l.putBack(item, in, filepath.Base(item.From))
	if err != nil {
		return nil, nil, err
	}
	p := &Page{Title: meta["title"], Dir: to, Coll: coll, Parent: parent}
	p.ReadMeta(meta)
	if p.Title == "" {
		p.Title = item.Title
	}
	if t, err := time.Parse(time.RFC3339, meta["created"]); err == nil {
		p.Created = t
	}
	p.Children = ReadPages(to, coll, p)
	p.Order = lastOrder(*siblings) + 1 // at the end of where it goes
	if damaged == nil {                // a damaged file is brought back as it is, not written over with an empty body
		if err := p.Save(body); err != nil {
			return nil, nil, err
		}
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
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) { // a name from a note in the trash: not a path out of the folder
		name = "geri"
	}
	to, err := UniqueDir(l.FS, in, name)
	if err != nil {
		return "", err
	}
	l.FS.Remove(to)
	if err := l.FS.Rename(item.Dir, to); err != nil {
		return "", err
	}
	l.FS.Remove(filepath.Join(to, TrashNote))
	return to, nil
}

// PurgeTrash deletes for good what has been in the trash more than trashDays, and says how many items.
func (l *Library) PurgeTrash(now time.Time) int {
	n := 0
	for _, item := range l.Trash() {
		if !item.Trashed.IsZero() && now.Sub(item.Trashed) > TrashDays*24*time.Hour && l.DeleteForever(item) == nil {
			n++
		}
	}
	return n
}

// DeleteForever deletes item from the trash, from the disk.
func (l *Library) DeleteForever(item *TrashItem) error {
	bin := filepath.Join(l.Root, TrashDir)
	if filepath.Dir(item.Dir) != bin { // only what is in the trash
		return errors.New("not in the trash")
	}
	return l.FS.RemoveAll(item.Dir)
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
	defer PagesChanged()
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
		dir, err := UniqueDir(l.FS, container, filepath.Base(p.Dir))
		if err == nil {
			l.FS.Remove(dir) // made only to find a free name; the rename takes its place
			err = l.FS.Rename(p.Dir, dir)
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
	if err := Renumber(*to); err != nil {
		return err
	}
	if from != to {
		return Renumber(*from)
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

// Renumber writes the order of siblings, as they stand, into the pages whose order changed.
// It is one change of the store: all of them or none (#138).
func Renumber(ps []*Page) error {
	if len(ps) == 0 {
		return nil
	}
	was := make([]int, len(ps))
	for i, p := range ps {
		was[i] = p.Order
	}
	err := InBatch(ps[0].FS(), func() error {
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
	})
	if err != nil { // none of it written: the pages say what the files say
		for i, p := range ps {
			p.Order = was[i]
		}
	}
	return err
}

// SortCollections puts collections in the sidebar's order: the pinned first,
// those placed by hand (Order) before the others, then by name.
func SortCollections(cs []*Collection) {
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
	SortCollections(l.Collections)
	return c.Save()
}

// MoveCollection moves c up (-1) or down (1) among the collections pinned
// as it is, or among the others, and keeps the places.
func (l *Library) MoveCollection(c *Collection, delta int) error {
	SortCollections(l.Collections)
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
			if err := x.Save(); err != nil {
				return err
			}
		}
	}
	SortCollections(l.Collections)
	return nil
}

// MoveCollectionBefore puts c before the collection before - of those pinned
// as c is, or the others - or last of them when it is nil, and keeps the places.
func (l *Library) MoveCollectionBefore(c, before *Collection) error {
	SortCollections(l.Collections)
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
			if err := x.Save(); err != nil {
				return err
			}
		}
	}
	SortCollections(l.Collections)
	return nil
}

// Dynamic reports whether the collection is a search: its pages are what its query finds.
func (c *Collection) Dynamic() bool { return strings.TrimSpace(c.Query) != "" }

// Parents is the titles of the pages above p, for a sub-page in a flat view.
func Parents(p *Page) string {
	var parts []string
	for q := p.Parent; q != nil; q = q.Parent {
		parts = append([]string{q.Title}, parts...)
	}
	return strings.Join(parts, " › ")
}

// NamedTemplate is a template of a collection.
type NamedTemplate struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

// configFields are the fields of a collection's collection.json, without
// the folder and pages.
func (c *Collection) ConfigJSON() []byte {
	data, _ := json.MarshalIndent(c, "", "  ")
	return data
}

// SetConfig gives c the configuration in data, every field of it.
func (c *Collection) SetConfig(data []byte) error {
	var fresh Collection
	if err := json.Unmarshal(data, &fresh); err != nil {
		return err
	}
	c.Name, c.Icon, c.Color, c.Description, c.PageNoun, c.Template, c.Templates = fresh.Name, fresh.Icon, fresh.Color, fresh.Description, fresh.PageNoun, fresh.Template, fresh.Templates
	c.SubPages, c.Views, c.Properties, c.ActiveView = fresh.SubPages, fresh.Views, fresh.Properties, fresh.ActiveView
	c.Query = fresh.Query
	c.JournalFormat, c.Banner = fresh.JournalFormat, fresh.Banner // c.Pinned and c.Order are the sidebar's, not the code tab's
	// c.Journal stays as it is: the code tab does not take it away
	if c.Name == "" {
		c.Name = "Adsız koleksiyon"
	}
	c.Normalize()
	return nil
}

// templatesOf are the templates a collection offers, the first one first.
func (c *Collection) AllTemplates() []NamedTemplate {
	var out []NamedTemplate
	if strings.TrimSpace(c.Template) != "" {
		out = append(out, NamedTemplate{Name: "Varsayılan", Body: c.Template})
	}
	for _, t := range c.Templates {
		if strings.TrimSpace(t.Name) != "" {
			out = append(out, t)
		}
	}
	return out
}

// ImageCover reports whether the page has a picture of its own as its cover.
func (p *Page) ImageCover() bool { return p.Cover != "" && p.Cover != CoverNone }

// CoverNone is the cover "none" in a page's file: no cover, not the made one either (autocover.go).
const CoverNone = "none"

// Path is the page's folders from the library's root, with slashes: what a page:// link and a page property name.
func (p *Page) Path() string {
	rel, err := filepath.Rel(filepath.Dir(p.Coll.Dir), p.Dir) // a collection's folder is in the library's root
	if err != nil {
		return filepath.ToSlash(p.Dir)
	}
	return filepath.ToSlash(rel)
}

// PageAt is the page whose folders from the library's root are path; failing that (it moved), the one page whose
// folder has path's last name, when there is just one; or nil.
func (l *Library) PageAt(path string) *Page {
	path = strings.Trim(filepath.ToSlash(path), "/")
	if path == "" {
		return nil
	}
	last := path[strings.LastIndex(path, "/")+1:]
	var byName *Page
	names := 0
	for _, c := range l.Collections {
		for _, p := range c.All() {
			if p.Path() == path {
				return p
			}
			if filepath.Base(p.Dir) == last {
				byName, names = p, names+1
			}
		}
	}
	if names == 1 {
		return byName
	}
	return nil
}
