package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	Root        string
	Collections []*Collection
}

type Collection struct {
	Name     string  `json:"name"`
	Icon     string  `json:"icon,omitempty"`
	SubPages bool    `json:"subPages"`
	Dir      string  `json:"-"`
	Pages    []*Page `json:"-"` // top-level pages, oldest first
	Open     bool    `json:"-"` // shown expanded in the sidebar
}

type Page struct {
	Title    string
	Created  time.Time
	Dir      string
	Coll     *Collection
	Parent   *Page
	Children []*Page
	Open     bool // shown expanded in the sidebar
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
		c.Dir = dir
		c.Pages = readPages(dir, c, nil)
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
		if t, err := time.Parse(time.RFC3339, meta["created"]); err == nil {
			p.Created = t
		} else if info, err := e.Info(); err == nil {
			p.Created = info.ModTime()
		}
		p.Children = readPages(pdir, c, p)
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
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
	return writeAtomic(filepath.Join(c.Dir, collectionFile), append(data, '\n'))
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
	p := &Page{Title: title, Created: time.Now().Truncate(time.Second), Dir: dir, Coll: c, Parent: parent}
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

// Save writes the page: its front matter and body.
func (p *Page) Save(body string) error {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("title: " + quoteYAML(p.Title) + "\n")
	sb.WriteString("created: " + p.Created.Format(time.RFC3339) + "\n")
	sb.WriteString("---\n\n")
	sb.WriteString(strings.TrimLeft(body, "\n"))
	if !strings.HasSuffix(body, "\n") {
		sb.WriteString("\n")
	}
	return writeAtomic(p.file(), []byte(sb.String()))
}

// SetTitle renames the page; its folder keeps its name.
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

// Front matter: a few "key: value" lines between "---" lines.
func splitFrontMatter(src string) (map[string]string, string) {
	meta := map[string]string{}
	if !strings.HasPrefix(src, "---\n") {
		return meta, src
	}
	end := strings.Index(src[4:], "\n---")
	if end < 0 {
		return meta, src
	}
	for _, line := range strings.Split(src[4:4+end], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			var s string
			if json.Unmarshal([]byte(v), &s) == nil {
				v = s
			}
		}
		meta[strings.TrimSpace(k)] = v
	}
	body := src[4+end+4:]
	return meta, strings.TrimLeft(body, "\n")
}

// quoteYAML quotes a value when YAML would read it as something else.
func quoteYAML(s string) string {
	if s == "" || strings.ContainsAny(s, ":#\"'\n[]{},&*!|>%@`") || strings.TrimSpace(s) != s {
		b, _ := json.Marshal(s)
		return string(b)
	}
	return s
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
